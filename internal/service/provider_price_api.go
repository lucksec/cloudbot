package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/meta-matrix/meta-matrix/internal/credentials"
	"github.com/meta-matrix/meta-matrix/internal/domain"
)

// fetchAliyunPrice 通过阿里云 API 获取价格
func (f *terraformPriceFetcher) fetchAliyunPrice(ctx context.Context, template, region string) (*domain.PriceInfo, error) {
	// 方法1: 使用 aliyun-cli 查询价格
	// aliyun ecs DescribeInstanceTypes --InstanceTypeFamily ecs.t5

	// 方法2: 直接调用阿里云 OpenAPI
	// https://help.aliyun.com/document_detail/100084.html

	// 尝试使用 aliyun-cli
	if f.hasCommand("aliyun") {
		price, err := f.fetchAliyunPriceViaCLI(ctx, template, region)
		if err == nil {
			return price, nil
		}
	}

	// 如果 CLI 不可用，尝试调用 API
	// 注意：需要配置 AccessKey 和 SecretKey
	return f.fetchAliyunPriceViaAPI(ctx, template, region)
}

// fetchAliyunPriceViaCLI 通过 aliyun-cli 获取价格
func (f *terraformPriceFetcher) fetchAliyunPriceViaCLI(ctx context.Context, template, region string) (*domain.PriceInfo, error) {
	// 根据模板确定实例类型
	instanceType := f.getAliyunInstanceType(template)
	if instanceType == "" {
		return nil, fmt.Errorf("无法确定实例类型")
	}

	// 执行 aliyun-cli 命令查询价格
	// 注意：这需要配置 aliyun-cli 的认证信息
	cmd := exec.CommandContext(ctx, "aliyun", "ecs", "DescribeInstanceTypes",
		"--InstanceTypeFamily", instanceType,
		"--RegionId", region)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("执行 aliyun-cli 失败: %w", err)
	}

	// 解析输出获取价格
	price, err := f.parseAliyunPriceOutput(string(output), template, region)
	if err != nil {
		return nil, err
	}

	return price, nil
}

// fetchAliyunPriceViaAPI 通过阿里云 OpenAPI 获取价格
// 使用 DescribePrice API: https://next.api.aliyun.com/document/Ecs/2014-05-26/DescribePrice
func (f *terraformPriceFetcher) fetchAliyunPriceViaAPI(ctx context.Context, template, region string) (*domain.PriceInfo, error) {
	// 尝试从凭据管理器获取 AccessKey
	var accessKey, secretKey string

	// 优先从凭据管理器获取
	credManager := credentials.GetDefaultManager()
	if credManager.HasCredentials(credentials.ProviderAliyun) {
		creds, err := credManager.GetCredentials(credentials.ProviderAliyun)
		if err == nil && creds != nil {
			accessKey = creds.AccessKey
			secretKey = creds.SecretKey
		}
	}

	// 如果凭据管理器中没有，尝试从环境变量获取（向后兼容）
	if accessKey == "" || secretKey == "" {
		accessKey = os.Getenv("ALICLOUD_ACCESS_KEY")
		secretKey = os.Getenv("ALICLOUD_SECRET_KEY")
	}

	if accessKey == "" || secretKey == "" {
		// 如果没有配置，尝试使用默认价格
		return f.getDefaultPrice("aliyun", template, region)
	}

	// 创建价格优化器
	optimizer := NewAliyunPriceOptimizer(f.config, accessKey, secretKey)

	// 获取实例类型
	instanceType := f.getAliyunInstanceType(template)
	if instanceType == "" {
		return f.getDefaultPrice("aliyun", template, region)
	}

	// 如果未指定区域，查找最便宜的区域
	if region == "" {
		optimal, err := optimizer.FindCheapestInstance(ctx, instanceType, nil)
		if err == nil && optimal != nil {
			return &domain.PriceInfo{
				Provider:      "aliyun",
				Template:      template,
				Region:        optimal.Region,
				PricePerHour:  optimal.Price,
				PricePerMonth: optimal.PricePerMonth,
				Currency:      optimal.Currency,
				Spec:          f.getAliyunSpec(instanceType),
				UpdatedAt:     time.Now().Format("2006-01-02"),
			}, nil
		}
		// 如果查找失败，使用默认区域
		region = "cn-beijing"
	}

	// 获取指定区域的价格
	price, err := optimizer.GetPriceForInstance(ctx, instanceType, region)
	if err != nil {
		return f.getDefaultPrice("aliyun", template, region)
	}

	return &domain.PriceInfo{
		Provider:      "aliyun",
		Template:      template,
		Region:        price.Region,
		PricePerHour:  price.PricePerHour,
		PricePerMonth: price.PricePerMonth,
		Currency:      price.Currency,
		Spec:          f.getAliyunSpec(instanceType),
		UpdatedAt:     time.Now().Format("2006-01-02"),
	}, nil
}

