package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/clipsync/admin/internal/config"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// AdminAction 与 ClipSync-Server 端约定的控制动作，保持字符串一致。
type AdminAction string

const (
	ActionKickUser      AdminAction = "kick_user"
	ActionKickDevice    AdminAction = "kick_device"
	ActionDisableDevice AdminAction = "disable_device"
	ActionEnableDevice  AdminAction = "enable_device"
	ActionDeleteDevice  AdminAction = "delete_device"
)

// Kick reason：与 Server 端 kickPayload.Reason 约定一致。
const (
	ReasonPasswordReset = "password_reset"
	ReasonUserDisabled  = "user_disabled"
	ReasonUserDeleted   = "user_deleted"
	ReasonDeviceKicked  = "device_kicked"
	ReasonDeviceBanned  = "device_banned"
)

// AdminCommand 管理端发给 ClipSync-Server 的一条控制指令。
type AdminCommand struct {
	Action   AdminAction `json:"action"`
	UserID   int64       `json:"user_id"`
	DeviceID string      `json:"device_id,omitempty"`
	Reason   string      `json:"reason,omitempty"`
}

// ServerNotifier 向 ClipSync-Server 下发控制指令（强制下线 / 设备禁用等）。
// 主通道 Redis Pub/Sub；server.addr 配置不为空时走 HTTP 兜底。
type ServerNotifier struct {
	cfg    config.ServerConfig
	rdb    *redis.Client
	logger *zap.Logger
	cli    *http.Client
}

func NewServerNotifier(cfg config.ServerConfig, rdb *redis.Client, logger *zap.Logger) *ServerNotifier {
	return &ServerNotifier{
		cfg:    cfg,
		rdb:    rdb,
		logger: logger,
		cli:    &http.Client{Timeout: 5 * time.Second},
	}
}

func (n *ServerNotifier) channel() string {
	prefix := n.cfg.KeyPrefix
	if prefix == "" {
		prefix = "clipsync:"
	}
	return prefix + "admin:kick_user"
}

// sendCommand 先尝试 Redis Pub/Sub，失败再走 HTTP 兜底（如果配置了 addr）。
func (n *ServerNotifier) sendCommand(ctx context.Context, cmd AdminCommand) error {
	ch := n.channel()
	if n.rdb != nil {
		data, _ := json.Marshal(cmd)
		if err := n.rdb.Publish(ctx, ch, data).Err(); err == nil {
			n.logger.Info("notify server via redis",
				zap.String("action", string(cmd.Action)),
				zap.Int64("user_id", cmd.UserID),
				zap.String("device_id", cmd.DeviceID),
				zap.String("channel", ch),
			)
			return nil
		} else {
			n.logger.Warn("notify server via redis failed, fallback to http",
				zap.String("action", string(cmd.Action)),
				zap.Int64("user_id", cmd.UserID),
				zap.Error(err),
			)
		}
	}
	if n.cfg.Addr == "" {
		return fmt.Errorf("notify server failed: redis down and server.addr not configured")
	}
	return n.commandViaHTTP(ctx, cmd)
}

// KickUser 通知 Server 强制下线 userID 下的所有连接。
func (n *ServerNotifier) KickUser(ctx context.Context, userID int64, reason string) error {
	if reason == "" {
		reason = ReasonPasswordReset
	}
	return n.sendCommand(ctx, AdminCommand{
		Action: ActionKickUser,
		UserID: userID,
		Reason: reason,
	})
}

// KickDevice 只踢某用户下的一台设备，其他端不受影响。
func (n *ServerNotifier) KickDevice(ctx context.Context, userID int64, deviceID, reason string) error {
	if reason == "" {
		reason = ReasonDeviceKicked
	}
	return n.sendCommand(ctx, AdminCommand{
		Action:   ActionKickDevice,
		UserID:   userID,
		DeviceID: deviceID,
		Reason:   reason,
	})
}

