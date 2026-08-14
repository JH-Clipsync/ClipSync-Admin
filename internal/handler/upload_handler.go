package handler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/clipsync/admin/internal/config"
	"github.com/clipsync/admin/internal/result"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// UploadHandler 处理图片上传
type UploadHandler struct {
	cfg config.UploadConfig
}

func NewUploadHandler(cfg config.UploadConfig) *UploadHandler {
	if cfg.Dir == "" {
		cfg.Dir = "./data/uploads"
	}
	if cfg.URLPrefix == "" {
		cfg.URLPrefix = "/static"
	}
	if cfg.MaxSize <= 0 {
		cfg.MaxSize = 10 * 1024 * 1024
	}
	if len(cfg.AllowExt) == 0 {
		cfg.AllowExt = []string{".jpg", ".jpeg", ".png", ".webp", ".gif"}
	}
	// 归一化允许列表：全小写、带点
	normalized := make([]string, 0, len(cfg.AllowExt))
	for _, e := range cfg.AllowExt {
		e = strings.ToLower(strings.TrimSpace(e))
		if e == "" {
			continue
		}
		if !strings.HasPrefix(e, ".") {
			e = "." + e
		}
		normalized = append(normalized, e)
	}
	cfg.AllowExt = normalized
	return &UploadHandler{cfg: cfg}
}

// Image POST /api/admin/upload/image
// 表单字段名：file
// 成功返回：{ url, filename, size }
func (h *UploadHandler) Image(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		result.FailWith(c, result.CodeParamError, "缺少上传文件字段 file")
		return
	}
	if fh.Size <= 0 {
		result.FailWith(c, result.CodeParamError, "空文件")
		return
	}
	if h.cfg.MaxSize > 0 && fh.Size > h.cfg.MaxSize {
		result.FailWith(c, result.CodeParamError, fmt.Sprintf("文件大小超过限制 %d 字节", h.cfg.MaxSize))
		return
	}
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	if !h.extAllowed(ext) {
		result.FailWith(c, result.CodeParamError,
			fmt.Sprintf("不支持的文件类型 %q，允许：%s", ext, strings.Join(h.cfg.AllowExt, ",")))
		return
	}

	now := time.Now()
	subDir := filepath.Join(fmt.Sprintf("%04d", now.Year()),
		fmt.Sprintf("%02d", int(now.Month())),
		fmt.Sprintf("%02d", now.Day()))
	diskDir := filepath.Join(h.cfg.Dir, subDir)
	if err := os.MkdirAll(diskDir, 0o755); err != nil {
		result.FailWith(c, result.CodeInternalError, "创建上传目录失败: "+err.Error())
		return
	}
	name := uuid.NewString() + ext
	diskPath := filepath.Join(diskDir, name)
	if err := c.SaveUploadedFile(fh, diskPath); err != nil {
		result.FailWith(c, result.CodeInternalError, "保存文件失败: "+err.Error())
		return
	}

	// 拼接 URL：url_prefix + / + subDir + / + name，全部用正斜杠
	relative := filepath.ToSlash(filepath.Join(subDir, name))
	url := strings.TrimRight(h.cfg.URLPrefix, "/") + "/" + relative
	result.Success(c, gin.H{
		"url":      url,
		"filename": name,
		"size":     fh.Size,
	})
}

func (h *UploadHandler) extAllowed(ext string) bool {
	for _, e := range h.cfg.AllowExt {
		if e == ext {
			return true
		}
	}
	return false
}
