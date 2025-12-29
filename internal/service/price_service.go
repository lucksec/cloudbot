package service

import (
	"context"
	"fmt"

	"github.com/lucksec/cloudbot/internal/domain"
	"github.com/lucksec/cloudbot/internal/repository"
)

// PriceService 价格服务接口
type PriceService interface {
	// GetPrice 获取指定模板的价格信息
	GetPrice(ctx context.Context, provider, template string) (*domain.PriceInfo, error)

	// ListPrices 列出所有价格信息
	ListPrices(ctx context.Context) ([]*domain.PriceInfo, error)

	// ComparePrices 比对指定模板类型的价格，返回最优方案
	ComparePrices(ctx context.Context, templateType string) (*domain.PriceComparison, error)

	// GetBestOption 获取指定类型的最优价格方案
	GetBestOption(ctx context.Context, templateType string) (*domain.PriceInfo, error)

	// GetPriceRecommendation 获取价格推荐（根据模板类型推荐最优方案）
	GetPriceRecommendation(ctx context.Context, templateType string) (string, error)
}

// priceService 价格服务实现
type priceService struct {
	priceRepo repository.PriceRepository
}

// NewPriceService 创建价格服务实例
func NewPriceService(priceRepo repository.PriceRepository) PriceService {
	return &priceService{
		priceRepo: priceRepo,
	}
}

// GetPrice 获取指定模板的价格信息
func (s *priceService) GetPrice(ctx context.Context, provider, template string) (*domain.PriceInfo, error) {
	return s.priceRepo.GetPrice(provider, template)
}

// ListPrices 列出所有价格信息
func (s *priceService) ListPrices(ctx context.Context) ([]*domain.PriceInfo, error) {
	return s.priceRepo.ListPrices()
}

// ComparePrices 比对指定模板类型的价格
func (s *priceService) ComparePrices(ctx context.Context, templateType string) (*domain.PriceComparison, error) {
	return s.priceRepo.ComparePrices(templateType)
}

// GetBestOption 获取指定类型的最优价格方案
func (s *priceService) GetBestOption(ctx context.Context, templateType string) (*domain.PriceInfo, error) {
	comparison, err := s.priceRepo.ComparePrices(templateType)
	if err != nil {
		return nil, err
	}

	if comparison.BestOption == nil {
		return nil, fmt.Errorf("未找到类型为 %s 的最优方案", templateType)
	}

	return comparison.BestOption, nil
}

// GetPriceRecommendation 获取价格推荐
// 返回一个可读的推荐信息字符串
func (s *priceService) GetPriceRecommendation(ctx context.Context, templateType string) (string, error) {
	comparison, err := s.priceRepo.ComparePrices(templateType)
	if err != nil {
		return "", err
	}

	if comparison.BestOption == nil {
		return "", fmt.Errorf("未找到类型为 %s 的推荐方案", templateType)
	}

	best := comparison.BestOption
	recommendation := fmt.Sprintf(
		"推荐方案: %s/%s (%s)\n"+
			"  价格: %.2f %s/月 (%.4f %s/小时)\n"+
			"  规格: %s\n"+
			"  区域: %s\n"+
			"  价格范围: %.2f - %.2f CNY/月",
		best.Provider,
		best.Template,
		best.Spec,
		best.PricePerMonth,
		best.Currency,
		best.PricePerHour,
		best.Currency,
		best.Spec,
		best.Region,
		comparison.PriceRange.MinPerMonth,
		comparison.PriceRange.MaxPerMonth,
	)

	return recommendation, nil
}
