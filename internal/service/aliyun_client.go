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
)

// aliyunClient 阿里云客户端实现
type aliyunClient struct {
	accessKey  string
	secretKey  string
	httpClient *http.Client
}

// NewAliyunClient 创建阿里云客户端
func NewAliyunClient(accessKey, secretKey string) (CloudProviderClient, error) {
	return &aliyunClient{
		accessKey:  accessKey,
		secretKey:  secretKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Provider 返回云服务商名称
func (c *aliyunClient) Provider() string {
	return "aliyun"
}

// GetAvailableRegions 获取可用区域列表
func (c *aliyunClient) GetAvailableRegions(ctx context.Context) ([]Region, error) {
	// 使用 DescribeRegions API
	params := map[string]string{
		"Action":  "DescribeRegions",
		"Version": "2014-05-26",
	}
	
	response, err := c.callAPI(ctx, "https://ecs.aliyuncs.com", params)
	if err != nil {
		return nil, fmt.Errorf("调用 DescribeRegions API 失败: %w", err)
	}
	
	var apiResponse struct {
		Regions struct {
			Region []struct {
				RegionId  string `json:"RegionId"`
				LocalName string `json:"LocalName"`
			} `json:"Region"`
		} `json:"Regions"`
	}
	
	if err := json.Unmarshal(response, &apiResponse); err != nil {
		return nil, fmt.Errorf("解析 API 响应失败: %w", err)
	}
	
	var regions []Region
	for _, r := range apiResponse.Regions.Region {
		regions = append(regions, Region{
			ID:          r.RegionId,
			Name:        r.LocalName,
			DisplayName: fmt.Sprintf("%s (%s)", r.LocalName, r.RegionId),
			Available:   true,
		})
	}
	
	return regions, nil
}

// GetAvailableInstanceTypes 获取指定区域的可用实例类型
func (c *aliyunClient) GetAvailableInstanceTypes(ctx context.Context, region string) ([]InstanceType, error) {
	// 使用 DescribeInstanceTypes API
	params := map[string]string{
		"Action":  "DescribeInstanceTypes",
		"Version": "2014-05-26",
		"RegionId": region,
	}
	
	response, err := c.callAPI(ctx, "https://ecs.aliyuncs.com", params)
	if err != nil {
		return nil, fmt.Errorf("调用 DescribeInstanceTypes API 失败: %w", err)
	}
	
	var apiResponse struct {
		InstanceTypes struct {
			InstanceType []struct {
				InstanceTypeId   string  `json:"InstanceTypeId"`
				CpuCoreCount      int     `json:"CpuCoreCount"`
				MemorySize        float64 `json:"MemorySize"`
			} `json:"InstanceType"`
		} `json:"InstanceTypes"`
	}
	
	if err := json.Unmarshal(response, &apiResponse); err != nil {
		return nil, fmt.Errorf("解析 API 响应失败: %w", err)
	}
	
	var instanceTypes []InstanceType
	for _, it := range apiResponse.InstanceTypes.InstanceType {
		// 尝试获取价格信息
		price, _ := c.GetInstancePrice(ctx, region, it.InstanceTypeId)
		
		instanceType := InstanceType{
			ID:          it.InstanceTypeId,
			Name:        it.InstanceTypeId,
			CPU:         it.CpuCoreCount,
			Memory:      it.MemorySize,
			Available:   true,
			Currency:    "CNY",
		}
		
		if price != nil {
			instanceType.PricePerHour = price.PricePerHour
			instanceType.PricePerMonth = price.PricePerMonth
		}
		
		instanceTypes = append(instanceTypes, instanceType)
	}
	
	return instanceTypes, nil
}

// GetInstancePrice 获取实例价格信息
func (c *aliyunClient) GetInstancePrice(ctx context.Context, region, instanceType string) (*InstancePrice, error) {
	params := map[string]string{
		"Action":       "DescribePrice",
		"Version":      "2014-05-26",
		"RegionId":     region,
		"InstanceType": instanceType,
		"PriceUnit":    "Hour",
	}
	
	response, err := c.callAPI(ctx, "https://ecs.aliyuncs.com", params)
	if err != nil {
		return nil, fmt.Errorf("调用 DescribePrice API 失败: %w", err)
	}
	
	var apiResponse struct {
		PriceInfo struct {
			Price struct {
				OriginalPrice float64 `json:"OriginalPrice"`
				TradePrice    float64 `json:"TradePrice"`
				DiscountPrice float64 `json:"DiscountPrice"`
			} `json:"Price"`
		} `json:"PriceInfo"`
	}
	
	if err := json.Unmarshal(response, &apiResponse); err != nil {
		return nil, fmt.Errorf("解析 API 响应失败: %w", err)
	}
	
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

// callAPI 调用阿里云 API
func (c *aliyunClient) callAPI(ctx context.Context, endpoint string, params map[string]string) ([]byte, error) {
	if c.accessKey == "" || c.secretKey == "" {
		return nil, fmt.Errorf("未配置阿里云 AccessKey 和 SecretKey")
	}
	
	// 添加公共参数
	params["Format"] = "JSON"
	params["AccessKeyId"] = c.accessKey
	params["SignatureMethod"] = "HMAC-SHA1"
	params["Timestamp"] = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	params["SignatureVersion"] = "1.0"
	params["SignatureNonce"] = fmt.Sprintf("%d", time.Now().UnixNano())
	
	// 构建查询字符串
	query := c.buildQueryString(params)
	
	// 生成签名
	signature := c.generateSignature("GET", query, c.secretKey)
	
	// 添加签名到查询字符串
	fullURL := fmt.Sprintf("%s?%s&Signature=%s", endpoint, query, url.QueryEscape(signature))
	
	// 发送请求
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	
	resp, err := c.httpClient.Do(req)
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
func (c *aliyunClient) buildQueryString(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", url.QueryEscape(k), url.QueryEscape(params[k])))
	}
	
	return strings.Join(parts, "&")
}

// generateSignature 生成阿里云 API 签名
func (c *aliyunClient) generateSignature(method, queryString, secretKey string) string {
	stringToSign := fmt.Sprintf("%s&%s&%s",
		method,
		url.QueryEscape("/"),
		url.QueryEscape(queryString))
	
	h := hmac.New(sha1.New, []byte(secretKey+"&"))
	h.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))
	
	return signature
}


