package model

import "time"

// RBAC 表统一沿用 BaseColumns（remark/status/sort/is_del/c_by/u_by/created_at/updated_at）。
// 表名加 admin_ 前缀，避免与 ClipSync-Server 的 users 表或其他表冲突。

// RbacAdmin —— 后台管理员
type RbacAdmin struct {
	ID                     uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name                   string     `gorm:"size:50" json:"name"`
	Account                string     `gorm:"uniqueIndex;size:60" json:"account"`
	Password               string     `gorm:"size:100" json:"-"`
	IsLock                 int8       `gorm:"default:0" json:"isLock"`
	Avatar                 string     `gorm:"size:256;not null;default:''" json:"avatar"`
	ValidityPeriod         *time.Time `json:"validityPeriod,omitempty"`
	LastUpdatePasswordTime *time.Time `json:"lastUpdatePasswordTime,omitempty"`
	LastLoginTime          *time.Time `json:"lastLoginTime,omitempty"`
	BaseColumns
}

func (RbacAdmin) TableName() string { return "admin_rbac_admin" }

// RbacRole —— 角色
type RbacRole struct {
	ID   uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	Name string `gorm:"size:60" json:"name"`
	Type int8   `gorm:"default:0" json:"type"` // 0正常 1超级管理员
	BaseColumns
}

func (RbacRole) TableName() string { return "admin_rbac_role" }

// RbacAdminRole —— 管理员 & 角色关系
type RbacAdminRole struct {
	ID      uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	AdminID uint64 `gorm:"index:idx_admin_role;default:0" json:"adminId"`
	RoleID  uint64 `gorm:"index:idx_admin_role;default:0" json:"roleId"`
	BaseColumns
}

func (RbacAdminRole) TableName() string { return "admin_rbac_admin_role" }

// RbacMenu —— 菜单/按钮/数据列
type RbacMenu struct {
	ID                 uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	Name               string `gorm:"size:60" json:"name"`
	ParentID           uint64 `gorm:"index;default:0" json:"parentId"`
	Path               string `gorm:"size:200" json:"path"`
	Icon               string `gorm:"size:300" json:"icon"`
	IsLink             int8   `gorm:"default:0" json:"isLink"`
	Title              string `gorm:"size:100" json:"title"`
	Code               string `gorm:"index;size:100" json:"code"`
	Include            string `gorm:"size:500" json:"include"`
	Type               int8   `gorm:"default:0" json:"type"`
	FieldValueKey      string `gorm:"size:100" json:"fieldValueKey"`
	FieldValueWidth    string `gorm:"size:100" json:"fieldValueWidth"`
	FieldValueEllipsis int8   `gorm:"default:0" json:"fieldValueEllipsis"`
	BaseColumns
}

func (RbacMenu) TableName() string { return "admin_rbac_menu" }

// RbacPerm —— 接口权限
// 通过 route + method 组合唯一定位（is_del 参与查询，避免"逻辑删+重建"冲突）。
type RbacPerm struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string `gorm:"size:60" json:"name"`
	ParentID    uint64 `gorm:"index;default:0" json:"parentId"`
	Method      int8   `gorm:"index:idx_perm_route_method;default:2" json:"method"`
	Route       string `gorm:"index:idx_perm_route_method;size:100" json:"route"`
	IsIntercept int8   `gorm:"default:1" json:"isIntercept"`
	BaseColumns
}

func (RbacPerm) TableName() string { return "admin_rbac_perm" }

// RbacRoleMenu —— 角色 & 菜单
type RbacRoleMenu struct {
	ID     uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	RoleID uint64 `gorm:"index:idx_role_menu;default:0" json:"roleId"`
	MenuID uint64 `gorm:"index:idx_role_menu;default:0" json:"menuId"`
	BaseColumns
}

func (RbacRoleMenu) TableName() string { return "admin_rbac_role_menu" }

// RbacMenuPerm —— 菜单 & 权限
type RbacMenuPerm struct {
	ID     uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	MenuID uint64 `gorm:"index:idx_menu_perm;default:0" json:"menuId"`
	PermID uint64 `gorm:"index:idx_menu_perm;default:0" json:"permId"`
	BaseColumns
}

func (RbacMenuPerm) TableName() string { return "admin_rbac_menu_perm" }
