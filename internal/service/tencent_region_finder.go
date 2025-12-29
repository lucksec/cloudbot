package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/lucksec/cloudbot/internal/credentials"
)

// TencentRegionFinder 腾讯云区域查找器
type TencentRegionFinder interface {
	// FindAvailableRegions 查找有抢占式实例配额的区域
	FindAvailableRegions(ctx context.Context, instanceType string) ([]string, error)

	// TestRegionSpotQuota 测试指定区域的抢占式实例配额
	TestRegionSpotQuota(ctx context.Context, region, instanceType string) (bool, error)
}

type tencentRegionFinder struct {
	credManager credentials.CredentialManager
}

// NewTencentRegionFinder 创建腾讯云区域查找器
func NewTencentRegionFinder() TencentRegionFinder {
	return &tencentRegionFinder{
		credManager: credentials.GetDefaultManager(),
	}
}

// 腾讯云可用区域列表（仅包含实际可用的6个区域）
var tencentRegions = []string{
	"ap-shanghai",  // 上海
	"ap-nanjing",   // 南京
	"ap-guangzhou", // 广州
	"ap-beijing",   // 北京
	"ap-chengdu",   // 成都
	"ap-chongqing", // 重庆
}

// GetDomesticRegions 获取国内区域列表（仅包含实际可用的6个区域）
func GetDomesticRegions() []string {
	return []string{
		"ap-shanghai",  // 上海
		"ap-nanjing",   // 南京
		"ap-guangzhou", // 广州
		"ap-beijing",   // 北京
		"ap-chengdu",   // 成都
		"ap-chongqing", // 重庆
	}
}

// FindAvailableRegions 查找有抢占式实例配额的区域
// 通过尝试创建实例来测试配额（使用 dry-run 或直接查询）
func (f *tencentRegionFinder) FindAvailableRegions(ctx context.Context, instanceType string) ([]string, error) {
	var availableRegions []string

	// 检查是否有腾讯云凭据
	if !f.credManager.HasCredentials(credentials.ProviderTencent) {
		return nil, fmt.Errorf("未配置腾讯云凭据，请先运行: credential set tencent")
	}

	creds, err := f.credManager.GetCredentials(credentials.ProviderTencent)
	if err != nil {
		return nil, fmt.Errorf("获取腾讯云凭据失败: %w", err)
	}

	// 使用 tccli 查询各区域的配额
	// 由于无法直接查询抢占式实例配额，我们尝试查询实例类型配置
	// 如果实例类型支持抢占式，则认为该区域可能有配额

	// 并发测试所有区域（限制并发数）
	type regionResult struct {
		region    string
		available bool
		err       error
	}

	results := make(chan regionResult, len(tencentRegions))
	semaphore := make(chan struct{}, 5) // 限制并发数为5

	for _, region := range tencentRegions {
		go func(r string) {
			semaphore <- struct{}{}        // 获取信号量
			defer func() { <-semaphore }() // 释放信号量

			available, err := f.testRegionAvailability(ctx, r, instanceType, creds.AccessKey, creds.SecretKey)
			results <- regionResult{
				region:    r,
				available: available,
				err:       err,
			}
		}(region)
	}

	// 收集结果
	for i := 0; i < len(tencentRegions); i++ {
		result := <-results
		if result.available {
			availableRegions = append(availableRegions, result.region)
		}
	}

	if len(availableRegions) == 0 {
		// 如果没有找到可用区域，返回所有区域（让用户尝试）
		return tencentRegions, nil
	}

	return availableRegions, nil
}

// testRegionAvailability 测试区域可用性
// 通过查询抢占式实例价格历史来判断是否支持抢占式实例
func (f *tencentRegionFinder) testRegionAvailability(ctx context.Context, region, instanceType, secretId, secretKey string) (bool, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// 方法1: 查询抢占式实例价格历史（更准确）
	// tccli cvm DescribeSpotPriceHistory --region ap-beijing --InstanceType S5.SMALL1
	cmd := exec.CommandContext(ctxWithTimeout, "tccli", "cvm", "DescribeSpotPriceHistory",
		"--region", region,
		"--InstanceType", instanceType,
		"--SecretId", secretId,
		"--SecretKey", secretKey)

	output, err := cmd.Output()
	if err != nil {
		// 如果查询价格历史失败，尝试查询实例类型配置
		return f.testRegionAvailabilityByInstanceType(ctx, region, instanceType, secretId, secretKey)
	}

	// 解析价格历史输出
	var priceResult struct {
		Response struct {
			SpotPriceHistorySet []struct {
				InstanceType string `json:"InstanceType"`
				Zone         string `json:"Zone"`
			} `json:"SpotPriceHistorySet"`
		} `json:"Response"`
	}

	if err := json.Unmarshal(output, &priceResult); err != nil {
		// 解析失败，尝试其他方法
		return f.testRegionAvailabilityByInstanceType(ctx, region, instanceType, secretId, secretKey)
	}

	// 如果有价格历史数据，说明该区域支持抢占式实例
	if len(priceResult.Response.SpotPriceHistorySet) > 0 {
		return true, nil
	}

	// 如果没有价格历史，尝试查询实例类型配置
	return f.testRegionAvailabilityByInstanceType(ctx, region, instanceType, secretId, secretKey)
}

