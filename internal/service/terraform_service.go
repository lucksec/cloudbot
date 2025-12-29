package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/meta-matrix/meta-matrix/internal/config"
	"github.com/meta-matrix/meta-matrix/internal/credentials"
	"github.com/meta-matrix/meta-matrix/internal/logger"
)

// TerraformService Terraform 操作服务接口
type TerraformService interface {
	// Init 初始化 Terraform
	Init(ctx context.Context, workDir string) error

	// Plan 执行 Terraform plan
	// 可选传入 vars，在执行时通过 -var 形式传递
	Plan(ctx context.Context, workDir string, vars map[string]string) error

	// Apply 执行 Terraform apply
	// 可选传入 vars，在执行时通过 -var 形式传递
	Apply(ctx context.Context, workDir string, autoApprove bool, vars map[string]string) error

	// Destroy 执行 Terraform destroy
	// 可选传入 vars，在执行时通过 -var 形式传递
	Destroy(ctx context.Context, workDir string, autoApprove bool, vars map[string]string) error

	// Output 获取 Terraform output
	Output(ctx context.Context, workDir string) (map[string]string, error)

	// Validate 验证 Terraform 配置
	Validate(ctx context.Context, workDir string) error

	// StateList 获取当前 Terraform 状态中的资源列表
	// 用于进行云资源验证，判断场景是否真正创建了云端资源
	StateList(ctx context.Context, workDir string) ([]string, error)

	// ShowInstances 获取状态中云主机的详细信息（针对 ECS/EC2 等实例类资源）
	ShowInstances(ctx context.Context, workDir string) ([]ECSInstanceDetail, error)
}

// terraformService Terraform 服务实现
type terraformService struct {
	config *config.Config
}

// NewTerraformService 创建 Terraform 服务实例
func NewTerraformService(cfg *config.Config) TerraformService {
	return &terraformService{
		config: cfg,
	}
}

// Init 初始化 Terraform
func (s *terraformService) Init(ctx context.Context, workDir string) error {
	log := logger.GetLogger()
	log.Info("开始初始化 Terraform: workDir=%s", workDir)

	// 设置云服务商凭证环境变量（即使 init 可能不需要，也设置以确保一致性）
	env := s.setupCloudProviderEnv(workDir, make(map[string]string))

	cmd := exec.CommandContext(ctx, s.config.Terraform.ExecPath, "init")
	cmd.Dir = workDir
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Error("Terraform init 失败: workDir=%s, error=%v", workDir, err)
		return fmt.Errorf("Terraform init 失败: %w", err)
	}

	log.Info("Terraform init 成功: workDir=%s", workDir)
	return nil
}

// Plan 执行 Terraform plan
func (s *terraformService) Plan(ctx context.Context, workDir string, vars map[string]string) error {
	log := logger.GetLogger()
	log.Debug("执行 Terraform plan: workDir=%s, vars=%v", workDir, vars)

	// 设置云服务商凭证环境变量
	env := s.setupCloudProviderEnv(workDir, vars)

	args := []string{"plan"}
	// 只传递非凭证变量（凭证通过环境变量传递）
	for k, v := range vars {
		// 跳过凭证相关变量，它们已通过环境变量传递
		if s.isCredentialVar(k) {
			continue
		}
		args = append(args, "-var", fmt.Sprintf("%s=%s", k, v))
	}

	cmd := exec.CommandContext(ctx, s.config.Terraform.ExecPath, args...)
	cmd.Dir = workDir
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Error("Terraform plan 失败: workDir=%s, error=%v", workDir, err)
		return fmt.Errorf("Terraform plan 失败: %w", err)
	}

	log.Info("Terraform plan 成功: workDir=%s", workDir)
	return nil
}

