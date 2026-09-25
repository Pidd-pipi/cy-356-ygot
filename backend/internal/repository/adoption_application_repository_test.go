package repository

import (
	"testing"

	"gorm.io/gorm"

	"github.com/communitygarden/server/internal/constants"
	"github.com/communitygarden/server/internal/model"
	"github.com/communitygarden/server/internal/util"
)

func seedPlotForApp(t *testing.T, db *gorm.DB, code, status string) *model.Plot {
	t.Helper()
	p := &model.Plot{Name: code, Code: code, Area: 10, SoilType: "loam", Sunlight: "full", Latitude: 31.0, Longitude: 121.0, Status: status}
	if err := db.Create(p).Error; err != nil {
		t.Fatalf("seed plot: %v", err)
	}
	return p
}

func TestAdoptionApplicationRepository_PendingQueries(t *testing.T) {
	db := newTestDB(t)
	repo := NewAdoptionApplicationRepository(db)
	userA := seedUser(t, db, "citizenA", "citizen")
	userB := seedUser(t, db, "citizenB", "citizen")
	plot := seedPlotForApp(t, db, "P-APP-REPO", "available")

	create := func(userID uint, status string) *model.AdoptionApplication {
		a := &model.AdoptionApplication{PlotID: plot.ID, UserID: userID, Message: "申请留言", Status: status}
		if err := repo.CreateWithTx(db, a); err != nil {
			t.Fatalf("create application: %v", err)
		}
		return a
	}
	pendingA := create(userA.ID, string(constants.ApplicationStatusPending))
	create(userB.ID, string(constants.ApplicationStatusPending))
	create(userA.ID, string(constants.ApplicationStatusWithdrawn))

	// 同一居民同一地块待审计数（排除已撤回）
	count, err := repo.CountPendingByPlotAndUser(db, plot.ID, userA.ID)
	if err != nil || count != 1 {
		t.Errorf("CountPendingByPlotAndUser count=%d err=%v, want 1", count, err)
	}

	// 同地块其余待审申请（排除自身）
	others, err := repo.ListPendingByPlot(db, plot.ID, pendingA.ID)
	if err != nil || len(others) != 1 || others[0].UserID != userB.ID {
		t.Errorf("ListPendingByPlot len=%d err=%v, want 1 (userB)", len(others), err)
	}

	// 分页列表：按用户/状态过滤
	list, total, err := repo.List(util.PageQuery{Page: 1, PageSize: 10}, userA.ID, 0, "")
	if err != nil || total != 2 || len(list) != 2 {
		t.Errorf("List userA: total=%d len=%d err=%v, want 2", total, len(list), err)
	}
	pendings, total, err := repo.List(util.PageQuery{Page: 1, PageSize: 10}, 0, plot.ID, string(constants.ApplicationStatusPending))
	if err != nil || total != 2 || len(pendings) != 2 {
		t.Errorf("List pending: total=%d len=%d err=%v, want 2", total, len(pendings), err)
	}
}
