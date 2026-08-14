package service

import (
	"context"
	"errors"

	"github.com/clipsync/admin/internal/auth"
	"github.com/clipsync/admin/internal/model"
	"github.com/clipsync/admin/internal/result"
	"gorm.io/gorm"
)

// RBACService owns admin/role/menu/perm CRUD.
type RBACService struct {
	db           *gorm.DB
	bcryptCo     int
	superAccount string // 内置超级管理员账号，不允许删除
}

func NewRBACService(db *gorm.DB, bcryptCost int, superAccount string) *RBACService {
	return &RBACService{db: db, bcryptCo: bcryptCost, superAccount: superAccount}
}

// notDel = is_del = 0
func notDel(q *gorm.DB) *gorm.DB { return q.Where("is_del = 0") }

// -------- Admins --------

func (s *RBACService) ListAdmins(ctx context.Context, keyword string, page, size int) (*Page, error) {
	page, size = normalize(page, size)
	q := notDel(s.db.WithContext(ctx).Model(&model.RbacAdmin{}))
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("account LIKE ? OR name LIKE ?", like, like)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, biz(result.CodeDBError, err.Error())
	}
	var list []model.RbacAdmin
	if err := q.Order("id DESC, created_at DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, biz(result.CodeDBError, err.Error())
	}
	return &Page{List: list, Total: total, Page: page, PageSize: size}, nil
}

func (s *RBACService) CreateAdmin(ctx context.Context, in *model.RbacAdmin, plainPass string, roleIDs []uint64, operator uint64) error {
	if in.Account == "" || plainPass == "" {
		return biz(result.CodeParamError, "账号或密码不能为空")
	}
	var cnt int64
	if err := notDel(s.db.WithContext(ctx).Model(&model.RbacAdmin{})).
		Where("account = ?", in.Account).Count(&cnt).Error; err != nil {
		return biz(result.CodeDBError, err.Error())
	}
	if cnt > 0 {
		return biz(result.CodeAccountExists, result.MessageOf(result.CodeAccountExists))
	}
	hash, err := auth.HashPassword(plainPass, s.bcryptCo)
	if err != nil {
		return biz(result.CodeInternalError, err.Error())
	}
	in.Password = hash
	in.CBy = operator
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(in).Error; err != nil {
			return biz(result.CodeDBError, err.Error())
		}
		for _, rid := range roleIDs {
			if rid == 0 {
				continue
			}
			ar := &model.RbacAdminRole{AdminID: in.ID, RoleID: rid}
			ar.CBy = operator
			if err := tx.Create(ar).Error; err != nil {
				return biz(result.CodeDBError, err.Error())
			}
		}
		return nil
	})
}

func (s *RBACService) UpdateAdmin(ctx context.Context, id uint64, in *model.RbacAdmin, roleIDs []uint64, operator uint64) error {
	if id == 0 {
		return biz(result.CodeParamError, "id 不能为空")
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.RbacAdmin{}).
			Where("id = ? AND is_del = 0", id).
			Updates(map[string]any{
				"name":    in.Name,
				"avatar":  in.Avatar,
				"status":  in.Status,
				"is_lock": in.IsLock,
				"remark":  in.Remark,
				"u_by":    operator,
			}).Error; err != nil {
			return biz(result.CodeDBError, err.Error())
		}
		if roleIDs != nil {
			if err := tx.Model(&model.RbacAdminRole{}).
				Where("admin_id = ?", id).
				Update("is_del", 1).Error; err != nil {
				return biz(result.CodeDBError, err.Error())
			}
			for _, rid := range roleIDs {
				if rid == 0 {
					continue
				}
				ar := &model.RbacAdminRole{AdminID: id, RoleID: rid}
				ar.CBy = operator
				if err := tx.Create(ar).Error; err != nil {
					return biz(result.CodeDBError, err.Error())
				}
			}
		}
		return nil
	})
}

