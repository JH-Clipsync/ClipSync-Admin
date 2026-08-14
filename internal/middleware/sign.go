package middleware

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"io"
	"strconv"
	"time"

	"github.com/clipsync/admin/internal/auth"
	"github.com/clipsync/admin/internal/config"
	"github.com/clipsync/admin/internal/result"
	"github.com/clipsync/admin/internal/service"
	"github.com/gin-gonic/gin"
)

const (
	// signWindow 签名时间戳允许偏差，超出视为重放。
	signWindow = 5 * time.Minute
	// nonceTTL nonce 防重放缓存时长。
	nonceTTL = 5 * time.Minute
	// apiPrefix 管理端路由前缀，签名路径需剥离此前缀与前端一致。
	apiPrefix = "/api/admin"
)

// 签名请求头。所有接口（含登录）必传这三个头。
const (
	HeaderTimestamp = "X-Timestamp"
	HeaderNonce     = "X-Nonce"
	HeaderSignature = "X-Signature"
)

// Sign 全局签名校验中间件。
//
// 所有 /api/admin 请求（含 /auth/login）必须携带 X-Timestamp / X-Nonce / X-Signature。
//
// 密钥选择规则：
//   - 有 Authorization 头 → 解析 JWT 拿 adminID → 取会话动态密钥
//   - 无 Authorization 头（仅 /auth/login）→ 用 config.SignStaticSecret 静态密钥
//
// 校验流程：取三头 → 时间戳偏差校验 → nonce 防重放 → 取密钥 → 读 body 算 MD5 →
// 算期望签名（含 query） → 常数时间比对。
func Sign(jwtCfg config.JWTConfig, mgr *auth.Manager, authSvc *service.AuthService, staticSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ts := c.GetHeader(HeaderTimestamp)
		nonce := c.GetHeader(HeaderNonce)
		sig := c.GetHeader(HeaderSignature)
		if ts == "" || nonce == "" || sig == "" {
			result.FailWith(c, result.CodeSignInvalid, "缺少签名参数")
			c.Abort()
			return
		}

		// 时间戳偏差校验
		ms, err := strconv.ParseInt(ts, 10, 64)
		if err != nil {
			result.FailWith(c, result.CodeSignInvalid, "时间戳格式错误")
			c.Abort()
			return
		}
		diff := time.Since(time.UnixMilli(ms))
		if diff < -signWindow || diff > signWindow {
			result.Fail(c, result.CodeSignExpired)
			c.Abort()
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), redisOpTimeout*2)
		defer cancel()

		// nonce 防重放
		ok, err := authSvc.SetNXNonce(ctx, "admin:sign_nonce:"+nonce, nonceTTL)
		if err != nil {
			result.Fail(c, result.CodeCacheError)
			c.Abort()
			return
		}
		if !ok {
			result.FailWith(c, result.CodeSignInvalid, "请求已过期或重复提交")
			c.Abort()
			return
		}

		// 取签名密钥：有 token 用动态密钥，无 token 用静态密钥
		var secret string
		rawToken := StripBearer(c.GetHeader(jwtCfg.Header), jwtCfg.Scheme)
		if rawToken != "" {
			claims, err := mgr.Parse(rawToken)
			if err == nil && claims != nil {
				secret, _ = authSvc.GetSignSecret(ctx, claims.AdminID)
			}
			// JWT 无效/过期时 secret 为空，下面校验会失败
		} else {
			secret = staticSecret
		}
		if secret == "" {
			result.FailWith(c, result.CodeSignInvalid, "签名密钥失效，请重新登录")
			c.Abort()
			return
		}

		// 读 body 算 MD5，再复原 body 给后续 handler
		var bodyMD5 string
		if c.Request.Body != nil {
			raw, err := io.ReadAll(c.Request.Body)
			if err != nil {
				result.FailWith(c, result.CodeSignInvalid, "读取请求体失败")
				c.Abort()
				return
			}
			c.Request.Body = io.NopCloser(bytes.NewBuffer(raw))
			if len(raw) > 0 {
				sum := md5.Sum(raw)
				bodyMD5 = hex.EncodeToString(sum[:])
			}
		}

		// 路径：剥离 /api/admin 前缀，与前端 config.url 一致
		path := c.Request.URL.Path
		if len(path) >= len(apiPrefix) && path[:len(apiPrefix)] == apiPrefix {
			path = path[len(apiPrefix):]
		}
		if path == "" {
			path = "/"
		}

		// query 参与签名：按 key 字典序拼接
		query := auth.BuildSortedQuery(c.Request.URL.Query())

		want := auth.CalcSign(c.Request.Method, path, query, ts, nonce, bodyMD5, secret)
		if !auth.Equal(want, sig) {
			result.FailWith(c, result.CodeSignInvalid, "签名校验失败")
			c.Abort()
			return
		}
		c.Next()
	}
}
