# 修复导入路径问题

## 问题描述

GitHub Actions 构建失败，错误信息显示找不到 `github.com/meta-matrix/meta-matrix` 包。

## 原因

本地代码已经修复为使用 `github.com/lucksec/cloudbot`，但：
1. 更改可能还没有推送到 GitHub
2. 或者标签指向了修复前的旧提交

## 解决步骤

### 步骤 1: 提交并推送所有更改

```bash
# 1. 添加所有更改
git add .

# 2. 提交
git commit -m "修复导入路径和优化 GitHub Actions 工作流"

# 3. 推送到 GitHub
git push origin main
```

### 步骤 2: 删除旧标签（如果存在）

如果之前已经创建了标签，需要删除并重新创建：

```bash
# 查看本地标签
git tag -l

# 删除本地标签（如果存在）
git tag -d v1.0.0  # 替换为你的标签名

# 删除远程标签（如果存在）
git push origin :refs/tags/v1.0.0  # 替换为你的标签名
```

### 步骤 3: 重新创建标签

```bash
# 确保在最新的提交上
git checkout main
git pull origin main

# 创建新标签
git tag -a v1.0.0 -m "Release v1.0.0"

# 推送标签
git push origin v1.0.0
```

### 步骤 4: 验证

1. 访问 GitHub Actions 页面查看构建状态
2. 确认所有平台的构建都成功
3. 检查 Release 是否已创建

## 快速修复脚本

```bash
#!/bin/bash
# 快速修复脚本

VERSION=${1:-v1.0.0}

echo "1. 提交并推送更改..."
git add .
git commit -m "修复导入路径和优化 GitHub Actions 工作流" || echo "没有需要提交的更改"
git push origin main

echo "2. 删除旧标签（如果存在）..."
git tag -d $VERSION 2>/dev/null || echo "本地标签不存在"
git push origin :refs/tags/$VERSION 2>/dev/null || echo "远程标签不存在"

echo "3. 创建新标签..."
git tag -a $VERSION -m "Release $VERSION"
git push origin $VERSION

echo "完成！GitHub Actions 将自动开始构建。"
echo "查看进度: https://github.com/lucksec/cloudbot/actions"
```

使用方法：
```bash
chmod +x fix_and_release.sh
./fix_and_release.sh v1.0.0
```

## 验证导入路径

确保所有文件都使用正确的导入路径：

```bash
# 检查是否还有旧的导入路径
grep -r "github.com/meta-matrix/meta-matrix" cmd/ internal/ --include="*.go"

# 应该没有输出，如果有输出，需要修复
```

## 如果问题仍然存在

1. **检查 go.mod 文件**
   ```bash
   cat go.mod | head -1
   # 应该显示: module github.com/lucksec/cloudbot
   ```

2. **检查 GitHub 上的代码**
   - 访问 https://github.com/lucksec/cloudbot
   - 查看 `cmd/cloudbot/main.go` 的导入路径
   - 确认使用的是 `github.com/lucksec/cloudbot`

3. **清理并重新构建**
   ```bash
   # 本地测试构建
   go mod tidy
   go build ./cmd/cloudbot
   ```

