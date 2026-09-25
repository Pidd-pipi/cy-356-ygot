package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/communitygarden/server/internal/constants"
	"github.com/communitygarden/server/internal/dto"
	"github.com/communitygarden/server/internal/middleware"
	"github.com/communitygarden/server/internal/service"
	"github.com/communitygarden/server/internal/util"
)

// AdoptionApplicationHandler 认养申请接口。
type AdoptionApplicationHandler struct {
	appService *service.AdoptionApplicationService
	audit      middleware.AuditWriter
}

// NewAdoptionApplicationHandler 构造认养申请接口。
func NewAdoptionApplicationHandler(appService *service.AdoptionApplicationService, audit middleware.AuditWriter) *AdoptionApplicationHandler {
	return &AdoptionApplicationHandler{appService: appService, audit: audit}
}

// Submit 提交认养申请（登录居民，附留言）。
func (h *AdoptionApplicationHandler) Submit(c *gin.Context) {
	var req dto.SubmitApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeValidationFailed, constants.ErrorText[constants.CodeValidationFailed]+": "+err.Error())
		return
	}
	claims, _ := util.GetClaims(c)
	app, err := h.appService.Submit(&req, claims.UserID, claims.Username)
	if err != nil {
		util.FailWithAppError(c, err)
		return
	}
	_ = h.audit.Write(claims.UserID, claims.Username, claims.Role, "SUBMIT_APPLICATION", "adoption_application", strconv.FormatUint(uint64(app.ID), 10),
		"提交认养申请，地块 id="+strconv.FormatUint(uint64(app.PlotID), 10), c.ClientIP(), util.GetRequestID(c))
	util.OK(c, dto.ToApplicationOutDTO(app))
}

// List 认养申请分页列表（居民看自己的审核结果；管理员看全部申请人和留言）。
func (h *AdoptionApplicationHandler) List(c *gin.Context) {
	pq := util.ParsePageQuery(c)
	status := c.Query("status")
	var plotID uint
	if v := c.Query("plot_id"); v != "" {
		id, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "查询参数 plot_id 必须为正整数")
			return
		}
		plotID = uint(id)
	}
	claims, _ := util.GetClaims(c)
	list, total, err := h.appService.List(pq, claims.UserID, claims.Role, plotID, status)
	if err != nil {
		util.FailWithAppError(c, err)
		return
	}
	out := make([]*dto.ApplicationOutDTO, 0, len(list))
	for i := range list {
		out = append(out, dto.ToApplicationOutDTO(&list[i]))
	}
	util.OK(c, util.PageResult{List: out, Total: total, Page: pq.Page, PageSize: pq.PageSize})
}

// Withdraw 撤回认养申请（申请人本人，仅待审核状态）。
func (h *AdoptionApplicationHandler) Withdraw(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "路径参数 id 必须为正整数")
		return
	}
	claims, _ := util.GetClaims(c)
	app, err := h.appService.Withdraw(uint(id), claims.UserID, claims.Role)
	if err != nil {
		util.FailWithAppError(c, err)
		return
	}
	_ = h.audit.Write(claims.UserID, claims.Username, claims.Role, "WITHDRAW_APPLICATION", "adoption_application", strconv.FormatUint(uint64(id), 10),
		"撤回认养申请", c.ClientIP(), util.GetRequestID(c))
	util.OK(c, dto.ToApplicationOutDTO(app))
}

// Approve 批准认养申请（管理员；批准后地块归申请人，同地块其余待审申请自动拒绝）。
func (h *AdoptionApplicationHandler) Approve(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "路径参数 id 必须为正整数")
		return
	}
	var req dto.ReviewApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeValidationFailed, constants.ErrorText[constants.CodeValidationFailed]+": "+err.Error())
		return
	}
	claims, _ := util.GetClaims(c)
	app, err := h.appService.Approve(uint(id), claims.UserID, claims.Username, req.Note)
	if err != nil {
		util.FailWithAppError(c, err)
		return
	}
	_ = h.audit.Write(claims.UserID, claims.Username, claims.Role, "APPROVE_APPLICATION", "adoption_application", strconv.FormatUint(uint64(id), 10),
		"批准认养申请，地块 id="+strconv.FormatUint(uint64(app.PlotID), 10)+" 归用户 id="+strconv.FormatUint(uint64(app.UserID), 10), c.ClientIP(), util.GetRequestID(c))
	util.OK(c, dto.ToApplicationOutDTO(app))
}

// Reject 拒绝认养申请（管理员）。
func (h *AdoptionApplicationHandler) Reject(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "路径参数 id 必须为正整数")
		return
	}
	var req dto.ReviewApplicationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeValidationFailed, constants.ErrorText[constants.CodeValidationFailed]+": "+err.Error())
		return
	}
	claims, _ := util.GetClaims(c)
	app, err := h.appService.Reject(uint(id), claims.UserID, claims.Username, req.Note)
	if err != nil {
		util.FailWithAppError(c, err)
		return
	}
	_ = h.audit.Write(claims.UserID, claims.Username, claims.Role, "REJECT_APPLICATION", "adoption_application", strconv.FormatUint(uint64(id), 10),
		"拒绝认养申请", c.ClientIP(), util.GetRequestID(c))
	util.OK(c, dto.ToApplicationOutDTO(app))
}