// Apply 执行 Terraform apply
func (s *terraformService) Apply(ctx context.Context, workDir string, autoApprove bool, vars map[string]string) error {
	log := logger.GetLogger()
	log.Info("执行 Terraform apply: workDir=%s, autoApprove=%v, vars=%v", workDir, autoApprove, vars)

	// 设置云服务商凭证环境变量
	env := s.setupCloudProviderEnv(workDir, vars)

	args := []string{"apply"}
	// 只传递非凭证变量（凭证通过环境变量传递）
	for k, v := range vars {
		// 跳过凭证相关变量，它们已通过环境变量传递
		if s.isCredentialVar(k) {
			continue
		}
		args = append(args, "-var", fmt.Sprintf("%s=%s", k, v))
	}
	if autoApprove {
		args = append(args, "-auto-approve")
	}

	cmd := exec.CommandContext(ctx, s.config.Terraform.ExecPath, args...)
	cmd.Dir = workDir
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Error("Terraform apply 失败: workDir=%s, error=%v", workDir, err)
		return fmt.Errorf("Terraform apply 失败: %w", err)
	}

	log.Info("Terraform apply 成功: workDir=%s", workDir)
	return nil
}

// Destroy 执行 Terraform destroy
func (s *terraformService) Destroy(ctx context.Context, workDir string, autoApprove bool, vars map[string]string) error {
	log := logger.GetLogger()
	log.Warn("执行 Terraform destroy: workDir=%s, autoApprove=%v, vars=%v", workDir, autoApprove, vars)

	// 设置云服务商凭证环境变量
	env := s.setupCloudProviderEnv(workDir, vars)

	args := []string{"destroy"}
	// 只传递非凭证变量（凭证通过环境变量传递）
	for k, v := range vars {
		// 跳过凭证相关变量，它们已通过环境变量传递
		if s.isCredentialVar(k) {
			continue
		}
		args = append(args, "-var", fmt.Sprintf("%s=%s", k, v))
	}
	if autoApprove {
		args = append(args, "-auto-approve")
	}

	cmd := exec.CommandContext(ctx, s.config.Terraform.ExecPath, args...)
	cmd.Dir = workDir
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Error("Terraform destroy 失败: workDir=%s, error=%v", workDir, err)
		return fmt.Errorf("Terraform destroy 失败: %w", err)
	}

	log.Info("Terraform destroy 成功: workDir=%s", workDir)
	return nil
}

// Output 获取 Terraform output
func (s *terraformService) Output(ctx context.Context, workDir string) (map[string]string, error) {
	// 设置云服务商凭证环境变量
	env := s.setupCloudProviderEnv(workDir, make(map[string]string))

	cmd := exec.CommandContext(ctx, s.config.Terraform.ExecPath, "output", "-json")
	cmd.Dir = workDir
	cmd.Env = env

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("获取 Terraform output 失败: %w", err)
	}

	// 解析 JSON 输出
	// 这里简化处理，实际应该解析 JSON
	outputs := make(map[string]string)
	outputs["raw"] = string(output)

	return outputs, nil
}

// Validate 验证 Terraform 配置
func (s *terraformService) Validate(ctx context.Context, workDir string) error {
	// 设置云服务商凭证环境变量（validate 可能不需要，但设置以确保一致性）
	env := s.setupCloudProviderEnv(workDir, make(map[string]string))

	cmd := exec.CommandContext(ctx, s.config.Terraform.ExecPath, "validate")
	cmd.Dir = workDir
	cmd.Env = env
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Terraform 配置验证失败: %w", err)
	}

	return nil
}

// StateList 获取当前 Terraform 状态中的资源列表
// 会调用 `terraform state list`，用于云资源验证
func (s *terraformService) StateList(ctx context.Context, workDir string) ([]string, error) {
	// 设置云服务商凭证环境变量
	env := s.setupCloudProviderEnv(workDir, make(map[string]string))

	cmd := exec.CommandContext(ctx, s.config.Terraform.ExecPath, "state", "list")
	cmd.Dir = workDir
	cmd.Env = env

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("获取 Terraform 云资源状态失败: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	var resources []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		resources = append(resources, line)
	}

	return resources, nil
}