func (s *RBACService) ResetAdminPassword(ctx context.Context, id uint64, plainPass string, operator uint64) error {
	if plainPass == "" {
		return biz(result.CodeParamError, "新密码不能为空")
	}
	hash, err := auth.HashPassword(plainPass, s.bcryptCo)
	if err != nil {
		return biz(result.CodeInternalError, err.Error())
	}
	res := s.db.WithContext(ctx).Model(&model.RbacAdmin{}).
		Where("id = ? AND is_del = 0", id).
		Updates(map[string]any{
			"password": hash,
			"u_by":     operator,
		})
	if res.Error != nil {
		return biz(result.CodeDBError, res.Error.Error())
	}
	if res.RowsAffected == 0 {
		return biz(result.CodeAccountNotFound, result.MessageOf(result.CodeAccountNotFound))
	}
	return nil
}

func (s *RBACService) DeleteAdmin(ctx context.Context, id uint64, operator uint64) error {
	// 保护内置超级管理员：既不允许自删，也不允许删除 super_admin_account
	if id == 0 {
		return biz(result.CodeParamError, "id 不能为空")
	}
	if id == operator {
		return biz(result.CodeParamError, "不能删除当前登录账号")
	}
	var target model.RbacAdmin
	if err := s.db.WithContext(ctx).Select("id", "account").
		Where("id = ? AND is_del = 0", id).First(&target).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return biz(result.CodeAccountNotFound, result.MessageOf(result.CodeAccountNotFound))
		}
		return biz(result.CodeDBError, err.Error())
	}
	if s.superAccount != "" && target.Account == s.superAccount {
		return biz(result.CodeAccessDenied, "内置超级管理员不可删除")
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.RbacAdmin{}).Where("id = ?", id).
			Updates(map[string]any{"is_del": 1, "u_by": operator}).Error; err != nil {
			return biz(result.CodeDBError, err.Error())
		}
		if err := tx.Model(&model.RbacAdminRole{}).
			Where("admin_id = ?", id).
			Update("is_del", 1).Error; err != nil {
			return biz(result.CodeDBError, err.Error())
		}
		return nil
	})
}

func (s *RBACService) UpdateAdminStatus(ctx context.Context, id uint64, status int, operator uint64) error {
	if id == 0 {
		return biz(result.CodeParamError, "id 不能为空")
	}
	var target model.RbacAdmin
	if err := s.db.WithContext(ctx).Select("id", "account").
		Where("id = ? AND is_del = 0", id).First(&target).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return biz(result.CodeAccountNotFound, result.MessageOf(result.CodeAccountNotFound))
		}
		return biz(result.CodeDBError, err.Error())
	}
	if s.superAccount != "" && target.Account == s.superAccount {
		return biz(result.CodeAccessDenied, "内置超级管理员状态不可变更")
	}
	res := s.db.WithContext(ctx).Model(&model.RbacAdmin{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": status, "u_by": operator})
	if res.Error != nil {
		return biz(result.CodeDBError, res.Error.Error())
	}
	return nil
}

func (s *RBACService) AdminRoleIDs(ctx context.Context, adminID uint64) ([]uint64, error) {
	var ids []uint64
	err := notDel(s.db.WithContext(ctx).Model(&model.RbacAdminRole{})).
		Where("admin_id = ?", adminID).Pluck("role_id", &ids).Error
	if err != nil {
		return nil, biz(result.CodeDBError, err.Error())
	}
	return ids, nil
}

// -------- Roles --------

func (s *RBACService) ListRoles(ctx context.Context) ([]model.RbacRole, error) {
	var list []model.RbacRole
	if err := notDel(s.db.WithContext(ctx)).Order("sort ASC, id ASC").Find(&list).Error; err != nil {
		return nil, biz(result.CodeDBError, err.Error())
	}
	return list, nil
}

func (s *RBACService) CreateRole(ctx context.Context, in *model.RbacRole, operator uint64) error {
	if in.Name == "" {
		return biz(result.CodeParamError, "角色名称不能为空")
	}
	in.CBy = operator
	return s.db.WithContext(ctx).Create(in).Error
}

func (s *RBACService) UpdateRole(ctx context.Context, id uint64, in *model.RbacRole, operator uint64) error {
	res := s.db.WithContext(ctx).Model(&model.RbacRole{}).
		Where("id = ? AND is_del = 0", id).
		Updates(map[string]any{
			"name":   in.Name,
			"remark": in.Remark,
			"type":   in.Type,
			"status": in.Status,
			"sort":   in.Sort,
			"u_by":   operator,
		})
	if res.Error != nil {
		return biz(result.CodeDBError, res.Error.Error())
	}
	if res.RowsAffected == 0 {
		return biz(result.CodeRecordNotFound, "角色不存在")
	}
	return nil
}

func (s *RBACService) DeleteRole(ctx context.Context, id uint64, operator uint64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.RbacRole{}).Where("id = ?", id).
			Updates(map[string]any{"is_del": 1, "u_by": operator}).Error; err != nil {
			return biz(result.CodeDBError, err.Error())
		}
		if err := tx.Model(&model.RbacAdminRole{}).Where("role_id = ?", id).
			Update("is_del", 1).Error; err != nil {
			return biz(result.CodeDBError, err.Error())
		}
		if err := tx.Model(&model.RbacRoleMenu{}).Where("role_id = ?", id).
			Update("is_del", 1).Error; err != nil {
			return biz(result.CodeDBError, err.Error())
		}
		return nil
	})
}

