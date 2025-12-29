package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/lucksec/cloudbot/internal/credentials"
	"github.com/lucksec/cloudbot/internal/logger"
)

// TemplateGenerator 动态模板生成器
type TemplateGenerator interface {
	// GenerateTemplate 生成Terraform模板
	// scenario: 场景类型 (proxy, task-executor)
	// provider: 云服务商
	// region: 区域
	// instanceType: 实例类型
	GenerateTemplate(ctx context.Context, scenario, provider, region, instanceType string, options map[string]interface{}) (string, error)
}

// templateGenerator 模板生成器实现
type templateGenerator struct {
	credManager credentials.CredentialManager
}

// NewTemplateGenerator 创建模板生成器
func NewTemplateGenerator(credManager credentials.CredentialManager) TemplateGenerator {
	return &templateGenerator{
		credManager: credManager,
	}
}

// GenerateTemplate 生成Terraform模板
func (g *templateGenerator) GenerateTemplate(ctx context.Context, scenario, provider, region, instanceType string, options map[string]interface{}) (string, error) {
	// 获取云服务商客户端
	client, err := g.getProviderClient(provider)
	if err != nil {
		return "", fmt.Errorf("获取云服务商客户端失败: %w", err)
	}

	// 验证区域和实例类型
	if region != "" {
		regions, err := client.GetAvailableRegions(ctx)
		if err == nil {
			validRegion := false
			for _, r := range regions {
				if r.ID == region {
					validRegion = true
					break
				}
			}
			if !validRegion {
				return "", fmt.Errorf("无效的区域: %s", region)
			}
		}
	}

	if instanceType != "" {
		instanceTypes, err := client.GetAvailableInstanceTypes(ctx, region)
		if err == nil {
			validInstanceType := false
			for _, it := range instanceTypes {
				if it.ID == instanceType {
					validInstanceType = true
					break
				}
			}
			if !validInstanceType {
				return "", fmt.Errorf("无效的实例类型: %s", instanceType)
			}
		}
	}

	// 根据场景类型生成模板
	switch scenario {
	case "proxy":
		return g.generateProxyTemplate(ctx, provider, region, instanceType, options)
	case "task-executor":
		return g.generateTaskExecutorTemplate(ctx, provider, region, instanceType, options)
	default:
		return "", fmt.Errorf("不支持的场景类型: %s", scenario)
	}
}

// getProviderClient 获取云服务商客户端
func (g *templateGenerator) getProviderClient(provider string) (CloudProviderClient, error) {
	providerEnum := credentials.Provider(provider)
	if !g.credManager.HasCredentials(providerEnum) {
		return nil, fmt.Errorf("未配置 %s 的凭据", provider)
	}

	creds, err := g.credManager.GetCredentials(providerEnum)
	if err != nil {
		return nil, fmt.Errorf("获取 %s 凭据失败: %w", provider, err)
	}

	return NewCloudProviderClient(provider, creds.AccessKey, creds.SecretKey)
}

// generateProxyTemplate 生成代理场景模板
func (g *templateGenerator) generateProxyTemplate(ctx context.Context, provider, region, instanceType string, options map[string]interface{}) (string, error) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "cloudbot-template-*")
	if err != nil {
		return "", fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// 根据云服务商生成不同的模板
	switch provider {
	case "aliyun":
		return g.generateAliyunProxyTemplate(tempDir, region, instanceType, options)
	case "tencent":
		return g.generateTencentProxyTemplate(tempDir, region, instanceType, options)
	case "aws":
		return g.generateAWSProxyTemplate(tempDir, region, instanceType, options)
	default:
		return "", fmt.Errorf("不支持的云服务商: %s", provider)
	}
}

