package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/clipsync/admin/internal/auth"
	"github.com/clipsync/admin/internal/config"
	"github.com/clipsync/admin/internal/model"
	"github.com/clipsync/admin/internal/result"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// bizError carries a subport-style code + message across layers.
type bizError struct {
	Code    int
	Message string
}

func (e *bizError) Error() string { return e.Message }

func biz(code int, msg string) error {
	return &bizError{Code: code, Message: msg}
}

// AsBiz extracts *bizError if any.
func AsBiz(err error) (*bizError, bool) {
	var be *bizError
	if errors.As(err, &be) {
		return be, true
	}
	return nil, false
}

// tokenKey returns Redis key for a valid admin JWT jti.
func tokenKey(jti string) string { return "admin:token:" + jti }

// loginErrKey returns Redis key for the login error counter.
func loginErrKey(account string) string { return "admin:login_err:" + account }

// signSecretKey returns Redis key for the admin session sign secret.
func signSecretKey(adminID uint64) string {
	return fmt.Sprintf("admin:sign_secret:%d", adminID)
}

// scopeActive: status = 0 AND is_del = 0
func scopeActive(q *gorm.DB) *gorm.DB {
	return q.Where("status = ? AND is_del = ?", 0, 0)
}

// AuthService owns admin login/logout/me.
type AuthService struct {
	cfg    config.Config
	db     *gorm.DB
	rdb    *redis.Client
	jwtMgr *auth.Manager
}

func NewAuthService(cfg config.Config, db *gorm.DB, rdb *redis.Client, mgr *auth.Manager) *AuthService {
	return &AuthService{cfg: cfg, db: db, rdb: rdb, jwtMgr: mgr}
}

// BuiltinSuperAccount 返回内置超级管理员账号（用于前端隐藏敏感操作）。
func (s *AuthService) BuiltinSuperAccount() string {
	return s.cfg.Bootstrap.SuperAdminAccount
}

// Login checks credentials, guards against brute force, issues a JWT, and
// stores its jti in Redis (subport-style revocable token).
func (s *AuthService) Login(ctx context.Context, account, password string) (string, *model.RbacAdmin, string, error) {
	account = strings.TrimSpace(account)
	if account == "" || password == "" {
		return "", nil, "", biz(result.CodeParamError, "账号或密码为空")
	}

	// login error limit
	errCnt, _ := s.rdb.Get(ctx, loginErrKey(account)).Int()
	if s.cfg.Security.LoginErrorLimit > 0 && errCnt >= s.cfg.Security.LoginErrorLimit {
		return "", nil, "", biz(result.CodeLoginErrorLimit, result.MessageOf(result.CodeLoginErrorLimit))
	}

	var admin model.RbacAdmin
	err := s.db.WithContext(ctx).Where("account = ? AND is_del = 0", account).First(&admin).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		s.bumpLoginErr(ctx, account)
		return "", nil, "", biz(result.CodeAuthFail, result.MessageOf(result.CodeAuthFail))
	}
	if err != nil {
		return "", nil, "", biz(result.CodeDBError, err.Error())
	}
	if admin.Status == 1 {
		return "", nil, "", biz(result.CodeAccountDisable, result.MessageOf(result.CodeAccountDisable))
	}
	if admin.IsLock == 1 {
		return "", nil, "", biz(result.CodeAccountLocked, result.MessageOf(result.CodeAccountLocked))
	}
	if !auth.CheckPassword(password, admin.Password) {
		s.bumpLoginErr(ctx, account)
		return "", nil, "", biz(result.CodeAuthFail, result.MessageOf(result.CodeAuthFail))
	}
	_ = s.rdb.Del(ctx, loginErrKey(account)).Err()

	tok, claims, err := s.jwtMgr.Generate(admin.ID, admin.Account)
	if err != nil {
		return "", nil, "", biz(result.CodeInternalError, err.Error())
	}
	if err := s.rdb.Set(ctx, tokenKey(claims.ID), admin.ID, s.jwtMgr.TTL()).Err(); err != nil {
		return "", nil, "", biz(result.CodeCacheError, err.Error())
	}
	now := time.Now()
	s.db.WithContext(ctx).Model(&admin).Update("last_login_time", now)

	// 生成会话签名密钥下发给前端，TTL 与 JWT 一致
	signSecret, err := s.IssueSignSecret(ctx, admin.ID)
	if err != nil {
		return "", nil, "", biz(result.CodeCacheError, err.Error())
	}
	return tok, &admin, signSecret, nil
}

func (s *AuthService) bumpLoginErr(ctx context.Context, account string) {
	ttl := time.Duration(s.cfg.Security.LoginErrorTTL) * time.Second
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	key := loginErrKey(account)
	n, _ := s.rdb.Incr(ctx, key).Result()
	if n == 1 {
		_ = s.rdb.Expire(ctx, key, ttl).Err()
	}
}

