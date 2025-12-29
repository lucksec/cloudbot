#!/bin/bash
# 修复导入路径问题并重新发布

set -e

VERSION=${1:-v1.0.0}

echo "🔍 检查当前状态..."
echo "当前分支: $(git branch --show-current)"
echo "本地提交数: $(git rev-list --count origin/main..HEAD 2>/dev/null || echo 0)"

# 检查是否有未提交的更改
if [ -n "$(git status --porcelain)" ]; then
    echo "⚠️  发现未提交的更改，正在提交..."
    git add .
    git commit -m "修复导入路径和优化 GitHub Actions 工作流"
fi

echo ""
echo "📤 步骤 1: 推送所有提交到 GitHub..."
git push origin main

echo ""
echo "🏷️  步骤 2: 删除旧标签（如果存在）..."
# 删除本地标签
if git rev-parse "$VERSION" >/dev/null 2>&1; then
    echo "删除本地标签: $VERSION"
    git tag -d "$VERSION"
else
    echo "本地标签不存在: $VERSION"
fi

# 删除远程标签
if git ls-remote --tags origin | grep -q "refs/tags/$VERSION"; then
    echo "删除远程标签: $VERSION"
    git push origin ":refs/tags/$VERSION" || echo "远程标签删除失败（可能不存在）"
else
    echo "远程标签不存在: $VERSION"
fi

echo ""
echo "🏷️  步骤 3: 创建新标签..."
git tag -a "$VERSION" -m "Release $VERSION"

echo ""
echo "📤 步骤 4: 推送标签..."
git push origin "$VERSION"

echo ""
echo "✅ 完成！"
echo ""
echo "📊 查看构建进度:"
echo "   https://github.com/lucksec/cloudbot/actions"
echo ""
echo "📦 查看 Releases:"
echo "   https://github.com/lucksec/cloudbot/releases"
echo ""
echo "🔍 验证标签:"
echo "   git show $VERSION:go.mod | head -1"
echo "   应该显示: module github.com/lucksec/cloudbot"

