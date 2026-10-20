package handler

import (
	"strconv"

	"github.com/clipsync/admin/internal/result"
	"github.com/gin-gonic/gin"
)

// ---- 开放接口 ApiKey 管理 ----

type createAPIKeyReq struct {
	Name      string `json:"name"`
	Phone     string `json:"phone"`
	DeviceID  string `json:"device_id"`
	ExpiresAt string `json:"expires_at"` // RFC3339；留空 = 无限期
}

// CreateAPIKey POST /users/:id/api-keys 为用户签发密钥，明文仅本次返回。
func (h *DataHandler) CreateAPIKey(c *gin.Context) {
	var req createAPIKeyReq
	if !bindOrFail(c, &req) {
		return
	}
	out, err := h.svc.CreateAPIKey(c.Request.Context(), int64P(c, "id"),
		req.Name, req.Phone, req.DeviceID, req.ExpiresAt)
	if err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, out)
}

// ListAPIKeys GET /users/:id/api-keys 列出某用户密钥。
func (h *DataHandler) ListAPIKeys(c *gin.Context) {
	items, err := h.svc.ListAPIKeys(c.Request.Context(), int64P(c, "id"))
	if err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, gin.H{"items": items, "total": len(items)})
}

// ListAllAPIKeys GET /api-keys 列出全部密钥。
func (h *DataHandler) ListAllAPIKeys(c *gin.Context) {
	items, err := h.svc.ListAPIKeys(c.Request.Context(), 0)
	if err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, gin.H{"items": items, "total": len(items)})
}

// RevokeAPIKey DELETE /api-keys/:keyId 吊销一条密钥。
func (h *DataHandler) RevokeAPIKey(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("keyId"), 10, 64)
	if err != nil || id <= 0 {
		result.Fail(c, result.CodeParamError)
		return
	}
	if err := h.svc.RevokeAPIKey(c.Request.Context(), id); err != nil {
		respBiz(c, err)
		return
	}
	result.Success(c, nil)
}
