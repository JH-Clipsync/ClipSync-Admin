package bootstrap

import (
	"errors"

	"github.com/clipsync/admin/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// menuSeed 描述菜单/按钮/数据列的种子数据。
// Code 是稳定唯一键（幂等 upsert），Perms 描述该菜单绑定的接口权限（按 route+method 匹配）。
type menuSeed struct {
	Code     string
	Title    string
	Name     string
	Path     string
	Icon     string
	Type     int8 // 2 左侧菜单，3 操作按钮，5 数据表列
	Sort     int
	Perms    []permSeed
	Children []menuSeed
}

type permSeed struct {
	Name        string
	Route       string
	Method      int8 // 0 ANY 1 GET 2 POST 3 PUT 4 DELETE
	IsIntercept int8 // 默认 1
	Sort        int
}

// method 缩写
const (
	_     int8 = 0 // mAny 保留位
	mGET  int8 = 1
	mPOST int8 = 2
	mPUT  int8 = 3
	mDEL  int8 = 4
)

// 内置菜单树 / 权限树（ClipSync 简化版）
var rbacMenuTree = []menuSeed{
	{
		Code: "dashboard", Title: "首页", Name: "dashboard", Path: "/dashboard",
		Icon: "HomeFilled", Type: 2, Sort: 0,
		Perms: []permSeed{
			{Name: "查看首页数据", Route: "/api/admin/dashboard", Method: mGET, IsIntercept: 0},
		},
	},
	{
		Code: "user-mgmt", Title: "用户管理", Name: "user-mgmt", Path: "/user-mgmt",
		Icon: "User", Type: 2, Sort: 20,
		Children: []menuSeed{
			{
				Code: "biz:users", Title: "用户列表", Name: "users", Path: "/users",
				Type: 2, Sort: 21,
				Perms: []permSeed{
					{Name: "用户列表", Route: "/api/admin/users", Method: mGET, IsIntercept: 1},
					{Name: "用户详情", Route: "/api/admin/users/:id", Method: mGET, IsIntercept: 1},
				},
				Children: []menuSeed{
					{
						Code: "biz:users:create", Title: "新增用户", Type: 3, Sort: 0,
						Perms: []permSeed{{Name: "新增用户", Route: "/api/admin/users", Method: mPOST, IsIntercept: 1}},
					},
					{
						Code: "biz:users:update", Title: "编辑用户", Type: 3, Sort: 1,
						Perms: []permSeed{{Name: "更新用户", Route: "/api/admin/users/:id", Method: mPUT, IsIntercept: 1}},
					},
					{
						Code: "biz:users:status", Title: "启用/禁用用户", Type: 3, Sort: 2,
						Perms: []permSeed{{Name: "更新用户状态", Route: "/api/admin/users/:id/status", Method: mPUT, IsIntercept: 1}},
					},
					{
						Code: "biz:users:reset-pwd", Title: "重置密码", Type: 3, Sort: 3,
						Perms: []permSeed{{Name: "重置密码", Route: "/api/admin/users/:id/reset-password", Method: mPOST, IsIntercept: 1}},
					},
					{
						Code: "biz:users:delete", Title: "删除用户", Type: 3, Sort: 4,
						Perms: []permSeed{{Name: "删除用户", Route: "/api/admin/users/:id", Method: mDEL, IsIntercept: 1}},
					},
					{
						Code: "biz:users:kick", Title: "踢用户下线", Type: 3, Sort: 5,
						Perms: []permSeed{{Name: "踢用户下线", Route: "/api/admin/users/:id/kick", Method: mPOST, IsIntercept: 1}},
					},
					{
						Code: "biz:users:devices", Title: "用户设备列表", Type: 3, Sort: 6,
						Perms: []permSeed{{Name: "用户设备列表", Route: "/api/admin/users/:id/devices", Method: mGET, IsIntercept: 1}},
					},
				},
			},
		},
	},
	{
		Code: "device-mgmt", Title: "设备管理", Name: "device-mgmt", Path: "/device-mgmt",
		Icon: "Cpu", Type: 2, Sort: 30,
		Children: []menuSeed{
			{
				Code: "biz:devices", Title: "设备列表", Name: "devices", Path: "/devices",
				Type: 2, Sort: 31,
				Perms: []permSeed{
					{Name: "设备列表", Route: "/api/admin/devices", Method: mGET, IsIntercept: 1},
					{Name: "更新设备状态", Route: "/api/admin/users/:id/devices/:did", Method: mPUT, IsIntercept: 1},
					{Name: "重命名设备", Route: "/api/admin/users/:id/devices/:did/name", Method: mPUT, IsIntercept: 1},
					{Name: "踢设备下线", Route: "/api/admin/users/:id/devices/:did/kick", Method: mPOST, IsIntercept: 1},
				},
			},
		},
	},
	{
		Code: "rbac", Title: "权限管理", Name: "rbac", Path: "/rbac",
		Icon: "Lock", Type: 2, Sort: 50,
		Children: []menuSeed{
			{
				Code: "rbac:admins", Title: "管理员", Name: "rbac-admins", Path: "/rbac/admins",
				Type: 2, Sort: 51,
				Perms: []permSeed{
					{Name: "管理员列表", Route: "/api/admin/rbac/admins", Method: mGET, IsIntercept: 1},
					{Name: "查看管理员角色", Route: "/api/admin/rbac/admins/:id/roles", Method: mGET, IsIntercept: 1},
				},
				Children: []menuSeed{
					{Code: "rbac:admins:create", Title: "新增管理员", Type: 3, Perms: []permSeed{{Name: "新增管理员", Route: "/api/admin/rbac/admins", Method: mPOST, IsIntercept: 1}}},
					{Code: "rbac:admins:update", Title: "编辑管理员", Type: 3, Perms: []permSeed{{Name: "更新管理员", Route: "/api/admin/rbac/admins/:id", Method: mPUT, IsIntercept: 1}}},
					{Code: "rbac:admins:reset-pass", Title: "重置密码", Type: 3, Perms: []permSeed{{Name: "重置密码", Route: "/api/admin/rbac/admins/:id/password", Method: mPUT, IsIntercept: 1}}},
					{Code: "rbac:admins:delete", Title: "删除管理员", Type: 3, Perms: []permSeed{{Name: "删除管理员", Route: "/api/admin/rbac/admins/:id", Method: mDEL, IsIntercept: 1}}},
				},
			},
			{
				Code: "rbac:roles", Title: "角色", Name: "rbac-roles", Path: "/rbac/roles",
				Type: 2, Sort: 52,
				Perms: []permSeed{
					{Name: "角色列表", Route: "/api/admin/rbac/roles", Method: mGET, IsIntercept: 1},
					{Name: "查看角色菜单", Route: "/api/admin/rbac/roles/:id/menus", Method: mGET, IsIntercept: 1},
				},
				Children: []menuSeed{
					{Code: "rbac:roles:create", Title: "新增角色", Type: 3, Perms: []permSeed{{Name: "新增角色", Route: "/api/admin/rbac/roles", Method: mPOST, IsIntercept: 1}}},
					{Code: "rbac:roles:update", Title: "编辑角色", Type: 3, Perms: []permSeed{{Name: "更新角色", Route: "/api/admin/rbac/roles/:id", Method: mPUT, IsIntercept: 1}}},
					{Code: "rbac:roles:assign-menu", Title: "分配菜单", Type: 3, Perms: []permSeed{{Name: "分配菜单", Route: "/api/admin/rbac/roles/:id/menus", Method: mPUT, IsIntercept: 1}}},
					{Code: "rbac:roles:delete", Title: "删除角色", Type: 3, Perms: []permSeed{{Name: "删除角色", Route: "/api/admin/rbac/roles/:id", Method: mDEL, IsIntercept: 1}}},
				},
			},
			{
				Code: "rbac:menus", Title: "菜单", Name: "rbac-menus", Path: "/rbac/menus",
				Type: 2, Sort: 53,
				Perms: []permSeed{
					{Name: "菜单列表", Route: "/api/admin/rbac/menus", Method: mGET, IsIntercept: 1},
				},
				Children: []menuSeed{
					{Code: "rbac:menus:create", Title: "新增菜单", Type: 3, Perms: []permSeed{{Name: "新增菜单", Route: "/api/admin/rbac/menus", Method: mPOST, IsIntercept: 1}}},
					{Code: "rbac:menus:update", Title: "编辑菜单", Type: 3, Perms: []permSeed{{Name: "更新菜单", Route: "/api/admin/rbac/menus/:id", Method: mPUT, IsIntercept: 1}}},
					{Code: "rbac:menus:assign-perm", Title: "分配权限", Type: 3, Perms: []permSeed{{Name: "分配权限", Route: "/api/admin/rbac/menus/:id/perms", Method: mPUT, IsIntercept: 1}}},
					{Code: "rbac:menus:delete", Title: "删除菜单", Type: 3, Perms: []permSeed{{Name: "删除菜单", Route: "/api/admin/rbac/menus/:id", Method: mDEL, IsIntercept: 1}}},
				},
			},
			{
				Code: "rbac:perms", Title: "权限接口", Name: "rbac-perms", Path: "/rbac/perms",
				Type: 2, Sort: 54,
				Perms: []permSeed{
					{Name: "接口列表", Route: "/api/admin/rbac/perms", Method: mGET, IsIntercept: 1},
				},
				Children: []menuSeed{
					{Code: "rbac:perms:create", Title: "新增接口", Type: 3, Perms: []permSeed{{Name: "新增接口", Route: "/api/admin/rbac/perms", Method: mPOST, IsIntercept: 1}}},
					{Code: "rbac:perms:update", Title: "编辑接口", Type: 3, Perms: []permSeed{{Name: "更新接口", Route: "/api/admin/rbac/perms/:id", Method: mPUT, IsIntercept: 1}}},
					{Code: "rbac:perms:delete", Title: "删除接口", Type: 3, Perms: []permSeed{{Name: "删除接口", Route: "/api/admin/rbac/perms/:id", Method: mDEL, IsIntercept: 1}}},
				},
			},
		},
	},
	{
		Code: "profile", Title: "个人中心", Name: "profile", Path: "/profile",
		Icon: "User", Type: 2, Sort: 90,
		Children: []menuSeed{
			{
				Code: "profile:password", Title: "修改密码", Name: "profile-password", Path: "/profile/password",
				Type: 2, Sort: 91,
				Perms: []permSeed{
					{Name: "修改自己密码", Route: "/api/admin/auth/password", Method: mPUT, IsIntercept: 0},
				},
			},
		},
	},
}

// SeedMenusAndPerms 幂等地初始化菜单/接口权限/菜单-权限绑定。
// 通过 code (菜单) 和 route+method (接口) 匹配已有记录，避免重复。
func SeedMenusAndPerms(db *gorm.DB, lg *zap.Logger) error {
	for _, root := range rbacMenuTree {
		if err := upsertMenu(db, 0, root, lg); err != nil {
			return err
		}
	}
	// 清理不在当前菜单树中的旧菜单（软删除）
	validCodes := collectCodes(rbacMenuTree)
	res := db.Model(&model.RbacMenu{}).
		Where("code NOT IN ? AND is_del = 0", validCodes).
		Update("is_del", 1)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		lg.Info("cleaned old menus", zap.Int64("count", res.RowsAffected))
	}
	return nil
}