// generateTaskExecutorTemplate 生成工具执行场景模板
func (g *templateGenerator) generateTaskExecutorTemplate(ctx context.Context, provider, region, instanceType string, options map[string]interface{}) (string, error) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "cloudbot-template-*")
	if err != nil {
		return "", fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// 根据云服务商生成不同的模板
	switch provider {
	case "aliyun":
		return g.generateAliyunTaskExecutorTemplate(tempDir, region, instanceType, options)
	case "tencent":
		return g.generateTencentTaskExecutorTemplate(tempDir, region, instanceType, options)
	case "aws":
		return g.generateAWSTaskExecutorTemplate(tempDir, region, instanceType, options)
	default:
		return "", fmt.Errorf("不支持的云服务商: %s", provider)
	}
}

// generateAliyunProxyTemplate 生成阿里云代理模板
func (g *templateGenerator) generateAliyunProxyTemplate(tempDir, region, instanceType string, options map[string]interface{}) (string, error) {
	// 设置默认值
	if region == "" {
		region = "cn-beijing"
	}
	if instanceType == "" {
		instanceType = "ecs.t6-c1m1.small"
	}
	nodeCount := 3
	if n, ok := options["node_count"].(int); ok && n > 0 {
		nodeCount = n
	}

	// 生成 main.tf
	mainTfContent := g.generateAliyunProxyMainTf(region, instanceType, nodeCount)
	if err := os.WriteFile(filepath.Join(tempDir, "main.tf"), []byte(mainTfContent), 0644); err != nil {
		return "", fmt.Errorf("写入 main.tf 失败: %w", err)
	}

	// 生成 versions.tf
	versionsTfContent := `terraform {
  required_version = ">= 1.0"
  
  required_providers {
    alicloud = {
      source  = "aliyun/alicloud"
      version = "~> 1.200"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.1"
    }
  }
}
`
	if err := os.WriteFile(filepath.Join(tempDir, "versions.tf"), []byte(versionsTfContent), 0644); err != nil {
		return "", fmt.Errorf("写入 versions.tf 失败: %w", err)
	}

	// 生成 outputs.tf
	outputsTfContent := `output "public_ips" {
  value = alicloud_instance.instance[*].public_ip
}

output "instance_ids" {
  value = alicloud_instance.instance[*].id
}
`
	if err := os.WriteFile(filepath.Join(tempDir, "outputs.tf"), []byte(outputsTfContent), 0644); err != nil {
		return "", fmt.Errorf("写入 outputs.tf 失败: %w", err)
	}

	return tempDir, nil
}

