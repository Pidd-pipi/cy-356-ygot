package service

import (
	"testing"

	"gorm.io/gorm"

	"github.com/communitygarden/server/internal/constants"
	"github.com/communitygarden/server/internal/dto"
	"github.com/communitygarden/server/internal/repository"
	"github.com/communitygarden/server/internal/util"
)

func newApplicationService(t *testing.T, db *gorm.DB) *AdoptionApplicationService {
	t.Helper()
	appRepo := repository.NewAdoptionApplicationRepository(db)
	plotRepo := repository.NewPlotRepository(db)
	return NewAdoptionApplicationService(appRepo, plotRepo, db, testLogger())
}

func TestApplicationService_Submit(t *testing.T) {
	db := newTestServiceDB(t)
	svc := newApplicationService(t, db)
	user := newTestUser(t, db, "citizen", "citizen")
	plot := newTestPlot(t, db, "P-APP", "available", nil)
	adoptedUID := user.ID
	adoptedPlot := newTestPlot(t, db, "P-APP-ADOPTED", "adopted", &adoptedUID)

	// 空闲地块可提交申请
	app, err := svc.Submit(&dto.SubmitApplicationRequest{PlotID: plot.ID, Message: "想种番茄"}, user.ID, user.Username)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if app.Status != string(constants.ApplicationStatusPending) {
		t.Errorf("submit status=%s, want pending", app.Status)
	}

	// 同一居民对同一地块只能保留一份待审申请
	if _, err := svc.Submit(&dto.SubmitApplicationRequest{PlotID: plot.ID, Message: "再申请一次"}, user.ID, user.Username); err == nil {
		t.Fatalf("expected duplicate pending application error")
	}

	// 已认养地块不能再产生新申请
	if _, err := svc.Submit(&dto.SubmitApplicationRequest{PlotID: adoptedPlot.ID, Message: "已被认养"}, user.ID, user.Username); err == nil {
		t.Fatalf("expected plot not available error")
	}
}

func TestApplicationService_Withdraw(t *testing.T) {
	db := newTestServiceDB(t)
	svc := newApplicationService(t, db)
	owner := newTestUser(t, db, "citizen", "citizen")
	other := newTestUser(t, db, "citizen2", "citizen")
	plot := newTestPlot(t, db, "P-WD", "available", nil)

	app, err := svc.Submit(&dto.SubmitApplicationRequest{PlotID: plot.ID, Message: "想种生菜"}, owner.ID, owner.Username)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	// 非申请人不能撤回
	if _, err := svc.Withdraw(app.ID, other.ID, "citizen"); err == nil {
		t.Fatalf("expected forbidden error for non-owner withdraw")
	}

	// 申请人可撤回待审申请
	got, err := svc.Withdraw(app.ID, owner.ID, "citizen")
	if err != nil {
		t.Fatalf("Withdraw: %v", err)
	}
	if got.Status != string(constants.ApplicationStatusWithdrawn) {
		t.Errorf("withdraw status=%s, want withdrawn", got.Status)
	}

	// 已撤回申请不能重复撤回
	if _, err := svc.Withdraw(app.ID, owner.ID, "citizen"); err == nil {
		t.Fatalf("expected not-pending error for repeated withdraw")
	}

	// 撤回后可重新申请同一地块
	if _, err := svc.Submit(&dto.SubmitApplicationRequest{PlotID: plot.ID, Message: "重新申请"}, owner.ID, owner.Username); err != nil {
		t.Fatalf("re-submit after withdraw: %v", err)
	}
}

