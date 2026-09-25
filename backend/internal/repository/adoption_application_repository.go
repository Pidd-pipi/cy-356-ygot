package repository

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/communitygarden/server/internal/constants"
	"github.com/communitygarden/server/internal/model"
	"github.com/communitygarden/server/internal/util"
)

// AdoptionApplicationRepository 认养申请仓储接口。
type AdoptionApplicationRepository interface {
	CreateWithTx(tx *gorm.DB, a *model.AdoptionApplication) error
	FindByID(id uint) (*model.AdoptionApplication, error)
	FindByIDForUpdate(tx *gorm.DB, id uint) (*model.AdoptionApplication, error)
	UpdateWithTx(tx *gorm.DB, a *model.AdoptionApplication) error
	CountPendingByPlotAndUser(tx *gorm.DB, plotID, userID uint) (int64, error)
	ListPendingByPlot(tx *gorm.DB, plotID, excludeID uint) ([]model.AdoptionApplication, error)
	List(pq util.PageQuery, userID, plotID uint, status string) ([]model.AdoptionApplication, int64, error)
}

type adoptionApplicationRepository struct {
	db *gorm.DB
}

// NewAdoptionApplicationRepository 构造认养申请仓储。
func NewAdoptionApplicationRepository(db *gorm.DB) AdoptionApplicationRepository {
	return &adoptionApplicationRepository{db: db}
}

// CreateWithTx 在事务内创建认养申请（配合地块行锁保证同一居民同一地块仅一份待审申请）。
func (r *adoptionApplicationRepository) CreateWithTx(tx *gorm.DB, a *model.AdoptionApplication) error {
	return tx.Create(a).Error
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

// UpdateWithTx 在指定事务内更新认养申请。
func (r *adoptionApplicationRepository) UpdateWithTx(tx *gorm.DB, a *model.AdoptionApplication) error {
	return tx.Save(a).Error
}

// CountPendingByPlotAndUser 统计同一居民对同一地块的待审核申请数（事务内调用，配合地块行锁防并发重复）。
func (r *adoptionApplicationRepository) CountPendingByPlotAndUser(tx *gorm.DB, plotID, userID uint) (int64, error) {
	var count int64
	err := tx.Model(&model.AdoptionApplication{}).
		Where("plot_id = ? AND user_id = ? AND status = ?", plotID, userID, string(constants.ApplicationStatusPending)).
		Count(&count).Error
	return count, err
}

// ListPendingByPlot 查询同地块除 excludeID 外的全部待审核申请并加行锁（批准一份后自动拒绝其余，防止审核途中被撤回）。
func (r *adoptionApplicationRepository) ListPendingByPlot(tx *gorm.DB, plotID, excludeID uint) ([]model.AdoptionApplication, error) {
	var list []model.AdoptionApplication
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("plot_id = ? AND id <> ? AND status = ?", plotID, excludeID, string(constants.ApplicationStatusPending)).
		Order("id ASC").Find(&list).Error
	return list, err
}

// List 分页查询认养申请（userID 为 0 表示不过滤，即管理员查看全部）。
func (r *adoptionApplicationRepository) List(pq util.PageQuery, userID, plotID uint, status string) ([]model.AdoptionApplication, int64, error) {
	var list []model.AdoptionApplication
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
	if err := util.Paginate(q.Order("id DESC"), pq).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
