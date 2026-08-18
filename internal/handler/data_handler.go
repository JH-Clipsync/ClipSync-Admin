package handler

import (
	"strconv"

	"github.com/clipsync/admin/internal/result"
	"github.com/clipsync/admin/internal/service"
	"github.com/gin-gonic/gin"
)

type DataHandler struct{ svc *service.AdminDataService }

func NewDataHandler(svc *service.AdminDataService) *DataHandler { return &DataHandler{svc: svc} }

func operatorID(c *gin.Context) uint64 {
	if cl := currentAdmin(c); cl != nil {
		return cl.AdminID
	}
	return 0
}

// ---- Users ----

// ListUsers GET /users
// 支持 keyword（搜 username）、disabled/status 过滤、分页。
// 前端传 status（0正常 1禁用）会映射到 disabled 字段。
func (h *DataHandler) ListUsers(c *gin.Context) {
	// disabled 过滤：-1 表示不过滤
	disabled := -1
	if s := c.Query("disabled"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			disabled = v
		}
	}
	// 兼容前端传 status 参数（status 0=正常 1=禁用，映射到 disabled）
	if s := c.Query("status"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			disabled = v
		}
	}
	p, err := h.svc.ListUsers(c.Request.Context(), c.Query("keyword"), disabled, intQ(c, "page", 1), intQ(c, "pageSize", 20))
	if err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, p)
}

// GetUser GET /users/:id  返回用户详情 + 当前在线设备列表
func (h *DataHandler) GetUser(c *gin.Context) {
	u, err := h.svc.GetUserDetail(c.Request.Context(), int64P(c, "id"))
	if err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, u)
}

type updateUserReq struct {
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Status   int    `json:"status"` // 0正常 1禁用，映射到 disabled
}

// UpdateUser PUT /users/:id 可改 username、nickname 和 disabled
func (h *DataHandler) UpdateUser(c *gin.Context) {
	var req updateUserReq
	if !bindOrFail(c, &req) {
		return
	}
	if err := h.svc.UpdateUser(c.Request.Context(), int64P(c, "id"), req.Username, req.Nickname, req.Status, operatorID(c)); err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, nil)
}

type createUserReq struct {
	Username string `json:"username" binding:"required"`
	Nickname string `json:"nickname"`
	Password string `json:"password" binding:"required"`
}

// CreateUser POST /users 管理端创建用户
func (h *DataHandler) CreateUser(c *gin.Context) {
	var req createUserReq
	if !bindOrFail(c, &req) {
		return
	}
	id, err := h.svc.CreateUser(c.Request.Context(), req.Username, req.Nickname, req.Password)
	if err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, map[string]int64{"id": id})
}

type updateUserStatusReq struct {
	Status int `json:"status"` // 0正常 1禁用，映射到 disabled
}

// UpdateUserStatus PUT /users/:id/status 改 disabled 字段
func (h *DataHandler) UpdateUserStatus(c *gin.Context) {
	var req updateUserStatusReq
	if !bindOrFail(c, &req) {
		return
	}
	if err := h.svc.UpdateUserStatus(c.Request.Context(), int64P(c, "id"), req.Status, operatorID(c)); err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, nil)
}

// ResetUserPassword POST /users/:id/reset-password
// 生成新密码，用 scrypt 哈希写回 password_hash，返回明文密码给前端。
func (h *DataHandler) ResetUserPassword(c *gin.Context) {
	pwd, err := h.svc.ResetUserPassword(c.Request.Context(), int64P(c, "id"), operatorID(c))
	if err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, map[string]string{"password": pwd})
}

// DeleteUser DELETE /users/:id 物理删除用户
func (h *DataHandler) DeleteUser(c *gin.Context) {
	if err := h.svc.DeleteUser(c.Request.Context(), int64P(c, "id"), operatorID(c)); err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, nil)
}

// ---- Dashboard ----

// Dashboard GET /dashboard 首页统计
func (h *DataHandler) Dashboard(c *gin.Context) {
	stat, err := h.svc.Dashboard(c.Request.Context())
	if err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, stat)
}

// ---- Devices ----

// KickUserNow POST /users/:id/kick 主动踢该用户全部设备下线
func (h *DataHandler) KickUserNow(c *gin.Context) {
	if err := h.svc.KickUserNow(c.Request.Context(), int64P(c, "id")); err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, nil)
}

// ListDevices GET /users/:id/devices 列出该用户所有设备
func (h *DataHandler) ListDevices(c *gin.Context) {
	devices, err := h.svc.ListDevices(c.Request.Context(), int64P(c, "id"))
	if err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, devices)
}

// ListAllDevices GET /devices 跨用户分页查询设备
// 查询参数：keyword（用户名/设备ID/名称/IP）、status（0=正常 1=禁用 -1=全部）、user_id、page、size
func (h *DataHandler) ListAllDevices(c *gin.Context) {
	keyword := c.Query("keyword")
	status := -1
	if s := c.Query("status"); s != "" {
		status = intQ(c, "status", -1)
	}
	userID := int64(0)
	if s := c.Query("user_id"); s != "" {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			userID = v
		}
	}
	page := intQ(c, "page", 1)
	size := intQ(c, "size", 20)
	pageData, err := h.svc.ListAllDevices(c.Request.Context(), keyword, status, userID, page, size)
	if err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, pageData)
}

type setDeviceStatusReq struct {
	Disabled bool `json:"disabled"`
}

// SetDeviceStatus PUT /users/:id/devices/:did 启用/禁用设备
func (h *DataHandler) SetDeviceStatus(c *gin.Context) {
	var req setDeviceStatusReq
	if !bindOrFail(c, &req) {
		return
	}
	deviceID := c.Param("did")
	if deviceID == "" {
		result.Fail(c, result.CodeParamError)
		return
	}
	if err := h.svc.SetDeviceStatus(c.Request.Context(), int64P(c, "id"), deviceID, req.Disabled); err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, nil)
}

type renameDeviceReq struct {
	Name string `json:"name"`
}

// RenameDevice PUT /users/:id/devices/:did/name 修改设备自定义名称
func (h *DataHandler) RenameDevice(c *gin.Context) {
	var req renameDeviceReq
	if !bindOrFail(c, &req) {
		return
	}
	deviceID := c.Param("did")
	if deviceID == "" {
		result.Fail(c, result.CodeParamError)
		return
	}
	if err := h.svc.RenameDevice(c.Request.Context(), int64P(c, "id"), deviceID, req.Name); err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, nil)
}

// KickDevice POST /users/:id/devices/:did/kick 主动踢该设备下线
func (h *DataHandler) KickDevice(c *gin.Context) {
	deviceID := c.Param("did")
	if deviceID == "" {
		result.Fail(c, result.CodeParamError)
		return
	}
	if err := h.svc.KickDevice(c.Request.Context(), int64P(c, "id"), deviceID); err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, nil)
}
