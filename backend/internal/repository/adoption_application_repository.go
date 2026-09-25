package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/communitygarden/server/internal/model"
	"github.com/communitygarden/server/internal/util"
)

// AdoptionApplicationRepository 认养申请仓储接口。
type AdoptionApplicationRepository interface {
	CreateWithTx(tx *gorm.DB, a *model.AdoptionApplication) error
	UpdateWithTx(tx *gorm.DB, a *model.AdoptionApplication) error
	FindByID(id uint) (*model.AdoptionApplication, error)
	FindByIDForUpdate(tx *gorm.DB, id uint) (*model.AdoptionApplication, error)
	FindPendingByPlotAndUser(plotID, userID uint) (*model.AdoptionApplication, error)
	List(pq util.PageQuery, userID, plotID uint, status string) ([]model.AdoptionApplication, int64, error)
	RejectOtherPendingWithTx(tx *gorm.DB, plotID, excludeID, reviewerID uint, note string, now time.Time) (int64, error)
	CountByStatus() (map[string]int64, error)
}

type adoptionApplicationRepository struct {
	db *gorm.DB
}

// NewAdoptionApplicationRepository 构造认养申请仓储。
func NewAdoptionApplicationRepository(db *gorm.DB) AdoptionApplicationRepository {
	return &adoptionApplicationRepository{db: db}
}

// CreateWithTx 在指定事务内创建认养申请。
func (r *adoptionApplicationRepository) CreateWithTx(tx *gorm.DB, a *model.AdoptionApplication) error {
	return tx.Create(a).Error
}

// UpdateWithTx 在指定事务内更新认养申请。
func (r *adoptionApplicationRepository) UpdateWithTx(tx *gorm.DB, a *model.AdoptionApplication) error {
	return tx.Save(a).Error
}

func (r *adoptionApplicationRepository) FindByID(id uint) (*model.AdoptionApplication, error) {
	var a model.AdoptionApplication
	if err := r.db.Preload("Plot").Preload("User").Preload("Reviewer").First(&a, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

// FindByIDForUpdate 审核/撤回使用 SELECT ... FOR UPDATE 行锁（事务内执行）。
func (r *adoptionApplicationRepository) FindByIDForUpdate(tx *gorm.DB, id uint) (*model.AdoptionApplication, error) {
	var a model.AdoptionApplication
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&a, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

// FindPendingByPlotAndUser 查询某居民对某地块的待审申请（唯一约束兜底之外的显式校验）。
func (r *adoptionApplicationRepository) FindPendingByPlotAndUser(plotID, userID uint) (*model.AdoptionApplication, error) {
	var a model.AdoptionApplication
	if err := r.db.Where("plot_id = ? AND user_id = ? AND status = ?", plotID, userID, "pending").First(&a).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

// List 分页查询认养申请（userID 查我的申请，plotID/status 供管理员过滤）。
func (r *adoptionApplicationRepository) List(pq util.PageQuery, userID, plotID uint, status string) ([]model.AdoptionApplication, int64, error) {
	var apps []model.AdoptionApplication
	var total int64
	q := r.db.Model(&model.AdoptionApplication{}).Preload("Plot").Preload("User").Preload("Reviewer")
	if userID > 0 {
		q = q.Where("user_id = ?", userID)
	}
	if plotID > 0 {
		q = q.Where("plot_id = ?", plotID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := util.Paginate(q.Order("id DESC"), pq).Find(&apps).Error; err != nil {
		return nil, 0, err
	}
	return apps, total, nil
}

// RejectOtherPendingWithTx 批准一份申请后，同事务内自动拒绝同地块其余待审申请。
func (r *adoptionApplicationRepository) RejectOtherPendingWithTx(tx *gorm.DB, plotID, excludeID, reviewerID uint, note string, now time.Time) (int64, error) {
	res := tx.Model(&model.AdoptionApplication{}).
		Where("plot_id = ? AND status = ? AND id <> ?", plotID, "pending", excludeID).
		Updates(map[string]interface{}{
			"status":      "rejected",
			"review_note": note,
			"reviewed_by": reviewerID,
			"reviewed_at": now,
			"updated_at":  now,
		})
	return res.RowsAffected, res.Error
}

func (r *adoptionApplicationRepository) CountByStatus() (map[string]int64, error) {
	type row struct {
		Status string
		Count  int64
	}
	var rows []row
	if err := r.db.Model(&model.AdoptionApplication{}).Select("status, count(*) as count").Group("status").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, v := range rows {
		out[v.Status] = v.Count
	}
	return out, nil
}
