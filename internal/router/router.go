package router

import (
	"github.com/clipsync/admin/internal/auth"
	"github.com/clipsync/admin/internal/config"
	"github.com/clipsync/admin/internal/handler"
	"github.com/clipsync/admin/internal/middleware"
	"github.com/clipsync/admin/internal/result"
	"github.com/clipsync/admin/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// New assembles the admin HTTP router.
func New(
	cfg *config.Config,
	lg *zap.Logger,
	db *gorm.DB,
	rdb *redis.Client,
	jwtMgr *auth.Manager,
) *gin.Engine {
	gin.SetMode(cfg.App.Mode)
	r := gin.New()

	r.Use(gin.Recovery())
	r.Use(middleware.TraceID())
	r.Use(middleware.AccessLog(lg))
	r.Use(middleware.CORS(cfg.CORS))

	r.NoRoute(func(c *gin.Context) { result.Fail(c, result.CodeAPINotFound) })

	authSvc := service.NewAuthService(*cfg, db, rdb, jwtMgr)
	// ClipSync-Server 联动：重置密码/封禁/删用户时，通知 Server 强制下线所有连接
	notifier := service.NewServerNotifier(cfg.Server, rdb, lg)
	dataSvc := service.NewAdminDataService(db, rdb, cfg.Server.KeyPrefix, notifier)
	rbacSvc := service.NewRBACService(db, cfg.Security.BcryptCost, cfg.Bootstrap.SuperAdminAccount)

	authH := handler.NewAuthHandler(authSvc)
	dataH := handler.NewDataHandler(dataSvc)
	rbacH := handler.NewRBACHandler(rbacSvc)
	uploadH := handler.NewUploadHandler(cfg.Upload)

	// 静态文件访问：/static/<yyyy>/<mm>/<dd>/xxx.jpg
	if cfg.Upload.Dir != "" && cfg.Upload.URLPrefix != "" {
		r.Static(cfg.Upload.URLPrefix, cfg.Upload.Dir)
	}

	api := r.Group("/api/admin")

	// 全局签名校验：所有接口（含登录）必须携带签名头
	api.Use(middleware.Sign(cfg.JWT, jwtMgr, authSvc, cfg.Security.SignStaticSecret))

	// public
	api.GET("/health", func(c *gin.Context) { result.Success(c, gin.H{"ok": true}) })
	api.POST("/auth/login", authH.Login)

	// protected
	priv := api.Group("")
	priv.Use(middleware.JWTAuth(cfg.JWT, jwtMgr, authSvc))
	{
		priv.POST("/auth/logout", authH.Logout)
		priv.GET("/auth/me", authH.Me)
		priv.GET("/auth/menus", authH.Menus)
		priv.PUT("/auth/password", authH.ChangePassword)
		priv.PUT("/auth/profile", authH.UpdateProfile)
		priv.POST("/upload/image", uploadH.Image)
	}

	// RBAC-checked routes
	rbacGroup := api.Group("")
	rbacGroup.Use(middleware.JWTAuth(cfg.JWT, jwtMgr, authSvc), middleware.RBAC(authSvc))
	{
		// dashboard
		rbacGroup.GET("/dashboard", dataH.Dashboard)

		// 用户管理
		rbacGroup.GET("/users", dataH.ListUsers)
		rbacGroup.GET("/users/:id", dataH.GetUser)
		rbacGroup.PUT("/users/:id", dataH.UpdateUser)
		rbacGroup.PUT("/users/:id/status", dataH.UpdateUserStatus)
		rbacGroup.POST("/users/:id/reset-password", dataH.ResetUserPassword)
		rbacGroup.DELETE("/users/:id", dataH.DeleteUser)

		// RBAC management
		rbacGroup.GET("/rbac/admins", rbacH.ListAdmins)
		rbacGroup.POST("/rbac/admins", rbacH.CreateAdmin)
		rbacGroup.PUT("/rbac/admins/:id", rbacH.UpdateAdmin)
		rbacGroup.PUT("/rbac/admins/:id/status", rbacH.UpdateAdminStatus)
		rbacGroup.PUT("/rbac/admins/:id/password", rbacH.ResetAdminPassword)
		rbacGroup.GET("/rbac/admins/:id/roles", rbacH.AdminRoleIDs)
		rbacGroup.DELETE("/rbac/admins/:id", rbacH.DeleteAdmin)

		rbacGroup.GET("/rbac/roles", rbacH.ListRoles)
		rbacGroup.POST("/rbac/roles", rbacH.CreateRole)
		rbacGroup.PUT("/rbac/roles/:id", rbacH.UpdateRole)
		rbacGroup.DELETE("/rbac/roles/:id", rbacH.DeleteRole)
		rbacGroup.PUT("/rbac/roles/:id/menus", rbacH.AssignRoleMenus)
		rbacGroup.GET("/rbac/roles/:id/menus", rbacH.RoleMenuIDs)

		rbacGroup.GET("/rbac/menus", rbacH.ListMenus)
		rbacGroup.POST("/rbac/menus", rbacH.CreateMenu)
		rbacGroup.PUT("/rbac/menus/:id", rbacH.UpdateMenu)
		rbacGroup.DELETE("/rbac/menus/:id", rbacH.DeleteMenu)
		rbacGroup.PUT("/rbac/menus/:id/perms", rbacH.AssignMenuPerms)

		rbacGroup.GET("/rbac/perms", rbacH.ListPerms)
		rbacGroup.POST("/rbac/perms", rbacH.CreatePerm)
		rbacGroup.PUT("/rbac/perms/:id", rbacH.UpdatePerm)
		rbacGroup.DELETE("/rbac/perms/:id", rbacH.DeletePerm)
	}

	return r
}