// SetDeviceStatus 启用/禁用某用户下的一台设备。禁用时 Server 会顺手踢它下线。
func (n *ServerNotifier) SetDeviceStatus(ctx context.Context, userID int64, deviceID string, disabled bool, reason string) error {
	action := ActionEnableDevice
	if disabled {
		action = ActionDisableDevice
		if reason == "" {
			reason = ReasonDeviceBanned
		}
	}
	return n.sendCommand(ctx, AdminCommand{
		Action:   action,
		UserID:   userID,
		DeviceID: deviceID,
		Reason:   reason,
	})
}

// DeleteDevice 删除某用户下的一台「已禁用」设备，Server 会对仍在线的连接兜底踢下线后
// 物理删除登记。删除有前置校验（未禁用返回 409），需要同步拿到结果，因此优先走 HTTP；
// 仅当未配置 server.addr 时才降级为 fire-and-forget 的 Pub/Sub。
func (n *ServerNotifier) DeleteDevice(ctx context.Context, userID int64, deviceID string) error {
	if n.cfg.Addr != "" {
		return n.commandViaHTTP(ctx, AdminCommand{
			Action:   ActionDeleteDevice,
			UserID:   userID,
			DeviceID: deviceID,
		})
	}
	return n.sendCommand(ctx, AdminCommand{
		Action:   ActionDeleteDevice,
		UserID:   userID,
		DeviceID: deviceID,
		Reason:   ReasonDeviceBanned,
	})
}

// ServerDevice 对应 Server 端 /server-admin/users/{id}/devices 返回的设备信息。
type ServerDevice struct {
	UserID     int64  `json:"user_id"`
	Username   string `json:"username"`
	DeviceID   string `json:"device_id"`
	Role       string `json:"role"`
	Platform   string `json:"platform"`
	Name       string `json:"name"`
	LastIP     string `json:"last_ip"`
	Disabled   bool   `json:"disabled"`
	Online     bool   `json:"online"`
	LastSeenAt string `json:"last_seen_at"`
	CreatedAt  string `json:"created_at"`
}

// FetchDevices 调用 Server GET /server-admin/users/{id}/devices 获取设备列表（含在线状态）。
func (n *ServerNotifier) FetchDevices(ctx context.Context, userID int64) ([]ServerDevice, error) {
	if n.cfg.Addr == "" {
		return nil, fmt.Errorf("server.addr 未配置，无法获取设备列表")
	}
	url := fmt.Sprintf("%s/server-admin/users/%d/devices", n.cfg.Addr, userID)
	var result struct {
		Devices []ServerDevice `json:"devices"`
	}
	if err := n.getJSON(ctx, url, &result); err != nil {
		return nil, err
	}
	return result.Devices, nil
}

// FetchAllDevices 调用 Server GET /server-admin/devices 跨用户分页查询设备。
// userID > 0 时只查该用户；online 非 nil 时按在线/离线过滤（Server 内存态）。
func (n *ServerNotifier) FetchAllDevices(ctx context.Context, keyword string, disabled *bool, online *bool, userID int64, page, pageSize int) ([]ServerDevice, int64, error) {
	if n.cfg.Addr == "" {
		return nil, 0, fmt.Errorf("server.addr 未配置，无法获取设备列表")
	}
	u := fmt.Sprintf("%s/server-admin/devices?page=%d&page_size=%d", n.cfg.Addr, page, pageSize)
	if userID > 0 {
		u += fmt.Sprintf("&user_id=%d", userID)
	}
	if keyword != "" {
		u += "&keyword=" + urlQueryEscape(keyword)
	}
	if disabled != nil {
		u += "&disabled=" + strconv.FormatBool(*disabled)
	}
	if online != nil {
		u += "&online=" + strconv.FormatBool(*online)
	}
	var result struct {
		List  []ServerDevice `json:"list"`
		Total int64          `json:"total"`
		Page  int            `json:"page"`
	}
	if err := n.getJSON(ctx, u, &result); err != nil {
		return nil, 0, err
	}
	return result.List, result.Total, nil
}

func (n *ServerNotifier) getJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if n.cfg.HTTPAdminToken != "" {
		req.Header.Set("Authorization", "Bearer "+n.cfg.HTTPAdminToken)
	}
	resp, err := n.cli.Do(req)
	if err != nil {
		return fmt.Errorf("请求 Server 失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("Server 返回错误 status=%d body=%s", resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("解析 Server 响应失败: %w", err)
	}
	return nil
}

