package handler

import (
	"github.com/clipsync/admin/internal/model"
	"github.com/clipsync/admin/internal/result"
	"github.com/clipsync/admin/internal/service"
	"github.com/gin-gonic/gin"
)

type RBACHandler struct{ svc *service.RBACService }

func NewRBACHandler(svc *service.RBACService) *RBACHandler { return &RBACHandler{svc: svc} }

// ---- Admins ----

func (h *RBACHandler) ListAdmins(c *gin.Context) {
	p, err := h.svc.ListAdmins(c.Request.Context(), c.Query("keyword"), intQ(c, "page", 1), intQ(c, "pageSize", 20))
	if err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, p)
}

type createAdminReq struct {
	model.RbacAdmin
	Password string   `json:"password"`
	RoleIDs  []uint64 `json:"roleIds"`
}

func (h *RBACHandler) CreateAdmin(c *gin.Context) {
	var req createAdminReq
	if !bindOrFail(c, &req) {
		return
	}
	op := uint64(0)
	if cl := currentAdmin(c); cl != nil {
		op = cl.AdminID
	}
	if err := h.svc.CreateAdmin(c.Request.Context(), &req.RbacAdmin, req.Password, req.RoleIDs, op); err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, req.RbacAdmin)
}

type updateAdminReq struct {
	model.RbacAdmin
	RoleIDs []uint64 `json:"roleIds"`
}

func (h *RBACHandler) UpdateAdmin(c *gin.Context) {
	var req updateAdminReq
	if !bindOrFail(c, &req) {
		return
	}
	op := uint64(0)
	if cl := currentAdmin(c); cl != nil {
		op = cl.AdminID
	}
	if err := h.svc.UpdateAdmin(c.Request.Context(), uintP(c, "id"), &req.RbacAdmin, req.RoleIDs, op); err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, nil)
}

type resetPassReq struct {
	Password string `json:"password"`
}

func (h *RBACHandler) ResetAdminPassword(c *gin.Context) {
	var req resetPassReq
	if !bindOrFail(c, &req) {
		return
	}
	op := uint64(0)
	if cl := currentAdmin(c); cl != nil {
		op = cl.AdminID
	}
	if err := h.svc.ResetAdminPassword(c.Request.Context(), uintP(c, "id"), req.Password, op); err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, nil)
}

func (h *RBACHandler) DeleteAdmin(c *gin.Context) {
	if err := h.svc.DeleteAdmin(c.Request.Context(), uintP(c, "id"), operatorID(c)); err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, nil)
}

func (h *RBACHandler) UpdateAdminStatus(c *gin.Context) {
	type req struct {
		Status int `json:"status"`
	}
	var r req
	if !bindOrFail(c, &r) {
		return
	}
	if err := h.svc.UpdateAdminStatus(c.Request.Context(), uintP(c, "id"), r.Status, operatorID(c)); err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, nil)
}

func (h *RBACHandler) AdminRoleIDs(c *gin.Context) {
	ids, err := h.svc.AdminRoleIDs(c.Request.Context(), uintP(c, "id"))
	if err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, ids)
}

// ---- Roles ----

func (h *RBACHandler) ListRoles(c *gin.Context) {
	list, err := h.svc.ListRoles(c.Request.Context())
	if err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, list)
}

func (h *RBACHandler) CreateRole(c *gin.Context) {
	var r model.RbacRole
	if !bindOrFail(c, &r) {
		return
	}
	if err := h.svc.CreateRole(c.Request.Context(), &r, operatorID(c)); err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, r)
}

func (h *RBACHandler) UpdateRole(c *gin.Context) {
	var r model.RbacRole
	if !bindOrFail(c, &r) {
		return
	}
	if err := h.svc.UpdateRole(c.Request.Context(), uintP(c, "id"), &r, operatorID(c)); err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, nil)
}

func (h *RBACHandler) DeleteRole(c *gin.Context) {
	if err := h.svc.DeleteRole(c.Request.Context(), uintP(c, "id"), operatorID(c)); err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, nil)
}

type assignMenusReq struct {
	MenuIDs []uint64 `json:"menuIds"`
}

func (h *RBACHandler) AssignRoleMenus(c *gin.Context) {
	var req assignMenusReq
	if !bindOrFail(c, &req) {
		return
	}
	op := uint64(0)
	if cl := currentAdmin(c); cl != nil {
		op = cl.AdminID
	}
	if err := h.svc.AssignRoleMenus(c.Request.Context(), uintP(c, "id"), req.MenuIDs, op); err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, nil)
}

func (h *RBACHandler) RoleMenuIDs(c *gin.Context) {
	ids, err := h.svc.RoleMenuIDs(c.Request.Context(), uintP(c, "id"))
	if err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, ids)
}

// ---- Menus ----

func (h *RBACHandler) ListMenus(c *gin.Context) {
	list, err := h.svc.ListMenus(c.Request.Context())
	if err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, list)
}

func (h *RBACHandler) CreateMenu(c *gin.Context) {
	var m model.RbacMenu
	if !bindOrFail(c, &m) {
		return
	}
	if err := h.svc.CreateMenu(c.Request.Context(), &m, operatorID(c)); err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, m)
}

func (h *RBACHandler) UpdateMenu(c *gin.Context) {
	// 接收原始字段：只更新前端明确传了的字段，避免拖拽排序（只传 parentId/sort）
	// 把 title/name/path 等其他字段覆盖成空
	var raw map[string]any
	if err := c.ShouldBindJSON(&raw); err != nil {
		result.Fail(c, result.CodeParamError)
		return
	}
	if err := h.svc.UpdateMenu(c.Request.Context(), uintP(c, "id"), raw, operatorID(c)); err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, nil)
}

func (h *RBACHandler) DeleteMenu(c *gin.Context) {
	if err := h.svc.DeleteMenu(c.Request.Context(), uintP(c, "id"), operatorID(c)); err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, nil)
}

type assignPermsReq struct {
	PermIDs []uint64 `json:"permIds"`
}

func (h *RBACHandler) AssignMenuPerms(c *gin.Context) {
	var req assignPermsReq
	if !bindOrFail(c, &req) {
		return
	}
	op := uint64(0)
	if cl := currentAdmin(c); cl != nil {
		op = cl.AdminID
	}
	if err := h.svc.AssignMenuPerms(c.Request.Context(), uintP(c, "id"), req.PermIDs, op); err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, nil)
}

// ---- Perms ----

func (h *RBACHandler) ListPerms(c *gin.Context) {
	p, err := h.svc.ListPerms(c.Request.Context(), c.Query("keyword"), intQ(c, "page", 1), intQ(c, "pageSize", 20))
	if err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, p)
}

func (h *RBACHandler) CreatePerm(c *gin.Context) {
	var p model.RbacPerm
	if !bindOrFail(c, &p) {
		return
	}
	if err := h.svc.CreatePerm(c.Request.Context(), &p, operatorID(c)); err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, p)
}

func (h *RBACHandler) UpdatePerm(c *gin.Context) {
	var p model.RbacPerm
	if !bindOrFail(c, &p) {
		return
	}
	if err := h.svc.UpdatePerm(c.Request.Context(), uintP(c, "id"), &p, operatorID(c)); err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, nil)
}

func (h *RBACHandler) DeletePerm(c *gin.Context) {
	if err := h.svc.DeletePerm(c.Request.Context(), uintP(c, "id"), operatorID(c)); err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, nil)
}