// testRegionAvailabilityByInstanceType 通过查询实例类型配置来测试区域可用性
func (f *tencentRegionFinder) testRegionAvailabilityByInstanceType(ctx context.Context, region, instanceType, secretId, secretKey string) (bool, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// 使用 tccli 查询实例类型配置
	// tccli cvm DescribeInstanceTypeConfigs --region ap-beijing --InstanceType S5.SMALL1
	cmd := exec.CommandContext(ctxWithTimeout, "tccli", "cvm", "DescribeInstanceTypeConfigs",
		"--region", region,
		"--InstanceType", instanceType,
		"--SecretId", secretId,
		"--SecretKey", secretKey)

	output, err := cmd.Output()
	if err != nil {
		// 如果命令执行失败，可能是区域不支持或网络问题
		return false, nil // 不返回错误，只是标记为不可用
	}

	// 解析输出，检查是否支持抢占式实例
	var result struct {
		Response struct {
			InstanceTypeConfigSet []struct {
				InstanceType   string `json:"InstanceType"`
				InstanceFamily string `json:"InstanceFamily"`
				Status         string `json:"Status"`
			} `json:"InstanceTypeConfigSet"`
		} `json:"Response"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return false, nil
	}

	// 如果查询成功且有结果，认为该区域可用
	if len(result.Response.InstanceTypeConfigSet) > 0 {
		return true, nil
	}

	return false, nil
}

// TestRegionSpotQuota 测试指定区域的抢占式实例配额
// 通过尝试创建实例（dry-run）来测试
func (f *tencentRegionFinder) TestRegionSpotQuota(ctx context.Context, region, instanceType string) (bool, error) {
	// 这个方法可以通过实际尝试创建实例来测试配额
	// 但由于需要实际创建资源，这里只做基本检查
	return f.testRegionAvailability(ctx, region, instanceType, "", "")
}

// GetTencentRegions 获取所有腾讯云区域列表
func GetTencentRegions() []string {
	return tencentRegions
}

// FindBestTencentRegion 查找最佳腾讯云区域（有配额且价格较低）
func FindBestTencentRegion(ctx context.Context, instanceType string) (string, error) {
	finder := NewTencentRegionFinder()

	// 查找可用区域
	availableRegions, err := finder.FindAvailableRegions(ctx, instanceType)
	if err != nil {
		return "", err
	}

	if len(availableRegions) == 0 {
		// 如果没有找到，返回默认区域
		return "ap-beijing", nil
	}

	// 优先选择国内区域（通常价格更低）
	domesticRegions := []string{
		"ap-beijing",
		"ap-shanghai",
		"ap-guangzhou",
		"ap-chengdu",
		"ap-chongqing",
		"ap-nanjing",
	}

	for _, region := range domesticRegions {
		for _, available := range availableRegions {
			if region == available {
				return region, nil
			}
		}
	}

	// 如果没有国内区域，返回第一个可用区域
	return availableRegions[0], nil
}

// QuerySpotInstanceAvailability 查询指定区域的抢占式实例可用性
// 返回可用区域和实例类型的组合
func QuerySpotInstanceAvailability(ctx context.Context, regions []string, instanceFamily string) ([]SpotAvailability, error) {
	credManager := credentials.GetDefaultManager()
	if !credManager.HasCredentials(credentials.ProviderTencent) {
		return nil, fmt.Errorf("未配置腾讯云凭据")
	}

	creds, err := credManager.GetCredentials(credentials.ProviderTencent)
	if err != nil {
		return nil, fmt.Errorf("获取腾讯云凭据失败: %w", err)
	}

	var results []SpotAvailability

	// 并发查询所有区域
	resultChan := make(chan []SpotAvailability, len(regions))
	semaphore := make(chan struct{}, 5) // 限制并发数

	for _, region := range regions {
		go func(r string) {
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			var regionResults []SpotAvailability

			// 查询该区域的实例类型配置
			ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctxWithTimeout, "tccli", "cvm", "DescribeInstanceTypeConfigs",
				"--region", r,
				"--InstanceFamily", instanceFamily,
				"--SecretId", creds.AccessKey,
				"--SecretKey", creds.SecretKey)

			output, err := cmd.Output()
			if err != nil {
				resultChan <- regionResults
				return
			}

			var configResult struct {
				Response struct {
					InstanceTypeConfigSet []struct {
						InstanceType   string `json:"InstanceType"`
						InstanceFamily string `json:"InstanceFamily"`
						Status         string `json:"Status"`
					} `json:"InstanceTypeConfigSet"`
				} `json:"Response"`
			}

			if err := json.Unmarshal(output, &configResult); err != nil {
				resultChan <- regionResults
				return
			}

			// 对每个实例类型，查询抢占式实例价格
			for _, config := range configResult.Response.InstanceTypeConfigSet {
				if config.Status != "SELL" {
					continue // 跳过不可售的实例类型
				}

				// 查询抢占式实例价格
				priceCmd := exec.CommandContext(ctxWithTimeout, "tccli", "cvm", "DescribeSpotPriceHistory",
					"--region", r,
					"--InstanceType", config.InstanceType,
					"--SecretId", creds.AccessKey,
					"--SecretKey", creds.SecretKey)

				priceOutput, err := priceCmd.Output()
				if err != nil {
					continue // 如果查询价格失败，跳过该实例类型
				}

				var priceResult struct {
					Response struct {
						SpotPriceHistorySet []interface{} `json:"SpotPriceHistorySet"`
					} `json:"Response"`
				}

				if err := json.Unmarshal(priceOutput, &priceResult); err != nil {
					continue
				}

				// 如果有价格历史，说明支持抢占式实例
				if len(priceResult.Response.SpotPriceHistorySet) > 0 {
					regionResults = append(regionResults, SpotAvailability{
						Region:       r,
						InstanceType: config.InstanceType,
						Available:    true,
					})
				}
			}

			resultChan <- regionResults
		}(region)
	}

	// 收集结果
	for i := 0; i < len(regions); i++ {
		regionResults := <-resultChan
		results = append(results, regionResults...)
	}

	return results, nil
}

// SpotAvailability 抢占式实例可用性信息
type SpotAvailability struct {
	Region       string
	InstanceType string
	Available    bool
}
