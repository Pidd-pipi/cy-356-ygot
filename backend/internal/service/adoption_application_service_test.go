package service

import (
	"testing"

	"gorm.io/gorm"

	"github.com/communitygarden/server/internal/constants"
	"github.com/communitygarden/server/internal/dto"
	"github.com/communitygarden/server/internal/repository"
	"github.com/communitygarden/server/internal/util"
)

func newAdoptionService(t *testing.T, db *gorm.DB) *AdoptionApplicationService {
	t.Helper()
	appRepo := repository.NewAdoptionApplicationRepository(db)
	plotRepo := repository.NewPlotRepository(db)
	return NewAdoptionApplicationService(appRepo, plotRepo, db, testLogger())
}

func applyReq(plotID uint, message string) *dto.CreateAdoptionApplicationRequest {
	return &dto.CreateAdoptionApplicationRequest{PlotID: plotID, Message: message}
}

func TestAdoptionService_Apply(t *testing.T) {
	db := newTestServiceDB(t)
	svc := newAdoptionService(t, db)
	user := newTestUser(t, db, "citizen", "citizen")
	plot := newTestPlot(t, db, "P-APPLY", "available", nil)

	// 空闲地块可提交申请
	app, err := svc.Apply(applyReq(plot.ID, "想种番茄，周末有空打理"), user.ID)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if app.Status != string(constants.AdoptionStatusPending) {
		t.Errorf("apply status=%s, want pending", app.Status)
	}

	// 同一居民对同一地块只能保留一份待审申请
	if _, err := svc.Apply(applyReq(plot.ID, "重复申请"), user.ID); err == nil {
		t.Fatalf("expected duplicate pending error")
	}

	// 已批准/已认养的地块不能再产生新申请
	owner := newTestUser(t, db, "farmer", "farmer")
	uid := owner.ID
	adopted := newTestPlot(t, db, "P-ADOPTED", "adopted", &uid)
	if _, err := svc.Apply(applyReq(adopted.ID, "已被认养"), user.ID); err == nil {
		t.Fatalf("expected plot not available error for adopted plot")
	}

	// 不存在的地块
	if _, err := svc.Apply(applyReq(99999, "不存在"), user.ID); err == nil {
		t.Fatalf("expected not found error")
	}
}

func TestAdoptionService_Withdraw(t *testing.T) {
	db := newTestServiceDB(t)
	svc := newAdoptionService(t, db)
	user := newTestUser(t, db, "citizen", "citizen")
	other := newTestUser(t, db, "citizen2", "citizen")
	plot := newTestPlot(t, db, "P-WD", "available", nil)

	app, err := svc.Apply(applyReq(plot.ID, "先占个坑"), user.ID)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// 他人不能撤回
	if _, err := svc.Withdraw(app.ID, other.ID, "citizen"); err == nil {
		t.Fatalf("expected forbidden error for non-owner withdraw")
	}

	// 本人可撤回待审申请
	got, err := svc.Withdraw(app.ID, user.ID, "citizen")
	if err != nil {
		t.Fatalf("Withdraw: %v", err)
	}
	if got.Status != string(constants.AdoptionStatusWithdrawn) {
		t.Errorf("withdraw status=%s, want withdrawn", got.Status)
	}

	// 已撤回的申请不能重复撤回
	if _, err := svc.Withdraw(app.ID, user.ID, "citizen"); err == nil {
		t.Fatalf("expected not-pending error for re-withdraw")
	}

	// 撤回后可重新申请同一地块
	again, err := svc.Apply(applyReq(plot.ID, "撤回后重新申请"), user.ID)
	if err != nil {
		t.Fatalf("re-Apply after withdraw: %v", err)
	}
	if again.Status != string(constants.AdoptionStatusPending) {
		t.Errorf("re-apply status=%s, want pending", again.Status)
	}
}

