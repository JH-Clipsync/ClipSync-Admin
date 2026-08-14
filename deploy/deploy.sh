#!/usr/bin/env bash
# ClipSync Admin 后端自动部署脚本（由 GitHub Actions 通过 SSH 执行）
set -euo pipefail

DEPLOY_DIR="${DEPLOY_DIR:-/app/Clipsync/admin}"
VERSION="${VERSION:?VERSION is required}"
SERVER_CFG_CANDIDATES=(
  "/app/Clipsync/server/config.yaml"
  "/app/Clipsync/server/config.yml"
  "/app/Clipsync/config.yaml"
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
  local redis_addr redis_pass redis_db redis_user key_prefix admin_token

  mysql_host=$(cfg_get "$server_cfg" mysql host)
  mysql_port=$(cfg_get "$server_cfg" mysql port)
  mysql_user=$(cfg_get "$server_cfg" mysql user)
  mysql_pass=$(cfg_get "$server_cfg" mysql password)
  mysql_db=$(cfg_get "$server_cfg" mysql database)
  redis_addr=$(cfg_get "$server_cfg" redis addr)
  redis_pass=$(cfg_get "$server_cfg" redis password)
  redis_db=$(cfg_get "$server_cfg" redis db)
  redis_user=$(cfg_get "$server_cfg" redis username)
  key_prefix=$(cfg_get "$server_cfg" redis key_prefix)
  admin_token=$(cfg_get "$server_cfg" server admin_token)

  MYSQL_HOST="${mysql_host:-127.0.0.1}"
  MYSQL_PORT="${mysql_port:-3306}"
  MYSQL_USER="${mysql_user:-clipsync}"
  MYSQL_PASS="${mysql_pass:-clipsync}"
  MYSQL_DB="${mysql_db:-clipsync}"
  REDIS_ADDR="${redis_addr:-127.0.0.1:6379}"
  REDIS_PASS="${redis_pass:-}"
  REDIS_DB="${redis_db:-0}"
  REDIS_USER="${redis_user:-}"
  KEY_PREFIX="${key_prefix:-clipsync:}"
  ADMIN_TOKEN="${admin_token:-}"

  local jwt_secret
  jwt_secret=$(head -c 32 /dev/urandom | base64 | tr -d '/+=' | head -c 40)

  echo "==> 从 $server_cfg 读取：mysql=${MYSQL_USER}@${MYSQL_HOST}:${MYSQL_PORT}/${MYSQL_DB}, redis=${REDIS_ADDR}"

  cat <<EOF
# ClipSync-Admin 配置（由 deploy.sh 自动生成）
app:
  name: clipsync-admin
  addr: ":28002"
  mode: release

mysql:
  host: "${MYSQL_HOST}"
  port: ${MYSQL_PORT}
  username: "${MYSQL_USER}"
  password: "${MYSQL_PASS}"
  database: "${MYSQL_DB}"
  max_idle_conns: 10
  max_open_conns: 100
  conn_max_lifetime: 3600

redis:
  addr: "${REDIS_ADDR}"
  username: "${REDIS_USER}"
  password: "${REDIS_PASS}"
  db: ${REDIS_DB}
  pool_size: 20

logs:
  level: info
  filename: ""
  max_size: 50
  max_backups: 7
  max_age: 30
  stdout: true

server:
  key_prefix: "${KEY_PREFIX}"
  addr: "https://www.95qw.com/clipsync"
  http_admin_token: "${ADMIN_TOKEN}"

upload:
  dir: uploads
  url_prefix: "/clipsync/admin/static"
  max_size: 10

security:
  jwt_secret: "${jwt_secret}"
  jwt_ttl_hours: 24
  sign_static_secret: "clipsync-admin-static-sign-secret-v1"
  cors_allow_origins:
    - "*"
EOF
}

if [ ! -f "$CFG" ]; then
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

# 拉取并启动
echo "==> docker compose pull admin..."
$SUDO docker compose pull admin
echo "==> docker compose up -d admin..."
$SUDO docker compose up -d admin

$SUDO docker image prune -f || true
rm -rf /tmp/clipsync-admin-deploy

echo ""
echo "==> 容器状态："
$SUDO docker compose ps
echo ""
echo "==> admin 最近日志："
$SUDO docker compose logs --tail=30 admin || true
