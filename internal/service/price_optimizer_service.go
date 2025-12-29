package service

import (
	"context"
	"fmt"

	"github.com/meta-matrix/meta-matrix/internal/config"
)

// PriceOptimizerService 价格优化服务
// 用于查找最优价格配置并应用到场景创建
type PriceOptimizerService interface {
	// FindOptimalConfig 查找最优配置（最低价格）
	FindOptimalConfig(ctx context.Context, provider, template string, instanceTypes, regions []string) (*OptimalInstanceConfig, error)

	// ApplyOptimalConfig 将最优配置应用到 Terraform 变量
	ApplyOptimalConfig(optimal *OptimalInstanceConfig) map[string]string

	// ListRegionPrices 列出各区域价格（按价格排序，标注最低价）
	ListRegionPrices(ctx context.Context, provider, template string, instanceTypes, regions []string) ([]InstancePrice, error)
}

// priceOptimizerService 价格优化服务实现
type priceOptimizerService struct {
	config    *config.Config
	optimizer AliyunPriceOptimizer
}

// NewPriceOptimizerService 创建价格优化服务
func NewPriceOptimizerService(cfg *config.Config, optimizer AliyunPriceOptimizer) PriceOptimizerService {
	return &priceOptimizerService{
		config:    cfg,
		optimizer: optimizer,
	}
}

// FindOptimalConfig 查找最优配置
func (s *priceOptimizerService) FindOptimalConfig(ctx context.Context, provider, template string, instanceTypes, regions []string) (*OptimalInstanceConfig, error) {
	if provider != "aliyun" {
		return nil, fmt.Errorf("当前仅支持阿里云价格优化")
	}

	if s.optimizer == nil {
		return nil, fmt.Errorf("价格优化器未初始化")
	}

	// 如果未指定实例类型，使用默认值
	if len(instanceTypes) == 0 {
		instanceTypes = []string{"ecs.t5-lc1m1.small", "ecs.t5-lc1m2.small"}
	}

	// 比较所有实例类型和区域的价格
	allPrices, err := s.optimizer.ComparePrices(ctx, instanceTypes, regions)
	if err != nil {
		return nil, fmt.Errorf("查询价格失败: %w", err)
	}

	if len(allPrices) == 0 {
		return nil, fmt.Errorf("未找到任何价格信息")
	}

	// 选择最便宜的配置
	cheapest := allPrices[0]

	return &OptimalInstanceConfig{
		InstanceType:  cheapest.InstanceType,
		Region:        cheapest.Region,
		Price:         cheapest.PricePerHour,
		PricePerMonth: cheapest.PricePerMonth,
		Currency:      cheapest.Currency,
	}, nil
}

// ApplyOptimalConfig 将最优配置应用到 Terraform 变量
func (s *priceOptimizerService) ApplyOptimalConfig(optimal *OptimalInstanceConfig) map[string]string {
	vars := make(map[string]string)

	if optimal != nil {
		vars["region"] = optimal.Region
		vars["instance_type"] = optimal.InstanceType
	}

	return vars
}

// ListRegionPrices 列出各区域价格
func (s *priceOptimizerService) ListRegionPrices(ctx context.Context, provider, template string, instanceTypes, regions []string) ([]InstancePrice, error) {
	if provider != "aliyun" {
		return nil, fmt.Errorf("当前仅支持阿里云价格优化")
	}

	if s.optimizer == nil {
		return nil, fmt.Errorf("价格优化器未初始化")
	}

	if len(instanceTypes) == 0 {
		instanceTypes = []string{"ecs.t5-lc1m1.small", "ecs.t5-lc1m2.small"}
	}

	prices, err := s.optimizer.ComparePrices(ctx, instanceTypes, regions)
	if err != nil {
		return nil, fmt.Errorf("查询价格失败: %w", err)
	}

	if len(prices) == 0 {
		return nil, fmt.Errorf("未找到任何价格信息")
	}

	return prices, nil
}
