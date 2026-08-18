<h1 align="center">ClipSync-Admin</h1>

<p align="center">
  <b>Admin backend for the ClipSync system</b><br/>
  <a href="README.md">简体中文</a> ·
  <a href="README.en.md">English</a> ·
  <a href="README.ja.md">日本語</a>
</p>

---

ClipSync-Admin is the admin dashboard backend for the self-hosted [ClipSync](https://github.com/JH-Clipsync) cross-device messaging system. It is built with **Go + Gin + GORM + MySQL + Redis + JWT** and targets ops / support / administrators. It provides dashboard stats, user management, device management, RBAC (admin / role / menu / permission) and administrator authentication, and integrates with [ClipSync-Server](https://github.com/JH-Clipsync/ClipSync-Server) over both Redis Pub/Sub and HTTP so that "reset password" instantly kicks devices and "ban device" instantly disconnects them.

The companion frontend is [ClipSync-Admin-Web](https://github.com/JH-Clipsync/ClipSync-Admin-Web) (Vue 3 + Element Plus).

---

## ✨ Features

| Area | What it does |
|------|--------------|
| 📊 **Dashboard** | Totals for users, active users, admins and roles |
| 👤 **User management** | List (search / disabled filter / paginated), detail, edit, enable/disable, reset password (auto-generated, plaintext returned once), delete, force-kick; each user shows total devices and online count |
| 📱 **Device management** | List all devices of a user (role / platform / custom name / last IP / online status), cross-user paginated search, enable/disable, rename, kick a single device |
| 🔌 **Real-time online status** | The device list first calls Server's `GET /server-admin/users/{id}/devices` (online state sourced from Server's in-memory Hub); falls back to local MySQL + Redis when Server is unavailable |
| 📣 **Server integration** | On password reset / user ban / user deletion / device ban, notifies Server via Redis Pub/Sub (channel `clipsync:admin:kick_user`) to disconnect immediately; HTTP fallback if Redis is down |
| 🛡️ **RBAC** | Full CRUD for admins / roles / menus / API permissions; many-to-many relations for role-menu, menu-permission, admin-role; endpoint-level enforcement; built-in superadmin `admin` is protected from deletion |
| 🔐 **Admin authentication** | JWT (2h TTL with sliding refresh), bcrypt for admin passwords, failed-attempt lockout, revocable tokens (jti stored in Redis) |
| ✍️ **API request signing** | Every request must carry an HMAC-SHA256 signature (method/path/query/timestamp/nonce/bodyMD5). Pre-login uses a static `sign_static_secret`; post-login switches to a per-session dynamic secret to prevent tampering and replay |
| 🖼️ **Image upload** | Admin avatars etc.; extension whitelist + size limit + date-partitioned storage |
| 🧩 **Shared database** | Shares the same MySQL `clipsync` database with ClipSync-Server. Business tables (`users` / `devices`) are owned by Server; Admin only migrates its own `admin_rbac_*` tables (all prefixed `admin_` to avoid collisions) |
| 🔑 **Dual hashing** | Admin passwords use bcrypt; business user passwords use scrypt (fully compatible with Server, N=32768/r=8/p=1) |
| 🐳 **Docker-native** | Multi-stage build producing a distroless nonroot image, host networking for direct MySQL/Redis access, listens on `:28002`; deploy script auto-generates the admin config from Server's `config.yaml` |

---

## 🏗️ Tech Stack

- **Language**: Go 1.22+
- **HTTP framework**: [Gin](https://github.com/gin-gonic/gin) v1.10
- **ORM**: [GORM](https://gorm.io) v1.25 + `gorm.io/driver/mysql`
- **Database**: MySQL 8 (shared with ClipSync-Server)
- **Cache**: Redis (separate DB, db=2 by default)
- **JWT**: `golang-jwt/jwt/v5`, revocable by jti
- **Config**: [viper](https://github.com/spf13/viper) (YAML + `CLIPSYNC_ADMIN_` env prefix)
- **Logger**: [zap](https://github.com/uber-go/zap)
- **Password hashing**: bcrypt for admins / scrypt for business users
- **Image**: `gcr.io/distroless/base-debian12:nonroot`

---

## 🚀 Quick Start

### Prerequisites

- MySQL 8 (with the `clipsync` database and `users` / `sessions` / `devices` tables already initialized by ClipSync-Server)
- Redis (same instance as Server; Admin uses db=2)
- ClipSync-Server running with `server.admin_token` configured

### Option 1: Docker (recommended)

`deploy/deploy.sh` auto-reads MySQL/Redis connection info from Server's `config.yaml` and generates the admin config:

```bash
git clone https://github.com/JH-Clipsync/ClipSync-Admin.git
cd ClipSync-Admin

# Deploy script places files under /app/ClipSync/admin/api by default
sudo mkdir -p /app/ClipSync/admin/api/config /app/ClipSync/admin/api/uploads
sudo cp deploy/docker-compose.yml /app/ClipSync/admin/api/

# config/config.yaml is auto-generated from Server config by the deploy script;
# make sure server.http_admin_token matches Server's server.admin_token

cd /app/ClipSync/admin/api
docker compose up -d
docker compose logs -f admin
```

Default listen address is `:28002`.

### Option 2: Pull the official image

```bash
docker run -d --name clipsync-admin \
  --network host \
  -v $(pwd)/config:/data/config:ro \
  -v $(pwd)/uploads:/data/uploads \
  -e TZ=Asia/Shanghai \
  ghcr.io/jh-clipsync/clipsync-admin:latest
```

Image registry: [ghcr.io/jh-clipsync/clipsync-admin](https://github.com/orgs/JH-Clipsync/packages)

### Option 3: Run from source

```bash
# Requires Go 1.22+ and reachable MySQL / Redis
cp config.example.yaml config.yaml
# Edit config.yaml:
#   - mysql.dsn points to the clipsync database
#   - redis.addr / redis.db
#   - jwt.secret to a random string
#   - server.http_admin_token matches Server's server.admin_token

go mod tidy
go run .
# Or specify a config file
go run . -c /path/to/config.yaml
```

### Default credentials

On first startup a superadmin is seeded automatically:

- Username: `admin`
- Password: `Admin**8`

**Change the password immediately after first login.** The built-in superadmin is protected and cannot be deleted.

---

## ⚙️ Configuration

See [config.example.yaml](config.example.yaml). The config file can be selected with `-c`, or you can use environment variables with the `CLIPSYNC_ADMIN_` prefix (e.g. `CLIPSYNC_ADMIN_APP_ADDR`).

| Section | Key fields | Description |
|---|---|---|
| `app` | `addr` / `mode` | Listen address (default `:28002`) / Gin mode (`debug` / `release`) |
| `mysql` | `dsn` / `max_idle_conns` / `max_open_conns` | Same `clipsync` DB as Server; Admin only migrates RBAC tables |
| `redis` | `addr` / `password` / `db` | Default db=2 to avoid conflict with Server (db=0) |
| `jwt` | `secret` / `ttl` / `refresh_on_access` | JWT signing secret, lifetime in seconds (default 7200=2h), sliding refresh |
| `security` | `bcrypt_cost` / `login_error_limit` / `login_error_ttl` / `sign_static_secret` | bcrypt cost, failed-login lock threshold/duration, pre-login static signing secret |
| `cors` | `allow_origins` / `allow_credentials` | Allowed frontend origins; local dev defaults to `http://localhost:5175` |
| `log` | `level` / `format` | Log level (debug/info/warn/error) / format (console/json) |
| `bootstrap` | `super_admin_account` / `super_admin_password` / `super_admin_name` | Superadmin seeded at startup (skipped if already exists) |
| `upload` | `dir` / `url_prefix` / `max_size` / `allow_ext` | Upload dir, URL prefix, per-file max, extension whitelist |
| `server` | `key_prefix` / `addr` / `http_admin_token` | Server integration: Redis key prefix (must match Server's), Server HTTP fallback address, Server admin token |

### Server integration

Admin talks to Server over two channels:

1. **Redis Pub/Sub (primary)**: channel = `server.key_prefix + "admin:kick_user"` (default `clipsync:admin:kick_user`). Zero config, most reliable. Requires Admin and Server to connect to the same Redis instance.
2. **HTTP fallback**: when Redis is unavailable, POSTs commands to `server.addr + "/server-admin/kick"` with `Authorization: Bearer <server.http_admin_token>`.

For **queries and writes** such as listing devices, creating users or renaming devices, Admin always calls Server over HTTP (`server.addr` must be set); Server is the authority for the `devices` table and the in-memory Hub.

---

## 🔌 API Reference

All endpoints are prefixed with `/api/admin` and must pass the signature middleware.

### Public endpoints

| Path | Method | Description |
|---|---|---|
| `/api/admin/health` | GET | Health check |
| `/api/admin/auth/login` | POST | Admin login (returns JWT + dynamic signing secret) |

### Authenticated endpoints (JWT)

| Path | Method | Description |
|---|---|---|
| `/api/admin/auth/logout` | POST | Logout (revokes jti) |
| `/api/admin/auth/me` | GET | Current admin info |
| `/api/admin/auth/menus` | GET | Menus visible to the current admin |
| `/api/admin/auth/password` | PUT | Change own password |
| `/api/admin/auth/profile` | PUT | Update own profile |
| `/api/admin/upload/image` | POST | Image upload |

### RBAC endpoints (JWT + permission check)

| Resource | Operations |
|---|---|
| Dashboard | `GET /dashboard` |
| Users | `GET/POST /users`, `GET/PUT/DELETE /users/:id`, `PUT /users/:id/status`, `POST /users/:id/reset-password`, `POST /users/:id/kick` |
| User devices | `GET /users/:id/devices`, `PUT /users/:id/devices/:did` (enable/disable), `PUT /users/:id/devices/:did/name` (rename), `POST /users/:id/devices/:did/kick` |
| All devices | `GET /devices` (cross-user search/filter/paginate) |
| Admins | `GET/POST /rbac/admins`, `PUT/DELETE /rbac/admins/:id`, `PUT /rbac/admins/:id/status`, `PUT /rbac/admins/:id/password`, `GET /rbac/admins/:id/roles` |
| Roles | `GET/POST/PUT/DELETE /rbac/roles`, `PUT /rbac/roles/:id/menus`, `GET /rbac/roles/:id/menus` |
| Menus | `GET/POST/PUT/DELETE /rbac/menus`, `PUT /rbac/menus/:id/perms` |
| Permissions | `GET/POST/PUT/DELETE /rbac/perms` |

### Signature rules

String to sign (separated by `\n`):

```
METHOD\nPATH\nQUERY\nTIMESTAMP\nNONCE\nBODY_MD5
```

- `PATH` is the relative path with the `/api/admin` prefix stripped
- `QUERY` is sorted by key in lexicographic order
- `TIMESTAMP` is milliseconds; `NONCE` is 16 random bytes in hex
- `BODY_MD5` is the MD5 of the request body (empty string for GET / no body)

Signature = lowercase hex of `HMAC-SHA256(secret, stringToSign)`. The frontend implementation lives in [ClipSync-Admin-Web/src/utils/sign.ts](https://github.com/JH-Clipsync/ClipSync-Admin-Web).

---

## 🔐 Security

| Aspect | Design |
|--------|--------|
| Admin passwords | bcrypt (cost 10 by default), independent from business users |
| Business user passwords | scrypt (N=32768/r=8/p=1), fully compatible with ClipSync-Server; Admin writes these directly when resetting passwords |
| JWT revocation | Each token carries a jti; Redis maps jti→adminID and deletion on logout revokes the token |
| Brute-force defense | Consecutive failures beyond a threshold (default 5) lock the account for a period (default 15 minutes) |
| Request signing | All endpoints require HMAC-SHA256 signature with timestamp + nonce (anti-tamper/anti-replay). Static secret before login, dynamic secret issued after login |
| CORS | Whitelisted origins, never `*` |
| Table isolation | Admin's own RBAC tables are all prefixed `admin_` to avoid collisions with business tables |
| DB privileges | In production, grant the MySQL account used by Admin only SELECT/UPDATE/INSERT/DELETE on the `clipsync` database — no DDL, no DROP |
| Image hardening | distroless nonroot (uid 65532), no shell, no package manager |

---

## 🐳 Deployment Architecture

### Nginx reverse-proxy routing

[deploy/nginx.clipsync.conf](deploy/nginx.clipsync.conf) shows a complete same-origin path layout:

```
/clipsync/admin/api/       → 127.0.0.1:28002/api/admin/   (Admin API)
/clipsync/admin/static/    → 127.0.0.1:28002/static/      (Admin uploads)
/clipsync/admin/           → /app/ClipSync/admin/web/      (Admin SPA static)
/clipsync/ws               → 127.0.0.1:28001/ws            (Server WebSocket)
/clipsync/                 → 127.0.0.1:28001/              (Server other APIs)
```

[deploy/deploy.sh](deploy/deploy.sh) writes the correct location blocks into the nginx config and reloads nginx during deployment.

### How it runs with Server

```
          Browser (Vue 3 SPA)
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
   MySQL (clipsync)   Redis db=0/2   In-memory Hub (device presence)
   shared users/      Pub/Sub channel
   devices
```

- Admin and Server **share MySQL and Redis**;
- After Admin writes the `users` / `devices` tables, it uses Pub/Sub to tell Server to actually perform the disconnect / status update (avoiding dual-write inconsistency);
- Server is the authority for device online status; Admin queries it over HTTP first.

---

## 📁 Project Structure

```
ClipSync-Admin/
├── main.go                       # Entry: load config → init DB/Redis/JWT → start Gin
├── config.example.yaml           # Config template
├── Dockerfile                    # Multi-stage build → distroless nonroot
├── internal/
│   ├── config/                   # viper config loader
│   ├── db/                       # MySQL / Redis connection init
│   ├── auth/
│   │   ├── jwt.go                # JWT issue/verify
│   │   ├── password.go           # bcrypt for admins + scrypt for business users
│   │   └── sign.go               # HMAC-SHA256 request signing
│   ├── model/
│   │   ├── base.go               # Common columns (status/is_del/c_by/...)
│   │   ├── biz.go                # User (maps to Server's users table)
│   │   └── rbac.go               # Admin/Role/Menu/Perm and join tables
│   ├── result/                   # Unified response structure & error codes
│   ├── middleware/
│   │   ├── sign.go               # Global signature verification
│   │   ├── rbac.go               # JWT + endpoint permission enforcement
│   │   └── common.go             # CORS / TraceID / AccessLog
│   ├── service/
│   │   ├── auth_service.go       # Admin login/session/dynamic signing secret
│   │   ├── rbac_service.go       # RBAC CRUD
│   │   ├── data_service.go       # User/device/dashboard data
│   │   └── server_notifier.go    # Redis Pub/Sub + HTTP dual-channel Server notifier
│   ├── handler/                  # HTTP handlers (auth/data/rbac/upload)
│   ├── bootstrap/                # AutoMigrate + superadmin/menu/perm seeding
│   ├── router/                   # Route assembly
│   └── logger/                   # zap logger
├── deploy/
│   ├── deploy.sh                 # One-shot deploy (SSH-invoked by CI)
│   ├── docker-compose.yml
│   ├── nginx.clipsync.conf       # Full reverse-proxy sample
│   └── .env.example
└── .github/workflows/docker-image.yml
```

---

## 🐛 Troubleshooting

| Symptom | What to check |
|---------|---------------|
| Login returns signature error | Is the frontend's `VITE_SIGN_STATIC_SECRET` equal to the backend's `security.sign_static_secret`? Is the system clock skewed too much? |
| Kick doesn't take effect | Is Server running? Does `server.key_prefix` match Server's `redis.key_prefix`? Does Server have `server.admin_token` set? Check Server logs for "admin event subscriber disconnected" |
| Device online status is inaccurate | When Admin can't reach Server's HTTP API it falls back to local Redis; verify `server.addr` and `http_admin_token`. Server's in-memory Hub is the most authoritative source |
| User creation fails | Can Admin reach Server from its container/process at `server.addr`? User creation goes through Server over HTTP — not a local DB insert |
| Container time is wrong | docker-compose sets `TZ=Asia/Shanghai`; confirm the host timezone is correct |
| RBAC tables missing | Does the MySQL account have CREATE TABLE privilege? On first startup the `admin_rbac_*` tables are auto-migrated |
| Image upload returns 413 | nginx `client_max_body_size` defaults to 1m and is raised to 12m in the sample config; check whether an outer gateway limits it |

Logs go to stdout (container) — view with `docker compose logs -f admin`.

---

## 🤝 Related Projects

| Project | Stack | Link |
|---------|-------|------|
| Relay server | Go + gorilla/websocket | [JH-Clipsync/ClipSync-Server](https://github.com/JH-Clipsync/ClipSync-Server) |
| Admin frontend | Vue 3 + Element Plus | [JH-Clipsync/ClipSync-Admin-Web](https://github.com/JH-Clipsync/ClipSync-Admin-Web) |
| Android client | Kotlin + OkHttp | [JH-Clipsync/ClipSync-Android](https://github.com/JH-Clipsync/ClipSync-Android) |
| macOS client | Swift + SwiftUI | [JH-Clipsync/ClipSync-Mac](https://github.com/JH-Clipsync/ClipSync-Mac) |
| Windows client | .NET 8 + WPF | [JH-Clipsync/ClipSync-Windows](https://github.com/JH-Clipsync/ClipSync-Windows) |

---

## 📄 License

Personal project — feel free to study, fork, and modify.

---

**Made with ❤️ · Fully self-built across all platforms · Your data stays yours**
