package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/clipsync/admin/internal/auth"
	"github.com/clipsync/admin/internal/model"
	"github.com/clipsync/admin/internal/result"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// AdminDataService bundles simple CRUD for ClipSync business tables (users).
// All list endpoints are paginated with (page, pageSize).
type AdminDataService struct {
	db      *gorm.DB
	rdb     *redis.Client
	prefix  string // ClipSync-Server 的 Redis key 前缀，如 "clipsync:"
	kick    *ServerNotifier
}

func NewAdminDataService(db *gorm.DB, rdb *redis.Client, keyPrefix string, kick *ServerNotifier) *AdminDataService {
	if keyPrefix == "" {
		keyPrefix = "clipsync:"
	}
	return &AdminDataService{db: db, rdb: rdb, prefix: keyPrefix, kick: kick}
}

// Page is the unified pagination response.
type Page struct {
	List     any   `json:"list"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

func normalize(page, size int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 200 {
		size = 20
	}
	return page, size
}

// ---------- Users ----------

// ListUsers 支持 keyword（搜 username）、disabled 过滤、分页。
// disabled < 0 表示不过滤。
func (s *AdminDataService) ListUsers(ctx context.Context, keyword string, disabled int, page, size int) (*Page, error) {
	page, size = normalize(page, size)
	q := s.db.WithContext(ctx).Model(&model.User{})
	if keyword != "" {
		q = q.Where("username LIKE ?", "%"+keyword+"%")
	}
	if disabled >= 0 {
		q = q.Where("disabled = ?", disabled)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, biz(result.CodeDBError, err.Error())
	}
	var list []model.User
	if err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, biz(result.CodeDBError, err.Error())
	}
	return &Page{List: list, Total: total, Page: page, PageSize: size}, nil
}

func (s *AdminDataService) GetUser(ctx context.Context, id int64) (*model.User, error) {
	var u model.User
	if err := s.db.WithContext(ctx).First(&u, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, biz(result.CodeRecordNotFound, "用户不存在")
		}
		return nil, biz(result.CodeDBError, err.Error())
	}
	return &u, nil
}

// OnlineDevice 在线设备（来自 ClipSync-Server 写入的 Redis Hash）。
type OnlineDevice struct {
	DeviceID string `json:"device_id"`
	Role     string `json:"role"` // pc | mobile
	Platform string `json:"platform"` // mac / windows / android / ios / unknown
}

func (s *AdminDataService) onlineKey(userID int64) string {
	return s.prefix + "online:" + strconv.FormatInt(userID, 10)
}

// platformFromDeviceID 根据 deviceID 前缀推断平台。
// 各端生成规则：Mac=mac-xxx，Android=android-xxx，Windows 直接用 GUID/带 win 前缀。
func platformFromDeviceID(id string) string {
	low := strings.ToLower(id)
	switch {
	case strings.HasPrefix(low, "mac-") || strings.HasPrefix(low, "imac") || strings.HasPrefix(low, "macbook"):
		return "mac"
	case strings.HasPrefix(low, "win-") || strings.HasPrefix(low, "desktop") || isWindowsGUID(id):
		return "windows"
	case strings.HasPrefix(low, "android-") || strings.HasPrefix(low, "samsung") || strings.HasPrefix(low, "xiaomi") || strings.HasPrefix(low, "huawei"):
		return "android"
	case strings.HasPrefix(low, "ios-") || strings.HasPrefix(low, "iphone") || strings.HasPrefix(low, "ipad"):
		return "ios"
	default:
		return "unknown"
	}
}

// isWindowsGUID 粗略判断是否为 Windows 端生成的 GUID（8-4-4-4-12 形式）。
func isWindowsGUID(id string) bool {
	if len(id) != 36 {
		return false
	}
	return id[8] == '-' && id[13] == '-' && id[18] == '-' && id[23] == '-'
}

// GetOnlineDevices 读取用户当前在线设备列表。Redis 不可用时返回空列表，不阻断请求。
func (s *AdminDataService) GetOnlineDevices(ctx context.Context, userID int64) []OnlineDevice {
	if s.rdb == nil {
		return nil
	}
	m, err := s.rdb.HGetAll(ctx, s.onlineKey(userID)).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil
	}
	devices := make([]OnlineDevice, 0, len(m))
	for deviceID, role := range m {
		devices = append(devices, OnlineDevice{
			DeviceID: deviceID,
			Role:     role,
			Platform: platformFromDeviceID(deviceID),
		})
	}
	return devices
}

// UserDetail 用户详情（含在线设备），给管理端用户详情页用。
type UserDetail struct {
	model.User
	OnlineDevices []OnlineDevice `json:"online_devices"`
}

// GetUserDetail 读取用户详情 + 在线设备列表。
func (s *AdminDataService) GetUserDetail(ctx context.Context, id int64) (*UserDetail, error) {
	u, err := s.GetUser(ctx, id)
	if err != nil {
		return nil, err
	}
	return &UserDetail{
		User:          *u,
		OnlineDevices: s.GetOnlineDevices(ctx, id),
	}, nil
}

// UpdateUser 只改 username 和 disabled（users 表没有 remark 等其他字段）。
func (s *AdminDataService) UpdateUser(ctx context.Context, id int64, username string, disabled int, _ uint64) error {
	updates := map[string]any{
		"username": username,
		"disabled": disabled,
	}
	res := s.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return biz(result.CodeDBError, res.Error.Error())
	}
	if res.RowsAffected == 0 {
		return biz(result.CodeRecordNotFound, "用户不存在")
	}
	return nil
}

// UpdateUserStatus 改 disabled 字段（前端传 status，handler 映射成 disabled）。
// 禁用（disabled=1）时额外通知 Server 强制下线该用户所有连接。
func (s *AdminDataService) UpdateUserStatus(ctx context.Context, id int64, disabled int, _ uint64) error {
	res := s.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).
		Updates(map[string]any{"disabled": disabled})
	if res.Error != nil {
		return biz(result.CodeDBError, res.Error.Error())
	}
	if res.RowsAffected == 0 {
		return biz(result.CodeRecordNotFound, "用户不存在")
	}
	// 禁用用户 → 踢掉所有已连接客户端，防止继续收发消息
	if disabled != 0 && s.kick != nil {
		_ = s.kick.KickUser(ctx, id, ReasonUserDisabled)
	}
	return nil
}

// ResetUserPassword 生成新密码，用 scrypt 哈希写回 password_hash，返回明文密码给前端。
// 密码变更后必须通知 Server 强制下线所有连接——老 token 对不上新密码，继续持有已没有意义。
func (s *AdminDataService) ResetUserPassword(ctx context.Context, id int64, _ uint64) (string, error) {
	const chars = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKMNPQRSTUVWXYZ23456789"
	pwd := make([]byte, 10)
	for i := range pwd {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", biz(result.CodeInternalError, err.Error())
		}
		pwd[i] = chars[n.Int64()]
	}
	plain := string(pwd)
	// 使用 scrypt（与 ClipSync-Server 兼容）
	hash, err := auth.HashUserPassword(plain)
	if err != nil {
		return "", biz(result.CodeInternalError, err.Error())
	}
	res := s.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).
		Updates(map[string]any{"password_hash": hash})
	if res.Error != nil {
		return "", biz(result.CodeDBError, res.Error.Error())
	}
	if res.RowsAffected == 0 {
		return "", biz(result.CodeRecordNotFound, "用户不存在")
	}
	if s.kick != nil {
		_ = s.kick.KickUser(ctx, id, ReasonPasswordReset)
	}
	return plain, nil
}

// DeleteUser 物理删除用户（users 表没有 is_del 字段，不做软删除）。
// 删除前先踢掉所有连接，避免用户表不存在后连接还持有 userID。
func (s *AdminDataService) DeleteUser(ctx context.Context, id int64, _ uint64) error {
	if s.kick != nil {
		_ = s.kick.KickUser(ctx, id, ReasonUserDeleted)
	}
	res := s.db.WithContext(ctx).Where("id = ?", id).Delete(&model.User{})
	if res.Error != nil {
		return biz(result.CodeDBError, res.Error.Error())
	}
	if res.RowsAffected == 0 {
		return biz(result.CodeRecordNotFound, "用户不存在")
	}
	return nil
}

// ---------- Devices ----------

// Device 对应 ClipSync-Server 的 devices 表。
// Admin 侧只读字段；启用/禁用由管理端写，Server 收到后同步给客户端。
type Device struct {
	UserID     int64     `gorm:"primaryKey;column:user_id" json:"user_id"`
	DeviceID   string    `gorm:"primaryKey;column:device_id" json:"device_id"`
	Role       string    `gorm:"column:role" json:"role"`
	Platform   string    `gorm:"column:platform" json:"platform"`
	Disabled   bool      `gorm:"column:disabled" json:"disabled"`
	LastSeenAt time.Time `gorm:"column:last_seen_at" json:"last_seen_at"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
	Online     bool      `gorm:"-" json:"online"`
}

