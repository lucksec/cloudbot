# GitHub Actions 发布和安装配置指南

本文档说明如何配置和使用 GitHub Actions 进行 cloudbot 的自动发布和安装。

## 📋 目录

1. [初始配置](#初始配置)
2. [发布流程](#发布流程)
3. [安装方式](#安装方式)
4. [工作流说明](#工作流说明)
5. [故障排查](#故障排查)

## 🚀 初始配置

### 1. 提交工作流文件到 GitHub

```bash
# 添加新文件
git add .github/workflows/
git add RELEASE.md
git add scripts/release.sh
git add .gitignore

# 提交
git commit -m "添加 GitHub Actions 工作流配置"

# 推送到 GitHub
git push origin main
```

### 2. 验证工作流

1. 访问 GitHub 仓库的 Actions 页面
   ```
   https://github.com/lucksec/cloudbot/actions
   ```

2. 确认工作流文件已加载
   - 应该能看到 "CI" 和 "Release" 工作流

3. 测试 CI 工作流
   - 推送任何更改到 main 分支
   - CI 工作流会自动运行测试和构建

## 📦 发布流程

### 方式一：使用发布脚本（推荐）

```bash
# 使用发布脚本
./scripts/release.sh v1.0.0

# 脚本会自动：
# 1. 验证版本号格式
# 2. 检查工作目录状态
# 3. 创建并推送 Git 标签
# 4. 触发 GitHub Actions 自动构建
```

### 方式二：手动创建标签

```bash
# 1. 确保所有更改已提交
git add .
git commit -m "准备发布 v1.0.0"
git push origin main

# 2. 创建标签
git tag -a v1.0.0 -m "Release v1.0.0"

# 3. 推送标签
git push origin v1.0.0
```

### 方式三：在 GitHub 上手动触发

1. 访问 Actions 页面
2. 选择 "Release" 工作流
3. 点击 "Run workflow"
4. 输入版本号（如：v1.0.0）
5. 点击 "Run workflow"

## 📥 安装方式

### 方式一：从 GitHub Releases 下载

1. **访问 Releases 页面**
   ```
   https://github.com/lucksec/cloudbot/releases
   ```

2. **下载对应平台的文件**
   - Linux amd64: `cloudbot-linux-amd64`
   - Linux arm64: `cloudbot-linux-arm64`
   - macOS amd64: `cloudbot-darwin-amd64`
   - macOS arm64: `cloudbot-darwin-arm64`
   - Windows amd64: `cloudbot-windows-amd64.exe`
   - Windows arm64: `cloudbot-windows-arm64.exe`

3. **安装步骤**

   **Linux:**
   ```bash
   # 下载
   wget https://github.com/lucksec/cloudbot/releases/download/v1.0.0/cloudbot-linux-amd64
   
   # 添加执行权限
   chmod +x cloudbot-linux-amd64
   
   # 安装到系统路径
   sudo mv cloudbot-linux-amd64 /usr/local/bin/cloudbot
   
   # 验证
   cloudbot --version
   ```

   **macOS:**
   ```bash
   # 下载
   curl -L -o cloudbot https://github.com/lucksec/cloudbot/releases/download/v1.0.0/cloudbot-darwin-amd64
   
   # 添加执行权限
   chmod +x cloudbot
   
   # 安装到系统路径
   sudo mv cloudbot /usr/local/bin/
   
   # 验证
   cloudbot --version
   ```

   **Windows (PowerShell):**
   ```powershell
   # 下载
   Invoke-WebRequest -Uri "https://github.com/lucksec/cloudbot/releases/download/v1.0.0/cloudbot-windows-amd64.exe" -OutFile "cloudbot.exe"
   
   # 创建安装目录
   New-Item -ItemType Directory -Force -Path "C:\Program Files\cloudbot"
   
   # 移动文件
   Move-Item cloudbot.exe "C:\Program Files\cloudbot\cloudbot.exe"
   
   # 添加到 PATH（如果还没有）
   $env:Path += ";C:\Program Files\cloudbot"
   [Environment]::SetEnvironmentVariable("Path", $env:Path + ";C:\Program Files\cloudbot", [EnvironmentVariableTarget]::User)
   
   # 验证
   cloudbot --version
   ```

### 方式二：使用 Go 安装

```bash
# 安装最新版本
go install github.com/lucksec/cloudbot/cmd/cloudbot@latest

# 安装特定版本
go install github.com/lucksec/cloudbot/cmd/cloudbot@v1.0.0
```

### 方式三：使用 Homebrew（需要配置 tap）

如果配置了 Homebrew tap，可以使用：

```bash
brew tap lucksec/cloudbot
brew install cloudbot
```

## 🔧 工作流说明

### 1. CI 工作流 (`.github/workflows/ci.yml`)

**触发条件:**
- 推送到 main/master/develop 分支
- 创建 Pull Request

**功能:**
- 运行测试
- 代码 lint 检查
- 多平台构建验证

### 2. Release 工作流 (`.github/workflows/release.yml`)

**触发条件:**
- 推送以 `v` 开头的标签（如 `v1.0.0`）
- 手动触发（workflow_dispatch）

**功能:**
- 为 6 个平台构建二进制文件
- 生成 SHA256 校验文件
- 自动创建 GitHub Release
- 上传所有构建产物

**支持的平台:**
- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64, arm64)

### 3. Homebrew 工作流 (`.github/workflows/homebrew.yml`)

**触发条件:**
- 发布新的 Release

**功能:**
- 自动更新 Homebrew formula（如果配置了 tap）

**注意:** 需要创建 `lucksec/homebrew-tap` 仓库并配置 `HOMEBREW_TAP_TOKEN` secret。

## 🔐 配置 Secrets

如果需要使用 Homebrew 自动更新功能，需要配置以下 Secret：

1. 访问仓库 Settings > Secrets and variables > Actions
2. 添加 Secret: `HOMEBREW_TAP_TOKEN`
   - 值：具有访问 `lucksec/homebrew-tap` 仓库权限的 Personal Access Token

## ✅ 验证发布

发布完成后，验证步骤：

1. **检查 GitHub Actions**
   ```
   https://github.com/lucksec/cloudbot/actions
   ```
   - 确认 Release 工作流成功完成

2. **检查 Releases 页面**
   ```
   https://github.com/lucksec/cloudbot/releases
   ```
   - 确认新版本已创建
   - 确认所有平台的二进制文件已上传

3. **测试下载和安装**
   - 下载对应平台的二进制文件
   - 验证可以正常运行

## 🐛 故障排查

### 问题 1: 工作流未触发

**原因:** 标签格式不正确
**解决:** 确保标签以 `v` 开头，如 `v1.0.0`

### 问题 2: 构建失败

**检查:**
1. 查看 Actions 日志
2. 确认 Go 版本兼容性
3. 检查代码是否有语法错误

### 问题 3: Release 未创建

**检查:**
1. 确认工作流有 `contents: write` 权限
2. 检查 GITHUB_TOKEN 是否有效
3. 查看工作流日志中的错误信息

### 问题 4: 二进制文件无法运行

**检查:**
1. 确认下载了正确的平台版本
2. 添加执行权限（Linux/macOS）
3. 验证文件完整性（使用 SHA256 校验）

## 📝 发布检查清单

发布前确认：

- [ ] 所有代码已提交并推送
- [ ] 版本号已更新（如需要）
- [ ] 测试全部通过
- [ ] 代码已通过 lint 检查
- [ ] README 和文档已更新
- [ ] 已创建 Git 标签
- [ ] 标签已推送到 GitHub

## 🔗 相关链接

- [GitHub Actions 文档](https://docs.github.com/en/actions)
- [Go 发布最佳实践](https://go.dev/doc/modules/release-workflow)
- [项目仓库](https://github.com/lucksec/cloudbot)

## 📞 获取帮助

如果遇到问题：

1. 查看 GitHub Actions 日志
2. 检查 [Issues](https://github.com/lucksec/cloudbot/issues)
3. 创建新的 Issue 描述问题

