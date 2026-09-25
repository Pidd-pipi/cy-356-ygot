package service

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/communitygarden/server/internal/constants"
	"github.com/communitygarden/server/internal/dto"
	"github.com/communitygarden/server/internal/model"
	"github.com/communitygarden/server/internal/repository"
	"github.com/communitygarden/server/internal/util"
)

// ApplicationStatusTransitions 认养申请状态机（服务层 + 前端按钮显隐 + 日志模板 + formatters 多处定义）。
var ApplicationStatusTransitions = map[constants.ApplicationStatus][]constants.ApplicationStatus{
	constants.ApplicationStatusPending:   {constants.ApplicationStatusApproved, constants.ApplicationStatusRejected, constants.ApplicationStatusWithdrawn},
	constants.ApplicationStatusApproved:  {},
	constants.ApplicationStatusRejected:  {},
	constants.ApplicationStatusWithdrawn: {},
}

// AdoptionApplicationService 认养申请服务（提交/撤回/审核均使用事务 + SELECT FOR UPDATE）。
type AdoptionApplicationService struct {
	appRepo  repository.AdoptionApplicationRepository
	plotRepo repository.PlotRepository
	db       *gorm.DB
	logger   *slog.Logger
}

// NewAdoptionApplicationService 构造认养申请服务。
func NewAdoptionApplicationService(appRepo repository.AdoptionApplicationRepository, plotRepo repository.PlotRepository, db *gorm.DB, logger *slog.Logger) *AdoptionApplicationService {
	return &AdoptionApplicationService{appRepo: appRepo, plotRepo: plotRepo, db: db, logger: logger}
}

// Submit 居民提交认养申请（事务：锁定地块 -> 校验空闲 -> 校验同人同地块仅一份待审 -> 创建）。
func (s *AdoptionApplicationService) Submit(req *dto.SubmitApplicationRequest, userID uint, username string) (*model.AdoptionApplication, error) {
	var created *model.AdoptionApplication
	err := s.db.Transaction(func(tx *gorm.DB) error {
		plot, err := s.plotRepo.FindByIDForUpdate(tx, req.PlotID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return util.NewAppError(constants.CodeNotFound, 404, fmt.Sprintf("地块实体 id=%d 不存在", req.PlotID))
			}
			return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
		}
		if plot.Status != string(constants.PlotStatusAvailable) {
			return util.NewAppError(constants.CodePlotNotAvailable, 409, fmt.Sprintf("地块 %s 当前状态为 %s，已批准或已认养的地块不能再提交认养申请", plot.Code, util.PlotStatusText(plot.Status)))
		}
		pending, err := s.appRepo.CountPendingByPlotAndUser(tx, req.PlotID, userID)
		if err != nil {
			return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
		}
		if pending > 0 {
			return util.NewAppError(constants.CodeDuplicateApplication, 409, fmt.Sprintf("用户 %s 对地块 %s 已存在待审核的认养申请，同一居民对同一地块只能保留一份待审申请", username, plot.Code))
		}
		app := &model.AdoptionApplication{
			PlotID:  req.PlotID,
			UserID:  userID,
			Message: req.Message,
			Status:  string(constants.ApplicationStatusPending),
		}
		if err := s.appRepo.CreateWithTx(tx, app); err != nil {
			return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
		}
		created = app
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logger.Info(constants.LogApplicationSubmitted, "application_id", created.ID, "plot_id", created.PlotID, "user_id", userID)
	return created, nil
}

// Withdraw 撤回认养申请（申请人本人，仅待审核状态可撤回）。
func (s *AdoptionApplicationService) Withdraw(id, userID uint, role string) (*model.AdoptionApplication, error) {
	var withdrawn *model.AdoptionApplication
	err := s.db.Transaction(func(tx *gorm.DB) error {
		app, err := s.appRepo.FindByIDForUpdate(tx, id)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return util.NewAppError(constants.CodeNotFound, 404, fmt.Sprintf("认养申请实体 id=%d 不存在", id))
			}
			return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
		}
		if app.UserID != userID && role != string(constants.RoleAdmin) {
			return util.NewAppError(constants.CodeForbidden, 403, fmt.Sprintf("角色 %s 无权撤回他人认养申请 id=%d", util.RoleText(role), id))
		}
		if app.Status != string(constants.ApplicationStatusPending) {
			return util.NewAppError(constants.CodeApplicationNotPending, 409, fmt.Sprintf("认养申请 id=%d 当前状态为 %s，仅待审核状态可撤回", id, util.ApplicationStatusText(app.Status)))
		}
		app.Status = string(constants.ApplicationStatusWithdrawn)
		if err := s.appRepo.UpdateWithTx(tx, app); err != nil {
			return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
		}
		withdrawn = app
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logger.Info(constants.LogApplicationWithdrawn, "application_id", withdrawn.ID, "plot_id", withdrawn.PlotID, "user_id", withdrawn.UserID)
	return withdrawn, nil
}

// Approve 批准认养申请（管理员；事务：锁定申请与地块 -> 地块归申请人 -> 同地块其余待审申请自动拒绝）。
func (s *AdoptionApplicationService) Approve(id, reviewerID uint, reviewerName, note string) (*model.AdoptionApplication, error) {
	var approved *model.AdoptionApplication
	var autoRejected []model.AdoptionApplication
	var plotCode string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		app, err := s.appRepo.FindByIDForUpdate(tx, id)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return util.NewAppError(constants.CodeNotFound, 404, fmt.Sprintf("认养申请实体 id=%d 不存在", id))
			}
			return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
		}
		if app.Status != string(constants.ApplicationStatusPending) {
			return util.NewAppError(constants.CodeApplicationNotPending, 409, fmt.Sprintf("认养申请 id=%d 当前状态为 %s，仅待审核状态可批准", id, util.ApplicationStatusText(app.Status)))
		}
		plot, err := s.plotRepo.FindByIDForUpdate(tx, app.PlotID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return util.NewAppError(constants.CodeNotFound, 404, fmt.Sprintf("地块实体 id=%d 不存在", app.PlotID))
			}
			return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
		}
		if plot.Status != string(constants.PlotStatusAvailable) {
			return util.NewAppError(constants.CodePlotNotAvailable, 409, fmt.Sprintf("地块 %s 当前状态为 %s，已批准过的地块不能再产生新认养", plot.Code, util.PlotStatusText(plot.Status)))
		}
		now := time.Now()
		// 地块归申请人认养
		plot.Status = string(constants.PlotStatusAdopted)
		plot.AdopterID = &app.UserID
		if err := s.plotRepo.UpdateWithTx(tx, plot); err != nil {
			return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
		}
		// 本申请批准
		app.Status = string(constants.ApplicationStatusApproved)
		app.ReviewerID = &reviewerID
		app.ReviewNote = note
		app.ReviewedAt = &now
		if err := s.appRepo.UpdateWithTx(tx, app); err != nil {
			return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
		}
		// 同地块其余待审申请自动拒绝
		others, err := s.appRepo.ListPendingByPlot(tx, app.PlotID, app.ID)
		if err != nil {
			return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
		}
		for i := range others {
			others[i].Status = string(constants.ApplicationStatusRejected)
			others[i].ReviewerID = &reviewerID
			others[i].ReviewNote = constants.MsgAutoRejectNote
			others[i].ReviewedAt = &now
			if err := s.appRepo.UpdateWithTx(tx, &others[i]); err != nil {
				return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
			}
		}
		autoRejected = others
		approved = app
		plotCode = plot.Code
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logger.Info(constants.LogApplicationApproved, "application_id", approved.ID, "plot_id", approved.PlotID, "user_id", approved.UserID, "reviewer", reviewerName)
	s.logger.Info(constants.LogPlotAdopted, "plot_id", approved.PlotID, "code", plotCode, "user_id", approved.UserID, "role", string(constants.RoleAdmin))
	for i := range autoRejected {
		s.logger.Info(constants.LogApplicationAutoRejected, "application_id", autoRejected[i].ID, "plot_id", autoRejected[i].PlotID, "reason", constants.MsgAutoRejectNote)
	}
	return approved, nil
}