func (Device) TableName() string { return "devices" }

// ListDevices 列出某账号下所有登记过的设备（含离线），优先从 Server 接口获取实时在线状态。
func (s *AdminDataService) ListDevices(ctx context.Context, userID int64) ([]Device, error) {
	if s.kick != nil {
		serverDevices, err := s.kick.FetchDevices(ctx, userID)
		if err == nil {
			list := make([]Device, 0, len(serverDevices))
			for _, sd := range serverDevices {
				d := Device{
					UserID:   sd.UserID,
					DeviceID: sd.DeviceID,
					Role:     sd.Role,
					Platform: sd.Platform,
					Disabled: sd.Disabled,
					Online:   sd.Online,
				}
				if t, err := time.Parse(time.RFC3339, sd.LastSeenAt); err == nil {
					d.LastSeenAt = t
				}
				if t, err := time.Parse(time.RFC3339, sd.CreatedAt); err == nil {
					d.CreatedAt = t
				}
				list = append(list, d)
			}
			return list, nil
		}
		fmt.Printf("fetch devices from server failed, fallback to db: %v\n", err)
	}

	var list []Device
	if err := s.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("last_seen_at DESC").
		Find(&list).Error; err != nil {
		return nil, biz(result.CodeDBError, err.Error())
	}
	onlineSet := s.getOnlineDeviceIDs(ctx, userID)
	for i := range list {
		_, list[i].Online = onlineSet[list[i].DeviceID]
	}
	return list, nil
}