func collectCodes(tree []menuSeed) []string {
	var codes []string
	for _, n := range tree {
		codes = append(codes, n.Code)
		codes = append(codes, collectCodes(n.Children)...)
	}
	return codes
}

func upsertMenu(db *gorm.DB, parentID uint64, s menuSeed, lg *zap.Logger) error {
	var m model.RbacMenu
	err := db.Where("code = ? AND is_del = 0", s.Code).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		m = model.RbacMenu{
			Name: s.Name, ParentID: parentID, Path: s.Path, Icon: s.Icon,
			Title: s.Title, Code: s.Code, Type: s.Type,
		}
		m.Sort = s.Sort
		m.Remark = "系统内置"
		if err := db.Create(&m).Error; err != nil {
			return err
		}
		lg.Info("seed menu", zap.String("code", s.Code), zap.Uint64("id", m.ID))
	} else if err != nil {
		return err
	} else {
		// 保持父级/标题最新，其他字段尊重管理员在后台的修改
		updates := map[string]any{
			"parent_id": parentID,
			"title":     s.Title,
			"type":      s.Type,
		}
		if s.Name != "" {
			updates["name"] = s.Name
		}
		if s.Path != "" {
			updates["path"] = s.Path
		}
		if s.Icon != "" {
			updates["icon"] = s.Icon
		}
		if err := db.Model(&m).Updates(updates).Error; err != nil {
			return err
		}
	}

	for _, ps := range s.Perms {
		if err := upsertPermAndBind(db, m.ID, ps, lg); err != nil {
			return err
		}
	}

	for _, child := range s.Children {
		if err := upsertMenu(db, m.ID, child, lg); err != nil {
			return err
		}
	}
	return nil
}

