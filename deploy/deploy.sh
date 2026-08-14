#!/usr/bin/env bash
# ClipSync Admin 后端自动部署脚本（由 GitHub Actions 通过 SSH 执行）
set -euo pipefail

DEPLOY_DIR="${DEPLOY_DIR:-/app/ClipSync/server/admin/api}"
VERSION="${VERSION:?VERSION is required}"
# Server 配置可能所在的路径（按优先级）
SERVER_CFG_CANDIDATES=(
  "/app/ClipSync/server/config/config.yaml"
  "/app/ClipSync/server/config.yml"
  "/app/ClipSync/config/config.yaml"
  "/app/Clipsync/server/config/config.yaml"
  "/opt/clipsync/config/config.yaml"
)

echo "==> whoami: $(whoami)"
echo "==> 部署目录: $DEPLOY_DIR"
echo "==> 版本: $VERSION"
echo "==> 服务器架构: $(uname -m)"
echo "==> Docker 版本: $(docker --version)"

if [ "$(id -u)" = "0" ]; then
  SUDO=""
else
  SUDO="sudo"
fi

$SUDO mkdir -p "$DEPLOY_DIR/config" "$DEPLOY_DIR/uploads"

# 清理旧部署目录和旧容器（历史遗留）
if [ "$DEPLOY_DIR" != "/app/Clipsync/admin" ] && [ -d "/app/Clipsync/admin" ]; then
  echo "==> 清理旧部署目录 /app/Clipsync/admin"
  cd /app/Clipsync/admin 2>/dev/null && $SUDO docker compose down 2>/dev/null || true
  cd "$DEPLOY_DIR"
fi

# 每次 CI 都更新 docker-compose.yml
$SUDO cp /tmp/clipsync-admin-deploy/deploy/docker-compose.yml "$DEPLOY_DIR/docker-compose.yml"

# 清理旧的 web 容器（之前用容器跑前端，现在改为静态部署）
if $SUDO docker ps -a --format '{{.Names}}' | grep -q '^clipsync-admin-web$'; then
  echo "==> 删除旧的 clipsync-admin-web 容器（前端已改为静态部署）"
  $SUDO docker rm -f clipsync-admin-web >/dev/null 2>&1 || true
fi

# ── config.yaml 自动生成/迁移 ──────────────────────────────────
CFG="$DEPLOY_DIR/config/config.yaml"

