#!/usr/bin/env bash
# ClipSync Admin 自动部署脚本（由 GitHub Actions 通过 SSH 执行）
# 必需环境变量：DEPLOY_DIR、VERSION
set -euo pipefail

DEPLOY_DIR="${DEPLOY_DIR:-/opt/clipsync-admin}"
VERSION="${VERSION:?VERSION is required}"

echo "==> whoami: $(whoami)"
echo "==> 部署目录: $DEPLOY_DIR"
echo "==> 版本: $VERSION"
echo "==> 服务器架构: $(uname -m)"
echo "==> Docker 版本: $(docker --version)"

# 已经是 root 就不用 sudo
if [ "$(id -u)" = "0" ]; then
  SUDO=""
else
  SUDO="sudo"
fi
echo "==> 使用前缀: ${SUDO:-（无）}"

$SUDO mkdir -p "$DEPLOY_DIR/config" "$DEPLOY_DIR/uploads"

# 用上传的文件覆盖 docker-compose.yml（每次 CI 都更新）
$SUDO cp /tmp/clipsync-admin-deploy/deploy/docker-compose.yml "$DEPLOY_DIR/docker-compose.yml"

# 首次部署时若 .env 不存在，从 example 拷贝
if [ ! -f "$DEPLOY_DIR/.env" ]; then
  $SUDO cp /tmp/clipsync-admin-deploy/deploy/.env.example "$DEPLOY_DIR/.env"
  echo "⚠ 已生成 .env 模板：$DEPLOY_DIR/.env"
fi

# config.yaml 处理：存在则迁移旧端口，不存在则从 example 拷贝
CFG="$DEPLOY_DIR/config/config.yaml"
if [ -f "$CFG" ]; then
  if $SUDO grep -qE '^[[:space:]]*addr:[[:space:]]*":18082"' "$CFG"; then
    $SUDO sed -i -E 's|^([[:space:]]*addr:[[:space:]]*)":18082"|\1":28002"|' "$CFG"
    echo "已将 config.yaml 中 app.addr 从 :18082 迁移到 :28002"
  fi
  $SUDO sed -i -E 's|http://127\.0\.0\.1:8080|http://127.0.0.1:28001|g' "$CFG" 2>/dev/null || true
else
  echo "⚠ $CFG 不存在；已从 example 拷贝，请编辑后重启："
  $SUDO cp /tmp/clipsync-admin-deploy/config.example.yaml "$CFG"
  echo "  vi $CFG"
fi

cd "$DEPLOY_DIR"

# 迁移 .env 中旧默认值（仅匹配精确的旧默认值，不误改用户自定义值）
if $SUDO grep -qE '^WEB_PORT=80$' .env 2>/dev/null; then
  $SUDO sed -i 's/^WEB_PORT=80$/WEB_PORT=28200/' .env
  echo "已将 .env 中 WEB_PORT 从 80 迁移到 28200"
fi
if $SUDO grep -q 'host.docker.internal:18082' .env 2>/dev/null; then
  $SUDO sed -i 's/host.docker.internal:18082/host.docker.internal:28002/g' .env
  echo "已将 .env 中 ADMIN_UPSTREAM 从 18082 迁移到 28002"
fi

# 部署统一使用 latest 标签（和 version tag 指向同一个镜像）
$SUDO sed -i '/^ADMIN_TAG=/d;/^WEB_TAG=/d' .env
echo "ADMIN_TAG=latest" | $SUDO tee -a .env > /dev/null
echo "WEB_TAG=latest"   | $SUDO tee -a .env > /dev/null

# 清除之前可能残留的过期 GHCR 凭据
$SUDO docker logout ghcr.io >/dev/null 2>&1 || true

# 拉取最新镜像并重启
echo "==> docker compose pull..."
$SUDO docker compose pull admin web
echo "==> docker compose up -d..."
$SUDO docker compose up -d admin web

# 清理悬空镜像
$SUDO docker image prune -f || true
rm -rf /tmp/clipsync-admin-deploy

echo "==> 部署完成，容器状态："
$SUDO docker compose ps
echo ""
echo "==> admin 日志（最后 20 行）："
$SUDO docker compose logs --tail=20 admin || true