// AssignRoleMenus overwrites role<->menu bindings.
func (s *RBACService) AssignRoleMenus(ctx context.Context, roleID uint64, menuIDs []uint64, operator uint64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.RbacRoleMenu{}).Where("role_id = ?", roleID).
			Update("is_del", 1).Error; err != nil {
			return biz(result.CodeDBError, err.Error())
		}
		for _, mid := range menuIDs {
			if mid == 0 {
				continue
			}
			rm := &model.RbacRoleMenu{RoleID: roleID, MenuID: mid}
			rm.CBy = operator
			if err := tx.Create(rm).Error; err != nil {
				return biz(result.CodeDBError, err.Error())
			}
		}
		return nil
	})
}

func (s *RBACService) RoleMenuIDs(ctx context.Context, roleID uint64) ([]uint64, error) {
	var ids []uint64
	err := notDel(s.db.WithContext(ctx).Model(&model.RbacRoleMenu{})).
		Where("role_id = ?", roleID).Pluck("menu_id", &ids).Error
	if err != nil {
		return nil, biz(result.CodeDBError, err.Error())
	}
	return ids, nil
}

// -------- Menus --------

func (s *RBACService) ListMenus(ctx context.Context) ([]model.RbacMenu, error) {
	var list []model.RbacMenu
	if err := notDel(s.db.WithContext(ctx)).Order("sort ASC, id ASC").Find(&list).Error; err != nil {
		return nil, biz(result.CodeDBError, err.Error())
	}
	return list, nil
}

func (s *RBACService) CreateMenu(ctx context.Context, in *model.RbacMenu, operator uint64) error {
	if in.Name == "" && in.Title == "" {
		return biz(result.CodeParamError, "菜单名称/标题不能为空")
	}
	in.CBy = operator
	return s.db.WithContext(ctx).Create(in).Error
}

// UpdateMenu 只更新 raw 里明确传了的字段（其他字段保持数据库现值不变）。
// 这样前端拖拽排序只传 {parentId, sort} 时不会把 title/name/path 等清空。
// 允许更新的字段白名单：仅业务字段，禁止越权改 id/is_del/c_by/created_at 等。
func (s *RBACService) UpdateMenu(ctx context.Context, id uint64, raw map[string]any, operator uint64) error {
	allow := map[string]string{
		// 前端字段（camelCase） → 数据库列（snake_case）
		"name":               "name",
		"parentId":           "parent_id",
		"path":               "path",
		"icon":               "icon",
		"isLink":             "is_link",
		"title":              "title",
		"code":               "code",
		"include":            "include",
		"type":               "type",
		"fieldValueKey":      "field_value_key",
		"fieldValueWidth":    "field_value_width",
		"fieldValueEllipsis": "field_value_ellipsis",
		"remark":             "remark",
		"status":             "status",
		"sort":               "sort",
	}
	updates := map[string]any{"u_by": operator}
	for k, v := range raw {
		if col, ok := allow[k]; ok {
			updates[col] = v
		}
	}
	if len(updates) == 1 {
		// 只有 u_by，说明前端啥都没传
		return biz(result.CodeParamError, "请求体为空")
	}
	res := s.db.WithContext(ctx).Model(&model.RbacMenu{}).
		Where("id = ? AND is_del = 0", id).
		Updates(updates)
	if res.Error != nil {
		return biz(result.CodeDBError, res.Error.Error())
	}
	if res.RowsAffected == 0 {
		return biz(result.CodeRecordNotFound, "菜单不存在")
	}
	return nil
}

