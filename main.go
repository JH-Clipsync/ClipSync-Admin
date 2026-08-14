package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/clipsync/admin/internal/auth"
	"github.com/clipsync/admin/internal/bootstrap"
	"github.com/clipsync/admin/internal/config"
	"github.com/clipsync/admin/internal/db"
	"github.com/clipsync/admin/internal/logger"
	"github.com/clipsync/admin/internal/router"
	"go.uber.org/zap"
)

func main() {
	cfgPath := flag.String("c", "", "path to config.yaml (default: ./config.yaml)")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	lg, err := logger.New(cfg.Log)
	if err != nil {
		log.Fatalf("init logger: %v", err)
	}
	defer func() { _ = lg.Sync() }()

	gdb, err := db.NewMySQL(cfg.MySQL)
	if err != nil {
		lg.Fatal("connect mysql", zap.Error(err))
	}
	rdb, err := db.NewRedis(cfg.Redis)
	if err != nil {
		lg.Fatal("connect redis", zap.Error(err))
	}

	// 仅迁移 RBAC 表（users 表由 ClipSync-Server 管理，不在此迁移）
	if err := bootstrap.Migrate(gdb); err != nil {
		lg.Fatal("auto migrate rbac tables", zap.Error(err))
	}
	if err := bootstrap.SeedSuperAdmin(gdb, cfg.Bootstrap, cfg.Security.BcryptCost, lg); err != nil {
		lg.Fatal("seed super admin", zap.Error(err))
	}
	if err := bootstrap.SeedMenusAndPerms(gdb, lg); err != nil {
		lg.Fatal("seed menus & perms", zap.Error(err))
	}

	jwtMgr := auth.NewManager(cfg.JWT.Secret, cfg.JWT.TTLDuration())
	r := router.New(cfg, lg, gdb, rdb, jwtMgr)

	srv := &http.Server{
		Addr:              cfg.App.Addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		lg.Info("admin http server starting", zap.String("addr", cfg.App.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			lg.Fatal("listen", zap.Error(err))
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	lg.Info("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		lg.Error("shutdown", zap.Error(err))
	}
	_ = rdb.Close()
}
