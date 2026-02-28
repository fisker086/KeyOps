#!/bin/bash

# 启动 Guacamole daemon (guacd) 的脚本

set -e

echo "=========================================="
echo "Starting Guacamole daemon (guacd)"
echo "=========================================="

# 检查 guacd 是否已运行
if docker ps | grep -q guacd; then
    echo "✓ guacd is already running"
    docker ps | grep guacd
    exit 0
fi

# 检查是否有停止的容器
if docker ps -a | grep -q guacd; then
    echo "Found stopped guacd container, removing..."
    docker rm guacd
fi

# 启动 guacd
# 使用 1.6.0 版本
# 明确指定平台为 linux/amd64，避免在 ARM64 系统上的平台警告
echo "Starting guacd container..."

# 创建录制目录（如果不存在）
# 默认使用当前目录下的 recordings 目录，可以通过 RECORDING_DIR 环境变量自定义
RECORDING_DIR="${RECORDING_DIR:-$(pwd)/recordings}"
mkdir -p "$RECORDING_DIR"
echo "Recording directory: $RECORDING_DIR (mounted to /replay in container)"

# 启动 guacd，挂载录制目录到容器的 /replay 路径
# 这样 guacd 就可以在 /replay 创建录制文件了
docker run -d \
  --name guacd \
  --platform linux/amd64 \
  -p 4822:4822 \
  -v "$RECORDING_DIR:/replay" \
  --restart unless-stopped \
  guacamole/guacd:1.6.0

# 等待启动
echo "Waiting for guacd to start..."
sleep 2

# 验证运行
if docker ps | grep -q guacd; then
    echo "✓ guacd started successfully"
    echo ""
    echo "Container info:"
    docker ps | grep guacd
    echo ""
    echo "To view logs: docker logs guacd"
    echo "To stop: docker stop guacd"
    echo "To remove: docker rm guacd"
else
    echo "✗ Failed to start guacd"
    echo "Check logs: docker logs guacd"
    exit 1
fi