// getAliyunSpec 根据实例类型获取规格描述
func (f *terraformPriceFetcher) getAliyunSpec(instanceType string) string {
	// 简单的规格映射，实际可以从 API 获取
	specMap := map[string]string{
		"ecs.t5-lc1m1.small": "1核1G",
		"ecs.t5-lc1m2.small": "1核2G",
		"ecs.t6-c1m1.large":  "1核2G",
	}

	if spec, ok := specMap[instanceType]; ok {
		return spec
	}

	return instanceType
}

// getAliyunInstanceType 根据模板名称获取阿里云实例类型
func (f *terraformPriceFetcher) getAliyunInstanceType(template string) string {
	typeMap := map[string]string{
		"ecs":          "ecs.t5-lc1m1",
		"ecs1c2g":      "ecs.t5-lc1m2",
		"aliyun-proxy": "ecs.t5-lc1m1",
	}

	if instanceType, ok := typeMap[template]; ok {
		return instanceType
	}

	return "ecs.t5-lc1m1" // 默认类型
}

// parseAliyunPriceOutput 解析阿里云 CLI 输出
func (f *terraformPriceFetcher) parseAliyunPriceOutput(output, template, region string) (*domain.PriceInfo, error) {
	// 解析 JSON 输出
	var result struct {
		InstanceTypes struct {
			InstanceType []struct {
				InstanceTypeId string
				CpuCoreCount   int
				MemorySize     float64
				Price          struct {
					Price float64
				}
			}
		}
	}

	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return nil, fmt.Errorf("解析输出失败: %w", err)
	}

	if len(result.InstanceTypes.InstanceType) == 0 {
		return nil, fmt.Errorf("未找到实例类型信息")
	}

	instance := result.InstanceTypes.InstanceType[0]

	// 计算每月价格（假设按小时计费）
	pricePerHour := instance.Price.Price
	pricePerMonth := pricePerHour * 24 * 30

	return &domain.PriceInfo{
		Provider:      "aliyun",
		Template:      template,
		Region:        region,
		PricePerHour:  pricePerHour,
		PricePerMonth: pricePerMonth,
		Currency:      "CNY",
		Spec:          fmt.Sprintf("%d核%.0fG", instance.CpuCoreCount, instance.MemorySize),
		UpdatedAt:     time.Now().Format("2006-01-02"),
	}, nil
}

// fetchTencentPrice 通过腾讯云 API 获取价格
func (f *terraformPriceFetcher) fetchTencentPrice(ctx context.Context, template, region string) (*domain.PriceInfo, error) {
	// 腾讯云价格查询 API
	// https://cloud.tencent.com/document/product/213/2177

	// 尝试使用 tccli
	if f.hasCommand("tccli") {
		price, err := f.fetchTencentPriceViaCLI(ctx, template, region)
		if err == nil {
			return price, nil
		}
	}

	return f.getDefaultPrice("tencent", template, region)
}

// fetchTencentPriceViaCLI 通过 tccli 获取价格
func (f *terraformPriceFetcher) fetchTencentPriceViaCLI(ctx context.Context, template, region string) (*domain.PriceInfo, error) {
	// tccli cvm DescribeInstanceTypeConfigs --region ap-beijing
	cmd := exec.CommandContext(ctx, "tccli", "cvm", "DescribeInstanceTypeConfigs",
		"--region", region)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("执行 tccli 失败: %w", err)
	}

	// 解析输出
	return f.parseTencentPriceOutput(string(output), template, region)
}

// parseTencentPriceOutput 解析腾讯云 CLI 输出
func (f *terraformPriceFetcher) parseTencentPriceOutput(output, template, region string) (*domain.PriceInfo, error) {
	// 解析 JSON 输出并提取价格信息
	// 实际实现需要根据腾讯云 API 返回格式解析
	return f.getDefaultPrice("tencent", template, region)
}

// fetchAWSPrice 通过 AWS API 获取价格
func (f *terraformPriceFetcher) fetchAWSPrice(ctx context.Context, template, region string) (*domain.PriceInfo, error) {
	// AWS Pricing API
	// https://docs.aws.amazon.com/aws-cost-management/latest/APIReference/API_pricing_GetProducts.html

	// 尝试使用 AWS CLI
	if f.hasCommand("aws") {
		price, err := f.fetchAWSPriceViaCLI(ctx, template, region)
		if err == nil {
			return price, nil
		}
	}

	return f.getDefaultPrice("aws", template, region)
}

// fetchAWSPriceViaCLI 通过 AWS CLI 获取价格
func (f *terraformPriceFetcher) fetchAWSPriceViaCLI(ctx context.Context, template, region string) (*domain.PriceInfo, error) {
	// 根据模板确定实例类型
	instanceType := f.getAWSInstanceType(template)
	if instanceType == "" {
		return nil, fmt.Errorf("无法确定实例类型")
	}

	// AWS Pricing API 查询
	// aws pricing get-products --service-code AmazonEC2 --region us-east-1
	cmd := exec.CommandContext(ctx, "aws", "pricing", "get-products",
		"--service-code", "AmazonEC2",
		"--region", "us-east-1", // Pricing API 只在 us-east-1 和 ap-south-1 可用
		"--filters", fmt.Sprintf("Type=TERM_MATCH,Field=instanceType,Value=%s", instanceType))

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("执行 AWS CLI 失败: %w", err)
	}

	return f.parseAWSPriceOutput(string(output), template, region)
}