// generateAliyunProxyMainTf 生成阿里云代理 main.tf 内容
func (g *templateGenerator) generateAliyunProxyMainTf(region, instanceType string, nodeCount int) string {
	tmpl := `provider "alicloud" {
  region = "{{.Region}}"
}

provider "random" {}

variable "node_count" {
  type        = number
  description = "节点数量"
  default     = {{.NodeCount}}
}

variable "ss_port" {
  type        = string
  description = "Shadowsocks 服务端口"
  default     = ""
}

variable "ss_pass" {
  type        = string
  description = "Shadowsocks 密码"
  default     = ""
}

variable "enable_spot" {
  type        = bool
  description = "是否使用抢占式实例"
  default     = true
}

resource "random_integer" "ss_port" {
  min = 20000
  max = 40000
}

resource "random_password" "ss_pass" {
  length           = 16
  special          = true
  override_special = "_%@"
}

resource "random_password" "password" {
  length           = 10
  special          = true
  override_special = "_%@"
}

data "alicloud_zones" "default" {
  available_resource_creation = "VSwitch"
}

locals {
  effective_node_count = var.node_count > 0 ? var.node_count : {{.NodeCount}}
  effective_ss_port    = var.ss_port != "" ? var.ss_port : tostring(random_integer.ss_port.result)
  effective_ss_pass    = var.ss_pass != "" ? var.ss_pass : random_password.ss_pass.result
  selected_zone        = data.alicloud_zones.default.zones[0].id
}

resource "alicloud_instance" "instance" {
  count                      = local.effective_node_count
  security_groups            = [alicloud_security_group.group.id]
  instance_type              = "{{.InstanceType}}"
  image_id                   = "debian_11_7_x64_20G_alibase_20230907.vhd"
  instance_name              = "proxy-node-${count.index + 1}"
  vswitch_id                 = alicloud_vswitch.vswitch.id
  system_disk_size           = 20
  internet_max_bandwidth_out = 100
  password                   = random_password.password.result
  instance_charge_type       = "PostPaid"
  spot_strategy              = var.enable_spot ? "SpotWithPriceLimit" : "NoSpot"
  spot_price_limit           = var.enable_spot ? 0 : 0
  
  user_data = <<EOF
#!/bin/bash
sudo apt-get update
sudo apt-get install -y ca-certificates shadowsocks-libev wget lrzsz tmux

sudo echo '{' > /etc/shadowsocks-libev/config.json
sudo echo '    "server":["0.0.0.0"],' >> /etc/shadowsocks-libev/config.json
sudo echo "    \"server_port\":${local.effective_ss_port}," >> /etc/shadowsocks-libev/config.json
sudo echo '    "method":"chacha20-ietf-poly1305",' >> /etc/shadowsocks-libev/config.json
sudo echo "    \"password\":\"${local.effective_ss_pass}\"," >> /etc/shadowsocks-libev/config.json
sudo echo '    "mode":"tcp_and_udp",' >> /etc/shadowsocks-libev/config.json
sudo echo '    "fast_open":false' >> /etc/shadowsocks-libev/config.json
sudo echo '}' >> /etc/shadowsocks-libev/config.json

sudo echo "net.core.default_qdisc=fq" >> /etc/sysctl.conf
sudo echo "net.ipv4.tcp_congestion_control=bbr" >> /etc/sysctl.conf
sudo sysctl -p

sudo echo "nameserver 223.5.5.5" > /etc/resolv.conf
sudo service shadowsocks-libev restart

sudo wget "http://update2.aegis.aliyun.com/download/uninstall.sh"
sudo chmod +x uninstall.sh
sudo ./uninstall.sh
EOF

  depends_on = [alicloud_security_group.group]
}

resource "alicloud_security_group" "group" {
  security_group_name = "proxy_security_group"
  vpc_id              = alicloud_vpc.vpc.id
}

resource "alicloud_security_group_rule" "allow_all_tcp" {
  type              = "ingress"
  ip_protocol       = "tcp"
  nic_type          = "intranet"
  policy            = "accept"
  port_range        = "1/65535"
  priority          = 1
  security_group_id = alicloud_security_group.group.id
  cidr_ip           = "0.0.0.0/0"
  depends_on        = [alicloud_security_group.group]
}

resource "alicloud_security_group_rule" "allow_all_udp" {
  type              = "ingress"
  ip_protocol       = "udp"
  nic_type          = "intranet"
  policy            = "accept"
  port_range        = "1/65535"
  priority          = 1
  security_group_id = alicloud_security_group.group.id
  cidr_ip           = "0.0.0.0/0"
  depends_on        = [alicloud_security_group.group]
}

resource "alicloud_vpc" "vpc" {
  vpc_name   = "proxy_vpc"
  cidr_block = "172.16.0.0/16"
}

resource "alicloud_vswitch" "vswitch" {
  vpc_id       = alicloud_vpc.vpc.id
  cidr_block   = "172.16.0.0/24"
  zone_id      = local.selected_zone
  vswitch_name = "proxy_vswitch"
}
`

	t := template.Must(template.New("main.tf").Parse(tmpl))
	var buf strings.Builder
	if err := t.Execute(&buf, map[string]interface{}{
		"Region":       region,
		"InstanceType": instanceType,
		"NodeCount":    nodeCount,
	}); err != nil {
		logger.GetLogger().Error("生成模板失败: %v", err)
		return ""
	}

	return buf.String()
}

