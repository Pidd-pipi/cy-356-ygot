package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/communitygarden/server/internal/constants"
	"github.com/communitygarden/server/internal/dto"
	"github.com/communitygarden/server/internal/middleware"
	"github.com/communitygarden/server/internal/model"
	"github.com/communitygarden/server/internal/service"
	"github.com/communitygarden/server/internal/util"
)

// AdoptionApplicationHandler 认养申请接口。
type AdoptionApplicationHandler struct {
	adoptionService *service.AdoptionApplicationService
	audit           middleware.AuditWriter
}

// NewAdoptionApplicationHandler 构造认养申请接口。
func NewAdoptionApplicationHandler(adoptionService *service.AdoptionApplicationService, audit middleware.AuditWriter) *AdoptionApplicationHandler {
	return &AdoptionApplicationHandler{adoptionService: adoptionService, audit: audit}
}

// Apply 提交认养申请（登录用户，附留言）。
func (h *AdoptionApplicationHandler) Apply(c *gin.Context) {
	var req dto.CreateAdoptionApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeValidationFailed, constants.ErrorText[constants.CodeValidationFailed]+": "+err.Error())
		return
	}
	claims, _ := util.GetClaims(c)
	app, err := h.adoptionService.Apply(&req, claims.UserID)
	if err != nil {
		util.FailWithAppError(c, err)
		return
	}
	out := dto.ToAdoptionApplicationOutDTO(app)
	_ = h.audit.Write(claims.UserID, claims.Username, claims.Role, "APPLY_ADOPTION", "adoption_application", strconv.FormatUint(uint64(app.ID), 10),
		"提交地块认养申请 "+out.PlotCode, c.ClientIP(), util.GetRequestID(c))
	util.OK(c, out)
}

// Mine 我的认养申请列表（审核结果可见）。
func (h *AdoptionApplicationHandler) Mine(c *gin.Context) {
	pq := util.ParsePageQuery(c)
	claims, _ := util.GetClaims(c)
	status := c.Query("status")
	apps, total, err := h.adoptionService.ListMine(pq, claims.UserID, status)
	if err != nil {
		util.FailWithAppError(c, err)
		return
	}
	util.OK(c, util.PageResult{List: toAdoptionOutList(apps), Total: total, Page: pq.Page, PageSize: pq.PageSize})
}

// Withdraw 撤回待审申请（申请人本人）。
func (h *AdoptionApplicationHandler) Withdraw(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "路径参数 id 必须为正整数")
		return
	}
	claims, _ := util.GetClaims(c)
	app, err := h.adoptionService.Withdraw(uint(id), claims.UserID, claims.Role)
	if err != nil {
		util.FailWithAppError(c, err)
		return
	}
	_ = h.audit.Write(claims.UserID, claims.Username, claims.Role, "WITHDRAW_ADOPTION", "adoption_application", strconv.FormatUint(uint64(id), 10),
		"撤回地块认养申请", c.ClientIP(), util.GetRequestID(c))
	util.OK(c, dto.ToAdoptionApplicationOutDTO(app))
}

// List 认养申请列表（管理员，可按地块/状态过滤）。
func (h *AdoptionApplicationHandler) List(c *gin.Context) {
	pq := util.ParsePageQuery(c)
	status := c.Query("status")
	plotID, _ := strconv.ParseUint(c.DefaultQuery("plot_id", "0"), 10, 64)
	apps, total, err := h.adoptionService.List(pq, uint(plotID), status)
	if err != nil {
		util.FailWithAppError(c, err)
		return
	}
	util.OK(c, util.PageResult{List: toAdoptionOutList(apps), Total: total, Page: pq.Page, PageSize: pq.PageSize})
}

// Review 审核认养申请（管理员；批准后同地块其余待审申请自动拒绝）。
func (h *AdoptionApplicationHandler) Review(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "路径参数 id 必须为正整数")
		return
	}
	var req dto.ReviewAdoptionApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeValidationFailed, constants.ErrorText[constants.CodeValidationFailed]+": "+err.Error())
		return
	}
	claims, _ := util.GetClaims(c)
	app, err := h.adoptionService.Review(uint(id), claims.UserID, claims.Username, &req)
	if err != nil {
		util.FailWithAppError(c, err)
		return
	}
	actionText := "拒绝"
	if req.Action == "approve" {
		actionText = "批准"
	}
	_ = h.audit.Write(claims.UserID, claims.Username, claims.Role, "REVIEW_ADOPTION", "adoption_application", strconv.FormatUint(uint64(id), 10),
		actionText+"地块认养申请", c.ClientIP(), util.GetRequestID(c))
	util.OK(c, dto.ToAdoptionApplicationOutDTO(app))
}

// toAdoptionOutList 模型列表转 DTO 列表。
func toAdoptionOutList(apps []model.AdoptionApplication) []*dto.AdoptionApplicationOutDTO {
	list := make([]*dto.AdoptionApplicationOutDTO, 0, len(apps))
	for i := range apps {
		list = append(list, dto.ToAdoptionApplicationOutDTO(&apps[i]))
	}
	return list
}
