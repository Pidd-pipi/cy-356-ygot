package router

import (
	"github.com/gin-gonic/gin"

	"github.com/communitygarden/server/internal/constants"
	"github.com/communitygarden/server/internal/middleware"
)

// registerAdoptionApplications 认养申请路由。
func (r *Router) registerAdoptionApplications(g *gin.RouterGroup) {
	apps := g.Group("/applications")
	apps.Use(middleware.Auth(r.cfg, r.logger))
	{
		apps.GET("", r.applicationHandler.List)
		apps.POST("", r.applicationHandler.Submit)
		apps.POST("/:id/withdraw", r.applicationHandler.Withdraw)
	}

	admin := apps.Group("")
	admin.Use(middleware.RequireRoles(string(constants.RoleAdmin)))
	{
		admin.POST("/:id/approve", r.applicationHandler.Approve)
		admin.POST("/:id/reject", r.applicationHandler.Reject)
	}
}
