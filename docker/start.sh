#!/usr/bin/env bash
set -euo pipefail

# 配置路径（可以通过环境变量覆盖）
# 注意：main.go 目前硬编码了 "config/config.yaml"，所以需要确保工作目录正确
CONFIG_PATH="${KEYOPS_CONFIG:-config/config.yaml}"

echo "[start] Starting KeyOps services..."
echo "[start] Working directory: $(pwd)"
echo "[start] Config path: ${CONFIG_PATH}"
echo "[start] HTTP port: ${KEYOPS_ADDR_HTTP:-:8080}"
echo "[start] SSH port: ${KEYOPS_ADDR_SSH:-:2222}"

# 切换到应用目录（确保相对路径正确）
cd /app || exit 1

# 启动后端服务（后台运行）
# 注意：main.go 目前不支持命令行参数，端口配置在 config.yaml 中
# 如果需要覆盖端口，可以通过环境变量修改配置文件，或者修改 main.go 支持命令行参数
echo "[start] Launching keyops-api..."
(
  /usr/local/bin/keyops-api 2>&1 | sed -e 's/^/[keyops-api] /'
) &

# 等待后端服务启动
echo "[start] Waiting for backend to start..."
sleep 3

# 检查后端是否启动成功
if ! pgrep -f keyops-api > /dev/null; then
  echo "[start] Error: keyops-api process not found after startup"
  exit 1
fi

# 启动 nginx（前台运行，作为主进程）
echo "[start] Launching nginx..."
exec nginx -g 'daemon off;'