// generateAliyunTaskExecutorTemplate 生成阿里云工具执行模板
func (g *templateGenerator) generateAliyunTaskExecutorTemplate(tempDir, region, instanceType string, options map[string]interface{}) (string, error) {
	// 设置默认值
	if region == "" {
		region = "cn-beijing"
	}
	if instanceType == "" {
		instanceType = "ecs.t6-c1m1.small"
	}

	// 生成 main.tf
	mainTfContent := g.generateAliyunTaskExecutorMainTf(region, instanceType, options)
	if err := os.WriteFile(filepath.Join(tempDir, "main.tf"), []byte(mainTfContent), 0644); err != nil {
		return "", fmt.Errorf("写入 main.tf 失败: %w", err)
	}

	// 生成 versions.tf
	versionsTfContent := `terraform {
  required_version = ">= 1.0"
  
  required_providers {
    alicloud = {
      source  = "aliyun/alicloud"
      version = "~> 1.200"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.1"
    }
  }
}
`
	if err := os.WriteFile(filepath.Join(tempDir, "versions.tf"), []byte(versionsTfContent), 0644); err != nil {
		return "", fmt.Errorf("写入 versions.tf 失败: %w", err)
	}

	// 生成 variables.tf
	variablesTfContent := `variable "instance_type" {
  type        = string
  description = "实例类型"
  default     = "{{.InstanceType}}"
}

variable "region" {
  type        = string
  description = "区域"
  default     = "{{.Region}}"
}

variable "program_oss_path" {
  type        = string
  description = "OSS中的工具路径"
  default     = ""
}

variable "execution_args" {
  type        = string
  description = "工具执行参数"
  default     = ""
}

variable "tool_oss_bucket" {
  type        = string
  description = "工具OSS存储桶"
  default     = "aliyuncloudtools"
}

variable "spot_strategy" {
  type        = string
  description = "抢占式策略"
  default     = "SpotWithPriceLimit"
}

variable "spot_price_limit" {
  type        = number
  description = "抢占式实例最高出价"
  default     = 0
}

variable "result_path" {
  type        = string
  description = "结果存储路径"
  default     = ""
}
`
	t := template.Must(template.New("variables.tf").Parse(variablesTfContent))
	var buf strings.Builder
	if err := t.Execute(&buf, map[string]interface{}{
		"Region":       region,
		"InstanceType": instanceType,
	}); err != nil {
		return "", fmt.Errorf("生成 variables.tf 失败: %w", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "variables.tf"), []byte(buf.String()), 0644); err != nil {
		return "", fmt.Errorf("写入 variables.tf 失败: %w", err)
	}

	// 生成 outputs.tf
	outputsTfContent := `output "instance_id" {
  value = alicloud_instance.instance.id
}

output "public_ip" {
  value = alicloud_instance.instance.public_ip
}

output "password" {
  value     = random_password.password.result
  sensitive = true
}
`
	if err := os.WriteFile(filepath.Join(tempDir, "outputs.tf"), []byte(outputsTfContent), 0644); err != nil {
		return "", fmt.Errorf("写入 outputs.tf 失败: %w", err)
	}

	return tempDir, nil
}

