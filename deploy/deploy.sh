#!/usr/bin/env bash
# ClipSync Admin 自动部署脚本（由 GitHub Actions 通过 SSH 执行）
set -euo pipefail

DEPLOY_DIR="${DEPLOY_DIR:-/app/Clipsync/admin}"
VERSION="${VERSION:?VERSION is required}"
# Server 配置可能所在的路径（按优先级）
SERVER_CFG_CANDIDATES=(
  "/app/Clipsync/server/config.yaml"
  "/app/Clipsync/server/config.yml"
  "/app/Clipsync/config.yaml"
  "/opt/clipsync/config.yaml"
  "/opt/clipsync-server/config.yaml"
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

# 首次部署：拷贝 .env 模板
if [ ! -f "$DEPLOY_DIR/.env" ]; then
  $SUDO cp /tmp/clipsync-admin-deploy/deploy/.env.example "$DEPLOY_DIR/.env"
  echo "⚠ 已生成 .env 模板：$DEPLOY_DIR/.env"
fi

# ── config.yaml 自动生成/迁移 ──────────────────────────────────
CFG="$DEPLOY_DIR/config/config.yaml"

# 从 Server config.yaml 的指定段里提取标量值（兼容 gawk/mawk/busybox awk）
# 用法：cfg_get <file> <section> <key>
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

  # 生成随机密钥（首次）
  local jwt_secret
  jwt_secret=$(head -c 32 /dev/urandom | base64 | tr -d '/+=' | head -c 40)

  echo "==> 从 $server_cfg 读取到数据库配置：mysql=${MYSQL_USER}@${MYSQL_HOST}:${MYSQL_PORT}/${MYSQL_DB}, redis=${REDIS_ADDR}"

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

# 与 ClipSync-Server 联动：踢用户/踢设备/封禁用户时通知 Server 强制下线
server:
  key_prefix: "${KEY_PREFIX}"
  # 通过外层反向代理调用 Server /admin/kick 接口
  addr: "https://www.95qw.com/clipsync"
  # 与 Server 端 server.admin_token 保持一致
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
    echo "============================================================"
    echo "⚠ 未找到 ClipSync-Server 的 config.yaml，无法自动生成配置。"
    echo "  请手动创建：$CFG"
    echo "  可参考仓库 config.example.yaml"
    echo "  配置完成后执行：cd $DEPLOY_DIR && sudo docker compose restart admin"
    echo "============================================================"
    $SUDO cp /tmp/clipsync-admin-deploy/config.example.yaml "$CFG"
  fi
else
  # 已存在：把旧默认端口 :18082 迁移到 :28002
  if $SUDO grep -qE '^[[:space:]]*addr:[[:space:]]*":18082"' "$CFG"; then
    $SUDO sed -i -E 's|^([[:space:]]*addr:[[:space:]]*)":18082"|\1":28002"|' "$CFG"
    echo "已将 app.addr 从 :18082 迁移到 :28002"
  fi
  # 把 server.addr 从本地地址更新为线上域名（如果还是旧值）
  if $SUDO grep -qE '^[[:space:]]*addr:[[:space:]]*"http://127\.0\.0\.1:28001"' "$CFG"; then
    $SUDO sed -i -E 's|http://127\.0\.0\.1:28001|https://www.95qw.com/clipsync|g' "$CFG"
    echo "已将 server.addr 更新为 https://www.95qw.com/clipsync"
  fi
fi

cd "$DEPLOY_DIR"

# 清理旧的 tag 行，统一用 latest
$SUDO sed -i '/^ADMIN_TAG=/d;/^WEB_TAG=/d' .env
echo "ADMIN_TAG=latest" | $SUDO tee -a .env > /dev/null
echo "WEB_TAG=latest"   | $SUDO tee -a .env > /dev/null

# 清除过期 GHCR 凭据
$SUDO docker logout ghcr.io >/dev/null 2>&1 || true

# 拉取镜像
echo "==> docker compose pull..."
$SUDO docker compose pull admin web

# 启动
echo "==> docker compose up -d..."
$SUDO docker compose up -d admin web

# 清理
$SUDO docker image prune -f || true
rm -rf /tmp/clipsync-admin-deploy

echo ""
echo "==> 容器状态："
$SUDO docker compose ps
echo ""
echo "==> admin 最近日志："
$SUDO docker compose logs --tail=30 admin || true