// Logout invalidates a jti in Redis and clears the session sign secret.
func (s *AuthService) Logout(ctx context.Context, jti string, adminID uint64) error {
	_ = s.ClearSignSecret(ctx, adminID)
	return s.rdb.Del(ctx, tokenKey(jti)).Err()
}

// IsTokenActive returns true if the jti is still whitelisted in Redis.
func (s *AuthService) IsTokenActive(ctx context.Context, jti string) (bool, error) {
	n, err := s.rdb.Exists(ctx, tokenKey(jti)).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// RefreshToken extends jti TTL if refresh_on_access is on.
func (s *AuthService) RefreshToken(ctx context.Context, jti string) {
	if !s.cfg.JWT.RefreshOnAccess {
		return
	}
	_ = s.rdb.Expire(ctx, tokenKey(jti), s.jwtMgr.TTL()).Err()
}

// IssueSignSecret 生成并存储会话签名密钥，TTL 与 JWT 一致。
func (s *AuthService) IssueSignSecret(ctx context.Context, adminID uint64) (string, error) {
	secret, err := auth.GenSignSecret()
	if err != nil {
		return "", err
	}
	if err := s.rdb.Set(ctx, signSecretKey(adminID), secret, s.jwtMgr.TTL()).Err(); err != nil {
		return "", err
	}
	return secret, nil
}

// GetSignSecret 取会话签名密钥。key 不存在时返回 redis.Nil 错误。
func (s *AuthService) GetSignSecret(ctx context.Context, adminID uint64) (string, error) {
	return s.rdb.Get(ctx, signSecretKey(adminID)).Result()
}

// ClearSignSecret 清除会话签名密钥（登出 / 改密码时调用）。
func (s *AuthService) ClearSignSecret(ctx context.Context, adminID uint64) error {
	return s.rdb.Del(ctx, signSecretKey(adminID)).Err()
}

// SetNXNonce 防重放：首次写入 nonce 返回 true，已存在返回 false。
func (s *AuthService) SetNXNonce(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return s.rdb.SetNX(ctx, key, 1, ttl).Result()
}

// Reissue 为已登录管理员在中间件里"就地续签"一份全新 JWT。
// 使用场景：token 剩余时间 < 一半 TTL 时，避免用户中途掉线。
// 语义：签发新 token → 新 jti 加入 Redis 白名单 → 旧 jti 立刻回收 → 返回新 token。
func (s *AuthService) Reissue(ctx context.Context, adminID uint64, account, oldJTI string) (string, error) {
	newTok, newClaims, err := s.jwtMgr.Generate(adminID, account)
	if err != nil {
		return "", err
	}
	if err := s.rdb.Set(ctx, tokenKey(newClaims.ID), adminID, s.jwtMgr.TTL()).Err(); err != nil {
		return "", err
	}
	if oldJTI != "" {
		_ = s.rdb.Del(ctx, tokenKey(oldJTI)).Err()
	}
	return newTok, nil
}

// GetAdmin fetches an admin by id.
func (s *AuthService) GetAdmin(ctx context.Context, id uint64) (*model.RbacAdmin, error) {
	var a model.RbacAdmin
	if err := s.db.WithContext(ctx).Where("is_del = 0").First(&a, id).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

// UserRoleIDs returns role ids bound to the given admin (status=0 only).
func (s *AuthService) UserRoleIDs(ctx context.Context, adminID uint64) ([]uint64, error) {
	var ids []uint64
	err := scopeActive(s.db.WithContext(ctx).Model(&model.RbacAdminRole{})).
		Where("admin_id = ?", adminID).
		Pluck("role_id", &ids).Error
	return ids, err
}

// IsSuper returns true if any of the given roles has type=1.
func (s *AuthService) IsSuper(ctx context.Context, roleIDs []uint64) (bool, error) {
	if len(roleIDs) == 0 {
		return false, nil
	}
	var cnt int64
	err := scopeActive(s.db.WithContext(ctx).Model(&model.RbacRole{})).
		Where("id IN ? AND type = 1", roleIDs).
		Count(&cnt).Error
	return cnt > 0, err
}

// PermIDs returns all perm ids reachable by the given role ids via
// role_menu -> menu_perm.
func (s *AuthService) PermIDsOfRoles(ctx context.Context, roleIDs []uint64) ([]uint64, error) {
	if len(roleIDs) == 0 {
		return nil, nil
	}
	var menuIDs []uint64
	if err := scopeActive(s.db.WithContext(ctx).Model(&model.RbacRoleMenu{})).
		Where("role_id IN ?", roleIDs).
		Distinct("menu_id").
		Pluck("menu_id", &menuIDs).Error; err != nil {
		return nil, err
	}
	if len(menuIDs) == 0 {
		return nil, nil
	}
	var permIDs []uint64
	if err := scopeActive(s.db.WithContext(ctx).Model(&model.RbacMenuPerm{})).
		Where("menu_id IN ?", menuIDs).
		Distinct("perm_id").
		Pluck("perm_id", &permIDs).Error; err != nil {
		return nil, err
	}
	return permIDs, nil
}

// CheckRoute returns nil if the (method, path) is allowed for the given
// perm ids, or an error otherwise.
//
// Interception rule (subport-style):
//   - If no perm row matches this route at all, we treat it as "not registered"
//     and DENY (safe default). Register in rbac_perm to expose.
//   - If a matching perm has is_intercept=0, we ALLOW without further check.
//   - Otherwise the perm id MUST appear in the caller's permIDs list.
func (s *AuthService) CheckRoute(ctx context.Context, method, path string, permIDs []uint64) error {
	m := methodCode(method)
	var perms []model.RbacPerm
	if err := scopeActive(s.db.WithContext(ctx)).
		Where("route = ?", path).
		Where("method = 0 OR method = ?", m).
		Find(&perms).Error; err != nil {
		return biz(result.CodeDBError, err.Error())
	}
	if len(perms) == 0 {
		return biz(result.CodeAccessDenied, fmt.Sprintf("接口未注册: %s %s", method, path))
	}
	// any non-intercept match short-circuits allow
	permSet := map[uint64]struct{}{}
	for _, id := range permIDs {
		permSet[id] = struct{}{}
	}
	for _, p := range perms {
		if p.IsIntercept == 0 {
			return nil
		}
		if _, ok := permSet[p.ID]; ok {
			return nil
		}
	}
	return biz(result.CodeAccessDenied, "无接口权限")
}

func methodCode(method string) int {
	switch strings.ToUpper(method) {
	case "GET":
		return 1
	case "POST":
		return 2
	case "PUT":
		return 3
	case "DELETE":
		return 4
	default:
		return 0
	}
}

// MenusForAdmin returns menu records that current admin is authorized to see:
// super admin sees all; normal admin only sees menus bound to any of their roles.
// The result is a flat list; frontend builds the tree by parent_id.
func (s *AuthService) MenusForAdmin(ctx context.Context, adminID uint64) ([]model.RbacMenu, error) {
	roleIDs, err := s.UserRoleIDs(ctx, adminID)
	if err != nil {
		return nil, err
	}
	super, err := s.IsSuper(ctx, roleIDs)
	if err != nil {
		return nil, err
	}
	q := scopeActive(s.db.WithContext(ctx).Model(&model.RbacMenu{}))
	if !super {
		if len(roleIDs) == 0 {
			return []model.RbacMenu{}, nil
		}
		var menuIDs []uint64
		if err := scopeActive(s.db.WithContext(ctx).Model(&model.RbacRoleMenu{})).
			Where("role_id IN ?", roleIDs).
			Distinct("menu_id").
			Pluck("menu_id", &menuIDs).Error; err != nil {
			return nil, err
		}
		if len(menuIDs) == 0 {
			return []model.RbacMenu{}, nil
		}
		q = q.Where("id IN ?", menuIDs)
	}
	var list []model.RbacMenu
	if err := q.Order("sort ASC, id ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// UpdateProfile lets the current admin update his/her own profile (name / avatar).
func (s *AuthService) UpdateProfile(ctx context.Context, adminID uint64, name, avatar string) error {
	if name == "" {
		return biz(result.CodeParamError, "姓名不能为空")
	}
	return s.db.WithContext(ctx).Model(&model.RbacAdmin{}).
		Where("id = ? AND is_del = 0", adminID).
		Updates(map[string]any{
			"name":   name,
			"avatar": avatar,
			"u_by":   adminID,
		}).Error
}

// ChangeOwnPassword lets the current admin change his/her own password.
func (s *AuthService) ChangeOwnPassword(ctx context.Context, adminID uint64, oldPass, newPass string) error {
	if oldPass == "" || newPass == "" {
		return biz(result.CodeParamError, "旧密码/新密码不能为空")
	}
	var admin model.RbacAdmin
	if err := s.db.WithContext(ctx).Where("id = ? AND is_del = 0", adminID).First(&admin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return biz(result.CodeAccountNotFound, result.MessageOf(result.CodeAccountNotFound))
		}
		return biz(result.CodeDBError, err.Error())
	}
	if !auth.CheckPassword(oldPass, admin.Password) {
		return biz(result.CodeAuthFail, "旧密码不正确")
	}
	hash, err := auth.HashPassword(newPass, s.cfg.Security.BcryptCost)
	if err != nil {
		return biz(result.CodeInternalError, err.Error())
	}
	now := time.Now()
	if err := s.db.WithContext(ctx).Model(&admin).
		Updates(map[string]any{
			"password":                  hash,
			"last_update_password_time": now,
			"u_by":                      adminID,
		}).Error; err != nil {
		return biz(result.CodeDBError, err.Error())
	}
	return nil
}
