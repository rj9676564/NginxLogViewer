#!/bin/bash

# Sonic Stellar 容器一键更新脚本

echo "🚀 开始更新 Sonic Stellar..."

# 1. 拉取最新的镜像
echo "📥 正在从 Docker Hub 拉取最新镜像..."
docker-compose pull

# 2. 重启容器 (会自动识别镜像变化并替换)
echo "🔄 正在重启容器以应用更新..."
docker-compose up -d

# 3. 清理无用的旧镜像
echo "🧹 正在清理旧的冗余镜像..."
docker image prune -f

echo "✅ 更新完成！当前版本已是最新。"
