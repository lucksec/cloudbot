package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/meta-matrix/meta-matrix/internal/config"
)

// AliyunPriceOptimizer 阿里云价格优化器
// 通过 DescribePrice API 获取实时价格并选择最优配置
type AliyunPriceOptimizer interface {
	// FindCheapestInstance 查找指定规格的最便宜实例配置（跨区域）
	FindCheapestInstance(ctx context.Context, instanceType string, regions []string) (*OptimalInstanceConfig, error)

	// GetPriceForInstance 获取指定实例类型和区域的价格
	GetPriceForInstance(ctx context.Context, instanceType, region string) (*InstancePrice, error)

	// ComparePrices 比较多个区域和实例类型的价格
	ComparePrices(ctx context.Context, instanceTypes, regions []string) ([]InstancePrice, error)
}

// OptimalInstanceConfig 最优实例配置
type OptimalInstanceConfig struct {
	InstanceType  string  // 实例类型
	Region        string  // 区域
	Price         float64 // 每小时价格（元）
	PricePerMonth float64 // 每月价格（元）
	Currency      string  // 货币单位
}

// InstancePrice 实例价格信息
type InstancePrice struct {
	InstanceType  string  // 实例类型
	Region        string  // 区域
	PricePerHour  float64 // 每小时价格
	PricePerMonth float64 // 每月价格
	Currency      string  // 货币单位
}

// aliyunPriceOptimizer 阿里云价格优化器实现
type aliyunPriceOptimizer struct {
	config     *config.Config
	httpClient *http.Client
	accessKey  string
	secretKey  string
}

// NewAliyunPriceOptimizer 创建阿里云价格优化器
func NewAliyunPriceOptimizer(cfg *config.Config, accessKey, secretKey string) AliyunPriceOptimizer {
	return &aliyunPriceOptimizer{
		config:     cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		accessKey:  accessKey,
		secretKey:  secretKey,
	}
}

// FindCheapestInstance 查找最便宜的实例配置
func (o *aliyunPriceOptimizer) FindCheapestInstance(ctx context.Context, instanceType string, regions []string) (*OptimalInstanceConfig, error) {
	if len(regions) == 0 {
		// 默认查询常用区域
		regions = []string{
			"cn-beijing", "cn-shanghai", "cn-hangzhou", "cn-shenzhen",
			"cn-hongkong", "ap-southeast-1", "us-east-1",
		}
	}

	var prices []InstancePrice

	// 并发查询所有区域的价格
	type priceResult struct {
		price *InstancePrice
		err   error
	}

	results := make(chan priceResult, len(regions))

	for _, region := range regions {
		go func(r string) {
			price, err := o.GetPriceForInstance(ctx, instanceType, r)
			if err != nil {
				results <- priceResult{nil, err}
				return
			}
			results <- priceResult{price, nil}
		}(region)
	}

	// 收集结果
	for i := 0; i < len(regions); i++ {
		result := <-results
		if result.err == nil && result.price != nil {
			prices = append(prices, *result.price)
		}
	}

	if len(prices) == 0 {
		return nil, fmt.Errorf("无法获取任何区域的价格信息")
	}

	// 按价格排序，选择最便宜的
	sort.Slice(prices, func(i, j int) bool {
		return prices[i].PricePerHour < prices[j].PricePerHour
	})

	cheapest := prices[0]

	return &OptimalInstanceConfig{
		InstanceType:  cheapest.InstanceType,
		Region:        cheapest.Region,
		Price:         cheapest.PricePerHour,
		PricePerMonth: cheapest.PricePerMonth,
		Currency:      cheapest.Currency,
	}, nil
}

