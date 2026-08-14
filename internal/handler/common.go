package handler

import (
	"strconv"

	"github.com/clipsync/admin/internal/auth"
	"github.com/clipsync/admin/internal/middleware"
	"github.com/clipsync/admin/internal/result"
	"github.com/clipsync/admin/internal/service"
	"github.com/gin-gonic/gin"
)

// currentAdmin extracts claims from context.
func currentAdmin(c *gin.Context) *auth.Claims {
	if v, ok := c.Get(middleware.ContextAdminKey); ok {
		if cl, ok := v.(*auth.Claims); ok {
			return cl
		}
	}
	return nil
}

// bindOrFail binds JSON to obj; returns false if bind fails and responds.
func bindOrFail(c *gin.Context, obj any) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		result.FailWith(c, result.CodeParamError, err.Error())
		return false
	}
	return true
}

// respBiz maps a service error into HTTP.
func respBiz(c *gin.Context, err error) {
	if be, ok := service.AsBiz(err); ok {
		result.FailWith(c, be.Code, be.Message)
		return
	}
	result.FailWith(c, result.CodeInternalError, err.Error())
}

// intQ reads an int query param (default d).
func intQ(c *gin.Context, key string, d int) int {
	s := c.Query(key)
	if s == "" {
		return d
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return d
	}
	return n
}

// uintP reads a uint64 URL param.
func uintP(c *gin.Context, key string) uint64 {
	s := c.Param(key)
	n, _ := strconv.ParseUint(s, 10, 64)
	return n
}

// int64P reads an int64 URL param（ClipSync User.ID 是 int64）。
func int64P(c *gin.Context, key string) int64 {
	s := c.Param(key)
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

// uintQ reads a uint64 query param.
func uintQ(c *gin.Context, key string) uint64 {
	s := c.Query(key)
	n, _ := strconv.ParseUint(s, 10, 64)
	return n
}