func upsertPermAndBind(db *gorm.DB, menuID uint64, ps permSeed, lg *zap.Logger) error {
	var p model.RbacPerm
	err := db.Where("route = ? AND method = ? AND is_del = 0", ps.Route, ps.Method).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		p = model.RbacPerm{
			Name: ps.Name, Route: ps.Route, Method: ps.Method,
			IsIntercept: ps.IsIntercept,
		}
		p.Sort = ps.Sort
		p.Remark = "系统内置"
		if err := db.Create(&p).Error; err != nil {
			return err
		}
		lg.Info("seed perm", zap.String("route", ps.Route), zap.Int8("method", ps.Method), zap.Uint64("id", p.ID))
	} else if err != nil {
		return err
	} else {
		if err := db.Model(&p).Updates(map[string]any{
			"name":         ps.Name,
			"is_intercept": ps.IsIntercept,
		}).Error; err != nil {
			return err
		}
	}

	var cnt int64
	if err := db.Model(&model.RbacMenuPerm{}).
		Where("menu_id = ? AND perm_id = ? AND is_del = 0", menuID, p.ID).
		Count(&cnt).Error; err != nil {
		return err
	}
	if cnt == 0 {
		mp := &model.RbacMenuPerm{MenuID: menuID, PermID: p.ID}
		mp.Remark = "系统内置"
		if err := db.Create(mp).Error; err != nil {
			return err
		}
	}
	return nil
}