// GetPriceForInstance 获取指定实例类型和区域的价格
func (o *aliyunPriceOptimizer) GetPriceForInstance(ctx context.Context, instanceType, region string) (*InstancePrice, error) {
	// 调用 DescribePrice API
	// API 文档: https://next.api.aliyun.com/document/Ecs/2014-05-26/DescribePrice

	// 构建请求参数
	params := map[string]string{
		"Action":       "DescribePrice",
		"Version":      "2014-05-26",
		"RegionId":     region,
		"InstanceType": instanceType,
		"PriceUnit":    "Hour", // 按小时计费
	}

	// 调用 API
	response, err := o.callAliyunAPI(ctx, "https://ecs.aliyuncs.com", params)
	if err != nil {
		return nil, fmt.Errorf("调用 DescribePrice API 失败: %w", err)
	}

	// 解析响应
	var apiResponse struct {
		PriceInfo struct {
			Price struct {
				OriginalPrice float64 `json:"OriginalPrice"` // 原价
				TradePrice    float64 `json:"TradePrice"`    // 成交价
				DiscountPrice float64 `json:"DiscountPrice"` // 折扣价
			} `json:"Price"`
			Rules struct {
				Rule []struct {
					RuleId string `json:"RuleId"`
					Title  string `json:"Title"`
					Name   string `json:"Name"`
				} `json:"Rule"`
			} `json:"Rules"`
		} `json:"PriceInfo"`
	}

	if err := json.Unmarshal(response, &apiResponse); err != nil {
		return nil, fmt.Errorf("解析 API 响应失败: %w", err)
	}

	// 使用成交价（TradePrice），如果没有则使用原价
	pricePerHour := apiResponse.PriceInfo.Price.TradePrice
	if pricePerHour == 0 {
		pricePerHour = apiResponse.PriceInfo.Price.OriginalPrice
	}
	if pricePerHour == 0 {
		pricePerHour = apiResponse.PriceInfo.Price.DiscountPrice
	}

	if pricePerHour == 0 {
		return nil, fmt.Errorf("无法获取有效价格")
	}

	pricePerMonth := pricePerHour * 24 * 30

	return &InstancePrice{
		InstanceType:  instanceType,
		Region:        region,
		PricePerHour:  pricePerHour,
		PricePerMonth: pricePerMonth,
		Currency:      "CNY",
	}, nil
}

// ComparePrices 比较多个区域和实例类型的价格
func (o *aliyunPriceOptimizer) ComparePrices(ctx context.Context, instanceTypes, regions []string) ([]InstancePrice, error) {
	if len(instanceTypes) == 0 {
		instanceTypes = []string{"ecs.t5-lc1m1.small", "ecs.t5-lc1m2.small", "ecs.t6-c1m1.large"}
	}

	if len(regions) == 0 {
		regions = []string{
			"cn-beijing", "cn-shanghai", "cn-hangzhou", "cn-shenzhen",
			"cn-hongkong", "ap-southeast-1",
		}
	}

	var allPrices []InstancePrice

	// 查询所有组合的价格
	for _, instanceType := range instanceTypes {
		for _, region := range regions {
			price, err := o.GetPriceForInstance(ctx, instanceType, region)
			if err == nil && price != nil {
				allPrices = append(allPrices, *price)
			}
		}
	}

	// 按价格排序
	sort.Slice(allPrices, func(i, j int) bool {
		return allPrices[i].PricePerHour < allPrices[j].PricePerHour
	})

	return allPrices, nil
}

// callAliyunAPI 调用阿里云 API（带签名认证）
func (o *aliyunPriceOptimizer) callAliyunAPI(ctx context.Context, endpoint string, params map[string]string) ([]byte, error) {
	if o.accessKey == "" || o.secretKey == "" {
		return nil, fmt.Errorf("未配置阿里云 AccessKey 和 SecretKey")
	}

	// 添加公共参数
	params["Format"] = "JSON"
	params["AccessKeyId"] = o.accessKey
	params["SignatureMethod"] = "HMAC-SHA1"
	params["Timestamp"] = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	params["SignatureVersion"] = "1.0"
	params["SignatureNonce"] = fmt.Sprintf("%d", time.Now().UnixNano())

	// 构建查询字符串
	query := o.buildQueryString(params)

	// 生成签名
	signature := o.generateSignature("GET", query, o.secretKey)

	// 添加签名到查询字符串
	fullURL := fmt.Sprintf("%s?%s&Signature=%s", endpoint, query, url.QueryEscape(signature))

	// 发送请求
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API 返回错误: %d, %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 检查是否有错误
	var errorResponse struct {
		Code    string `json:"Code"`
		Message string `json:"Message"`
	}
	if err := json.Unmarshal(body, &errorResponse); err == nil {
		if errorResponse.Code != "" {
			return nil, fmt.Errorf("API 错误: %s - %s", errorResponse.Code, errorResponse.Message)
		}
	}

	return body, nil
}

// buildQueryString 构建查询字符串（按字典序排序）
func (o *aliyunPriceOptimizer) buildQueryString(params map[string]string) string {
	// 获取所有键并排序
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 构建查询字符串
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", url.QueryEscape(k), url.QueryEscape(params[k])))
	}

	return strings.Join(parts, "&")
}

// generateSignature 生成阿里云 API 签名
func (o *aliyunPriceOptimizer) generateSignature(method, queryString, secretKey string) string {
	// 构建待签名字符串
	stringToSign := fmt.Sprintf("%s&%s&%s",
		method,
		url.QueryEscape("/"),
		url.QueryEscape(queryString))

	// 使用 HMAC-SHA1 签名
	h := hmac.New(sha1.New, []byte(secretKey+"&"))
	h.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	return signature
}