// ShowInstances 通过 terraform show -json 解析实例资源详情
func (s *terraformService) ShowInstances(ctx context.Context, workDir string) ([]ECSInstanceDetail, error) {
	// 设置云服务商凭证环境变量
	env := s.setupCloudProviderEnv(workDir, make(map[string]string))

	cmd := exec.CommandContext(ctx, s.config.Terraform.ExecPath, "show", "-json")
	cmd.Dir = workDir
	cmd.Env = env

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("terraform show 失败: %w", err)
	}

	var state terraformState
	if err := json.Unmarshal(output, &state); err != nil {
		return nil, fmt.Errorf("解析 terraform show 输出失败: %w", err)
	}

	var instances []ECSInstanceDetail
	collectInstances(&instances, state.Values.RootModule)
	return instances, nil
}

// 解析 terraform show -json 的关键结构
type terraformState struct {
	Values struct {
		RootModule *tfModule `json:"root_module"`
	} `json:"values"`
}

type tfModule struct {
	Resources    []*tfResource `json:"resources"`
	ChildModules []*tfModule   `json:"child_modules"`
}

type tfResource struct {
	Address string                 `json:"address"`
	Type    string                 `json:"type"`
	Name    string                 `json:"name"`
	Values  map[string]interface{} `json:"values"`
}

func collectInstances(out *[]ECSInstanceDetail, m *tfModule) {
	if m == nil {
		return
	}
	for _, r := range m.Resources {
		if r == nil {
			continue
		}
		// 仅关心实例类资源
		if r.Type == "alicloud_instance" || r.Type == "aws_instance" || strings.Contains(r.Type, "instance") {
			detail := ECSInstanceDetail{
				Name:         r.Address,
				ID:           getString(r.Values, "id"),
				Region:       getString(r.Values, "region_id"),
				InstanceType: getString(r.Values, "instance_type"),
				Status:       getString(r.Values, "status"),
				PublicIPs:    getStringSlice(r.Values, "public_ip"),
				PrivateIPs:   getStringSlice(r.Values, "private_ip"),
			}
			// 兼容阿里云字段
			if detail.Region == "" {
				detail.Region = getString(r.Values, "region")
			}
			*out = append(*out, detail)
		}
	}
	for _, c := range m.ChildModules {
		collectInstances(out, c)
	}
}

func getString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getStringSlice(m map[string]interface{}, key string) []string {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	var res []string
	switch vv := v.(type) {
	case []interface{}:
		for _, item := range vv {
			if s, ok := item.(string); ok {
				res = append(res, s)
			}
		}
	case []string:
		res = append(res, vv...)
	case string:
		if vv != "" {
			res = append(res, vv)
		}
	}
	return res
}

// CheckTerraformInstalled 检查 Terraform 是否已安装
func CheckTerraformInstalled(execPath string) error {
	cmd := exec.Command(execPath, "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Terraform 未安装或不在 PATH 中: %w", err)
	}
	return nil
}

// ECSInstanceDetail 云主机详细信息（用于状态展示）
type ECSInstanceDetail struct {
	Name         string   `json:"name"`
	ID           string   `json:"id"`
	Region       string   `json:"region"`
	InstanceType string   `json:"instance_type"`
	Status       string   `json:"status"`
	PublicIPs    []string `json:"public_ips"`
	PrivateIPs   []string `json:"private_ips"`
}

// isCredentialVar 检查变量名是否为凭证相关变量
func (s *terraformService) isCredentialVar(varName string) bool {
	credentialVars := []string{
		"tencentcloud_secret_id",
		"tencentcloud_secret_key",
		"access_key",
		"secret_key",
		"alicloud_access_key",
		"alicloud_secret_key",
		"huaweicloud_access_key",
		"huaweicloud_secret_key",
		"aws_access_key_id",
		"aws_secret_access_key",
	}
	for _, credVar := range credentialVars {
		if varName == credVar {
			return true
		}
	}
	return false
}

