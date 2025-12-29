#!/bin/bash
# Cloudbot 发布脚本
# 用法: ./scripts/release.sh v1.0.0

set -e

VERSION=$1

if [ -z "$VERSION" ]; then
    echo "❌ 错误: 请提供版本号"
    echo "用法: $0 v1.0.0"
    exit 1
fi

# 验证版本号格式
if [[ ! $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "❌ 错误: 版本号格式不正确，应为 v1.0.0"
    exit 1
fi

# 检查工作目录是否干净
if [ -n "$(git status --porcelain)" ]; then
    echo "⚠️  警告: 工作目录有未提交的更改"
    read -p "是否继续? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# 检查标签是否已存在
if git rev-parse "$VERSION" >/dev/null 2>&1; then
    echo "❌ 错误: 标签 $VERSION 已存在"
    exit 1
fi

echo "📦 准备发布 $VERSION"
echo ""

# 显示当前更改
echo "📋 当前更改:"
git status --short
echo ""

# 确认发布
read -p "确认发布 $VERSION? (y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "已取消"
    exit 0
fi

# 创建标签
echo "🏷️  创建标签 $VERSION..."
git tag -a "$VERSION" -m "Release $VERSION"

# 推送标签
echo "🚀 推送标签到 GitHub..."
git push origin "$VERSION"

echo ""
echo "✅ 标签 $VERSION 已推送"
echo "📦 GitHub Actions 将自动构建和发布"
echo ""
echo "查看构建进度: https://github.com/lucksec/cloudbot/actions"
echo "查看 Releases: https://github.com/lucksec/cloudbot/releases"