// getAWSInstanceType 根据模板名称获取 AWS 实例类型
func (f *terraformPriceFetcher) getAWSInstanceType(template string) string {
	typeMap := map[string]string{
		"ec2":    "t3.micro",
		"ec2-1G": "t3.small",
		"ec2-4G": "t3.medium",
	}

	if instanceType, ok := typeMap[template]; ok {
		return instanceType
	}

	return "t3.micro" // 默认类型
}

// parseAWSPriceOutput 解析 AWS CLI 输出
func (f *terraformPriceFetcher) parseAWSPriceOutput(output, template, region string) (*domain.PriceInfo, error) {
	// AWS Pricing API 返回的是复杂的 JSON 结构
	// 需要解析 OnDemand 价格
	var result struct {
		PriceList []struct {
			Product struct {
				Attributes struct {
					InstanceType string `json:"instanceType"`
				}
			}
			Terms struct {
				OnDemand map[string]struct {
					PriceDimensions map[string]struct {
						PricePerUnit struct {
							USD string
						}
					}
				}
			}
		}
	}

	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return nil, fmt.Errorf("解析输出失败: %w", err)
	}

	if len(result.PriceList) == 0 {
		return nil, fmt.Errorf("未找到价格信息")
	}

	// 提取价格
	priceItem := result.PriceList[0]
	var pricePerHour float64

	for _, term := range priceItem.Terms.OnDemand {
		for _, dimension := range term.PriceDimensions {
			if priceStr := dimension.PricePerUnit.USD; priceStr != "" {
				if p, err := parseFloat(priceStr); err == nil {
					pricePerHour = p
					break
				}
			}
		}
		if pricePerHour > 0 {
			break
		}
	}

	if pricePerHour == 0 {
		return nil, fmt.Errorf("无法提取价格")
	}

	pricePerMonth := pricePerHour * 24 * 30

	return &domain.PriceInfo{
		Provider:      "aws",
		Template:      template,
		Region:        region,
		PricePerHour:  pricePerHour,
		PricePerMonth: pricePerMonth,
		Currency:      "USD",
		Spec:          f.getAWSSpec(template),
		UpdatedAt:     time.Now().Format("2006-01-02"),
	}, nil
}

// getAWSSpec 获取 AWS 规格描述
func (f *terraformPriceFetcher) getAWSSpec(template string) string {
	specMap := map[string]string{
		"ec2":    "1核1G",
		"ec2-1G": "1核2G",
		"ec2-4G": "2核4G",
	}

	if spec, ok := specMap[template]; ok {
		return spec
	}

	return "1核1G"
}

// fetchVultrPrice 通过 Vultr API 获取价格
func (f *terraformPriceFetcher) fetchVultrPrice(ctx context.Context, template, region string) (*domain.PriceInfo, error) {
	// Vultr API: https://www.vultr.com/api/#tag/plans
	url := "https://api.vultr.com/v2/plans"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API 返回错误: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	return f.parseVultrPriceOutput(string(body), template, region)
}

// parseVultrPriceOutput 解析 Vultr API 输出
func (f *terraformPriceFetcher) parseVultrPriceOutput(output, template, region string) (*domain.PriceInfo, error) {
	var result struct {
		Plans []struct {
			ID          string
			VcpuCount   int    `json:"vcpu_count"`
			RAM         int    `json:"ram"`
			Disk        int    `json:"disk"`
			MonthlyCost string `json:"monthly_cost"`
		}
	}

	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return nil, fmt.Errorf("解析输出失败: %w", err)
	}

	if len(result.Plans) == 0 {
		return nil, fmt.Errorf("未找到价格信息")
	}

	// 选择最便宜的方案（或根据模板匹配）
	plan := result.Plans[0]
	monthlyPrice, err := parseFloat(plan.MonthlyCost)
	if err != nil {
		return nil, fmt.Errorf("解析价格失败: %w", err)
	}

	hourlyPrice := monthlyPrice / (24 * 30)

	return &domain.PriceInfo{
		Provider:      "vultr",
		Template:      template,
		Region:        region,
		PricePerHour:  hourlyPrice,
		PricePerMonth: monthlyPrice,
		Currency:      "USD",
		Spec:          fmt.Sprintf("%d核%dG", plan.VcpuCount, plan.RAM/1024),
		UpdatedAt:     time.Now().Format("2006-01-02"),
	}, nil
}

// hasCommand 检查命令是否存在
func (f *terraformPriceFetcher) hasCommand(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// parseFloat 解析浮点数字符串
func parseFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	return strconv.ParseFloat(s, 64)
}
