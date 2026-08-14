package middleware

import (
	"context"
	"time"

	"github.com/clipsync/admin/internal/auth"
	"github.com/clipsync/admin/internal/config"
	"github.com/clipsync/admin/internal/result"
	"github.com/clipsync/admin/internal/service"
	"github.com/gin-gonic/gin"
)

// ContextAdminKey stores *auth.Claims of the currently-logged admin.
const (
	ContextAdminKey   = "authAdmin"
	ContextIsSuperKey = "isSuper"
	ContextPermIDsKey = "permIDs"
)

// RefreshedTokenHeader 中间件为管理员自动续签一份新 token 时通过该响应头下发。
// 前端 admin axios 拦截器读到就把本地 token 换掉。
const RefreshedTokenHeader = "X-Refresh-Token"

// JWTAuth validates the Bearer token AND ensures it is still active in Redis.
// 若剩余寿命 < TTL/2 → 直接自动续签一份新 token，通过 X-Refresh-Token 响应头返回。
func JWTAuth(cfg config.JWTConfig, mgr *auth.Manager, authSvc *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := StripBearer(c.GetHeader(cfg.Header), cfg.Scheme)
		if raw == "" {
			result.Fail(c, result.CodeUnauthorized)
			c.Abort()
			return
		}
		claims, err := mgr.Parse(raw)
		if err != nil {
			switch err {
			case auth.ErrTokenExpired:
				result.Fail(c, result.CodeJWTExpired)
			default:
				result.Fail(c, result.CodeJWTInvalid)
			}
			c.Abort()
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), redisOpTimeout)
		defer cancel()

		active, err := authSvc.IsTokenActive(ctx, claims.ID)
		if err != nil {
			result.Fail(c, result.CodeCacheError)
			c.Abort()
			return
		}
		if !active {
			result.Fail(c, result.CodeJWTExpired)
			c.Abort()
			return
		}
		authSvc.RefreshToken(ctx, claims.ID)

		// —— 自动续签：剩余 < TTL/2 时给客户端签发新 token ——
		if cfg.RefreshOnAccess && claims.ExpiresAt != nil {
			ttl := cfg.TTLDuration()
			remaining := time.Until(claims.ExpiresAt.Time)
			if remaining > 0 && remaining < ttl/2 {
				newTok, err := authSvc.Reissue(ctx, claims.AdminID, claims.Account, claims.ID)
				if err == nil {
					c.Header(RefreshedTokenHeader, newTok)
					c.Header("Access-Control-Expose-Headers", RefreshedTokenHeader)
					raw = newTok
				}
			}
		}

		c.Set(ContextAdminKey, claims)
		c.Set("token", raw)
		c.Next()
	}
}

// RBAC checks whether the current admin has permission for (method, path).
// Super admin (role.type=1) bypasses the check.
func RBAC(authSvc *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		v, ok := c.Get(ContextAdminKey)
		if !ok {
			result.Fail(c, result.CodeUnauthorized)
			c.Abort()
			return
		}
		claims := v.(*auth.Claims)

		ctx, cancel := context.WithTimeout(c.Request.Context(), redisOpTimeout*2)
		defer cancel()

		roleIDs, err := authSvc.UserRoleIDs(ctx, claims.AdminID)
		if err != nil {
			result.Fail(c, result.CodeDBError)
			c.Abort()
			return
		}
		super, err := authSvc.IsSuper(ctx, roleIDs)
		if err != nil {
			result.Fail(c, result.CodeDBError)
			c.Abort()
			return
		}
		c.Set(ContextIsSuperKey, super)
		if super {
			c.Next()
			return
		}

		permIDs, err := authSvc.PermIDsOfRoles(ctx, roleIDs)
		if err != nil {
			result.Fail(c, result.CodeDBError)
			c.Abort()
			return
		}
		c.Set(ContextPermIDsKey, permIDs)

		if err := authSvc.CheckRoute(ctx, c.Request.Method, c.FullPath(), permIDs); err != nil {
			if be, ok := service.AsBiz(err); ok {
				result.FailWith(c, be.Code, be.Message)
			} else {
				result.Fail(c, result.CodeAccessDenied)
			}
			c.Abort()
			return
		}
		c.Next()
	}
}