// RenameDevice 调用 Server PUT /server-admin/users/{id}/devices/{deviceID}/name 修改设备名称。
func (n *ServerNotifier) RenameDevice(ctx context.Context, userID int64, deviceID, name string) error {
	if n.cfg.Addr == "" {
		return fmt.Errorf("server.addr 未配置，无法重命名设备")
	}
	u := fmt.Sprintf("%s/server-admin/users/%d/devices/%s/name", n.cfg.Addr, userID, url.PathEscape(deviceID))
	body, _ := json.Marshal(map[string]string{"name": name})
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if n.cfg.HTTPAdminToken != "" {
		req.Header.Set("Authorization", "Bearer "+n.cfg.HTTPAdminToken)
	}
	resp, err := n.cli.Do(req)
	if err != nil {
		return fmt.Errorf("请求 Server 失败: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("Server 返回错误 status=%d body=%s", resp.StatusCode, string(respBody))
	}
	n.logger.Info("rename device via server http",
		zap.Int64("user_id", userID),
		zap.String("device_id", deviceID),
		zap.String("name", name),
		zap.Int("status", resp.StatusCode),
	)
	return nil
}

func urlQueryEscape(s string) string {
	return url.QueryEscape(s)
}

// CreatedUser Server 创建用户后返回的信息。
type CreatedUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
}

