package dto

import (
	"github.com/communitygarden/server/internal/model"
)

// CreateAdoptionApplicationRequest 提交认养申请（居民附留言）。
type CreateAdoptionApplicationRequest struct {
	PlotID  uint   `json:"plot_id" binding:"required,gt=0"`
	Message string `json:"message" binding:"required,max=500"`
}

// ReviewAdoptionApplicationRequest 管理员审核认养申请。
type ReviewAdoptionApplicationRequest struct {
	Action     string `json:"action" binding:"required,oneof=approve reject"`
	ReviewNote string `json:"review_note" binding:"omitempty,max=500"`
}

// AdoptionApplicationOutDTO 认养申请输出。
type AdoptionApplicationOutDTO struct {
	ID           uint    `json:"id"`
	PlotID       uint    `json:"plot_id"`
	PlotCode     string  `json:"plot_code"`
	PlotName     string  `json:"plot_name"`
	UserID       uint    `json:"user_id"`
	Username     string  `json:"username"`
	Nickname     string  `json:"nickname"`
	Message      string  `json:"message"`
	Status       string  `json:"status"`
	ReviewNote   string  `json:"review_note"`
	ReviewedBy   *uint   `json:"reviewed_by"`
	ReviewerName string  `json:"reviewer_name"`
	ReviewedAt   *string `json:"reviewed_at"`
	CreatedAt    string  `json:"created_at"`
}

// ToAdoptionApplicationOutDTO 模型转 DTO。
func ToAdoptionApplicationOutDTO(a *model.AdoptionApplication) *AdoptionApplicationOutDTO {
	dto := &AdoptionApplicationOutDTO{
		ID:         a.ID,
		PlotID:     a.PlotID,
		UserID:     a.UserID,
		Message:    a.Message,
		Status:     a.Status,
		ReviewNote: a.ReviewNote,
		ReviewedBy: a.ReviewedBy,
		CreatedAt:  a.CreatedAt.Format("2006-01-02 15:04:05"),
	}
	if a.Plot != nil {
		dto.PlotCode = a.Plot.Code
		dto.PlotName = a.Plot.Name
	}
	if a.User != nil {
		dto.Username = a.User.Username
		dto.Nickname = a.User.Nickname
	}
	if a.Reviewer != nil {
		dto.ReviewerName = a.Reviewer.Nickname
		if dto.ReviewerName == "" {
			dto.ReviewerName = a.Reviewer.Username
		}
	}
	if a.ReviewedAt != nil {
		s := a.ReviewedAt.Format("2006-01-02 15:04:05")
		dto.ReviewedAt = &s
	}
	return dto
}