func (s *RBACService) DeleteMenu(ctx context.Context, id uint64, operator uint64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.RbacMenu{}).Where("id = ?", id).
			Updates(map[string]any{"is_del": 1, "u_by": operator}).Error; err != nil {
			return biz(result.CodeDBError, err.Error())
		}
		if err := tx.Model(&model.RbacRoleMenu{}).Where("menu_id = ?", id).
			Update("is_del", 1).Error; err != nil {
			return biz(result.CodeDBError, err.Error())
		}
		if err := tx.Model(&model.RbacMenuPerm{}).Where("menu_id = ?", id).
			Update("is_del", 1).Error; err != nil {
			return biz(result.CodeDBError, err.Error())
		}
		return nil
	})
}

// AssignMenuPerms overwrites menu<->perm bindings.
func (s *RBACService) AssignMenuPerms(ctx context.Context, menuID uint64, permIDs []uint64, operator uint64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.RbacMenuPerm{}).Where("menu_id = ?", menuID).
			Update("is_del", 1).Error; err != nil {
			return biz(result.CodeDBError, err.Error())
		}
		for _, pid := range permIDs {
			if pid == 0 {
				continue
			}
			mp := &model.RbacMenuPerm{MenuID: menuID, PermID: pid}
			mp.CBy = operator
			if err := tx.Create(mp).Error; err != nil {
				return biz(result.CodeDBError, err.Error())
			}
		}
		return nil
	})
}

// -------- Perms --------

func (s *RBACService) ListPerms(ctx context.Context, keyword string, page, size int) (*Page, error) {
	page, size = normalize(page, size)
	q := notDel(s.db.WithContext(ctx).Model(&model.RbacPerm{}))
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("name LIKE ? OR route LIKE ?", like, like)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, biz(result.CodeDBError, err.Error())
	}
	var list []model.RbacPerm
	if err := q.Order("id DESC, created_at DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, biz(result.CodeDBError, err.Error())
	}
	return &Page{List: list, Total: total, Page: page, PageSize: size}, nil
}

func (s *RBACService) CreatePerm(ctx context.Context, in *model.RbacPerm, operator uint64) error {
	if in.Route == "" {
		return biz(result.CodeParamError, "路由不能为空")
	}
	in.CBy = operator
	return s.db.WithContext(ctx).Create(in).Error
}

func (s *RBACService) UpdatePerm(ctx context.Context, id uint64, in *model.RbacPerm, operator uint64) error {
	res := s.db.WithContext(ctx).Model(&model.RbacPerm{}).
		Where("id = ? AND is_del = 0", id).
		Updates(map[string]any{
			"name":         in.Name,
			"parent_id":    in.ParentID,
			"method":       in.Method,
			"route":        in.Route,
			"is_intercept": in.IsIntercept,
			"remark":       in.Remark,
			"status":       in.Status,
			"sort":         in.Sort,
			"u_by":         operator,
		})
	if res.Error != nil {
		return biz(result.CodeDBError, res.Error.Error())
	}
	if res.RowsAffected == 0 {
		return biz(result.CodeRecordNotFound, "权限不存在")
	}
	return nil
}

func (s *RBACService) DeletePerm(ctx context.Context, id uint64, operator uint64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.RbacPerm{}).Where("id = ?", id).
			Updates(map[string]any{"is_del": 1, "u_by": operator}).Error; err != nil {
			return biz(result.CodeDBError, err.Error())
		}
		if err := tx.Model(&model.RbacMenuPerm{}).Where("perm_id = ?", id).
			Update("is_del", 1).Error; err != nil {
			return biz(result.CodeDBError, err.Error())
		}
		return nil
	})
}
