package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	App       AppConfig       `mapstructure:"app"`
	MySQL     MySQLConfig     `mapstructure:"mysql"`
	Redis     RedisConfig     `mapstructure:"redis"`
	JWT       JWTConfig       `mapstructure:"jwt"`
	Security  SecurityConfig  `mapstructure:"security"`
	CORS      CORSConfig      `mapstructure:"cors"`
	Log       LogConfig       `mapstructure:"log"`
	Bootstrap BootstrapConfig `mapstructure:"bootstrap"`
	Upload    UploadConfig    `mapstructure:"upload"`
	// Server 与 ClipSync-Server 的联动配置。
	Server ServerConfig `mapstructure:"server"`
}

type AppConfig struct {
	Name string `mapstructure:"name"`
	Addr string `mapstructure:"addr"`
	Mode string `mapstructure:"mode"`
}

type MySQLConfig struct {
	DSN             string `mapstructure:"dsn"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type JWTConfig struct {
	Secret          string `mapstructure:"secret"`
	Header          string `mapstructure:"header"`
	Scheme          string `mapstructure:"scheme"`
	TTL             int    `mapstructure:"ttl"`
	RefreshOnAccess bool   `mapstructure:"refresh_on_access"`
}

func (j JWTConfig) TTLDuration() time.Duration {
	return time.Duration(j.TTL) * time.Second
}

type SecurityConfig struct {
	BcryptCost      int `mapstructure:"bcrypt_cost"`
	LoginErrorLimit int `mapstructure:"login_error_limit"`
	LoginErrorTTL   int `mapstructure:"login_error_ttl"`
	// SignStaticSecret 登录前签名密钥（登录接口本身用此密钥校验签名）。
	// 前端硬编码同一字符串，登录成功后切换为动态下发的 signSecret。
	SignStaticSecret string `mapstructure:"sign_static_secret"`
}

type CORSConfig struct {
	AllowOrigins     []string `mapstructure:"allow_origins"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
}

// ServerConfig 管理端向 ClipSync-Server 下发控制指令（强制下线等）的通道配置。
// 默认用 Redis Pub/Sub（两边共享同一 Redis 实例时），频道名 = KeyPrefix + "admin:kick_user"。
// 如果管理端和 Server 不共享 Redis，可改用 HTTP 调用：填 Server.Addr，会以 POST {addr}/admin/kick 兜底。
type ServerConfig struct {
	// KeyPrefix 与 ClipSync-Server 的 redis.key_prefix 保持一致（默认 "clipsync:"）。
	// 决定 Pub/Sub 频道名；频道名 = KeyPrefix + "admin:kick_user"。
	KeyPrefix string `mapstructure:"key_prefix"`
	// Addr ClipSync-Server 的 HTTP 监听地址（如 "http://127.0.0.1:28001"）。
	// 不为空时，若 Redis Pub/Sub 下发失败则走 HTTP 兜底。
	Addr string `mapstructure:"addr"`
	// HTTPAdminToken 走 HTTP 兜底时的 Bearer Token（Server 侧自行校验）。
	HTTPAdminToken string `mapstructure:"http_admin_token"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type BootstrapConfig struct {
	SuperAdminAccount  string `mapstructure:"super_admin_account"`
	SuperAdminPassword string `mapstructure:"super_admin_password"`
	SuperAdminName     string `mapstructure:"super_admin_name"`
}

// UploadConfig 图片/文件上传相关配置
type UploadConfig struct {
	Dir       string   `mapstructure:"dir"`        // 磁盘存放目录
	URLPrefix string   `mapstructure:"url_prefix"` // 对外访问前缀
	MaxSize   int64    `mapstructure:"max_size"`   // 单文件最大字节数
	AllowExt  []string `mapstructure:"allow_ext"`  // 允许扩展名（含点）
}

func Load(path string) (*Config, error) {
	v := viper.New()
	if path != "" {
		v.SetConfigFile(path)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
	}
	v.AutomaticEnv()
	v.SetEnvPrefix("CLIPSYNC_ADMIN")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &cfg, nil
}
