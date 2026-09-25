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

// AdoptionStatusTransitions 认养申请状态机（服务层 + 前端按钮显隐 + 日志模板 + formatters 多处定义）。
var AdoptionStatusTransitions = map[constants.AdoptionStatus][]constants.AdoptionStatus{
	constants.AdoptionStatusPending:   {constants.AdoptionStatusApproved, constants.AdoptionStatusRejected, constants.AdoptionStatusWithdrawn},
	constants.AdoptionStatusApproved:  {},
	constants.AdoptionStatusRejected:  {},
	constants.AdoptionStatusWithdrawn: {},
}

// AdoptionApplicationService 认养申请服务（审核流程全部在事务 + 行锁内完成）。
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

// Apply 居民提交认养申请（事务：锁定地块、校验空闲、同一居民同一地块仅一份待审）。
func (s *AdoptionApplicationService) Apply(req *dto.CreateAdoptionApplicationRequest, userID uint) (*model.AdoptionApplication, error) {
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
			return util.NewAppError(constants.CodePlotNotAvailable, 409, fmt.Sprintf("地块 %s 当前状态为 %s，不可提交认养申请", plot.Code, util.PlotStatusText(plot.Status)))
		}
		if _, err := s.appRepo.FindPendingByPlotAndUser(req.PlotID, userID); err == nil {
			return util.NewAppError(constants.CodeAdoptionDuplicatePending, 409, fmt.Sprintf("居民 user_id=%d 对地块 %s 已存在待审认养申请，同一居民对同一地块只能保留一份待审申请", userID, plot.Code))
		} else if !errors.Is(err, repository.ErrNotFound) {
			return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
		}
		app := &model.AdoptionApplication{
			PlotID:  req.PlotID,
			UserID:  userID,
			Message: req.Message,
			Status:  string(constants.AdoptionStatusPending),
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
	s.logger.Info(constants.LogAdoptionApplied, "application_id", created.ID, "plot_id", created.PlotID, "user_id", userID)
	return s.reload(created.ID)
}

// Withdraw 撤回待审申请（仅申请人本人，pending -> withdrawn）。
func (s *AdoptionApplicationService) Withdraw(id, userID uint, role string) (*model.AdoptionApplication, error) {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		app, err := s.appRepo.FindByIDForUpdate(tx, id)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return util.NewAppError(constants.CodeNotFound, 404, fmt.Sprintf("认养申请实体 id=%d 不存在", id))
			}
			return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
		}
		if app.UserID != userID {
			return util.NewAppError(constants.CodeForbidden, 403, fmt.Sprintf("角色 %s 无权撤回他人认养申请 id=%d", util.RoleText(role), id))
		}
		if !canTransitionAdoption(app.Status, constants.AdoptionStatusWithdrawn) {
			return util.NewAppError(constants.CodeAdoptionNotPending, 409, fmt.Sprintf("认养申请 id=%d 当前状态为 %s，仅待审申请可撤回", id, util.AdoptionStatusText(app.Status)))
		}
		app.Status = string(constants.AdoptionStatusWithdrawn)
		if err := s.appRepo.UpdateWithTx(tx, app); err != nil {
			return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logger.Info(constants.LogAdoptionWithdrawn, "application_id", id, "user_id", userID)
	return s.reload(id)
}

