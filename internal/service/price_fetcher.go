package service

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"time"

	"github.com/lucksec/cloudbot/internal/config"
	"github.com/lucksec/cloudbot/internal/domain"
	"github.com/lucksec/cloudbot/internal/repository"
)

// PriceFetcher 价格查询器接口
// 用于从云服务商动态获取实时价格
type PriceFetcher interface {
	// FetchPrice 获取指定模板的实时价格
	FetchPrice(ctx context.Context, provider, template, region string) (*domain.PriceInfo, error)

	// FetchPricesByType 获取指定类型的所有模板价格
	FetchPricesByType(ctx context.Context, templateType string) ([]*domain.PriceInfo, error)
}

// terraformPriceFetcher 通过 Terraform plan 和云服务商 API 获取价格
type terraformPriceFetcher struct {
	config       *config.Config
	templateRepo repository.TemplateRepository
	terraformSvc TerraformService
	httpClient   *http.Client
}

// NewTerraformPriceFetcher 创建 Terraform 价格查询器
func NewTerraformPriceFetcher(cfg *config.Config, templateRepo repository.TemplateRepository, terraformSvc TerraformService) PriceFetcher {
	return &terraformPriceFetcher{
		config:       cfg,
		templateRepo: templateRepo,
		terraformSvc: terraformSvc,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

// FetchPrice 通过 Terraform plan 获取价格
func (f *terraformPriceFetcher) FetchPrice(ctx context.Context, provider, template, region string) (*domain.PriceInfo, error) {
	// 检查模板是否存在
	_, err := f.templateRepo.GetTemplate(provider, template)
	if err != nil {
		return nil, fmt.Errorf("获取模板失败: %w", err)
	}

	// 创建临时目录用于执行 plan
	tempDir, err := os.MkdirTemp("", "price-fetch-*")
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// 复制模板到临时目录
	if err := f.templateRepo.CopyTemplate(provider, template, tempDir); err != nil {
		return nil, fmt.Errorf("复制模板失败: %w", err)
	}

	// 初始化 Terraform
	if err := f.terraformSvc.Init(ctx, tempDir); err != nil {
		// 如果初始化失败，尝试使用默认价格
		return f.getDefaultPrice(provider, template, region)
	}

	// 执行 plan 并捕获输出
	price, err := f.parsePlanOutput(ctx, tempDir, provider, template, region)
	if err != nil {
		// 如果解析失败，使用默认价格
		return f.getDefaultPrice(provider, template, region)
	}

	return price, nil
}

// parsePlanOutput 解析 Terraform plan 输出获取价格信息
func (f *terraformPriceFetcher) parsePlanOutput(ctx context.Context, workDir, provider, template, region string) (*domain.PriceInfo, error) {
	// 执行 terraform plan -out=plan.out
	planCmd := exec.CommandContext(ctx, f.config.Terraform.ExecPath, "plan", "-out=plan.out", "-no-color")
	planCmd.Dir = workDir
	planOutput, err := planCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("执行 plan 失败: %w", err)
	}

	// 尝试从输出中提取价格信息
	// 注意：Terraform 本身不直接输出价格，这里需要根据不同的 provider 解析
	price := f.extractPriceFromPlan(string(planOutput), provider, template, region)
	if price != nil {
		return price, nil
	}

	// 如果无法从 plan 输出中提取，尝试调用云服务商 API
	return f.fetchFromProviderAPI(ctx, provider, template, region)
}

// extractPriceFromPlan 从 plan 输出中提取价格信息
// 注意：Terraform plan 通常不包含价格信息，这里主要作为占位符
func (f *terraformPriceFetcher) extractPriceFromPlan(planOutput, provider, template, region string) *domain.PriceInfo {
	// 尝试从输出中查找实例类型和规格
	// 不同 provider 的输出格式不同，这里提供基础框架

	// 解析实例类型（例如：ecs.e-c1m2.large, t3.micro 等）
	instanceType := f.extractInstanceType(planOutput, provider)
	if instanceType == "" {
		return nil
	}

	// 根据实例类型和 provider 查询价格
	// 这里需要调用云服务商的定价 API
	return nil // 暂时返回 nil，由 fetchFromProviderAPI 处理
}

// extractInstanceType 从 plan 输出中提取实例类型
func (f *terraformPriceFetcher) extractInstanceType(planOutput, provider string) string {
	var pattern string
	switch provider {
	case "aliyun":
		// 阿里云实例类型格式：ecs.e-c1m2.large
		pattern = `instance_type\s*=\s*"([^"]+)"`
	case "aws":
		// AWS 实例类型格式：t3.micro
		pattern = `instance_type\s*=\s*"([^"]+)"`
	case "tencent":
		// 腾讯云实例类型格式：S1.SMALL1
		pattern = `instance_type\s*=\s*"([^"]+)"`
	default:
		return ""
	}

	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(planOutput)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// fetchFromProviderAPI 调用云服务商 API 获取价格
func (f *terraformPriceFetcher) fetchFromProviderAPI(ctx context.Context, provider, template, region string) (*domain.PriceInfo, error) {
	// 根据不同的 provider 调用相应的 API
	switch provider {
	case "aliyun":
		return f.fetchAliyunPrice(ctx, template, region)
	case "tencent":
		return f.fetchTencentPrice(ctx, template, region)
	case "aws":
		return f.fetchAWSPrice(ctx, template, region)
	case "vultr":
		return f.fetchVultrPrice(ctx, template, region)
	default:
		return f.getDefaultPrice(provider, template, region)
	}
}

// fetchAliyunPrice, fetchTencentPrice, fetchAWSPrice, fetchVultrPrice
// 这些方法的实现已移至 provider_price_api.go 文件

// getDefaultPrice 获取默认价格（当 API 调用失败时使用）
func (f *terraformPriceFetcher) getDefaultPrice(provider, template, region string) (*domain.PriceInfo, error) {
	// 从配置文件或缓存中获取价格
	// 这里返回一个基础结构，实际应该从价格仓库获取
	now := time.Now().Format("2006-01-02")

	// 根据 provider 和 template 返回默认价格
	defaultPrices := map[string]*domain.PriceInfo{
		"aliyun/ecs": {
			Provider:      "aliyun",
			Template:      "ecs",
			Region:        region,
			PricePerHour:  0.08,
			PricePerMonth: 58.0,
			Currency:      "CNY",
			Spec:          "1核2G",
			UpdatedAt:     now,
		},
		"tencent/ecs": {
			Provider:      "tencent",
			Template:      "ecs",
			Region:        region,
			PricePerHour:  0.07,
			PricePerMonth: 51.0,
			Currency:      "CNY",
			Spec:          "1核2G",
			UpdatedAt:     now,
		},
		"aws/ec2": {
			Provider:      "aws",
			Template:      "ec2",
			Region:        region,
			PricePerHour:  0.0104,
			PricePerMonth: 7.5,
			Currency:      "USD",
			Spec:          "1核1G",
			UpdatedAt:     now,
		},
	}

	key := fmt.Sprintf("%s/%s", provider, template)
	if price, ok := defaultPrices[key]; ok {
		return price, nil
	}

	return nil, fmt.Errorf("未找到模板 %s/%s 的默认价格", provider, template)
}

// FetchPricesByType 获取指定类型的所有模板价格
func (f *terraformPriceFetcher) FetchPricesByType(ctx context.Context, templateType string) ([]*domain.PriceInfo, error) {
	// 根据类型获取所有相关模板
	templates := f.getTemplatesByType(templateType)

	var prices []*domain.PriceInfo
	for _, tmpl := range templates {
		price, err := f.FetchPrice(ctx, tmpl.Provider, tmpl.Template, tmpl.Region)
		if err == nil {
			prices = append(prices, price)
		}
	}

	return prices, nil
}

// getTemplatesByType 根据类型获取模板列表
func (f *terraformPriceFetcher) getTemplatesByType(templateType string) []struct {
	Provider string
	Template string
	Region   string
} {
	typeMap := map[string][]struct {
		Provider string
		Template string
		Region   string
	}{
		"ecs": {
			{"aliyun", "ecs", "cn-beijing"},
			{"tencent", "ecs", "ap-beijing"},
		},
		"proxy": {
			{"aliyun", "aliyun-proxy", "cn-beijing"},
		},
		"ec2": {
			{"aws", "ec2", "us-east-1"},
		},
		"vps": {
			{"vultr", "hk-vps", "hk"},
		},
	}

	if templates, ok := typeMap[templateType]; ok {
		return templates
	}

	return []struct {
		Provider string
		Template string
		Region   string
	}{}
}

// 注意：TemplateRepository 接口已在 repository 包中定义
// 这里需要导入 domain 包
