package model

import "time"

// AdoptionApplication 地块认养申请实体（居民提交 -> 管理员审核 -> 批准/拒绝/撤回）。
type AdoptionApplication struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	PlotID     uint       `gorm:"not null;index" json:"plot_id"`
	Plot       *Plot      `gorm:"foreignKey:PlotID" json:"plot"`
	UserID     uint       `gorm:"not null;index" json:"user_id"`
	User       *User      `gorm:"foreignKey:UserID" json:"user"`
	Message    string     `gorm:"size:512;not null" json:"message"`
	Status     string     `gorm:"size:32;not null;default:pending;index" json:"status"`
	ReviewerID *uint      `gorm:"index" json:"reviewer_id"`
	Reviewer   *User      `gorm:"foreignKey:ReviewerID" json:"reviewer"`
	ReviewNote string     `gorm:"size:512" json:"review_note"`
	ReviewedAt *time.Time `json:"reviewed_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}
