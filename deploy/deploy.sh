#!/bin/sh
# XYZW Analyzer - Docker 部署脚本（通过阿里云 ACR 中转）
#
# 用法:
#   sh deploy/deploy.sh              # 构建 + push + 服务器部署（全流程）
#   sh deploy/deploy.sh build        # 仅本地构建镜像
#   sh deploy/deploy.sh push         # 仅推送镜像到 ACR
#   sh deploy/deploy.sh remote       # 仅在服务器上拉取并启动
#
# 网络受限时通过代理拉取基础镜像:
#   sh deploy/deploy.sh build --proxy
#   sh deploy/deploy.sh --proxy       # 全流程 + 代理
#
# 前提: 本地已安装 Docker
#       本地已登录 ACR: docker login crpi-ijyip3mvrstcfvh2.cn-heyuan.personal.cr.aliyuncs.com

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# 加载公共配置
. "$SCRIPT_DIR/deploy.conf"

APP_NAME="xyzw_analyzer"
APP_IMAGE="$APP_NAME:latest"
ACR_IMAGE="$ACR_REGISTRY/$ACR_NAMESPACE/$APP_NAME:latest"
ACR_IMAGE_VPC="$ACR_REGISTRY_VPC/$ACR_NAMESPACE/$APP_NAME:latest"
REMOTE_DIR="/root/xyzw-analyzer"

# 代理地址（与 xyzw_web_helper 保持一致）
PROXY="http://10.129.59.114:10889"

# ---- 解析参数 ----
# 支持: sh deploy.sh [build|push|remote|all] [--proxy]
CMD="${1:-all}"
USE_PROXY=0
for arg in "$@"; do
  [ "$arg" = "--proxy" ] && USE_PROXY=1
done

# ---- 步骤函数 ----

do_build() {
  echo "[build] Building image: $APP_IMAGE ..."
  if [ "$USE_PROXY" = "1" ]; then
    echo "        使用代理: $PROXY"
    DOCKER_BUILDKIT=0 HTTPS_PROXY="$PROXY" HTTP_PROXY="$PROXY" \
      docker build \
        -f "$PROJECT_DIR/docker/Dockerfile" \
        -t "$APP_IMAGE" \
        "$PROJECT_DIR"
  else
    docker build \
      -f "$PROJECT_DIR/docker/Dockerfile" \
      -t "$APP_IMAGE" \
      "$PROJECT_DIR"
  fi
  echo "       Built -> $APP_IMAGE"
}

do_push() {
  echo "[push] Pushing image to ACR ..."
  if ! docker image inspect "$APP_IMAGE" >/dev/null 2>&1; then
    echo "  ERROR: Image '$APP_IMAGE' not found. Run 'sh deploy/deploy.sh build' first."
    exit 1
  fi
  docker tag "$APP_IMAGE" "$ACR_IMAGE"
  docker push "$ACR_IMAGE"
  echo "       Pushed -> $ACR_IMAGE"
}

do_remote() {
  echo "[remote] Deploying on remote server ..."

  # 上传 docker-compose.yml 到服务器
  ssh -p "$REMOTE_PORT" "$REMOTE_USER@$REMOTE_HOST" "mkdir -p $REMOTE_DIR"
  scp -P "$REMOTE_PORT" \
    "$PROJECT_DIR/docker/docker-compose.yml" \
    "$REMOTE_USER@$REMOTE_HOST:$REMOTE_DIR/docker-compose.yml"

  ssh -p "$REMOTE_PORT" "$REMOTE_USER@$REMOTE_HOST" \
    ACR_REGISTRY_VPC="$ACR_REGISTRY_VPC" \
    ACR_IMAGE_VPC="$ACR_IMAGE_VPC" \
    ACR_USERNAME="$ACR_USERNAME" \
    REMOTE_DIR="$REMOTE_DIR" \
    'sh -s' <<'ENDSSH'
set -e

# 安装 Docker（如未安装）
if ! command -v docker >/dev/null 2>&1; then
  echo "  Installing Docker..."
  apt-get update -qq
  apt-get install -y ca-certificates curl
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
  chmod a+r /etc/apt/keyrings/docker.asc
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] \
    https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
    > /etc/apt/sources.list.d/docker.list
  apt-get update -qq
  apt-get install -y docker-ce docker-ce-cli containerd.io
  systemctl enable --now docker
fi

# 登录 ACR（服务器端，需环境变量 ACR_PASSWORD 或提前 docker login）
if [ -n "$ACR_PASSWORD" ]; then
  echo "$ACR_PASSWORD" | docker login "$ACR_REGISTRY_VPC" -u "$ACR_USERNAME" --password-stdin
fi

cd "$REMOTE_DIR"

# 替换 docker-compose.yml 中的 build 指令，服务器直接使用 ACR 镜像
sed -i "s|image: xyzw-analyzer:latest|image: $ACR_IMAGE_VPC|g" docker-compose.yml
sed -i '/build:/,/dockerfile:.*$/d' docker-compose.yml

# 拉取最新镜像并启动
echo "  Pulling image: $ACR_IMAGE_VPC ..."
docker pull "$ACR_IMAGE_VPC"

echo "  Starting container..."
docker compose up -d

sleep 2
if docker ps | grep -q xyzw-analyzer; then
  echo "  Deployment successful!"
  echo "  Web UI:     http://$(curl -s ifconfig.me 2>/dev/null || hostname -I | awk '{print $1}'):12582"
  echo "  MITM Proxy: $(curl -s ifconfig.me 2>/dev/null || hostname -I | awk '{print $1}'):12311"
else
  echo "  ERROR: Container failed to start."
  docker compose logs --tail=30
  exit 1
fi
ENDSSH
}

# ---- 入口 ----

case "$CMD" in
  build)
    do_build
    ;;
  push)
    do_push
    ;;
  remote)
    do_remote
    ;;
  all)
    do_build
    do_push
    do_remote
    echo ""
    echo "Deployment complete!"
    ;;
  *)
    echo "Usage: sh deploy/deploy.sh [build|push|remote|all] [--proxy]"
    exit 1
    ;;
esac
