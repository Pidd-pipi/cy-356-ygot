package router

import (
	"github.com/gin-gonic/gin"

	"github.com/communitygarden/server/internal/constants"
	"github.com/communitygarden/server/internal/middleware"
)

// registerAdoptionApplications 认养申请路由（居民提交/撤回，管理员审核）。
func (r *Router) registerAdoptionApplications(g *gin.RouterGroup) {
	auth := g.Group("/adoption-applications")
	auth.Use(middleware.Auth(r.cfg, r.logger))
	{
		auth.POST("", r.adoptionHandler.Apply)
		auth.GET("/mine", r.adoptionHandler.Mine)
		auth.POST("/:id/withdraw", r.adoptionHandler.Withdraw)
	}

	admin := g.Group("/adoption-applications")
	admin.Use(middleware.Auth(r.cfg, r.logger), middleware.RequireRoles(string(constants.RoleAdmin)))
	{
		admin.GET("", r.adoptionHandler.List)
		admin.POST("/:id/review", r.adoptionHandler.Review)
	}
}