// Reject 拒绝认养申请（管理员，仅待审核状态；被拒绝的居民可选择其他空闲地块再次申请）。
func (s *AdoptionApplicationService) Reject(id, reviewerID uint, reviewerName, note string) (*model.AdoptionApplication, error) {
	var rejected *model.AdoptionApplication
	err := s.db.Transaction(func(tx *gorm.DB) error {
		app, err := s.appRepo.FindByIDForUpdate(tx, id)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return util.NewAppError(constants.CodeNotFound, 404, fmt.Sprintf("认养申请实体 id=%d 不存在", id))
			}
			return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
		}
		if app.Status != string(constants.ApplicationStatusPending) {
			return util.NewAppError(constants.CodeApplicationNotPending, 409, fmt.Sprintf("认养申请 id=%d 当前状态为 %s，仅待审核状态可拒绝", id, util.ApplicationStatusText(app.Status)))
		}
		now := time.Now()
		app.Status = string(constants.ApplicationStatusRejected)
		app.ReviewerID = &reviewerID
		app.ReviewNote = note
		app.ReviewedAt = &now
		if err := s.appRepo.UpdateWithTx(tx, app); err != nil {
			return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
		}
		rejected = app
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logger.Info(constants.LogApplicationRejected, "application_id", rejected.ID, "plot_id", rejected.PlotID, "user_id", rejected.UserID, "reviewer", reviewerName)
	return rejected, nil
}

// List 分页查询认养申请（非管理员仅能看到自己的申请；管理员可按地块/状态过滤查看申请人和留言）。
func (s *AdoptionApplicationService) List(pq util.PageQuery, callerID uint, role string, plotID uint, status string) ([]model.AdoptionApplication, int64, error) {
	if status != "" {
		switch constants.ApplicationStatus(status) {
		case constants.ApplicationStatusPending, constants.ApplicationStatusApproved, constants.ApplicationStatusRejected, constants.ApplicationStatusWithdrawn:
		default:
			return nil, 0, util.NewAppError(constants.CodeValidationFailed, 400, fmt.Sprintf("status 字段 %s 非法，必须是 pending/approved/rejected/withdrawn 之一", status))
		}
	}
	userID := uint(0)
	if role != string(constants.RoleAdmin) {
		userID = callerID
	}
	list, total, err := s.appRepo.List(pq, userID, plotID, status)
	if err != nil {
		return nil, 0, util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
	}
	return list, total, nil
}
