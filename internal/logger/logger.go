package logger

import (
	"github.com/clipsync/admin/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func New(cfg config.LogConfig) (*zap.Logger, error) {
	lvl, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		lvl = zapcore.InfoLevel
	}
	zc := zap.NewProductionConfig()
	zc.Level = zap.NewAtomicLevelAt(lvl)
	if cfg.Format == "console" {
		zc.Encoding = "console"
		zc.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}
	return zc.Build(zap.AddCaller())
}