// generateAliyunTaskExecutorMainTf 生成阿里云工具执行 main.tf 内容
func (g *templateGenerator) generateAliyunTaskExecutorMainTf(region, instanceType string, options map[string]interface{}) string {
	tmpl := `provider "alicloud" {
  region = var.region
}

provider "random" {}

resource "random_password" "password" {
  length           = 25
  special          = true
  override_special = "_+-."
}

locals {
  instance_name = "task-executor-spot"
  result_dir    = "/tmp/task-results"
  result_path   = var.result_path != "" ? var.result_path : "results/${replace(timestamp(), ":", "-")}/"
  region_host   = "oss-${var.region}.aliyuncs.com"
  tool_bucket   = var.tool_oss_bucket != "" ? var.tool_oss_bucket : "aliyuncloudtools"
}

resource "alicloud_vpc" "vpc" {
  vpc_name   = "${local.instance_name}-vpc"
  cidr_block = "172.16.0.0/16"
}

data "alicloud_zones" "with_instance_type" {
  available_resource_creation = "VSwitch"
  available_instance_type     = var.instance_type
}

data "alicloud_zones" "default" {
  available_resource_creation = "VSwitch"
}

locals {
  zones         = length(data.alicloud_zones.with_instance_type.zones) > 0 ? data.alicloud_zones.with_instance_type.zones : data.alicloud_zones.default.zones
  selected_zone = length(local.zones) > 0 ? local.zones[0].id : ""
}

resource "alicloud_vswitch" "vswitch" {
  vpc_id       = alicloud_vpc.vpc.id
  cidr_block   = "172.16.0.0/24"
  zone_id      = local.selected_zone
  vswitch_name = "${local.instance_name}-vsw"
}

resource "alicloud_security_group" "group" {
  security_group_name = "${local.instance_name}-sg"
  vpc_id              = alicloud_vpc.vpc.id
}

resource "alicloud_security_group_rule" "allow_ssh" {
  type              = "ingress"
  ip_protocol       = "tcp"
  nic_type          = "intranet"
  policy            = "accept"
  port_range        = "22/22"
  priority          = 1
  security_group_id = alicloud_security_group.group.id
  cidr_ip           = "0.0.0.0/0"
}

resource "alicloud_security_group_rule" "allow_all_egress" {
  type              = "egress"
  ip_protocol       = "all"
  nic_type          = "intranet"
  policy            = "accept"
  port_range        = "-1/-1"
  priority          = 1
  security_group_id = alicloud_security_group.group.id
  cidr_ip           = "0.0.0.0/0"
}

resource "alicloud_instance" "instance" {
  security_groups            = [alicloud_security_group.group.id]
  instance_type              = var.instance_type
  image_id                   = "debian_12_2_x64_20G_alibase_20231012.vhd"
  instance_name              = local.instance_name
  vswitch_id                 = alicloud_vswitch.vswitch.id
  system_disk_category       = "cloud_efficiency"
  system_disk_size           = 20
  internet_max_bandwidth_out = 100
  password                   = random_password.password.result
  instance_charge_type       = "PostPaid"
  spot_strategy              = var.spot_strategy
  spot_price_limit           = var.spot_price_limit

  user_data = <<EOF
#!/bin/bash
set -e

EXEC_LOG="/tmp/task-results/execution.log"
mkdir -p /tmp/task-results
echo "=== Task Execution Started ===" > $EXEC_LOG
echo "Timestamp: $(date)" >> $EXEC_LOG

# 安装 ossutil
echo "=== Installing ossutil ===" >> $EXEC_LOG
wget -q http://gosspublic.alicdn.com/ossutil/1.7.14/ossutil64 -O /usr/local/bin/ossutil
chmod +x /usr/local/bin/ossutil

# 配置 OSS（使用实例角色或环境变量）
if [ -n "$ALICLOUD_ACCESS_KEY_ID" ] && [ -n "$ALICLOUD_ACCESS_KEY_SECRET" ]; then
  /usr/local/bin/ossutil config -i "$ALICLOUD_ACCESS_KEY_ID" -k "$ALICLOUD_ACCESS_KEY_SECRET" -e ${local.region_host}
fi

# 下载工具
PROGRAM_PATH="${var.program_oss_path}"
TOOL_BUCKET="${local.tool_bucket}"
PROGRAM_DIR="/tmp/tools"
mkdir -p $PROGRAM_DIR

if [ -z "$PROGRAM_PATH" ]; then
  echo "ERROR: program_oss_path is required" >> $EXEC_LOG
  exit 1
fi

TOOL_NAME=$(basename "$PROGRAM_PATH")
PROGRAM_FILE="$PROGRAM_DIR/$TOOL_NAME"

echo "Downloading tool: oss://$TOOL_BUCKET/$PROGRAM_PATH" >> $EXEC_LOG
if /usr/local/bin/ossutil cp "oss://$TOOL_BUCKET/$PROGRAM_PATH" "$PROGRAM_FILE"; then
  chmod +x "$PROGRAM_FILE"
  echo "Tool downloaded successfully" >> $EXEC_LOG
else
  echo "ERROR: Failed to download tool" >> $EXEC_LOG
  exit 1
fi

# 执行工具
echo "=== Executing Tool ===" >> $EXEC_LOG
echo "Program: $PROGRAM_FILE" >> $EXEC_LOG
echo "Arguments: ${var.execution_args}" >> $EXEC_LOG
echo "Start Time: $(date)" >> $EXEC_LOG

START_TIME=$(date +%s)
if [ -n "${var.execution_args}" ]; then
  $PROGRAM_FILE ${var.execution_args} > /tmp/task-results/output.txt 2>&1
else
  $PROGRAM_FILE > /tmp/task-results/output.txt 2>&1
fi
EXIT_CODE=$?
END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

if [ $EXIT_CODE -eq 0 ]; then
  echo "Execution Status: SUCCESS" >> $EXEC_LOG
else
  echo "Execution Status: FAILED (Exit Code: $EXIT_CODE)" >> $EXEC_LOG
fi
echo "End Time: $(date)" >> $EXEC_LOG
echo "Duration: $${DURATION}s" >> $EXEC_LOG

# 准备结果
RESULT_FILE="/tmp/task-results/result.txt"
{
  echo "=== Execution Log ==="
  cat $EXEC_LOG
  echo ""
  echo "=== Program Output ==="
  cat /tmp/task-results/output.txt
} > $RESULT_FILE

# 上传结果到OSS（如果配置了存储桶）
if [ -n "$TOOL_BUCKET" ]; then
  echo "Uploading results to OSS..." >> $EXEC_LOG
  /usr/local/bin/ossutil cp $RESULT_FILE "oss://$TOOL_BUCKET/${local.result_path}result.txt" || true
  /usr/local/bin/ossutil cp /tmp/task-results/output.txt "oss://$TOOL_BUCKET/${local.result_path}output.txt" || true
fi

echo "Task execution completed" >> $EXEC_LOG
EOF

  depends_on = [alicloud_security_group.group]
}
`

	t := template.Must(template.New("main.tf").Parse(tmpl))
	var buf strings.Builder
	if err := t.Execute(&buf, map[string]interface{}{
		"Region":       region,
		"InstanceType": instanceType,
	}); err != nil {
		logger.GetLogger().Error("生成模板失败: %v", err)
		return ""
	}

	return buf.String()
}

// generateTencentProxyTemplate 生成腾讯云代理模板
func (g *templateGenerator) generateTencentProxyTemplate(tempDir, region, instanceType string, options map[string]interface{}) (string, error) {
	return "", fmt.Errorf("腾讯云代理模板生成功能待实现")
}

// generateTencentTaskExecutorTemplate 生成腾讯云工具执行模板
func (g *templateGenerator) generateTencentTaskExecutorTemplate(tempDir, region, instanceType string, options map[string]interface{}) (string, error) {
	return "", fmt.Errorf("腾讯云工具执行模板生成功能待实现")
}

// generateAWSProxyTemplate 生成AWS代理模板
func (g *templateGenerator) generateAWSProxyTemplate(tempDir, region, instanceType string, options map[string]interface{}) (string, error) {
	return "", fmt.Errorf("AWS代理模板生成功能待实现")
}

// generateAWSTaskExecutorTemplate 生成AWS工具执行模板
func (g *templateGenerator) generateAWSTaskExecutorTemplate(tempDir, region, instanceType string, options map[string]interface{}) (string, error) {
	return "", fmt.Errorf("AWS工具执行模板生成功能待实现")
}
