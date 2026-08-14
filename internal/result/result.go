package result

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Result struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Result{
		Code:    CodeSuccess,
		Message: MessageOf(CodeSuccess),
		Data:    data,
	})
}

func Fail(c *gin.Context, code int) {
	c.JSON(http.StatusOK, Result{
		Code:    code,
		Message: prefix(code, MessageOf(code)),
	})
}

func FailWith(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, Result{
		Code:    code,
		Message: prefix(code, msg),
	})
}

func prefix(code int, msg string) string {
	switch {
	case code >= 1200 && code <= 1299:
		return msg
	case code >= 1400 && code <= 1499:
		return "抱歉！" + msg
	case code >= 1500 && code <= 1599:
		return "抱歉！系统异常，" + msg
	default:
		return msg
	}
}
