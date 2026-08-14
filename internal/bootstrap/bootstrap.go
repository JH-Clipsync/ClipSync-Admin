package bootstrap

import (
	"errors"
	"time"

	"github.com/clipsync/admin/internal/auth"
	"github.com/clipsync/admin/internal/config"
	"github.com/clipsync/admin/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Migrate runs GORM AutoMigrate for all RBAC tables.
// users 表由 ClipSync-Server 管理，不在此迁移，避免冲突。
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.RbacAdmin{},
		&model.RbacRole{},
		&model.RbacAdminRole{},
		&model.RbacMenu{},
		&model.RbacPerm{},
		&model.RbacRoleMenu{},
		&model.RbacMenuPerm{},
	)
}

// SeedSuperAdmin ensures a super-admin exists: a role of type=1 and an admin
// account (default admin/Admin**8). Idempotent.
func SeedSuperAdmin(db *gorm.DB, cfg config.BootstrapConfig, secCost int, lg *zap.Logger) error {
	if cfg.SuperAdminAccount == "" {
		cfg.SuperAdminAccount = "admin"
	}
	if cfg.SuperAdminPassword == "" {
		cfg.SuperAdminPassword = "Admin**8"
	}
	if cfg.SuperAdminName == "" {
		cfg.SuperAdminName = "超级管理员"
	}

	var role model.RbacRole
	err := db.Where("type = ? AND is_del = 0", 1).First(&role).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		role = model.RbacRole{
			Name: cfg.SuperAdminName,
			Type: 1,
		}
		role.Remark = "系统内置：拥有全部权限"
		if err := db.Create(&role).Error; err != nil {
			return err
		}
		lg.Info("seed super role created", zap.Uint64("roleId", role.ID))
	} else if err != nil {
		return err
	}

	var admin model.RbacAdmin
	err = db.Where("account = ? AND is_del = 0", cfg.SuperAdminAccount).First(&admin).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		hash, err := auth.HashPassword(cfg.SuperAdminPassword, secCost)
		if err != nil {
			return err
		}
		now := time.Now()
		admin = model.RbacAdmin{
			Name:                   cfg.SuperAdminName,
			Account:                cfg.SuperAdminAccount,
			Password:               hash,
			IsLock:                 0,
			LastUpdatePasswordTime: &now,
		}
		admin.Remark = "系统内置超级管理员"
		if err := db.Create(&admin).Error; err != nil {
			return err
		}
		lg.Info("seed super admin created",
			zap.Uint64("adminId", admin.ID),
			zap.String("account", admin.Account),
		)
	} else if err != nil {
		return err
	}

	// bind admin<->role
	var cnt int64
	if err := db.Model(&model.RbacAdminRole{}).
		Where("admin_id = ? AND role_id = ? AND is_del = 0", admin.ID, role.ID).
		Count(&cnt).Error; err != nil {
		return err
	}
	if cnt == 0 {
		bind := &model.RbacAdminRole{
			AdminID: admin.ID,
			RoleID:  role.ID,
		}
		bind.Remark = "系统内置绑定"
		if err := db.Create(bind).Error; err != nil {
			return err
		}
		lg.Info("bind super admin<->role",
			zap.Uint64("adminId", admin.ID),
			zap.Uint64("roleId", role.ID),
		)
	}
	return nil
}