// Review 管理员审核认养申请（事务：锁定申请与地块；批准后地块归申请人，同地块其余待审申请自动拒绝）。
func (s *AdoptionApplicationService) Review(id, reviewerID uint, reviewerName string, req *dto.ReviewAdoptionApplicationRequest) (*model.AdoptionApplication, error) {
	approve := req.Action == "approve"
	var autoRejected int64
	var approvedPlot *model.Plot
	var applicantID uint
	var appPlotID uint
	err := s.db.Transaction(func(tx *gorm.DB) error {
		app, err := s.appRepo.FindByIDForUpdate(tx, id)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return util.NewAppError(constants.CodeNotFound, 404, fmt.Sprintf("认养申请实体 id=%d 不存在", id))
			}
			return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
		}
		target := constants.AdoptionStatusRejected
		if approve {
			target = constants.AdoptionStatusApproved
		}
		if !canTransitionAdoption(app.Status, target) {
			return util.NewAppError(constants.CodeAdoptionNotPending, 409, fmt.Sprintf("认养申请 id=%d 当前状态为 %s，仅待审申请可审核", id, util.AdoptionStatusText(app.Status)))
		}
		now := time.Now()
		app.ReviewedBy = &reviewerID
		app.ReviewedAt = &now
		app.ReviewNote = req.ReviewNote
		applicantID = app.UserID
		appPlotID = app.PlotID
		if approve {
			plot, err := s.plotRepo.FindByIDForUpdate(tx, app.PlotID)
			if err != nil {
				if errors.Is(err, repository.ErrNotFound) {
					return util.NewAppError(constants.CodeNotFound, 404, fmt.Sprintf("地块实体 id=%d 不存在", app.PlotID))
				}
				return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
			}
			if plot.Status != string(constants.PlotStatusAvailable) {
				return util.NewAppError(constants.CodePlotNotAvailable, 409, fmt.Sprintf("地块 %s 当前状态为 %s，无法批准认养申请", plot.Code, util.PlotStatusText(plot.Status)))
			}
			app.Status = string(constants.AdoptionStatusApproved)
			plot.Status = string(constants.PlotStatusAdopted)
			plot.AdopterID = &app.UserID
			if err := s.plotRepo.UpdateWithTx(tx, plot); err != nil {
				return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
			}
			n, err := s.appRepo.RejectOtherPendingWithTx(tx, app.PlotID, app.ID, reviewerID, "同地块其他认养申请已获批准，系统自动拒绝", now)
			if err != nil {
				return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
			}
			autoRejected = n
			approvedPlot = plot
		} else {
			app.Status = string(constants.AdoptionStatusRejected)
		}
		if err := s.appRepo.UpdateWithTx(tx, app); err != nil {
			return util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if approve {
		s.logger.Info(constants.LogAdoptionApproved, "application_id", id, "plot_id", approvedPlot.ID, "user_id", applicantID, "reviewer", reviewerName, "auto_rejected", autoRejected)
		s.logger.Info(constants.LogPlotAdopted, "plot_id", approvedPlot.ID, "code", approvedPlot.Code, "user_id", applicantID, "role", string(constants.RoleAdmin))
	} else {
		s.logger.Info(constants.LogAdoptionRejected, "application_id", id, "plot_id", appPlotID, "reviewer", reviewerName)
	}
	return s.reload(id)
}

// ListMine 我的认养申请列表（审核结果对居民可见）。
func (s *AdoptionApplicationService) ListMine(pq util.PageQuery, userID uint, status string) ([]model.AdoptionApplication, int64, error) {
	apps, total, err := s.appRepo.List(pq, userID, 0, status)
	if err != nil {
		return nil, 0, util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
	}
	return apps, total, nil
}

// List 管理员查看全部认养申请（可按地块/状态过滤，复用同一仓储 List 方法）。
func (s *AdoptionApplicationService) List(pq util.PageQuery, plotID uint, status string) ([]model.AdoptionApplication, int64, error) {
	apps, total, err := s.appRepo.List(pq, 0, plotID, status)
	if err != nil {
		return nil, 0, util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
	}
	return apps, total, nil
}

// GetByID 查询认养申请详情。
func (s *AdoptionApplicationService) GetByID(id uint) (*model.AdoptionApplication, error) {
	app, err := s.appRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(constants.CodeNotFound, 404, fmt.Sprintf("认养申请实体 id=%d 不存在", id))
		}
		return nil, util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
	}
	return app, nil
}

// reload 事务提交后重新加载（含 Plot/User/Reviewer 预加载）用于响应。
func (s *AdoptionApplicationService) reload(id uint) (*model.AdoptionApplication, error) {
	app, err := s.appRepo.FindByID(id)
	if err != nil {
		return nil, util.NewAppError(constants.CodeInternalError, 500, constants.ErrorText[constants.CodeInternalError]).Wrap(err)
	}
	return app, nil
}

// canTransitionAdoption 依据状态机校验认养申请状态流转。
func canTransitionAdoption(from string, to constants.AdoptionStatus) bool {
	for _, s := range AdoptionStatusTransitions[constants.AdoptionStatus(from)] {
		if s == to {
			return true
		}
	}
	return false
}
