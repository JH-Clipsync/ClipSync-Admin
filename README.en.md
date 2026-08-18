<h1 align="center">ClipSync-Admin</h1>

<p align="center">
  <b>Admin backend for ClipSync</b><br/>
  <a href="README.md">简体中文</a> ·
  <a href="README.en.md">English</a> ·
  <a href="README.ja.md">日本語</a>
</p>

---

ClipSync-Admin is the admin backend for the [ClipSync](https://github.com/JH-Clipsync) self-hosted cross-device messaging sync system, built with **Go + Gin + GORM + MySQL + Redis + JWT**. It targets ops / support / administrators and provides user management, device management, dashboard statistics, RBAC role permissions, administrator authentication, and more. It integrates with [ClipSync-Server](https://github.com/JH-Clipsync/ClipSync-Server) through both Redis Pub/Sub and HTTP channels, achieving "password reset means immediate kick, disabled device means immediate disconnect." It listens on port **28002** by default.

The companion frontend is [ClipSync-Admin-Web](https://github.com/JH-Clipsync/ClipSync-Admin-Web) (Vue 3 + Element Plus).

---

## ✨ Key Features

| Module | Description |
|--------|-------------|
| 📊 **Dashboard statistics** | Total users, active users (`disabled=0`), number of administrators, number of roles |
| 👤 **User management** | User list (search by username / filter by status / pagination), details, create, edit, enable / disable, reset password (randomly generates 10-character plaintext and returns it), physical delete, proactive kick; each user aggregates total devices and current online count |
| 📱 **Device management** | List all devices of a user (role / platform / custom name / last IP / online status), cross-user paginated search (fuzzy matching by username / device ID / name / IP), enable / disable devices, rename devices, kick a single device |
| 🔌 **Real-time online status** | The device list **prioritizes calling the Server via HTTP** at `GET /server-admin/users/{id}/devices` to obtain real-time online status (based on the Server's in-memory Hub); when the Server is unavailable, it automatically falls back to local MySQL + Redis |
| 📣 **Server integration notifications** | On password reset / user ban / user deletion / device disable / device kick, the Server is notified to disconnect immediately via Redis Pub/Sub (channel `clipsync:admin:kick_user`); if Redis is unavailable, it falls back to HTTP `POST {server.addr}/server-admin/kick` — double insurance |
| 🛡️ **RBAC permissions** | Full CRUD for admins / roles / menus / API permissions; many-to-many relationships for role-menu, menu-permission, admin-role; interface-level interception; the built-in super admin `admin` is protected and cannot be deleted |
| 🔐 **Admin authentication** | JWT (HS256, TTL 2 hours, sliding refresh), bcrypt admin password hashing (cost configurable), login failure count lockout, revocable tokens |
| ✍️ **API request signing** | Every request requires an HMAC-SHA256 signature (method / path / query / timestamp / nonce / bodyMD5). Before login, a static key `sign_static_secret` is used; after login, it switches to a session-level dynamic key to prevent replay and tampering |
| 🖼️ **Image upload** | Image uploads such as admin avatars, with extension whitelist + size limit + date-based directory storage, served externally as a static directory |
| 🗄️ **Shared database with Server** | Directly reads Server's `users` / `devices` tables; all admin-related tables use the `admin_` prefix to avoid interference |
| 🐳 **Docker-native** | Multi-stage build → distroless nonroot image; host networking to connect directly to the host's MySQL / Redis; `deploy.sh` can auto-generate the admin config from the Server config |

---

## 🚀 Quick Start

### Prerequisites

- [ClipSync-Server](https://github.com/JH-Clipsync/ClipSync-Server) is already deployed and can connect to the same MySQL / Redis instance
- The `clipsync` database already exists in MySQL (Server startup will auto-create the `users` / `sessions` / `devices` tables)
- Docker (recommended) or Go 1.22+

### Option 1: Docker Compose (recommended)

```bash
# 1. Prepare the deployment directory
mkdir -p /app/ClipSync/admin/api/config /app/ClipSync/admin/api/uploads
cd /app/ClipSync/admin/api

# 2. Copy the compose file
cp deploy/docker-compose.yml .
cp deploy/.env.example .env

# 3. Auto-generate the admin config.yaml from the Server config
#    The script looks for the Server config in this order:
#      /app/ClipSync/server/config/config.yaml
#      /app/ClipSync/server/config.yml
#      /app/ClipSync/config/config.yaml
#      /app/Clipsync/server/config/config.yaml
#      /opt/clipsync/config/config.yaml
bash deploy/deploy.sh
```

`deploy.sh` will:

1. Automatically read MySQL / Redis / `key_prefix` / `admin_token` from the Server `config.yaml`
2. Generate local `config/config.yaml`, with the JWT secret randomly generated from `/dev/urandom`
3. Detect and repair config corruption caused by historical migration scripts (e.g., `redis.addr` mistakenly changed to an http URL)
4. `docker compose pull && up -d --force-recreate`
5. Check and repair the nginx config (API paths, static resource locations)

Default super admin account:

```
Account: admin
Password: Admin**8
```

> Please change the password immediately after first login.

### Option 2: Docker one-liner

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

### Option 3: Run from source

```bash
git clone https://github.com/JH-Clipsync/ClipSync-Admin.git
cd ClipSync-Admin

# Prepare config
cp config.example.yaml config.yaml
vim config.yaml   # Fill in MySQL / Redis / JWT secret / Server integration info

# Run
go run .

# Or build
CGO_ENABLED=0 go build -ldflags "-s -w" -o clipsync-admin .
./clipsync-admin -c config.yaml
```

On startup it will automatically:

- Migrate all `admin_`-prefixed RBAC tables
- Seed the super admin account `admin / Admin**8` (skipped if it already exists)
- Seed default menus and permission nodes

---

## ⚙️ Configuration

See [config.example.yaml](config.example.yaml) for a config example. It is loaded with viper and supports environment variables (prefix `CLIPSYNC_ADMIN_`).

```yaml
app:
  name: clipsync-admin
  addr: ":28002"        # Listen port
  mode: debug           # gin mode: debug / release / test

mysql:
  # Shares the same database as ClipSync-Server
  dsn: "clipsync:clipsync@tcp(127.0.0.1:3306)/clipsync?charset=utf8mb4&parseTime=True&loc=Local"
  max_idle_conns: 10
  max_open_conns: 100
  conn_max_lifetime: 3600

redis:
  addr: "127.0.0.1:6379"
  password: ""
  db: 2                 # Recommend a different db from Server to avoid key conflicts (despite the prefix)

jwt:
  secret: "change-me-clipsync-admin-secret"   # ⚠️ Must be changed in production
  header: "Authorization"
  scheme: "Bearer"
  ttl: 7200             # Token validity in seconds, default 2 hours
  refresh_on_access: true

security:
  bcrypt_cost: 10       # bcrypt cost for admin passwords
  login_error_limit: 5  # Number of failed logins before lockout
  login_error_ttl: 900  # Failure count window in seconds
  # Signing secret for pre-login endpoints (frontend hardcodes the same string; after successful login it switches to a dynamic key)
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
  super_admin_name: "Super Administrator"

upload:
  dir: "./data/uploads"
  url_prefix: "/static"
  max_size: 10485760    # 10MB
  allow_ext: [".jpg", ".jpeg", ".png", ".webp", ".gif"]

# Integration channel with ClipSync-Server
server:
  key_prefix: "clipsync:"         # Must match Server's redis.key_prefix
  addr: "http://127.0.0.1:28001"  # Server HTTP address (for HTTP fallback)
  http_admin_token: ""            # Must match Server's server.admin_token
```

### Key configuration notes

| Config | Description |
|--------|-------------|
| `mysql.dsn` | Must connect to the same database as Server; Admin only auto-migrates `admin_*` tables and will not alter the `users` / `devices` table structures |
| `redis.db` | Recommend choosing a different db from Server (Server defaults to db=0, Admin example uses db=2) to avoid stepping on each other; `key_prefix` must exactly match Server to receive Pub/Sub messages |
| `jwt.secret` | Generate with `openssl rand -hex 32` or `head -c 32 /dev/urandom \| base64` |
| `server.key_prefix` | Determines the Pub/Sub channel name `{prefix}admin:kick_user`; must match Server |
| `server.addr` | Fallback channel for HTTP-based device list retrieval, user creation, and device renaming; recommend filling in an internal / localhost address |
| `server.http_admin_token` | Bearer Token for the HTTP fallback; must exactly match Server's `server.admin_token` |

---

## 🏗️ Project Architecture

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
             │ users/     │       │ online /   │       │  (HTTP fallback)│
             │ devices/   │       │ Pub/Sub /  │       │ /server-admin/*│
             │ admin_*    │       │ JWT/jti    │       └────────────────┘
             └────────────┘       └────────────┘
```

### Layered structure

```
internal/
├── main.go
├── config/           # Config struct + viper loading
├── router/           # Route assembly, middleware mounting
├── middleware/       # JWT auth, RBAC, signature verification, CORS, TraceID, access logs
├── handler/          # HTTP controllers (auth / data / rbac / upload)
├── service/          # Business logic
│   ├── auth_service.go
│   ├── data_service.go       # Users / devices / dashboard
│   ├── rbac_service.go
│   └── server_notifier.go    # Redis Pub/Sub + HTTP dual-channel delivery
├── model/            # GORM models (biz shares Server tables, rbac uses admin_ prefix)
├── auth/             # JWT, bcrypt + scrypt password hashing, HMAC signing
├── bootstrap/        # Table migration, super admin and menu seeding
├── db/               # MySQL / Redis connection initialization
├── logger/           # zap logger
└── result/           # Unified response structure and error codes
```

### Data model

- **Tables shared with Server**: `users`, `devices` (Admin only writes to some fields; table structure is maintained by Server)
- **RBAC tables (all with `admin_` prefix)**:
  - `admin_rbac_admin`: administrators
  - `admin_rbac_role`: roles
  - `admin_rbac_admin_role`: admin ↔ role
  - `admin_rbac_menu`: menus / buttons / data columns
  - `admin_rbac_perm`: API permissions (route + method unique)
  - `admin_rbac_role_menu`: role ↔ menu
  - `admin_rbac_menu_perm`: menu ↔ permission

### Integration mechanism with Server

| Trigger action | Redis Pub/Sub | HTTP fallback |
|----------------|---------------|---------------|
| Reset user password | ✅ `kick_user` / `password_reset` | ✅ |
| Disable user | ✅ `kick_user` / `user_disabled` | ✅ |
| Delete user | ✅ `kick_user` / `user_deleted` | ✅ |
| Proactively kick user | ✅ `kick_user` / `device_kicked` | ✅ |
| Disable device | ✅ `disable_device` | ✅ |
| Enable device | ✅ `enable_device` | ✅ |
| Kick single device | ✅ `kick_device` | ✅ |
| Rename device | — | ✅ `PUT /server-admin/users/{id}/devices/{did}/name` |
| Create user | — | ✅ `POST /server-admin/users` (password is hashed by Server using scrypt) |
| Device list / online status | — | ✅ `GET /server-admin/users/{id}/devices` and `GET /server-admin/devices` |

The device list **prioritizes Server HTTP**, because online status is most authoritative in Server's in-memory Hub; it only falls back to local MySQL + Redis queries when Server is unreachable.

---

## 🌐 Deployment

### Docker Compose (host networking)

[deploy/docker-compose.yml](deploy/docker-compose.yml) uses host networking; the container listens directly on host `:28002` and can connect to Server and MySQL / Redis via `127.0.0.1`:

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

### Nginx reverse proxy

See [deploy/nginx.clipsync.conf](deploy/nginx.clipsync.conf) for a full example:

```nginx
# Admin backend API
location ^~ /clipsync/admin/api/ {
    proxy_pass http://127.0.0.1:28002/api/admin/;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}

# Admin uploaded files
location ^~ /clipsync/admin/static/ {
    proxy_pass http://127.0.0.1:28002/static/;
}

# Admin frontend static files
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

### Directory layout (production)

```
/app/ClipSync/
├── server/              # ClipSync-Server
│   └── config/config.yaml
└── admin/
    ├── api/             # This project
    │   ├── config/config.yaml
    │   ├── uploads/
    │   └── docker-compose.yml
    └── web/             # ClipSync-Admin-Web build output (static files)
        ├── index.html
        └── assets/
```

---

## 🔐 Security Notes

- **Admin passwords**: bcrypt hash, default cost 10 (adjustable via `security.bcrypt_cost`)
- **Business user passwords**: When Admin resets a user password, it uses exactly the same **scrypt** hash as Server (N=32768, r=8, p=1), ensuring the shared database is readable
- **JWT**: HS256 signed, payload contains `aid` (admin ID) + `acc` (account) + `jti`; supports Redis revocation and sliding refresh
- **Login lockout**: By default, 5 consecutive failures within 15 minutes locks the account
- **API signature (HMAC-SHA256)**:
  - String to sign: `METHOD\nPATH\nQUERY\nTIMESTAMP\nNONCE\nBODY_MD5`
  - `PATH` is the relative path with the `/api/admin` prefix stripped
  - `QUERY` is sorted by key in lexicographic order
  - Before login, the static key `sign_static_secret` is used; after successful login, the server issues a session-level random `signSecret`, which becomes invalid on logout
  - Signature comparison uses `hmac.Equal`, constant-time to prevent timing attacks
- **CORS**: By default only configured origins are allowed; in production be sure to change `cors.allow_origins` to the actual frontend domain
- **Server communication**: The HTTP fallback must include `Authorization: Bearer <server.admin_token>`; if left empty it will be rejected by Server; Redis Pub/Sub runs on the internal network and is not exposed to the public internet
- **Container security**: distroless nonroot image, no shell, runs as non-root user
- **Default password**: The super admin default password `Admin**8` is only for first startup; **it must be changed immediately after deployment**; `jwt.secret` and `sign_static_secret` must also be replaced

---

## 🤝 Related Projects

- [ClipSync-Server](https://github.com/JH-Clipsync/ClipSync-Server): Three-endpoint sync relay server (Go + gorilla/websocket)
- [ClipSync-Admin-Web](https://github.com/JH-Clipsync/ClipSync-Admin-Web): Admin frontend (Vue 3 + Element Plus)
- [ClipSync-Windows](https://github.com/JH-Clipsync/ClipSync-Windows): Windows client
- [ClipSync-Mac](https://github.com/JH-Clipsync/ClipSync-Mac): macOS client
- [ClipSync-Android](https://github.com/JH-Clipsync/ClipSync-Android): Android client
