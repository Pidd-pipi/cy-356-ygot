package model

import "time"

// AdoptionApplication 地块认养申请实体（状态机：pending -> approved / rejected / withdrawn）。
// 部分唯一索引 uni_adoption_pending 保证同一居民对同一地块只能保留一份待审申请。
type AdoptionApplication struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	PlotID     uint       `gorm:"index;not null;uniqueIndex:uni_adoption_pending,priority:1,where:status = 'pending'" json:"plot_id"`
	Plot       *Plot      `gorm:"foreignKey:PlotID" json:"plot"`
	UserID     uint       `gorm:"index;not null;uniqueIndex:uni_adoption_pending,priority:2,where:status = 'pending'" json:"user_id"`
	User       *User      `gorm:"foreignKey:UserID" json:"user"`
	Message    string     `gorm:"size:500;not null" json:"message"`
	Status     string     `gorm:"size:32;not null;default:pending;index" json:"status"`
	ReviewNote string     `gorm:"size:500" json:"review_note"`
	ReviewedBy *uint      `gorm:"index" json:"reviewed_by"`
	Reviewer   *User      `gorm:"foreignKey:ReviewedBy" json:"reviewer"`
	ReviewedAt *time.Time `json:"reviewed_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}
