package repository

import (
	"testing"
	"time"

	"github.com/communitygarden/server/internal/model"
	"github.com/communitygarden/server/internal/util"
)

func TestAdoptionApplicationRepository_PendingUnique(t *testing.T) {
	db := newTestDB(t)
	repo := NewAdoptionApplicationRepository(db)
	user := seedUser(t, db, "citizen", "citizen")
	plot := &model.Plot{Name: "P-AD", Code: "P-AD", Area: 10, SoilType: "loam", Sunlight: "full", Latitude: 31.0, Longitude: 121.0, Status: "available"}
	if err := db.Create(plot).Error; err != nil {
		t.Fatalf("create plot: %v", err)
	}

	a1 := &model.AdoptionApplication{PlotID: plot.ID, UserID: user.ID, Message: "第一份", Status: "pending"}
	if err := db.Create(a1).Error; err != nil {
		t.Fatalf("create first application: %v", err)
	}

	// 部分唯一索引：同一居民同一地块第二份待审申请必须失败
	a2 := &model.AdoptionApplication{PlotID: plot.ID, UserID: user.ID, Message: "第二份", Status: "pending"}
	if err := db.Create(a2).Error; err == nil {
		t.Fatalf("expected unique index violation for second pending application")
	}

	// FindPendingByPlotAndUser 能查到待审申请
	found, err := repo.FindPendingByPlotAndUser(plot.ID, user.ID)
	if err != nil || found.ID != a1.ID {
		t.Errorf("FindPendingByPlotAndUser id=%d err=%v", found.ID, err)
	}

	// 撤回后可再次提交（唯一索引仅约束 pending）
	a1.Status = "withdrawn"
	if err := db.Save(a1).Error; err != nil {
		t.Fatalf("withdraw first: %v", err)
	}
	a3 := &model.AdoptionApplication{PlotID: plot.ID, UserID: user.ID, Message: "撤回后再申请", Status: "pending"}
	if err := db.Create(a3).Error; err != nil {
		t.Fatalf("create after withdraw: %v", err)
	}
}

func TestAdoptionApplicationRepository_RejectOtherPending(t *testing.T) {
	db := newTestDB(t)
	repo := NewAdoptionApplicationRepository(db)
	admin := seedUser(t, db, "admin", "admin")
	userA := seedUser(t, db, "userA", "citizen")
	userB := seedUser(t, db, "userB", "citizen")
	plot := &model.Plot{Name: "P-RJ", Code: "P-RJ", Area: 10, SoilType: "loam", Sunlight: "full", Latitude: 31.0, Longitude: 121.0, Status: "available"}
	if err := db.Create(plot).Error; err != nil {
		t.Fatalf("create plot: %v", err)
	}
	mk := func(uid uint) *model.AdoptionApplication {
		a := &model.AdoptionApplication{PlotID: plot.ID, UserID: uid, Message: "留言", Status: "pending"}
		if err := db.Create(a).Error; err != nil {
			t.Fatalf("create application: %v", err)
		}
		return a
	}
	aA := mk(userA.ID)
	aB := mk(userB.ID)

	now := time.Now()
	n, err := repo.RejectOtherPendingWithTx(db, plot.ID, aA.ID, admin.ID, "同地块其他认养申请已获批准，系统自动拒绝", now)
	if err != nil {
		t.Fatalf("RejectOtherPendingWithTx: %v", err)
	}
	if n != 1 {
		t.Errorf("auto rejected rows=%d, want 1", n)
	}
	gotB, err := repo.FindByID(aB.ID)
	if err != nil {
		t.Fatalf("FindByID B: %v", err)
	}
	if gotB.Status != "rejected" || gotB.ReviewedBy == nil || *gotB.ReviewedBy != admin.ID {
		t.Errorf("B status=%s reviewed_by=%v", gotB.Status, gotB.ReviewedBy)
	}
	gotA, err := repo.FindByID(aA.ID)
	if err != nil {
		t.Fatalf("FindByID A: %v", err)
	}
	if gotA.Status != "pending" {
		t.Errorf("A status=%s, want still pending (excluded)", gotA.Status)
	}

	// List 过滤复用
	list, total, err := repo.List(util.PageQuery{Page: 1, PageSize: 10}, 0, plot.ID, "rejected")
	if err != nil || total != 1 || len(list) != 1 {
		t.Errorf("List rejected total=%d len=%d err=%v", total, len(list), err)
	}
}
