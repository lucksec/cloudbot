# cloudbot
Cloud-bot 是一个基于 **Infrastructure as Code (IaC)** 理念开发的云资源编排工具，通过 Terraform 实现跨云服务商的统一资源管理。只需一个命令，即可将工作节点部署到全球各地的云服务商，支持抢占式实例大幅降低成本。

# Cloud-bot

<div align="center">

![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go)
![License](https://img.shields.io/badge/License-MIT-green?style=flat-square)
![Terraform](https://img.shields.io/badge/Terraform-Ready-623CE4?style=flat-square&logo=terraform)

**基于 Infrastructure as Code (IaC) 的智能多云资源编排工具**

一键部署、成本优化、自动化管理

[快速开始](#快速开始) • [功能特性](#功能特性) • [使用文档](doc/USAGE.md) • [问题反馈](https://github.com/lucksec/issues)

</div>

---

## 📖 项目简介

Cloud-bot 是一个基于 **Infrastructure as Code (IaC)** 理念开发的云资源编排工具，通过 Terraform 实现跨云服务商的统一资源管理。只需一个命令，即可将工作节点部署到全球各地的云服务商，支持抢占式实例大幅降低成本。

### 核心能力

- 🚀 **一键部署**: 通过 Terraform 模板快速部署云资源到多个云服务商
- 💰 **成本优化**: 支持抢占式实例，自动查找最低价格配置
- 🌍 **多云支持**: 统一管理阿里云、腾讯云、AWS、华为云等云服务商
- 🔄 **动态模板**: 基于云服务商 API 动态生成 Terraform 模板
- 📊 **价格比对**: 实时查询和比对不同云服务商的价格
- 🎯 **智能补全**: 支持 Bash/Zsh/Fish/PowerShell 命令自动补全

## ✨ 功能特性

### 1. 项目管理

- ✅ 创建、列出、删除项目
- ✅ 项目级别的场景管理
- ✅ 项目初始化（预加载 Terraform providers）

### 2. 场景管理

- ✅ 从模板库创建场景
- ✅ 动态生成场景（基于云服务商 API）
- ✅ 场景部署和销毁
- ✅ 场景状态查询和资源验证

### 3. 模板系统

- ✅ 丰富的模板库（ECS、代理、文件服务器等）
- ✅ 支持静态模板和动态模板
- ✅ 模板价格信息管理

### 4. 价格优化

- ✅ 实时价格查询（阿里云 DescribePrice API）
- ✅ 跨区域价格比对
- ✅ 自动选择最低价格配置
- ✅ 价格信息缓存

### 5. 多云支持

| 云服务商 | 支持状态 | 功能 |
|---------|---------|------|
| 阿里云 | ✅ 完整支持 | ECS、抢占式实例、价格优化 |
| 腾讯云 | ✅ 完整支持 | CVM、竞价实例 |
| AWS | ✅ 完整支持 | EC2、Spot 实例 |
| 华为云 | ✅ 基础支持 | ECS、竞价实例 |

### 6. 开发者体验

- ✅ 命令自动补全（Bash/Zsh/Fish/PowerShell）
- ✅ 交互式控制台
- ✅ 详细的帮助信息和使用示例
- ✅ 结构化日志输出

## 🚀 快速开始

### 安装

#### 方式 1: 从源码构建

```bash
# 克隆仓库
git clone https://github.com/luckone/cloud-bot.git
cd cloud-bot

# 构建
make build

# 或安装到系统
make install
```

#### 方式 2: 使用 Go 安装

```bash
go install github.com/lucksec/cloud-bot/cmd/cloud-bot@latest
```

### 配置

创建配置文件（可选，有默认值）：

```bash
cp .cloudboot.ini.example .cloudboot.ini
vim .cloudboot.ini
```

### 基本使用

```bash
# 1. 查看可用模板
cloud-bot template list

# 2. 创建项目
cloud-bot project create my-project

# 3. 从模板创建场景
cloud-bot scenario create my-project aliyun ecs

# 4. 部署场景（需要先配置云服务商凭据）
cloud-bot scenario deploy my-project <scenario-id>

# 5. 查看场景状态
cloud-bot scenario status my-project

# 6. 销毁场景
cloud-bot scenario destroy my-project <scenario-id>
```

### 配置云服务商凭据

#### 使用命令行配置

```bash
# 配置阿里云凭据
cloud-bot credential set aliyun

# 配置腾讯云凭据
cloud-bot credential set tencent
```

#### 使用环境变量

```bash
# 阿里云
export ALICLOUD_ACCESS_KEY="your-access-key"
export ALICLOUD_SECRET_KEY="your-secret-key"

# AWS
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
```

## 📚 使用示例

### 示例 1: 创建并部署 ECS 实例

```bash
# 创建项目
cloud-bot project create web-server

# 创建场景（使用阿里云 ECS 模板）
cloud-bot scenario create web-server aliyun ecs

# 部署场景
cloud-bot scenario deploy web-server <scenario-id>
```

### 示例 2: 使用价格优化创建场景

```bash
# 创建场景并自动应用最优价格配置
cloud-bot scenario create my-project aliyun ecs --optimal

# 输出示例：
# ✨ 找到最优配置:
#   区域: cn-hangzhou
#   实例类型: ecs.t5-lc1m1.small
#   价格: 0.0650 CNY/小时 (46.80 CNY/月)
```

### 示例 3: 动态创建代理场景

```bash
# 动态创建代理场景（自动选择区域和实例类型）
cloud-bot scenario create-dynamic my-project aliyun proxy

# 指定区域和实例类型
cloud-bot scenario create-dynamic my-project aliyun proxy cn-beijing \
  --instance-type ecs.t6-c1m1.small --node-count 5
```

### 示例 4: 价格比对

```bash
# 比对 ECS 类型模板的价格
cloud-bot price compare ecs

# 查找最优价格配置
cloud-bot price optimal aliyun ecs

# 列出各区域价格
cloud-bot price regions aliyun ecs
```

### 示例 5: 使用交互式控制台

```bash
# 启动交互式控制台
cloud-bot console

# 在控制台中执行命令
cloud-bot> project create my-project
cloud-bot> scenario create my-project aliyun ecs
cloud-bot> scenario deploy my-project <scenario-id>
```

## 📖 详细文档

- [使用指南](doc/USAGE.md) - 完整的使用文档和示例
- [功能特性](doc/FEATURES.md) - 详细的功能说明
- [价格优化](doc/PRICE_OPTIMIZATION.md) - 价格优化功能使用指南
- [动态模板](doc/DYNAMIC_TEMPLATE_REFACTOR.md) - 动态模板生成说明
- [部署指南](doc/DEPLOY_GUIDE.md) - 部署和配置说明
- [故障排查](doc/TROUBLESHOOTING.md) - 常见问题和解决方案

## 🏗️ 项目结构

```
cloud-bot/
├── cmd/
│   └── cloud-bot/          # 主程序入口
│       ├── main.go           # 命令定义
│       ├── completion.go     # 自动补全
│       └── console.go        # 交互式控制台
├── internal/
│   ├── config/               # 配置管理
│   ├── credentials/          # 凭据管理
│   ├── domain/               # 领域模型
│   ├── logger/               # 日志系统
│   ├── repository/           # 数据访问层
│   └── service/              # 业务逻辑层
│       ├── aliyun_client.go  # 阿里云客户端
│       ├── aws_client.go     # AWS 客户端
│       ├── price_optimizer_service.go  # 价格优化服务
│       ├── dynamic_template_service.go  # 动态模板服务
│       └── terraform_service.go         # Terraform 服务
├── templates/           # Terraform 模板库
│   ├── aliyun/               # 阿里云模板
│   ├── tencent/              # 腾讯云模板
│   ├── aws/                  # AWS 模板
│   └── huaweicloud/          # 华为云模板
├── projects/                 # 项目目录（运行时生成）
├── doc/                      # 文档目录
├── go.mod
├── Makefile
└── README.md
```

## 🎯 核心命令

### 项目管理

```bash
cloud-bot project create <name>      # 创建项目
cloud-bot project list               # 列出所有项目
cloud-bot project init <name>        # 初始化项目
cloud-bot project delete <name>      # 删除项目
```

### 场景管理

```bash
cloud-bot scenario create <project> <provider> <template> [region]  # 创建场景
cloud-bot scenario create-dynamic <project> <provider> <type>      # 动态创建场景
cloud-bot scenario list <project>                                   # 列出场景
cloud-bot scenario deploy <project> <scenario-id>                   # 部署场景
cloud-bot scenario destroy <project> <scenario-id>                 # 销毁场景
cloud-bot scenario status <project> [scenario-id]                  # 查看状态
```

### 模板管理

```bash
cloud-bot template list              # 列出所有模板
```

### 价格管理

```bash
cloud-bot price list                 # 列出所有价格信息
cloud-bot price compare <type>       # 比对价格
cloud-bot price optimal <provider> <template>  # 查找最优配置
cloud-bot price regions <provider> <template> # 列出各区域价格
```

### 凭据管理

```bash
cloud-bot credential set <provider>  # 设置凭据
cloud-bot credential list            # 列出已配置的凭据
```

## 🔧 开发指南

### 环境要求

- Go 1.21+
- Terraform 1.0+
- 云服务商账户和凭据

### 构建和测试

```bash
# 下载依赖
make deps

# 构建
make build

# 运行测试
make test

# 代码格式化
make fmt

# 代码检查
make lint
```

### 添加新模板

1. 在 `templates/<provider>/<template-name>/` 目录下创建模板文件
2. 确保包含 `main.tf` 文件
3. 可选：添加 `versions.tf`, `outputs.tf`, `variables.tf` 等文件
4. 运行 `cloud-bot template list` 验证模板是否被识别

### 代码结构

项目采用 **Clean Architecture** 设计：

- **domain**: 领域模型，定义核心业务实体
- **repository**: 数据访问层，负责数据持久化
- **service**: 业务逻辑层，实现核心业务功能
- **config**: 配置管理，统一管理应用配置

## 💡 最佳实践

1. **项目命名**: 使用有意义的项目名称，便于管理
2. **场景隔离**: 每个场景使用独立的 UUID，互不干扰
3. **及时清理**: 测试完成后及时销毁场景，避免资源浪费
4. **配置管理**: 敏感信息（如 AK/SK）不要提交到版本控制
5. **成本控制**: 使用抢占式实例和价格优化功能降低成本
6. **状态备份**: 重要的 Terraform 状态文件建议备份

## ⚠️ 注意事项

1. **Terraform 要求**: 确保已安装 Terraform 并在 PATH 中
2. **云服务商权限**: 确保 AK/SK 具有创建 VPC、安全组、实例等权限
3. **成本控制**: 使用抢占式实例可以大幅降低成本，但可能被回收
4. **资源清理**: 及时销毁不需要的场景，避免资源浪费
5. **状态管理**: 每个场景的 Terraform 状态文件保存在场景目录下

## 🤝 贡献指南

欢迎贡献代码！请遵循以下步骤：

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

## 📄 许可证

本项目采用 MIT 许可证。详情请参阅 [LICENSE](LICENSE) 文件。

## 🙏 致谢

- [Terraform](https://www.terraform.io/) - Infrastructure as Code 工具
- [Cobra](https://github.com/spf13/cobra) - CLI 框架
- 所有贡献者和用户

## 📮 联系方式

- 问题反馈: [GitHub Issues](https://github.com/lucksec/cloudbot/issues)
- 功能建议: [GitHub Discussions](https://github.com/lucksec/cloudbot/discussions)

---

<div align="center">

**如果这个项目对你有帮助，请给一个 ⭐ Star！**

Made with ❤️ by cloud-bot Team

</div>

