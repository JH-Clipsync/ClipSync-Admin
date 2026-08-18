<h1 align="center">ClipSync-Admin</h1>

<p align="center">
  <b>ClipSync 管理后台后端</b><br/>
  <a href="README.md">简体中文</a> ·
  <a href="README.en.md">English</a> ·
  <a href="README.ja.md">日本語</a>
</p>

---

ClipSync-Admin 是 [ClipSync](https://github.com/JH-Clipsync) 自建跨端消息同步体系的管理后台后端，使用 **Go + Gin + GORM + MySQL + Redis + JWT** 开发。它面向运维 / 客服 / 管理员，提供用户管理、设备管理、仪表盘统计、RBAC 角色权限、管理员认证等能力，并通过 Redis Pub/Sub 与 HTTP 双通道与 [ClipSync-Server](https://github.com/JH-Clipsync/ClipSync-Server) 联动，实现"改密码即踢下线、禁用设备即断连"。默认监听 **28002** 端口。

配套前端为 [ClipSync-Admin-Web](https://github.com/JH-Clipsync/ClipSync-Admin-Web)（Vue 3 + Element Plus）。

---

## ✨ 核心功能

| 模块 | 说明 |
|------|------|
| 📊 **仪表盘统计** | 用户总数、活跃用户数（`disabled=0`）、管理员数量、角色数量 |
| 👤 **用户管理** | 用户列表（按用户名搜索 / 按状态过滤 / 分页）、详情、创建、编辑、启用 / 禁用、重置密码（随机生成 10 位明文返回）、物理删除、主动踢下线；每个用户聚合设备总数与当前在线数 |
| 📱 **设备管理** | 列出某用户全部设备（角色 / 平台 / 自定义名 / 最近 IP / 在线状态）、跨用户分页搜索（关键字模糊匹配用户名 / 设备 ID / 名称 / IP）、启用 / 禁用设备、重命名设备、踢单台设备下线 |
| 🔌 **实时在线状态** | 设备列表**优先 HTTP 调用 Server** 的 `GET /server-admin/users/{id}/devices` 获取实时在线状态（以 Server 内存 Hub 为准）；Server 不可用时自动回退本地 MySQL + Redis |
| 📣 **Server 联动通知** | 重置密码 / 封禁用户 / 删除用户 / 禁用设备 / 踢设备时，通过 Redis Pub/Sub（频道 `clipsync:admin:kick_user`）通知 Server 立即断连；Redis 不通时走 HTTP `POST {server.addr}/server-admin/kick` 兜底，双保险 |
| 🛡️ **RBAC 权限** | 管理员 / 角色 / 菜单 / 接口权限的完整 CRUD；角色-菜单、菜单-权限、管理员-角色多对多关系；接口级拦截；内置超管 `admin` 受保护不可删 |
| 🔐 **管理员认证** | JWT（HS256，TTL 2 小时，滑动刷新）、bcrypt 管理员密码哈希（cost 可配）、登录失败计数锁定、可吊销 token |
| ✍️ **API 请求签名** | 每个请求都要带 HMAC-SHA256 签名（method / path / query / timestamp / nonce / bodyMD5），登录前用静态密钥 `sign_static_secret`，登录后切换为会话级动态密钥，防重放 / 防篡改 |
| 🖼️ **图片上传** | 管理员头像等图片上传，扩展名白名单 + 大小限制 + 按日期分目录存储，对外以静态目录提供访问 |
| 🗄️ **与 Server 共库** | 直接读取 Server 的 `users` / `devices` 表，管理员相关表全部使用 `admin_` 前缀，互不干扰 |
| 🐳 **Docker 原生** | 多阶段构建 → distroless nonroot 镜像；host 网络直连宿主机 MySQL / Redis；`deploy.sh` 可从 Server 配置自动生成 admin 配置 |

---

## 🚀 快速开始

### 前置条件

- 已部署 [ClipSync-Server](https://github.com/JH-Clipsync/ClipSync-Server) 并能连接到同一套 MySQL / Redis
- MySQL 中已存在 `clipsync` 数据库（Server 启动会自动建 `users` / `sessions` / `devices` 表）
- Docker（推荐）或 Go 1.22+

### 方式一：Docker Compose（推荐）

```bash
# 1. 准备部署目录
mkdir -p /app/ClipSync/admin/api/config /app/ClipSync/admin/api/uploads
cd /app/ClipSync/admin/api

# 2. 拷贝 compose 文件
cp deploy/docker-compose.yml .
cp deploy/.env.example .env

# 3. 自动从 Server 配置生成 admin config.yaml
#    脚本会按顺序查找以下 Server 配置：
#      /app/ClipSync/server/config/config.yaml
#      /app/ClipSync/server/config.yml
#      /app/ClipSync/config/config.yaml
#      /app/Clipsync/server/config/config.yaml
#      /opt/clipsync/config/config.yaml
bash deploy/deploy.sh
```

`deploy.sh` 会：

1. 自动从 Server `config.yaml` 读取 MySQL / Redis / `key_prefix` / `admin_token`
2. 生成本地 `config/config.yaml`，JWT 密钥用 `/dev/urandom` 随机生成
3. 检测并修复历史迁移脚本造成的配置损坏（如 `redis.addr` 被误改成 http URL）
4. `docker compose pull && up -d --force-recreate`
5. 检查并修复 nginx 配置（API 路径、静态资源 location）

默认超管账号：

```
账号：admin
密码：Admin**8
```

> 首次登录后请立即修改密码。

### 方式二：Docker 一行命令

```bash
docker run -d --name clipsync-admin \
  --network host \
  --restart unless-stopped \
  -v $(pwd)/config:/data/config:ro \
  -v $(pwd)/uploads:/data/uploads \
  -e TZ=Asia/Shanghai \
  ghcr.io/jh-clipsync/clipsync-admin:latest \
  -c /data/config/config.yaml
```

### 方式三：源码运行

```bash
git clone https://github.com/JH-Clipsync/ClipSync-Admin.git
cd ClipSync-Admin

# 准备配置
cp config.example.yaml config.yaml
vim config.yaml   # 填入 MySQL / Redis / JWT 密钥 / Server 联动信息

# 运行
go run .

# 或编译
CGO_ENABLED=0 go build -ldflags "-s -w" -o clipsync-admin .
./clipsync-admin -c config.yaml
```

启动后会自动：

- 迁移所有 `admin_` 前缀的 RBAC 表
- 种子超管账号 `admin / Admin**8`（已存在则跳过）
- 种子默认菜单与权限节点

---

## ⚙️ 配置说明

配置文件示例见 [config.example.yaml](config.example.yaml)，使用 viper 加载，支持环境变量（前缀 `CLIPSYNC_ADMIN_`）。

```yaml
app:
  name: clipsync-admin
  addr: ":28002"        # 监听端口
  mode: debug           # gin 模式：debug / release / test

mysql:
  # 与 ClipSync-Server 共用同一个库
  dsn: "clipsync:clipsync@tcp(127.0.0.1:3306)/clipsync?charset=utf8mb4&parseTime=True&loc=Local"
  max_idle_conns: 10
  max_open_conns: 100
  conn_max_lifetime: 3600

redis:
  addr: "127.0.0.1:6379"
  password: ""
  db: 2                 # 建议与 Server 分开 db，避免 key 冲突（虽然有前缀）

jwt:
  secret: "change-me-clipsync-admin-secret"   # ⚠️ 生产环境务必修改
  header: "Authorization"
  scheme: "Bearer"
  ttl: 7200             # token 有效期（秒），默认 2 小时
  refresh_on_access: true

security:
  bcrypt_cost: 10       # 管理员密码 bcrypt cost
  login_error_limit: 5  # 登录失败多少次后锁定
  login_error_ttl: 900  # 失败计数窗口（秒）
  # 登录前接口的签名密钥（前端硬编码同一字符串，登录成功后切换为动态密钥）
  sign_static_secret: "clipsync-admin-static-sign-secret-v1"

cors:
  allow_origins:
    - "http://localhost:5175"
  allow_credentials: true

log:
  level: "info"
  format: "console"     # console / json

bootstrap:
  super_admin_account: "admin"
  super_admin_password: "Admin**8"
  super_admin_name: "超级管理员"

upload:
  dir: "./data/uploads"
  url_prefix: "/static"
  max_size: 10485760    # 10MB
  allow_ext: [".jpg", ".jpeg", ".png", ".webp", ".gif"]

# 与 ClipSync-Server 的联动通道
server:
  key_prefix: "clipsync:"         # 必须与 Server 的 redis.key_prefix 一致
  addr: "http://127.0.0.1:28001"  # Server HTTP 地址（HTTP 兜底用）
  http_admin_token: ""            # 必须与 Server 的 server.admin_token 一致
```

### 关键配置注意事项

| 配置 | 说明 |
|------|------|
| `mysql.dsn` | 必须与 Server 连接同一个库；Admin 只会自动迁移 `admin_*` 表，不会改 `users` / `devices` 表结构 |
| `redis.db` | 建议选一个与 Server 不同的 db（Server 默认 db=0，Admin 示例用 db=2），避免互踩；`key_prefix` 必须与 Server 完全一致才能收到 Pub/Sub 消息 |
| `jwt.secret` | 用 `openssl rand -hex 32` 或 `head -c 32 /dev/urandom \| base64` 生成 |
| `server.key_prefix` | 决定 Pub/Sub 频道名 `{prefix}admin:kick_user`，必须和 Server 一致 |
| `server.addr` | 设备列表 HTTP 获取、创建用户、重命名设备的兜底通道；建议填内网 / 本机地址 |
| `server.http_admin_token` | 走 HTTP 兜底时的 Bearer Token，必须与 Server `server.admin_token` 完全一致 |

---

## 🏗️ 项目架构

```
┌────────────────────┐         ┌────────────────────────────┐
│ ClipSync-Admin-Web │ ──API──▶│     ClipSync-Admin         │
│   (Vue 3 + EP)     │ ◀────── │  Gin + GORM + JWT + HMAC   │
└────────────────────┘         └──────────┬─────────────────┘
                                          │
                    ┌─────────────────────┼─────────────────────┐
                    │                     │                     │
                    ▼                     ▼                     ▼
             ┌────────────┐       ┌────────────┐       ┌────────────────┐
             │   MySQL    │       │   Redis    │       │ ClipSync-Server│
             │ users/     │       │ 在线态 /   │       │  (HTTP 兜底)   │
             │ devices/   │       │ Pub/Sub /  │       │ /server-admin/*│
             │ admin_*    │       │ JWT/jti    │       └────────────────┘
             └────────────┘       └────────────┘
```

### 分层结构

```
internal/
├── main.go
├── config/           # 配置结构 + viper 加载
├── router/           # 路由装配、中间件挂载
├── middleware/       # JWT 认证、RBAC、签名校验、CORS、TraceID、访问日志
├── handler/          # HTTP 控制器（auth / data / rbac / upload）
├── service/          # 业务逻辑
│   ├── auth_service.go
│   ├── data_service.go       # 用户 / 设备 / 仪表盘
│   ├── rbac_service.go
│   └── server_notifier.go    # Redis Pub/Sub + HTTP 双通道下发
├── model/            # GORM 模型（biz 共享 Server 表，rbac 用 admin_ 前缀）
├── auth/             # JWT、bcrypt + scrypt 密码哈希、HMAC 签名
├── bootstrap/        # 表迁移、超管与菜单种子
├── db/               # MySQL / Redis 连接初始化
├── logger/           # zap 日志
└── result/           # 统一响应结构与错误码
```

### 数据模型

- **共享 Server 的表**：`users`、`devices`（Admin 只做部分字段写入，表结构由 Server 维护）
- **RBAC 表（全部 `admin_` 前缀）**：
  - `admin_rbac_admin`：管理员
  - `admin_rbac_role`：角色
  - `admin_rbac_admin_role`：管理员 ↔ 角色
  - `admin_rbac_menu`：菜单 / 按钮 / 数据列
  - `admin_rbac_perm`：接口权限（route + method 唯一）
  - `admin_rbac_role_menu`：角色 ↔ 菜单
  - `admin_rbac_menu_perm`：菜单 ↔ 权限

### 与 Server 的联动机制

| 触发动作 | Redis Pub/Sub | HTTP 兜底 |
|----------|---------------|-----------|
| 重置用户密码 | ✅ `kick_user` / `password_reset` | ✅ |
| 禁用用户 | ✅ `kick_user` / `user_disabled` | ✅ |
| 删除用户 | ✅ `kick_user` / `user_deleted` | ✅ |
| 主动踢用户下线 | ✅ `kick_user` / `device_kicked` | ✅ |
| 禁用设备 | ✅ `disable_device` | ✅ |
| 启用设备 | ✅ `enable_device` | ✅ |
| 踢单台设备 | ✅ `kick_device` | ✅ |
| 重命名设备 | — | ✅ `PUT /server-admin/users/{id}/devices/{did}/name` |
| 创建用户 | — | ✅ `POST /server-admin/users`（密码由 Server 用 scrypt 哈希） |
| 设备列表 / 在线状态 | — | ✅ `GET /server-admin/users/{id}/devices` 与 `GET /server-admin/devices` |

设备列表**优先走 Server HTTP**，因为在线状态以 Server 内存 Hub 最权威；只有当 Server 不通时才回退到本地 MySQL + Redis 查询。

---

## 🌐 部署

### Docker Compose（host 网络）

[deploy/docker-compose.yml](deploy/docker-compose.yml) 使用 host 网络，容器直接监听宿主机 `:28002`，可通过 `127.0.0.1` 直连 Server 和 MySQL / Redis：

```yaml
services:
  admin:
    image: ghcr.io/jh-clipsync/clipsync-admin:${ADMIN_TAG:-latest}
    container_name: clipsync-admin
    restart: unless-stopped
    network_mode: host
    command: ["-c", "/data/config/config.yaml"]
    volumes:
      - ./config:/data/config:ro
      - ./uploads:/data/uploads
    environment:
      - TZ=Asia/Shanghai
```

### Nginx 反代

完整示例见 [deploy/nginx.clipsync.conf](deploy/nginx.clipsync.conf)：

```nginx
# Admin 后端 API
location ^~ /clipsync/admin/api/ {
    proxy_pass http://127.0.0.1:28002/api/admin/;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}

# Admin 上传文件
location ^~ /clipsync/admin/static/ {
    proxy_pass http://127.0.0.1:28002/static/;
}

# Admin 前端静态文件
location /clipsync/admin/ {
    alias /app/ClipSync/admin/web/;
    index index.html;
    try_files $uri $uri/ /clipsync/admin/index.html;
}

# ClipSync-Server WebSocket
location = /clipsync/ws {
    proxy_pass http://127.0.0.1:28001/ws;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_read_timeout 3600s;
}
```

### 目录约定（生产环境）

```
/app/ClipSync/
├── server/              # ClipSync-Server
│   └── config/config.yaml
└── admin/
    ├── api/             # 本项目
    │   ├── config/config.yaml
    │   ├── uploads/
    │   └── docker-compose.yml
    └── web/             # ClipSync-Admin-Web 构建产物（静态文件）
        ├── index.html
        └── assets/
```

---

## 🔐 安全说明

- **管理员密码**：bcrypt 哈希，cost 默认 10（可通过 `security.bcrypt_cost` 调整）
- **业务用户密码**：Admin 重置用户密码时使用与 Server 完全一致的 **scrypt** 哈希（N=32768, r=8, p=1），保证共库可读
- **JWT**：HS256 签名，载荷含 `aid`（管理员 ID）+ `acc`（账号）+ `jti`；支持 Redis 吊销、滑动刷新
- **登录锁定**：默认 15 分钟内连续失败 5 次锁定账号
- **API 签名（HMAC-SHA256）**：
  - 待签串：`METHOD\nPATH\nQUERY\nTIMESTAMP\nNONCE\nBODY_MD5`
  - `PATH` 是去掉 `/api/admin` 前缀的相对路径
  - `QUERY` 按 key 字典序排序
  - 登录前用静态密钥 `sign_static_secret`，登录成功后服务端下发会话级随机 `signSecret`，登出即失效
  - 签名比较走 `hmac.Equal`，常数时间防时序攻击
- **CORS**：默认只放行配置的来源，生产环境务必把 `cors.allow_origins` 改成实际前端域名
- **Server 通信**：HTTP 兜底必须带 `Authorization: Bearer <server.admin_token>`，留空会被 Server 拒绝；Redis Pub/Sub 走内网，不暴露公网
- **容器安全**：distroless nonroot 镜像，无 shell、非 root 用户运行
- **默认密码**：超管默认密码 `Admin**8` 仅用于首次启动，**部署后必须立即修改**；`jwt.secret` 与 `sign_static_secret` 也必须替换

---

## 🤝 相关项目

- [ClipSync-Server](https://github.com/JH-Clipsync/ClipSync-Server)：三端同步中转服务端（Go + gorilla/websocket）
- [ClipSync-Admin-Web](https://github.com/JH-Clipsync/ClipSync-Admin-Web)：管理后台前端（Vue 3 + Element Plus）
- [ClipSync-Windows](https://github.com/JH-Clipsync/ClipSync-Windows)：Windows 客户端
- [ClipSync-Mac](https://github.com/JH-Clipsync/ClipSync-Mac)：macOS 客户端
- [ClipSync-Android](https://github.com/JH-Clipsync/ClipSync-Android)：Android 客户端
