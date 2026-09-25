package dto

import (
	"github.com/communitygarden/server/internal/model"
)

// SubmitApplicationRequest 提交认养申请（居民，附留言）。
type SubmitApplicationRequest struct {
	PlotID  uint   `json:"plot_id" binding:"required,gt=0"`
	Message string `json:"message" binding:"required,max=512"`
}

// ReviewApplicationRequest 审核认养申请（管理员，可附审核备注）。
type ReviewApplicationRequest struct {
	Note string `json:"note" binding:"omitempty,max=512"`
}

// ApplicationOutDTO 认养申请输出。
type ApplicationOutDTO struct {
	ID         uint        `json:"id"`
	PlotID     uint        `json:"plot_id"`
	Plot       *PlotOutDTO `json:"plot"`
	UserID     uint        `json:"user_id"`
	User       *UserOutDTO `json:"user"`
	Message    string      `json:"message"`
	Status     string      `json:"status"`
	ReviewerID *uint       `json:"reviewer_id"`
	Reviewer   *UserOutDTO `json:"reviewer"`
	ReviewNote string      `json:"review_note"`
	ReviewedAt string      `json:"reviewed_at"`
	CreatedAt  string      `json:"created_at"`
}

// ToApplicationOutDTO 模型转 DTO。
func ToApplicationOutDTO(a *model.AdoptionApplication) *ApplicationOutDTO {
	out := &ApplicationOutDTO{
		ID:         a.ID,
		PlotID:     a.PlotID,
		UserID:     a.UserID,
		Message:    a.Message,
		Status:     a.Status,
		ReviewerID: a.ReviewerID,
		ReviewNote: a.ReviewNote,
		CreatedAt:  a.CreatedAt.Format("2006-01-02 15:04:05"),
	}
	if a.Plot != nil {
		out.Plot = ToPlotOutDTO(a.Plot)
	}
	if a.User != nil {
		out.User = ToUserOutDTO(a.User)
	}
	if a.Reviewer != nil {
		out.Reviewer = ToUserOutDTO(a.Reviewer)
	}
	if a.ReviewedAt != nil {
		out.ReviewedAt = a.ReviewedAt.Format("2006-01-02 15:04:05")
	}
	return out
}
