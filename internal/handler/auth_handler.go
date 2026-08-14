package handler

import (
	"github.com/clipsync/admin/internal/result"
	"github.com/clipsync/admin/internal/service"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct{ svc *service.AuthService }

func NewAuthHandler(svc *service.AuthService) *AuthHandler { return &AuthHandler{svc: svc} }

type loginReq struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

// Login POST /api/admin/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginReq
	if !bindOrFail(c, &req) {
		return
	}
	tok, admin, signSecret, err := h.svc.Login(c.Request.Context(), req.Account, req.Password)
	if err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, gin.H{
		"token":      tok,
		"admin":      admin,
		"signSecret": signSecret,
	})
}

// Logout POST /api/admin/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	claims := currentAdmin(c)
	if claims == nil {
		result.Fail(c, result.CodeUnauthorized)
		return
	}
	if err := h.svc.Logout(c.Request.Context(), claims.ID, claims.AdminID); err != nil {
		result.FailWith(c, result.CodeCacheError, err.Error())
		return
	}
	result.Success(c, nil)
}

// Me GET /api/admin/auth/me
func (h *AuthHandler) Me(c *gin.Context) {
	claims := currentAdmin(c)
	if claims == nil {
		result.Fail(c, result.CodeUnauthorized)
		return
	}
	ctx := c.Request.Context()
	admin, err := h.svc.GetAdmin(ctx, claims.AdminID)
	if err != nil {
		result.FailWith(c, result.CodeAccountNotFound, err.Error())
		return
	}
	roleIDs, _ := h.svc.UserRoleIDs(ctx, claims.AdminID)
	super, _ := h.svc.IsSuper(ctx, roleIDs)
	permIDs, _ := h.svc.PermIDsOfRoles(ctx, roleIDs)
	result.Success(c, gin.H{
		"admin":               admin,
		"roleIds":             roleIDs,
		"isSuper":             super,
		"permIds":             permIDs,
		"builtinSuperAccount": h.svc.BuiltinSuperAccount(),
	})
}

// Menus GET /api/admin/auth/menus 返回当前登录管理员可见的菜单（含按钮/数据列）
func (h *AuthHandler) Menus(c *gin.Context) {
	claims := currentAdmin(c)
	if claims == nil {
		result.Fail(c, result.CodeUnauthorized)
		return
	}
	list, err := h.svc.MenusForAdmin(c.Request.Context(), claims.AdminID)
	if err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, list)
}

type changePassReq struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

// ChangePassword PUT /api/admin/auth/password 修改自己的密码
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	claims := currentAdmin(c)
	if claims == nil {
		result.Fail(c, result.CodeUnauthorized)
		return
	}
	var req changePassReq
	if !bindOrFail(c, &req) {
		return
	}
	if err := h.svc.ChangeOwnPassword(c.Request.Context(), claims.AdminID, req.OldPassword, req.NewPassword); err != nil {
		respBiz(c, err)
		return
	}
	// 修改成功后立即让当前 token 失效，前端需要重新登录
	_ = h.svc.Logout(c.Request.Context(), claims.ID, claims.AdminID)
	result.Success(c, nil)
}

type profileReq struct {
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

// UpdateProfile PUT /api/admin/auth/profile 修改个人资料
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	claims := currentAdmin(c)
	if claims == nil {
		result.Fail(c, result.CodeUnauthorized)
		return
	}
	var req profileReq
	if !bindOrFail(c, &req) {
		return
	}
	if err := h.svc.UpdateProfile(c.Request.Context(), claims.AdminID, req.Name, req.Avatar); err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, nil)
}
