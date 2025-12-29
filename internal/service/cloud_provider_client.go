package service

import (
	"context"
	"fmt"
)

// CloudProviderClient 云服务商客户端接口
// 用于动态获取可用区域和实例类型
type CloudProviderClient interface {
	// GetAvailableRegions 获取可用区域列表
	GetAvailableRegions(ctx context.Context) ([]Region, error)
	
	// GetAvailableInstanceTypes 获取指定区域的可用实例类型
	GetAvailableInstanceTypes(ctx context.Context, region string) ([]InstanceType, error)
	
	// GetInstancePrice 获取实例价格信息
	GetInstancePrice(ctx context.Context, region, instanceType string) (*InstancePrice, error)
	
	// Provider 返回云服务商名称
	Provider() string
}

// Region 区域信息
type Region struct {
	ID          string // 区域ID，如 cn-beijing
	Name        string // 区域名称，如 北京
	DisplayName string // 显示名称
	Available   bool   // 是否可用
}

// InstanceType 实例类型信息
type InstanceType struct {
	ID          string  // 实例类型ID，如 ecs.t5-lc1m1.small
	Name        string  // 实例类型名称
	CPU         int     // CPU核心数
	Memory      float64 // 内存大小(GB)
	Available   bool    // 是否可用
	PricePerHour float64 // 每小时价格
	PricePerMonth float64 // 每月价格
	Currency    string  // 货币单位
}

// NewCloudProviderClient 创建云服务商客户端
func NewCloudProviderClient(provider string, accessKey, secretKey string) (CloudProviderClient, error) {
	switch provider {
	case "aliyun":
		return NewAliyunClient(accessKey, secretKey)
	case "tencent":
		return NewTencentClient(accessKey, secretKey)
	case "aws":
		return NewAWSClient(accessKey, secretKey)
	case "huaweicloud":
		return NewHuaweicloudClient(accessKey, secretKey)
	default:
		return nil, fmt.Errorf("不支持的云服务商: %s", provider)
	}
}