// getOnlineDeviceIDs 返回该用户当前在线的 deviceID 集合。
func (s *AdminDataService) getOnlineDeviceIDs(ctx context.Context, userID int64) map[string]struct{} {
	m := make(map[string]struct{})
	if s.rdb == nil {
		return m
	}
	res, err := s.rdb.HGetAll(ctx, s.onlineKey(userID)).Result()
	if err != nil {
		return m
	}
	for k := range res {
		m[k] = struct{}{}
	}
	return m
}

// SetDeviceStatus 启用/禁用设备，并通知 Server 立即生效（禁用时踢该设备下线）。
// Server 是真正持有设备表写权限的一方：Admin 改库后，Server 收到 Pub/Sub
// 再把 disabled 写回去并执行踢连接。这里先本地更新，失败也不阻塞通知，
// 让 Server 端的事件处理成为权威路径。
func (s *AdminDataService) SetDeviceStatus(ctx context.Context, userID int64, deviceID string, disabled bool) error {
	res := s.db.WithContext(ctx).Model(&Device{}).
		Where("user_id = ? AND device_id = ?", userID, deviceID).
		Update("disabled", disabled)
	if res.Error != nil {
		return biz(result.CodeDBError, res.Error.Error())
	}
	if res.RowsAffected == 0 {
		return biz(result.CodeRecordNotFound, "设备不存在")
	}
	if s.kick != nil {
		if err := s.kick.SetDeviceStatus(ctx, userID, deviceID, disabled, ReasonDeviceBanned); err != nil {
			// 通知失败不阻断管理端操作：DB 已经改了，设备下次握手也会被拒
			fmt.Printf("notify device status failed: %v\n", err)
		}
	}
	return nil
}

// KickDevice 主动踢某用户下的一台设备下线（不改变禁用状态，重连后可继续使用）。
func (s *AdminDataService) KickDevice(ctx context.Context, userID int64, deviceID string) error {
	if s.kick == nil {
		return biz(result.CodeInternalError, "未配置 Server 联动，无法踢设备")
	}
	if err := s.kick.KickDevice(ctx, userID, deviceID, ReasonDeviceKicked); err != nil {
		return biz(result.CodeInternalError, err.Error())
	}
	return nil
}

// KickUser 主动踢某用户全部设备下线（不改密码、不禁用，相当于"让他重登一次"）。
func (s *AdminDataService) KickUserNow(ctx context.Context, userID int64) error {
	if s.kick == nil {
		return biz(result.CodeInternalError, "未配置 Server 联动，无法踢用户")
	}
	if err := s.kick.KickUser(ctx, userID, ReasonDeviceKicked); err != nil {
		return biz(result.CodeInternalError, err.Error())
	}
	return nil
}

// ---------- Dashboard ----------

// DashboardStat 简化版仪表盘统计：用户总数、活跃用户、管理员数、角色数。
type DashboardStat struct {
	UserTotal  int64 `json:"userTotal"`
	UserActive int64 `json:"userActive"` // disabled=0
	AdminTotal int64 `json:"adminTotal"`
	RoleTotal  int64 `json:"roleTotal"`
}

func (s *AdminDataService) Dashboard(ctx context.Context) (*DashboardStat, error) {
	stat := &DashboardStat{}
	stat.UserTotal = countModel(ctx, s.db, &model.User{}, "")
	stat.UserActive = countModel(ctx, s.db, &model.User{}, "disabled = 0")
	stat.AdminTotal = countModel(ctx, s.db, &model.RbacAdmin{}, "is_del = 0")
	stat.RoleTotal = countModel(ctx, s.db, &model.RbacRole{}, "is_del = 0")
	return stat, nil
}

func countModel(ctx context.Context, db *gorm.DB, model any, where string) int64 {
	q := db.WithContext(ctx).Model(model)
	if where != "" {
		q = q.Where(where)
	}
	var n int64
	q.Count(&n)
	return n
}