func TestApplicationService_ApproveAutoRejectsOthers(t *testing.T) {
	db := newTestServiceDB(t)
	svc := newApplicationService(t, db)
	admin := newTestUser(t, db, "admin", "admin")
	userA := newTestUser(t, db, "citizenA", "citizen")
	userB := newTestUser(t, db, "citizenB", "citizen")
	userC := newTestUser(t, db, "citizenC", "citizen")
	plot := newTestPlot(t, db, "P-APR", "available", nil)

	appA, err := svc.Submit(&dto.SubmitApplicationRequest{PlotID: plot.ID, Message: "A 申请"}, userA.ID, userA.Username)
	if err != nil {
		t.Fatalf("submit A: %v", err)
	}
	appB, err := svc.Submit(&dto.SubmitApplicationRequest{PlotID: plot.ID, Message: "B 申请"}, userB.ID, userB.Username)
	if err != nil {
		t.Fatalf("submit B: %v", err)
	}
	appC, err := svc.Submit(&dto.SubmitApplicationRequest{PlotID: plot.ID, Message: "C 申请"}, userC.ID, userC.Username)
	if err != nil {
		t.Fatalf("submit C: %v", err)
	}

	// 管理员批准 A：地块归 A，B/C 自动拒绝
	got, err := svc.Approve(appA.ID, admin.ID, admin.Username, "同意")
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if got.Status != string(constants.ApplicationStatusApproved) || got.ReviewerID == nil || *got.ReviewerID != admin.ID {
		t.Errorf("approve result invalid: status=%s reviewer=%v", got.Status, got.ReviewerID)
	}

	plotRepo := repository.NewPlotRepository(db)
	updated, err := plotRepo.FindByID(plot.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if updated.Status != string(constants.PlotStatusAdopted) || updated.AdopterID == nil || *updated.AdopterID != userA.ID {
		t.Errorf("plot not adopted by A: status=%s adopter=%v", updated.Status, updated.AdopterID)
	}

	appRepo := repository.NewAdoptionApplicationRepository(db)
	for _, id := range []uint{appB.ID, appC.ID} {
		other, err := appRepo.FindByID(id)
		if err != nil {
			t.Fatalf("FindByID %d: %v", id, err)
		}
		if other.Status != string(constants.ApplicationStatusRejected) {
			t.Errorf("application %d status=%s, want auto rejected", id, other.Status)
		}
	}

	// 已批准的申请不能重复审核
	if _, err := svc.Approve(appA.ID, admin.ID, admin.Username, ""); err == nil {
		t.Fatalf("expected not-pending error for re-approve")
	}
	if _, err := svc.Reject(appB.ID, admin.ID, admin.Username, ""); err == nil {
		t.Fatalf("expected not-pending error for reject auto-rejected")
	}

	// 已批准过的地块不能再产生新申请
	if _, err := svc.Submit(&dto.SubmitApplicationRequest{PlotID: plot.ID, Message: "再来"}, userC.ID, userC.Username); err == nil {
		t.Fatalf("expected plot not available error after approval")
	}
}

func TestApplicationService_RejectAndReapplyElsewhere(t *testing.T) {
	db := newTestServiceDB(t)
	svc := newApplicationService(t, db)
	admin := newTestUser(t, db, "admin", "admin")
	user := newTestUser(t, db, "citizen", "citizen")
	plot1 := newTestPlot(t, db, "P-REJ-1", "available", nil)
	plot2 := newTestPlot(t, db, "P-REJ-2", "available", nil)

	app, err := svc.Submit(&dto.SubmitApplicationRequest{PlotID: plot1.ID, Message: "申请地块1"}, user.ID, user.Username)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	got, err := svc.Reject(app.ID, admin.ID, admin.Username, "本周名额已满")
	if err != nil {
		t.Fatalf("Reject: %v", err)
	}
	if got.Status != string(constants.ApplicationStatusRejected) || got.ReviewNote != "本周名额已满" {
		t.Errorf("reject result invalid: status=%s note=%s", got.Status, got.ReviewNote)
	}

	// 被拒绝的居民可以选择其他空闲地块再次申请
	if _, err := svc.Submit(&dto.SubmitApplicationRequest{PlotID: plot2.ID, Message: "改申请地块2"}, user.ID, user.Username); err != nil {
		t.Fatalf("re-apply another plot: %v", err)
	}
	// 被拒绝后也可重新申请原地块（仍空闲）
	if _, err := svc.Submit(&dto.SubmitApplicationRequest{PlotID: plot1.ID, Message: "再次申请地块1"}, user.ID, user.Username); err != nil {
		t.Fatalf("re-apply same plot after reject: %v", err)
	}
}

func TestApplicationService_ListScope(t *testing.T) {
	db := newTestServiceDB(t)
	svc := newApplicationService(t, db)
	userA := newTestUser(t, db, "citizenA", "citizen")
	userB := newTestUser(t, db, "citizenB", "citizen")
	plot := newTestPlot(t, db, "P-LST", "available", nil)

	if _, err := svc.Submit(&dto.SubmitApplicationRequest{PlotID: plot.ID, Message: "A 申请"}, userA.ID, userA.Username); err != nil {
		t.Fatalf("submit A: %v", err)
	}
	if _, err := svc.Submit(&dto.SubmitApplicationRequest{PlotID: plot.ID, Message: "B 申请"}, userB.ID, userB.Username); err != nil {
		t.Fatalf("submit B: %v", err)
	}

	// 居民只能看到自己的申请
	mine, total, err := svc.List(util.PageQuery{Page: 1, PageSize: 10}, userA.ID, "citizen", 0, "")
	if err != nil || total != 1 || len(mine) != 1 || mine[0].UserID != userA.ID {
		t.Errorf("citizen list: total=%d len=%d err=%v", total, len(mine), err)
	}
	// 管理员可看到全部申请并按状态过滤
	all, total, err := svc.List(util.PageQuery{Page: 1, PageSize: 10}, 0, "admin", 0, "pending")
	if err != nil || total != 2 || len(all) != 2 {
		t.Errorf("admin list: total=%d len=%d err=%v", total, len(all), err)
	}
	// 非法状态过滤报错
	if _, _, err := svc.List(util.PageQuery{Page: 1, PageSize: 10}, 0, "admin", 0, "bogus"); err == nil {
		t.Fatalf("expected validation error for bogus status")
	}
}