// CreateUser 调用 Server POST /server-admin/users 创建用户。
// 密码由 Server 端 bcrypt 哈希，Admin 不接触明文密码存储。
func (n *ServerNotifier) CreateUser(ctx context.Context, username, nickname, password string) (*CreatedUser, error) {
	if n.cfg.Addr == "" {
		return nil, fmt.Errorf("server.addr 未配置，无法创建用户")
	}
	u := n.cfg.Addr + "/server-admin/users"
	body, _ := json.Marshal(map[string]string{
		"username": username,
		"nickname": nickname,
		"password": password,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if n.cfg.HTTPAdminToken != "" {
		req.Header.Set("Authorization", "Bearer "+n.cfg.HTTPAdminToken)
	}
	resp, err := n.cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Server 失败: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusConflict {
		return nil, fmt.Errorf("用户名已存在")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Server 返回错误 status=%d body=%s", resp.StatusCode, string(respBody))
	}
	var cu CreatedUser
	if err := json.Unmarshal(respBody, &cu); err != nil {
		return nil, fmt.Errorf("解析 Server 响应失败: %w", err)
	}
	n.logger.Info("create user via server http",
		zap.Int64("id", cu.ID),
		zap.String("username", cu.Username),
	)
	return &cu, nil
}

// commandViaHTTP HTTP 兜底。Server 端 /server-admin/kick 支持全动作。
func (n *ServerNotifier) commandViaHTTP(ctx context.Context, cmd AdminCommand) error {
	body, _ := json.Marshal(cmd)
	url := n.cfg.Addr + "/server-admin/kick"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if n.cfg.HTTPAdminToken != "" {
		req.Header.Set("Authorization", "Bearer "+n.cfg.HTTPAdminToken)
	}
	resp, err := n.cli.Do(req)
	if err != nil {
		return fmt.Errorf("notify server via http: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusConflict {
		// 删除设备前置校验失败（未禁用）：把 Server 的中文原因原样抛给上层/前端。
		return fmt.Errorf("%s", strings.TrimSpace(string(respBody)))
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("notify server via http: status=%d body=%s", resp.StatusCode, string(respBody))
	}
	n.logger.Info("notify server via http",
		zap.String("action", string(cmd.Action)),
		zap.Int64("user_id", cmd.UserID),
		zap.String("device_id", cmd.DeviceID),
		zap.String("url", url),
		zap.Int("status", resp.StatusCode),
	)
	return nil
}

// ==================== 开放接口 ApiKey 管理（透传到 Server） ====================

// APIKeyView ApiKey 列表项（不含任何密钥材料，明文仅创建时返回一次）。
type APIKeyView struct {
	ID         int64  `json:"id"`
	UserID     int64  `json:"user_id"`
	Username   string `json:"username"`
	KeyPrefix  string `json:"key_prefix"`
	Name       string `json:"name"`
	Phone      string `json:"phone"`
	DeviceID   string `json:"device_id"`
	ExpiresAt  string `json:"expires_at"`
	Disabled   bool   `json:"disabled"`
	Expired    bool   `json:"expired"`
	LastUsedAt string `json:"last_used_at"`
	CreatedAt  string `json:"created_at"`
}

// CreatedAPIKey 创建 ApiKey 的返回，api_key 明文仅此一次。
type CreatedAPIKey struct {
	ID        int64  `json:"id"`
	APIKey    string `json:"api_key"`
	KeyPrefix string `json:"key_prefix"`
	Name      string `json:"name"`
	Phone     string `json:"phone"`
	DeviceID  string `json:"device_id"`
	ExpiresAt string `json:"expires_at"`
	CreatedAt string `json:"created_at"`
	Hint      string `json:"hint"`
}

// CreateAPIKey 调用 Server POST /server-admin/users/{id}/api-keys。
func (n *ServerNotifier) CreateAPIKey(ctx context.Context, userID int64, name, phone, deviceID, expiresAt string) (*CreatedAPIKey, error) {
	if n.cfg.Addr == "" {
		return nil, fmt.Errorf("server.addr 未配置，无法创建 API Key")
	}
	u := fmt.Sprintf("%s/server-admin/users/%d/api-keys", n.cfg.Addr, userID)
	body, _ := json.Marshal(map[string]string{
		"name":       name,
		"phone":      phone,
		"device_id":  deviceID,
		"expires_at": expiresAt,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if n.cfg.HTTPAdminToken != "" {
		req.Header.Set("Authorization", "Bearer "+n.cfg.HTTPAdminToken)
	}
	resp, err := n.cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Server 失败: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Server 返回错误 status=%d body=%s", resp.StatusCode, string(respBody))
	}
	var out CreatedAPIKey
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("解析 Server 响应失败: %w", err)
	}
	n.logger.Info("create api key via server http",
		zap.Int64("user_id", userID),
		zap.Int64("id", out.ID),
		zap.String("phone", phone),
	)
	return &out, nil
}

// ListAPIKeys 调用 Server 列出某用户（userID>0）或全部（userID=0）ApiKey。
func (n *ServerNotifier) ListAPIKeys(ctx context.Context, userID int64) ([]APIKeyView, error) {
	if n.cfg.Addr == "" {
		return nil, fmt.Errorf("server.addr 未配置，无法获取 API Key 列表")
	}
	var u string
	if userID > 0 {
		u = fmt.Sprintf("%s/server-admin/users/%d/api-keys", n.cfg.Addr, userID)
	} else {
		u = n.cfg.Addr + "/server-admin/api-keys"
	}
	var result struct {
		Items []APIKeyView `json:"items"`
		Total int          `json:"total"`
	}
	if err := n.getJSON(ctx, u, &result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

// RevokeAPIKey 调用 Server DELETE /server-admin/api-keys/{keyID}。
func (n *ServerNotifier) RevokeAPIKey(ctx context.Context, keyID int64) error {
	if n.cfg.Addr == "" {
		return fmt.Errorf("server.addr 未配置，无法吊销 API Key")
	}
	u := fmt.Sprintf("%s/server-admin/api-keys/%d", n.cfg.Addr, keyID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return err
	}
	if n.cfg.HTTPAdminToken != "" {
		req.Header.Set("Authorization", "Bearer "+n.cfg.HTTPAdminToken)
	}
	resp, err := n.cli.Do(req)
	if err != nil {
		return fmt.Errorf("请求 Server 失败: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("密钥不存在")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("Server 返回错误 status=%d body=%s", resp.StatusCode, string(respBody))
	}
	n.logger.Info("revoke api key via server http", zap.Int64("id", keyID))
	return nil
}