cfg_get() {
  local file="$1" section="$2" key="$3"
  awk -v sec="$section" -v k="$key" '
    /^[a-zA-Z_][a-zA-Z0-9_]*:[[:space:]]*$/ { cur=$1; sub(/:$/,"",cur); next }
    /^[a-zA-Z_]/ { cur=""; next }
    cur==sec {
      line=$0
      sub(/^[[:space:]]+/,"",line)
      if (line ~ "^"k":[[:space:]]*") {
        val=line
        sub("^"k":[[:space:]]*\"?","",val)
        sub(/"[[:space:]]*$/,"",val)
        sub(/[[:space:]]+#.*/,"",val)
        print val
        exit
      }
    }
  ' "$file"
}

gen_config() {
  local server_cfg="$1"
  local mysql_host mysql_port mysql_user mysql_pass mysql_db
  local redis_addr redis_pass redis_db key_prefix admin_token jwt_secret

  mysql_host=$(cfg_get "$server_cfg" mysql host)
  mysql_port=$(cfg_get "$server_cfg" mysql port)
  mysql_user=$(cfg_get "$server_cfg" mysql user)
  mysql_pass=$(cfg_get "$server_cfg" mysql password)
  mysql_db=$(cfg_get "$server_cfg" mysql database)
  redis_addr=$(cfg_get "$server_cfg" redis addr)
  redis_pass=$(cfg_get "$server_cfg" redis password)
  redis_db=$(cfg_get "$server_cfg" redis db)
  key_prefix=$(cfg_get "$server_cfg" redis key_prefix)
  admin_token=$(cfg_get "$server_cfg" server admin_token)

  mysql_host="${mysql_host:-127.0.0.1}"
  mysql_port="${mysql_port:-3306}"
  mysql_user="${mysql_user:-clipsync}"
  mysql_pass="${mysql_pass:-}"
  mysql_db="${mysql_db:-clipsync}"
  redis_addr="${redis_addr:-127.0.0.1:6379}"
  redis_pass="${redis_pass:-}"
  redis_db="${redis_db:-1}"
  key_prefix="${key_prefix:-clipsync:}"
  admin_token="${admin_token:-}"
  jwt_secret=$(head -c 32 /dev/urandom | base64 | tr -d '/+=' | head -c 40)

  # DSN 中密码可能含特殊字符（如 **），直接拼进 YAML 双引号字符串即可
  echo "==> 从 $server_cfg 读取：mysql=${mysql_user}@${mysql_host}:${mysql_port}/${mysql_db}, redis=${redis_addr}/${redis_db}"

  cat <<EOF
app:
  name: clipsync-admin
  addr: ":28002"
  mode: release

mysql:
  dsn: "${mysql_user}:${mysql_pass}@tcp(${mysql_host}:${mysql_port})/${mysql_db}?charset=utf8mb4&parseTime=True&loc=Local"
  max_idle_conns: 10
  max_open_conns: 100
  conn_max_lifetime: 3600

redis:
  addr: "${redis_addr}"
  password: "${redis_pass}"
  db: ${redis_db}

jwt:
  secret: "${jwt_secret}"
  header: "Authorization"
  scheme: "Bearer"
  ttl: 7200
  refresh_on_access: true

security:
  bcrypt_cost: 10
  login_error_limit: 5
  login_error_ttl: 900
  sign_static_secret: "clipsync-admin-static-sign-secret-v1"

cors:
  allow_origins:
    - "https://www.95qw.com"
  allow_credentials: true

log:
  level: "info"
  format: "console"

bootstrap:
  super_admin_account: "admin"
  super_admin_password: "Admin**8"
  super_admin_name: "超级管理员"

upload:
  dir: "./data/uploads"
  url_prefix: "/clipsync/admin/static"
  max_size: 10485760
  allow_ext: [".jpg", ".jpeg", ".png", ".webp", ".gif"]

server:
  key_prefix: "${key_prefix}"
  addr: "https://www.95qw.com/clipsync"
  http_admin_token: "${admin_token}"
EOF
}

if [ ! -f "$CFG" ] || ! $SUDO grep -q "mysql:" "$CFG" || ! $SUDO grep -qE "^\s*dsn:" "$CFG"; then
  # 配置不存在、或不是 admin 合法配置（缺少 mysql.dsn），重新生成
  if [ -f "$CFG" ]; then
    echo "==> 旧的 config.yaml 结构不正确，备份并重新生成"
    $SUDO cp "$CFG" "${CFG}.bak.$(date +%s)"
  fi
  SERVER_CFG=""
  for c in "${SERVER_CFG_CANDIDATES[@]}"; do
    if [ -f "$c" ]; then SERVER_CFG="$c"; break; fi
  done

  if [ -n "$SERVER_CFG" ]; then
    echo "==> 找到 Server 配置：$SERVER_CFG，自动生成 admin config.yaml"
    gen_config "$SERVER_CFG" | $SUDO tee "$CFG" > /dev/null
  else
    echo "⚠ 未找到 Server config.yaml，请手动创建 $CFG"
  fi
else
  if $SUDO grep -qE '^[[:space:]]*addr:[[:space:]]*":18082"' "$CFG"; then
    $SUDO sed -i -E 's|^([[:space:]]*addr:[[:space:]]*)":18082"|\1":28002"|' "$CFG"
  fi
fi

cd "$DEPLOY_DIR"

# 用 latest 标签
$SUDO sed -i '/^ADMIN_TAG=/d' .env 2>/dev/null || true
echo "ADMIN_TAG=latest" | $SUDO tee -a .env > /dev/null

# 清除过期 GHCR 凭据
$SUDO docker logout ghcr.io >/dev/null 2>&1 || true

# 拉取并启动（配置可能变化，用 --force-recreate）
echo "==> docker compose pull admin..."
$SUDO docker compose pull admin
echo "==> docker compose up -d --force-recreate admin..."
$SUDO docker compose up -d --force-recreate admin

$SUDO docker image prune -f || true

echo ""
echo "==> 容器状态："
$SUDO docker compose ps
echo ""
echo "==> admin 最近日志："
$SUDO docker compose logs --tail=30 admin || true

# ── nginx 配置检查 ────────────────────────────────────────────
echo ""
echo "==> 检查 nginx 配置..."
NGINX_CHECKED=0
for f in /etc/nginx/conf.d/*.conf /etc/nginx/sites-enabled/*; do
  [ -f "$f" ] || continue
  if $SUDO grep -q "95qw" "$f" 2>/dev/null; then
    NGINX_CHECKED=1
    if ! $SUDO grep -q "/app/ClipSync/server/admin/web" "$f"; then
      echo "  ⚠ $f 里的静态路径还是旧路径，更新中..."
      $SUDO sed -i 's|/app/Clipsync/admin/web|/app/ClipSync/server/admin/web|g' "$f"
    fi
    if ! $SUDO grep -q "28002" "$f"; then
      echo "  ⚠ $f 里没有 28002 反代规则，请参考 deploy/nginx.clipsync.conf 手动添加"
    fi
  fi
done
if [ "$NGINX_CHECKED" = "1" ] && $SUDO nginx -t 2>/dev/null; then
  $SUDO nginx -s reload && echo "==> nginx 已 reload"
else
  echo "==> 参考配置：/tmp/clipsync-admin-deploy/deploy/nginx.clipsync.conf"
fi
rm -rf /tmp/clipsync-admin-deploy
