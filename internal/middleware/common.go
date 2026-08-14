package middleware

import (
	"strings"
	"time"

	"github.com/clipsync/admin/internal/config"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const redisOpTimeout = 2 * time.Second

// StripBearer removes an optional "Bearer " prefix.
func StripBearer(h, scheme string) string {
	if h == "" {
		return ""
	}
	if scheme != "" {
		p := scheme + " "
		if strings.HasPrefix(h, p) {
			return strings.TrimSpace(h[len(p):])
		}
	}
	return strings.TrimSpace(h)
}

// TraceID assigns each request a UUID for log correlation.
func TraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Trace-Id")
		if id == "" {
			id = uuid.NewString()
		}
		c.Set("traceId", id)
		c.Writer.Header().Set("X-Trace-Id", id)
		c.Next()
	}
}

func AccessLog(lg *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		lg.Info("access",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("ip", c.ClientIP()),
			zap.String("traceId", c.GetString("traceId")),
		)
	}
}

func CORS(cfg config.CORSConfig) gin.HandlerFunc {
	c := cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Trace-Id", "X-Timestamp", "X-Nonce", "X-Signature"},
		ExposeHeaders:    []string{"Content-Length", "X-Trace-Id", RefreshedTokenHeader},
		AllowCredentials: cfg.AllowCredentials,
		MaxAge:           12 * time.Hour,
	}
	if len(cfg.AllowOrigins) == 0 || (len(cfg.AllowOrigins) == 1 && cfg.AllowOrigins[0] == "*") {
		if cfg.AllowCredentials {
			c.AllowOriginFunc = func(origin string) bool { return true }
		} else {
			c.AllowAllOrigins = true
		}
	} else {
		c.AllowOrigins = cfg.AllowOrigins
	}
	return cors.New(c)
}
