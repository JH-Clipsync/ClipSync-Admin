<h1 align="center">ClipSync-Admin</h1>

<p align="center">
  <b>ClipSync 管理后台后端</b><br/>
  <a href="README.md">简体中文</a> ·
  <a href="README.en.md">English</a> ·
  <a href="README.ja.md">日本語</a>
</p>

---

ClipSync-Admin 是 [ClipSync](https://github.com/JH-Clipsync) 自建跨端消息同步体系的管理后台后端，使用 **Go + Gin + GORM + MySQL + Redis + JWT** 开发。它面向运维/客服/管理员，提供用户管理、设备管理、仪表盘统计、RBAC 角色权限、管理员认证等能力，并通过 Redis Pub/Sub 与 HTTP 双通道与 [ClipSync-Server](https://github.com/JH-Clipsync/ClipSync-Server) 联动，实现"改密码即踢下线、禁用设备即断连"。

配套前端为 [ClipSync-Admin-Web](https://github.com/JH-Clipsync/ClipSync-Admin-Web)（Vue 3 + Element Plus）。

---

## ✨ 核心功能

| 模块 | 说明 |
|------|------|
| 📊 **仪表盘** | 用户总数、活跃用户、管理员数量、角色数量统计 |
| 👤 **用户管理** | 用户列表（搜索/禁用过滤/分页）、详情、编辑、启用/禁用、重置密码（自动生成随机密码返回明文）、删除、主动踢下线；每个用户展示设备总数与在线数 |
| 📱 **设备管理** | 列出某用户全部设备（角色/平台/自定义名/最近 IP/在线状态）、跨用户分页搜索、启用/禁用设备、重命名设备、踢单台设备下线 |
| 🔌 **实时在线状态** | 设备列表优先调用 Server 的 `GET /server-admin/users/{id}/devices`，在线状态以 Server 内存 Hub 为准；Server 不可用时回退本地 MySQL + Redis |
| 📣 **Server 联动通知** | 重置密码/封禁用户/删除用户/禁用设备时，通过 Redis Pub/Sub（频道 `clipsync:admin:kick_user`）通知 Server 立即断连；Redis 不通时走 HTTP 兜底 |
| 🛡️ **RBAC 权限** | 管理员/角色/菜单/接口权限的完整 CRUD；角色-菜单、菜单-权限、管理员-角色多对多关系；接口级拦截；内置超管 `admin` 受保护不可删 |
| 🔐 **管理员认证** | JWT（TTL 2h，滑动刷新）、bcrypt 管理员密码哈希、登录失败计数锁定、可吊销 token（Redis 存 jti） |
| ✍️ **API 请求签名** | 每个请求都要带 HMAC-SHA256 签名（method/path/query/timestamp/nonce/bodyMD5），登录前用静态密钥 `sign_static_secret`，登录后切换为会话级动态密钥，防重放/防篡改 |
| 🖼️ **图片上传** | 管理员头像等图片上传，扩展名白名单 + 大小限制 + 按日期分目录存储 |
| 🧩 **共用数据库** | 与 ClipSync-Server 共用同一个 MySQL `clipsync` 库；业务表 `users`/`devices` 由 Server 建表，Admin 只迁移自己的 `admin_rbac_*` 表（统一加 `admin_` 前缀避免冲突） |
| 🔑 **双哈希体系** | 管理员密码用 bcrypt；业务用户密码用 scrypt（与 Server 完全兼容，参数 N=32768/r=8/p=1） |
| 🐳 **Docker 原生** | 多阶段构建 → distroless nonroot 镜像，host 网络直连宿主机 MySQL/Redis，监听 `:28002`；部署脚本自动从 Server 配置生成 admin 配置 |

---

## 🏗️ 技术栈

- **语言**：Go 1.22+
- **HTTP 框架**：[Gin](https://github.com/gin-gonic/gin) v1.10
- **ORM**：[GORM](https://gorm.io) v1.25 + `gorm.io/driver/mysql`
- **数据库**：MySQL 8（与 ClipSync-Server 共用）
- **缓存**：Redis（独立 db，默认 db=2）
- **JWT**：`golang-jwt/jwt/v5`，支持按 jti 吊销
- **配置**：[viper](https://github.com/spf13/viper)（YAML + `CLIPSYNC_ADMIN_` 环境变量前缀）
- **日志**：[zap](https://github.com/uber-go/zap)
- **密码哈希**：管理员 bcrypt / 业务用户 scrypt
- **镜像**：`gcr.io/distroless/base-debian12:nonroot`

---

## 🚀 快速开始

### 前置依赖

- MySQL 8（已由 ClipSync-Server 初始化 `clipsync` 库及 `users` / `sessions` / `devices` 表）
- Redis（与 Server 共用同一实例，Admin 使用 db=2）
- ClipSync-Server 已运行并配置好 `server.admin_token`

### 方式 1：Docker（推荐）

`deploy/deploy.sh` 会从 Server 的 `config.yaml` 自动读取 MySQL/Redis 连接信息并生成 Admin 的配置文件：

```bash
git clone https://github.com/JH-Clipsync/ClipSync-Admin.git
cd ClipSync-Admin

# 部署脚本默认把文件放到 /app/ClipSync/admin/api
sudo mkdir -p /app/ClipSync/admin/api/config /app/ClipSync/admin/api/uploads
sudo cp deploy/docker-compose.yml /app/ClipSync/admin/api/

# 从 Server 配置自动生成 config.yaml（脚本里已内置，这里手动演示也可）
# 编辑 config/config.yaml：确保 server.http_admin_token 与 Server 的 server.admin_token 一致

cd /app/ClipSync/admin/api
docker compose up -d
docker compose logs -f admin
```

默认监听 `:28002`。

### 方式 2：拉取官方镜像

```bash
docker run -d --name clipsync-admin \
  --network host \
  -v $(pwd)/config:/data/config:ro \
  -v $(pwd)/uploads:/data/uploads \
  -e TZ=Asia/Shanghai \
  ghcr.io/jh-clipsync/clipsync-admin:latest
```

镜像地址：[ghcr.io/jh-clipsync/clipsync-admin](https://github.com/orgs/JH-Clipsync/packages)

### 方式 3：源码运行

```bash
# 前置：Go 1.22+，可达的 MySQL 和 Redis
cp config.example.yaml config.yaml
# 编辑 config.yaml：
#   - mysql.dsn 指向 clipsync 库
#   - redis.addr / redis.db
#   - jwt.secret 改成随机串
#   - server.http_admin_token 与 Server 的 server.admin_token 一致

go mod tidy
go run .
# 或指定配置文件
go run . -c /path/to/config.yaml
```

### 默认账号

首次启动会自动播种超级管理员：

- 账号：`admin`
- 密码：`Admin**8`

**登录后请立即修改密码**。内置超管账号受保护，不允许删除。

---

## ⚙️ 配置详解

参考 [config.example.yaml](config.example.yaml)。配置文件可通过 `-c` 指定，或使用环境变量（前缀 `CLIPSYNC_ADMIN_`，如 `CLIPSYNC_ADMIN_APP_ADDR`）。

| 段 | 关键字段 | 说明 |
|---|---|---|
| `app` | `addr` / `mode` | 监听地址（默认 `:28002`）/ Gin 模式（`debug` / `release`） |
| `mysql` | `dsn` / `max_idle_conns` / `max_open_conns` | 与 Server 共用同一个 `clipsync` 库；Admin 只迁移 RBAC 表 |
| `redis` | `addr` / `password` / `db` | 默认 db=2，避免与 Server（db=0）冲突 |
| `jwt` | `secret` / `ttl` / `refresh_on_access` | JWT 签名密钥、有效期（秒，默认 7200=2h）、是否滑动刷新 |
| `security` | `bcrypt_cost` / `login_error_limit` / `login_error_ttl` / `sign_static_secret` | bcrypt cost、登录失败锁定阈值/时长、登录前签名静态密钥 |
| `cors` | `allow_origins` / `allow_credentials` | 允许的前端来源，本地开发默认 `http://localhost:5175` |
| `log` | `level` / `format` | 日志级别（debug/info/warn/error）/ 格式（console/json） |
| `bootstrap` | `super_admin_account` / `super_admin_password` / `super_admin_name` | 启动时自动播种的超管（已存在则跳过） |
| `upload` | `dir` / `url_prefix` / `max_size` / `allow_ext` | 上传目录、访问前缀、单文件上限、扩展名白名单 |
| `server` | `key_prefix` / `addr` / `http_admin_token` | 与 Server 联动：Redis key 前缀（须与 Server 一致）、Server HTTP 兜底地址、Server 端 admin_token |

### 与 Server 联动配置

Admin 通过两条通道通知 Server：

1. **Redis Pub/Sub（主通道）**：频道名 = `server.key_prefix + "admin:kick_user"`（默认 `clipsync:admin:kick_user`），零配置，最稳定。要求 Admin 与 Server 连接同一个 Redis 实例。
2. **HTTP 兜底**：当 Redis 不可用时，向 `server.addr + "/server-admin/kick"` POST 指令，携带 `Authorization: Bearer <server.http_admin_token>`。

设备列表/创建用户/重命名等**查询与写操作**则始终走 HTTP 调用 Server（`server.addr` 必须配置），Server 是 `devices` 表和内存 Hub 的权威源。

---

## 🔌 接口一览

所有接口统一前缀 `/api/admin`，必须通过签名校验中间件。

### 公开接口

| 路径 | 方法 | 说明 |
|---|---|---|
| `/api/admin/health` | GET | 健康检查 |
| `/api/admin/auth/login` | POST | 管理员登录（返回 JWT + 动态签名密钥） |

### 登录后接口（JWT 鉴权）

| 路径 | 方法 | 说明 |
|---|---|---|
| `/api/admin/auth/logout` | POST | 登出（吊销 jti） |
| `/api/admin/auth/me` | GET | 当前管理员信息 |
| `/api/admin/auth/menus` | GET | 当前管理员可见菜单 |
| `/api/admin/auth/password` | PUT | 修改自己的密码 |
| `/api/admin/auth/profile` | PUT | 修改自己的资料 |
| `/api/admin/upload/image` | POST | 图片上传 |

### RBAC 接口（JWT + 权限校验）

| 资源 | 操作 |
|---|---|
| 仪表盘 | `GET /dashboard` |
| 用户 | `GET/POST /users`、`GET/PUT/DELETE /users/:id`、`PUT /users/:id/status`、`POST /users/:id/reset-password`、`POST /users/:id/kick` |
| 用户设备 | `GET /users/:id/devices`、`PUT /users/:id/devices/:did`（启停）、`PUT /users/:id/devices/:did/name`（重命名）、`POST /users/:id/devices/:did/kick` |
| 全量设备 | `GET /devices`（跨用户搜索/过滤/分页） |
| 管理员 | `GET/POST /rbac/admins`、`PUT/DELETE /rbac/admins/:id`、`PUT /rbac/admins/:id/status`、`PUT /rbac/admins/:id/password`、`GET /rbac/admins/:id/roles` |
| 角色 | `GET/POST/PUT/DELETE /rbac/roles`、`PUT /rbac/roles/:id/menus`、`GET /rbac/roles/:id/menus` |
| 菜单 | `GET/POST/PUT/DELETE /rbac/menus`、`PUT /rbac/menus/:id/perms` |
| 权限 | `GET/POST/PUT/DELETE /rbac/perms` |

### 签名规则

待签名串（`\n` 分隔）：

```
METHOD\nPATH\nQUERY\nTIMESTAMP\nNONCE\nBODY_MD5
```

- `PATH` 是去掉 `/api/admin` 前缀的相对路径
- `QUERY` 按 key 字典序排列
- `TIMESTAMP` 毫秒时间戳，`NONCE` 16 字节随机 hex
- `BODY_MD5` 请求体 MD5（GET/无 body 为空串）

签名 = `HMAC-SHA256(secret, 待签串)` 的 hex 小写。前端逻辑见 [ClipSync-Admin-Web/src/utils/sign.ts](https://github.com/JH-Clipsync/ClipSync-Admin-Web)。

---

## 🔐 安全设计

| 维度 | 设计 |
|------|------|
| 管理员密码 | bcrypt（cost 默认 10），独立于业务用户 |
| 业务用户密码 | scrypt（N=32768/r=8/p=1），与 ClipSync-Server 完全兼容，重置密码时由 Admin 直接写库 |
| JWT 吊销 | 每个 token 带 jti，Redis 记录 jti→adminID，登出即删，实现可吊销 |
| 登录防爆破 | 单账号连续失败超阈值（默认 5 次）锁定一段时间（默认 15 分钟） |
| 请求签名 | 所有接口强制 HMAC-SHA256 签名，含时间戳与 nonce，防篡改/防重放；登录前静态密钥，登录后动态下发 |
| CORS | 白名单来源，不使用 `*` |
| 表名隔离 | Admin 自有的 RBAC 表统一 `admin_` 前缀，避免与业务表冲突 |
| 数据库权限 | 生产建议给 Admin 使用的 MySQL 账号仅授予 `clipsync` 库的 SELECT/UPDATE/INSERT/DELETE，不建表/不删库 |
| 镜像安全 | distroless nonroot（uid 65532），无 shell、无包管理器 |

---

## 🐳 部署架构

### Nginx 反代路径规划

[deploy/nginx.clipsync.conf](deploy/nginx.clipsync.conf) 给出了完整的同域路径规划：

```
/clipsync/admin/api/       → 127.0.0.1:28002/api/admin/   （Admin 后端 API）
/clipsync/admin/static/    → 127.0.0.1:28002/static/      （Admin 上传文件）
/clipsync/admin/           → /app/ClipSync/admin/web/      （Admin 前端静态文件）
/clipsync/ws               → 127.0.0.1:28001/ws            （Server WebSocket）
/clipsync/                 → 127.0.0.1:28001/              （Server 其他接口）
```

[deploy/deploy.sh](deploy/deploy.sh) 在部署时会自动把正确的 location 块写入 nginx 配置并 reload。

### 与 Server 的运行关系

```
          浏览器 (Vue 3 SPA)
                 │  HTTPS
                 ▼
        ┌── Nginx (/clipsync/admin/) ──┐
        │                              │
        ▼                              ▼
  ClipSync-Admin (:28002)       ClipSync-Server (:28001)
        │                              │
        │  ① Redis Pub/Sub             │
        ├──────────────┬───────────────┤
        │              │               │
        ▼              ▼               ▼
   MySQL (clipsync)   Redis db=0/2   内存 Hub（设备在线）
   共用 users/devices   Pub/Sub 频道
```

- Admin 与 Server **共享 MySQL 与 Redis**；
- Admin 写 `users`/`devices` 表后，通过 Pub/Sub 让 Server 真正执行踢连接/写设备状态（避免双写不一致）；
- Server 是设备在线状态的权威源，Admin 查询设备时优先 HTTP 调用 Server。

---

## 📁 项目结构

```
ClipSync-Admin/
├── main.go                       # 入口：加载配置 → 初始化 DB/Redis/JWT → 启动 Gin
├── config.example.yaml           # 配置模板
├── Dockerfile                    # 多阶段构建 → distroless nonroot
├── internal/
│   ├── config/                   # viper 配置加载
│   ├── db/                       # MySQL / Redis 连接初始化
│   ├── auth/
│   │   ├── jwt.go                # JWT 签发与校验
│   │   ├── password.go           # bcrypt 管理员 + scrypt 业务用户
│   │   └── sign.go               # HMAC-SHA256 请求签名
│   ├── model/
│   │   ├── base.go               # 公共列（status/is_del/c_by/...）
│   │   ├── biz.go                # User（映射 Server 的 users 表）
│   │   └── rbac.go               # Admin/Role/Menu/Perm 及关联表
│   ├── result/                   # 统一响应结构与错误码
│   ├── middleware/
│   │   ├── sign.go               # 全局签名校验
│   │   ├── rbac.go               # JWT + 接口权限拦截
│   │   └── common.go             # CORS / TraceID / AccessLog
│   ├── service/
│   │   ├── auth_service.go       # 管理员登录/会话/动态签名密钥
│   │   ├── rbac_service.go       # RBAC CRUD
│   │   ├── data_service.go       # 用户/设备/仪表盘
│   │   └── server_notifier.go    # Redis Pub/Sub + HTTP 双通道通知 Server
│   ├── handler/                  # HTTP 处理器（auth/data/rbac/upload）
│   ├── bootstrap/                # AutoMigrate + 超管/菜单/权限种子
│   ├── router/                   # 路由组装
│   └── logger/                   # zap 日志
├── deploy/
│   ├── deploy.sh                 # 一键部署（SSH 由 CI 调用）
│   ├── docker-compose.yml
│   ├── nginx.clipsync.conf       # 完整反代示例
│   └── .env.example
└── .github/workflows/docker-image.yml
```

---

## 🐛 故障排查

| 现象 | 排查 |
|------|------|
| 登录返回签名错误 | 检查前端 `VITE_SIGN_STATIC_SECRET` 是否与后端 `security.sign_static_secret` 一致；检查系统时间是否偏差过大 |
| 踢下线不生效 | Server 是否运行；`server.key_prefix` 是否与 Server 的 `redis.key_prefix` 一致；Server 是否配置了 `server.admin_token`；查看 Server 日志是否有"管理端事件订阅断开" |
| 设备列表在线状态不准 | Admin 调 Server HTTP 接口失败时会回退本地 Redis，检查 `server.addr` 与 `http_admin_token`；Server 内存 Hub 才是最权威的 |
| 创建用户失败 | `server.addr` 是否能从 Admin 容器/进程访问到 Server；创建用户走 HTTP 调用 Server，不是本地写库 |
| 容器时间不对 | docker-compose 已设置 `TZ=Asia/Shanghai`，确认宿主机时区正确 |
| RBAC 表未生成 | 检查 MySQL 账号是否有 CREATE TABLE 权限；首次启动会自动迁移 `admin_rbac_*` 表 |
| 上传图片 413 | nginx `client_max_body_size` 默认 1m，已在示例配置调到 12m；检查是否被外层网关限制 |

日志为 stdout（容器），通过 `docker compose logs -f admin` 查看。

---

## 🤝 相关项目

| 项目 | 技术栈 | 链接 |
|------|--------|------|
| 中转服务端 | Go + gorilla/websocket | [JH-Clipsync/ClipSync-Server](https://github.com/JH-Clipsync/ClipSync-Server) |
| 管理后台前端 | Vue 3 + Element Plus | [JH-Clipsync/ClipSync-Admin-Web](https://github.com/JH-Clipsync/ClipSync-Admin-Web) |
| Android 客户端 | Kotlin + OkHttp | [JH-Clipsync/ClipSync-Android](https://github.com/JH-Clipsync/ClipSync-Android) |
| macOS 客户端 | Swift + SwiftUI | [JH-Clipsync/ClipSync-Mac](https://github.com/JH-Clipsync/ClipSync-Mac) |
| Windows 客户端 | .NET 8 + WPF | [JH-Clipsync/ClipSync-Windows](https://github.com/JH-Clipsync/ClipSync-Windows) |

---

## 📄 License

个人自用项目，代码可自由参考修改。

---

**Made with ❤️ · 三端全自研 · 隐私归你自己**