func TestAdoptionService_ReviewApprove(t *testing.T) {
	db := newTestServiceDB(t)
	svc := newAdoptionService(t, db)
	admin := newTestUser(t, db, "admin", "admin")
	userA := newTestUser(t, db, "citizenA", "citizen")
	userB := newTestUser(t, db, "citizenB", "citizen")
	userC := newTestUser(t, db, "citizenC", "citizen")
	plot := newTestPlot(t, db, "P-REVIEW", "available", nil)

	appA, err := svc.Apply(applyReq(plot.ID, "A 的留言"), userA.ID)
	if err != nil {
		t.Fatalf("Apply A: %v", err)
	}
	appB, err := svc.Apply(applyReq(plot.ID, "B 的留言"), userB.ID)
	if err != nil {
		t.Fatalf("Apply B: %v", err)
	}
	appC, err := svc.Apply(applyReq(plot.ID, "C 的留言"), userC.ID)
	if err != nil {
		t.Fatalf("Apply C: %v", err)
	}

	// 管理员批准 A：地块归 A，B/C 的待审申请自动拒绝
	got, err := svc.Review(appA.ID, admin.ID, "admin", &dto.ReviewAdoptionApplicationRequest{Action: "approve", ReviewNote: "优先考虑老农友"})
	if err != nil {
		t.Fatalf("Review approve: %v", err)
	}
	if got.Status != string(constants.AdoptionStatusApproved) {
		t.Errorf("approved status=%s", got.Status)
	}
	if got.ReviewedBy == nil || *got.ReviewedBy != admin.ID {
		t.Errorf("reviewed_by=%v, want admin id", got.ReviewedBy)
	}

	plotRepo := repository.NewPlotRepository(db)
	p, err := plotRepo.FindByID(plot.ID)
	if err != nil {
		t.Fatalf("FindByID plot: %v", err)
	}
	if p.Status != string(constants.PlotStatusAdopted) || p.AdopterID == nil || *p.AdopterID != userA.ID {
		t.Errorf("plot after approve: status=%s adopter=%v", p.Status, p.AdopterID)
	}

	for id, name := range map[uint]string{appB.ID: "B", appC.ID: "C"} {
		other, err := svc.GetByID(id)
		if err != nil {
			t.Fatalf("GetByID %s: %v", name, err)
		}
		if other.Status != string(constants.AdoptionStatusRejected) {
			t.Errorf("application %s status=%s, want auto rejected", name, other.Status)
		}
	}

	// 已批准过的地块不能再产生新申请
	if _, err := svc.Apply(applyReq(plot.ID, "地块已批准"), userC.ID); err == nil {
		t.Fatalf("expected plot not available error after approval")
	}

	// 已审核的申请不能重复审核
	if _, err := svc.Review(appA.ID, admin.ID, "admin", &dto.ReviewAdoptionApplicationRequest{Action: "reject"}); err == nil {
		t.Fatalf("expected not-pending error for re-review")
	}
}

func TestAdoptionService_ReviewReject(t *testing.T) {
	db := newTestServiceDB(t)
	svc := newAdoptionService(t, db)
	admin := newTestUser(t, db, "admin", "admin")
	user := newTestUser(t, db, "citizen", "citizen")
	plot := newTestPlot(t, db, "P-REJ", "available", nil)
	otherPlot := newTestPlot(t, db, "P-REJ-2", "available", nil)

	app, err := svc.Apply(applyReq(plot.ID, "想认养这块"), user.ID)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got, err := svc.Review(app.ID, admin.ID, "admin", &dto.ReviewAdoptionApplicationRequest{Action: "reject", ReviewNote: "留言过于简单"})
	if err != nil {
		t.Fatalf("Review reject: %v", err)
	}
	if got.Status != string(constants.AdoptionStatusRejected) {
		t.Errorf("rejected status=%s", got.Status)
	}

	// 拒绝后地块仍然空闲，被拒绝的居民可以选择其他空闲地块
	plotRepo := repository.NewPlotRepository(db)
	p, err := plotRepo.FindByID(plot.ID)
	if err != nil {
		t.Fatalf("FindByID plot: %v", err)
	}
	if p.Status != string(constants.PlotStatusAvailable) {
		t.Errorf("plot after reject: status=%s, want available", p.Status)
	}
	if _, err := svc.Apply(applyReq(otherPlot.ID, "改申请另一块空闲地"), user.ID); err != nil {
		t.Fatalf("rejected user should apply other plot: %v", err)
	}
}

func TestAdoptionService_Lists(t *testing.T) {
	db := newTestServiceDB(t)
	svc := newAdoptionService(t, db)
	user := newTestUser(t, db, "citizen", "citizen")
	plot := newTestPlot(t, db, "P-LIST-AD", "available", nil)
	if _, err := svc.Apply(applyReq(plot.ID, "留言一"), user.ID); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	mine, total, err := svc.ListMine(util.PageQuery{Page: 1, PageSize: 10}, user.ID, "")
	if err != nil || total != 1 || len(mine) != 1 {
		t.Errorf("ListMine total=%d len=%d err=%v", total, len(mine), err)
	}
	all, total, err := svc.List(util.PageQuery{Page: 1, PageSize: 10}, plot.ID, string(constants.AdoptionStatusPending))
	if err != nil || total != 1 || len(all) != 1 {
		t.Errorf("List total=%d len=%d err=%v", total, len(all), err)
	}
	if all[0].User == nil || all[0].Plot == nil {
		t.Errorf("admin list should preload applicant and plot")
	}
}
