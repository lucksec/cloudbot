# 发布和安装指南

本文档说明如何使用 GitHub Actions 发布和安装 cloudbot。

## 📦 发布流程

### 方式一：通过 Git Tag 发布（推荐）

1. **更新版本号**
   ```bash
   # 确保所有更改已提交
   git add .
   git commit -m "准备发布 v1.0.0"
   ```

2. **创建并推送标签**
   ```bash
   # 创建标签
   git tag -a v1.0.0 -m "Release v1.0.0"
   
   # 推送标签到 GitHub
   git push origin v1.0.0
   ```

3. **GitHub Actions 自动构建**
   - 推送标签后，GitHub Actions 会自动触发 `release.yml` 工作流
   - 工作流会为以下平台构建二进制文件：
     - Linux (amd64, arm64)
     - macOS (amd64, arm64)
     - Windows (amd64, arm64)

4. **自动创建 Release**
   - 构建完成后，会自动在 GitHub Releases 页面创建新的 release
   - 所有平台的二进制文件和校验文件会自动上传

### 方式二：手动触发发布

1. **在 GitHub 上手动触发**
   - 进入 Actions 页面
   - 选择 "Release" 工作流
   - 点击 "Run workflow"
   - 输入版本号（如：v1.0.0）
   - 点击 "Run workflow" 按钮

2. **等待构建完成**
   - 工作流会自动构建所有平台的二进制文件
   - 构建完成后会自动创建 release

## 🔧 安装方式

### 方式一：从 GitHub Releases 下载（推荐）

1. **访问 Releases 页面**
   ```
   https://github.com/lucksec/cloudbot/releases
   ```

2. **下载对应平台的二进制文件**
   - Linux: `cloudbot-linux-amd64` 或 `cloudbot-linux-arm64`
   - macOS: `cloudbot-darwin-amd64` 或 `cloudbot-darwin-arm64`
   - Windows: `cloudbot-windows-amd64.exe` 或 `cloudbot-windows-arm64.exe`

3. **安装步骤**

   **Linux/macOS:**
   ```bash
   # 下载文件
   wget https://github.com/lucksec/cloudbot/releases/download/v1.0.0/cloudbot-linux-amd64
   
   # 添加执行权限
   chmod +x cloudbot-linux-amd64
   
   # 移动到 PATH 目录
   sudo mv cloudbot-linux-amd64 /usr/local/bin/cloudbot
   
   # 验证安装
   cloudbot --version
   ```

   **Windows:**
   ```powershell
   # 下载文件
   Invoke-WebRequest -Uri "https://github.com/lucksec/cloudbot/releases/download/v1.0.0/cloudbot-windows-amd64.exe" -OutFile "cloudbot.exe"
   
   # 移动到 PATH 目录（例如 C:\Program Files\cloudbot\）
   Move-Item cloudbot.exe "C:\Program Files\cloudbot\cloudbot.exe"
   
   # 添加到 PATH 环境变量（如果还没有）
   [Environment]::SetEnvironmentVariable("Path", $env:Path + ";C:\Program Files\cloudbot", [EnvironmentVariableTarget]::User)
   ```

### 方式二：使用 Homebrew（macOS/Linux）

如果已配置 Homebrew tap，可以使用以下命令安装：

```bash
# 添加 tap（首次使用）
brew tap lucksec/cloudbot

# 安装
brew install cloudbot

# 更新
brew upgrade cloudbot
```

### 方式三：使用 Go 安装（开发环境）

```bash
# 安装最新版本
go install github.com/lucksec/cloudbot/cmd/cloudbot@latest

# 安装特定版本
go install github.com/lucksec/cloudbot/cmd/cloudbot@v1.0.0
```

## ✅ 验证安装

安装完成后，可以通过以下命令验证：

```bash
# 查看版本
cloudbot --version

# 查看帮助
cloudbot --help

# 列出可用命令
cloudbot
```

## 🔐 校验文件完整性

每个 release 都包含 SHA256 校验文件，可以用来验证下载的文件是否完整：

**Linux/macOS:**
```bash
# 下载二进制文件和校验文件
wget https://github.com/lucksec/cloudbot/releases/download/v1.0.0/cloudbot-linux-amd64
wget https://github.com/lucksec/cloudbot/releases/download/v1.0.0/cloudbot-linux-amd64.sha256

# 验证
shasum -a 256 -c cloudbot-linux-amd64.sha256
```

**Windows (PowerShell):**
```powershell
# 下载文件
Invoke-WebRequest -Uri "https://github.com/lucksec/cloudbot/releases/download/v1.0.0/cloudbot-windows-amd64.exe" -OutFile "cloudbot.exe"
Invoke-WebRequest -Uri "https://github.com/lucksec/cloudbot/releases/download/v1.0.0/cloudbot-windows-amd64.exe.sha256" -OutFile "cloudbot.exe.sha256"

# 验证
$hash = Get-FileHash cloudbot.exe -Algorithm SHA256
$expected = Get-Content cloudbot.exe.sha256 | Select-Object -First 1
if ($hash.Hash -eq $expected.Split()[0]) {
    Write-Host "校验通过"
} else {
    Write-Host "校验失败"
}
```

## 📝 发布检查清单

发布新版本前，请确保：

- [ ] 所有代码已提交并推送到 GitHub
- [ ] 已更新版本号（如需要）
- [ ] 已更新 CHANGELOG.md（如存在）
- [ ] 已更新 README.md（如需要）
- [ ] 所有测试通过
- [ ] 代码已通过 lint 检查
- [ ] 已创建并推送 Git 标签

## 🚀 快速发布命令

```bash
# 一键发布脚本
#!/bin/bash
VERSION=$1
if [ -z "$VERSION" ]; then
    echo "用法: ./release.sh v1.0.0"
    exit 1
fi

# 确保工作目录干净
git status
read -p "确认发布 $VERSION? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    git tag -a "$VERSION" -m "Release $VERSION"
    git push origin "$VERSION"
    echo "已推送标签 $VERSION，GitHub Actions 将自动构建和发布"
fi
```

## 🔗 相关链接

- [GitHub Releases](https://github.com/lucksec/cloudbot/releases)
- [GitHub Actions](https://github.com/lucksec/cloudbot/actions)
- [项目主页](https://github.com/lucksec/cloudbot)