// setupCloudProviderEnv 设置云服务商凭证环境变量
// 根据工作目录中的 Terraform 配置文件或传入的 vars 判断云服务商类型，并设置相应的环境变量
func (s *terraformService) setupCloudProviderEnv(workDir string, vars map[string]string) []string {
	// 获取当前环境变量
	env := os.Environ()
	envMap := make(map[string]string)
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// 从凭据管理器获取并设置环境变量
	credManager := credentials.GetDefaultManager()

	// 腾讯云：优先从凭据管理器获取（推荐方式）
	if credManager.HasCredentials(credentials.ProviderTencent) {
		creds, err := credManager.GetCredentials(credentials.ProviderTencent)
		if err == nil && creds != nil && creds.AccessKey != "" && creds.SecretKey != "" {
			envMap["TENCENTCLOUD_SECRET_ID"] = creds.AccessKey
			envMap["TENCENTCLOUD_SECRET_KEY"] = creds.SecretKey
			if creds.Region != "" {
				envMap["TENCENTCLOUD_REGION"] = creds.Region
			}
		}
	}
	// 如果 vars 中有腾讯云凭证变量（向后兼容），覆盖凭据管理器的值
	if secretId, ok := vars["tencentcloud_secret_id"]; ok && secretId != "" {
		if secretKey, ok := vars["tencentcloud_secret_key"]; ok && secretKey != "" {
			envMap["TENCENTCLOUD_SECRET_ID"] = secretId
			envMap["TENCENTCLOUD_SECRET_KEY"] = secretKey
		}
	}

	// 阿里云：优先从凭据管理器获取（推荐方式）
	if credManager.HasCredentials(credentials.ProviderAliyun) {
		creds, err := credManager.GetCredentials(credentials.ProviderAliyun)
		if err == nil && creds != nil && creds.AccessKey != "" && creds.SecretKey != "" {
			envMap["ALICLOUD_ACCESS_KEY"] = creds.AccessKey
			envMap["ALICLOUD_SECRET_KEY"] = creds.SecretKey
			if creds.Region != "" {
				envMap["ALICLOUD_REGION"] = creds.Region
			}
		}
	}
	// 如果 vars 中有阿里云凭证变量（向后兼容），覆盖凭据管理器的值
	if accessKey, ok := vars["access_key"]; ok && accessKey != "" {
		if secretKey, ok := vars["secret_key"]; ok && secretKey != "" {
			envMap["ALICLOUD_ACCESS_KEY"] = accessKey
			envMap["ALICLOUD_SECRET_KEY"] = secretKey
		}
	}

	// 检查 vars 中是否有华为云凭证变量
	if credManager.HasCredentials(credentials.ProviderHuaweicloud) {
		creds, err := credManager.GetCredentials(credentials.ProviderHuaweicloud)
		if err == nil && creds != nil {
			envMap["HUAWEICLOUD_ACCESS_KEY"] = creds.AccessKey
			envMap["HUAWEICLOUD_SECRET_KEY"] = creds.SecretKey
			if creds.Region != "" {
				envMap["HUAWEICLOUD_REGION"] = creds.Region
			}
		}
	}

	// 检查 vars 中是否有 AWS 凭证变量
	if credManager.HasCredentials(credentials.ProviderAWS) {
		creds, err := credManager.GetCredentials(credentials.ProviderAWS)
		if err == nil && creds != nil {
			envMap["AWS_ACCESS_KEY_ID"] = creds.AccessKey
			envMap["AWS_SECRET_ACCESS_KEY"] = creds.SecretKey
			if creds.Region != "" {
				envMap["AWS_REGION"] = creds.Region
			}
		}
	}

	// 将 envMap 转换回 []string
	result := make([]string, 0, len(envMap))
	for k, v := range envMap {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}

	return result
}
