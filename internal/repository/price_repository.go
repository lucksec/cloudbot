package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/lucksec/cloudbot/internal/config"
	"github.com/lucksec/cloudbot/internal/domain"
)

// PriceRepository 价格仓库接口
type PriceRepository interface {
	// GetPrice 获取指定模板的价格信息
	GetPrice(provider, template string) (*domain.PriceInfo, error)

	// ListPrices 列出所有价格信息
	ListPrices() ([]*domain.PriceInfo, error)

	// GetPricesByType 根据模板类型获取价格列表（如 ecs, proxy）
	GetPricesByType(templateType string) ([]*domain.PriceInfo, error)

	// ComparePrices 比对指定模板类型的价格
	ComparePrices(templateType string) (*domain.PriceComparison, error)
}

// PriceFetcher 价格查询器接口（避免循环依赖）
type PriceFetcher interface {
	FetchPrice(ctx context.Context, provider, template, region string) (*domain.PriceInfo, error)
	FetchPricesByType(ctx context.Context, templateType string) ([]*domain.PriceInfo, error)
}

// priceRepository 价格仓库实现
type priceRepository struct {
	config       *config.Config
	prices       []*domain.PriceInfo
	priceFetcher PriceFetcher            // 动态价格查询器
	cache        map[string]*cachedPrice // 价格缓存
	cacheMutex   sync.RWMutex
	cacheExpiry  time.Duration // 缓存过期时间（默认1小时）
}

// cachedPrice 缓存的价格信息
type cachedPrice struct {
	price   *domain.PriceInfo
	expires time.Time
}

// NewPriceRepository 创建价格仓库实例
func NewPriceRepository(cfg *config.Config) PriceRepository {
	repo := &priceRepository{
		config:      cfg,
		cache:       make(map[string]*cachedPrice),
		cacheExpiry: 1 * time.Hour, // 默认缓存1小时
	}
	// 加载静态价格数据（作为后备）
	repo.loadPrices()
	return repo
}

// SetPriceFetcher 设置价格查询器（用于动态获取价格）
func (r *priceRepository) SetPriceFetcher(fetcher PriceFetcher) {
	r.priceFetcher = fetcher
}

// loadPrices 加载价格数据
func (r *priceRepository) loadPrices() {
	// 尝试从配置文件加载价格
	priceFile := filepath.Join(r.config.WorkDir, "prices.json")
	if _, err := os.Stat(priceFile); os.IsNotExist(err) {
		// 如果文件不存在，使用默认价格数据
		r.prices = r.getDefaultPrices()
		return
	}

	// 读取价格文件
	data, err := os.ReadFile(priceFile)
	if err != nil {
		// 读取失败，使用默认价格
		r.prices = r.getDefaultPrices()
		return
	}

	// 解析 JSON
	var prices []*domain.PriceInfo
	if err := json.Unmarshal(data, &prices); err != nil {
		// 解析失败，使用默认价格
		r.prices = r.getDefaultPrices()
		return
	}

	r.prices = prices
}

// getDefaultPrices 获取默认价格数据
// 这些是示例价格，实际使用时应该从配置文件或 API 获取
func (r *priceRepository) getDefaultPrices() []*domain.PriceInfo {
	return []*domain.PriceInfo{
		// 阿里云 ECS 1核2G
		{
			Provider:      "aliyun",
			Template:      "ecs",
			Region:        "cn-beijing",
			PricePerHour:  0.08,
			PricePerMonth: 58.0,
			Currency:      "CNY",
			Spec:          "1核2G",
			UpdatedAt:     "2024-01-01",
		},
		// 阿里云 ECS 2核4G
		{
			Provider:      "aliyun",
			Template:      "ecs1c2g",
			Region:        "cn-beijing",
			PricePerHour:  0.16,
			PricePerMonth: 116.0,
			Currency:      "CNY",
			Spec:          "2核4G",
			UpdatedAt:     "2024-01-01",
		},
		// 阿里云代理节点
		{
			Provider:      "aliyun",
			Template:      "aliyun-proxy",
			Region:        "cn-beijing",
			PricePerHour:  0.05,
			PricePerMonth: 36.0,
			Currency:      "CNY",
			Spec:          "1核1G",
			UpdatedAt:     "2024-01-01",
		},
		// 腾讯云 ECS 1核2G
		{
			Provider:      "tencent",
			Template:      "ecs",
			Region:        "ap-beijing",
			PricePerHour:  0.07,
			PricePerMonth: 51.0,
			Currency:      "CNY",
			Spec:          "1核2G",
			UpdatedAt:     "2024-01-01",
		},
		// AWS EC2 t3.micro
		{
			Provider:      "aws",
			Template:      "ec2",
			Region:        "us-east-1",
			PricePerHour:  0.0104,
			PricePerMonth: 7.5,
			Currency:      "USD",
			Spec:          "1核1G",
			UpdatedAt:     "2024-01-01",
		},
		// AWS EC2 t3.small
		{
			Provider:      "aws",
			Template:      "ec2-1G",
			Region:        "us-east-1",
			PricePerHour:  0.0208,
			PricePerMonth: 15.0,
			Currency:      "USD",
			Spec:          "1核2G",
			UpdatedAt:     "2024-01-01",
		},
		// Vultr 1核1G
		{
			Provider:      "vultr",
			Template:      "hk-vps",
			Region:        "hk",
			PricePerHour:  0.006,
			PricePerMonth: 4.5,
			Currency:      "USD",
			Spec:          "1核1G",
			UpdatedAt:     "2024-01-01",
		},
	}
}

// GetPrice 获取指定模板的价格信息
// 优先从缓存获取，如果缓存过期或不存在，则动态查询
func (r *priceRepository) GetPrice(provider, template string) (*domain.PriceInfo, error) {
	// 检查缓存
	cacheKey := fmt.Sprintf("%s/%s", provider, template)
	r.cacheMutex.RLock()
	cached, ok := r.cache[cacheKey]
	r.cacheMutex.RUnlock()

	if ok && time.Now().Before(cached.expires) {
		// 缓存有效，直接返回
		return cached.price, nil
	}

	// 缓存过期或不存在，尝试动态获取
	if r.priceFetcher != nil {
		ctx := context.Background()
		price, err := r.priceFetcher.FetchPrice(ctx, provider, template, "")
		if err == nil {
			// 更新缓存
			r.cacheMutex.Lock()
			r.cache[cacheKey] = &cachedPrice{
				price:   price,
				expires: time.Now().Add(r.cacheExpiry),
			}
			r.cacheMutex.Unlock()
			return price, nil
		}
		// 如果动态获取失败，继续使用静态价格
	}

	// 从静态价格数据中查找
	for _, price := range r.prices {
		if price.Provider == provider && price.Template == template {
			return price, nil
		}
	}

	return nil, fmt.Errorf("未找到模板 %s/%s 的价格信息", provider, template)
}

// ListPrices 列出所有价格信息
func (r *priceRepository) ListPrices() ([]*domain.PriceInfo, error) {
	return r.prices, nil
}

// GetPricesByType 根据模板类型获取价格列表
// templateType 可以是 "ecs", "proxy", "ec2" 等
// 如果配置了价格查询器，会尝试动态获取实时价格
func (r *priceRepository) GetPricesByType(templateType string) ([]*domain.PriceInfo, error) {
	// 如果配置了价格查询器，尝试动态获取
	if r.priceFetcher != nil {
		ctx := context.Background()
		dynamicPrices, err := r.priceFetcher.FetchPricesByType(ctx, templateType)
		if err == nil && len(dynamicPrices) > 0 {
			// 更新缓存
			for _, price := range dynamicPrices {
				cacheKey := fmt.Sprintf("%s/%s", price.Provider, price.Template)
				r.cacheMutex.Lock()
				r.cache[cacheKey] = &cachedPrice{
					price:   price,
					expires: time.Now().Add(r.cacheExpiry),
				}
				r.cacheMutex.Unlock()
			}
			return dynamicPrices, nil
		}
		// 如果动态获取失败，继续使用静态价格
	}

	// 从静态价格数据中查找
	var result []*domain.PriceInfo
	for _, price := range r.prices {
		if matchesType(price.Template, templateType) {
			result = append(result, price)
		}
	}

	return result, nil
}

// matchesType 判断模板是否匹配指定类型
func matchesType(template, templateType string) bool {
	// 简单的匹配逻辑
	typeMap := map[string][]string{
		"ecs":   {"ecs", "ecs1c2g"},
		"proxy": {"aliyun-proxy", "aws-proxy"},
		"ec2":   {"ec2", "ec2-1G", "ec2-4G"},
		"vps":   {"hk-vps"},
	}

	if types, ok := typeMap[templateType]; ok {
		for _, t := range types {
			if template == t {
				return true
			}
		}
	}

	// 如果模板名称包含类型关键词，也认为匹配
	return contains(template, templateType)
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ComparePrices 比对指定模板类型的价格
func (r *priceRepository) ComparePrices(templateType string) (*domain.PriceComparison, error) {
	prices, err := r.GetPricesByType(templateType)
	if err != nil {
		return nil, err
	}

	if len(prices) == 0 {
		return nil, fmt.Errorf("未找到类型为 %s 的模板价格信息", templateType)
	}

	// 转换为值切片以便排序
	priceInfos := make([]domain.PriceInfo, len(prices))
	for i, p := range prices {
		priceInfos[i] = *p
	}

	// 按每月价格排序（统一转换为 CNY 进行比较）
	sort.Slice(priceInfos, func(i, j int) bool {
		priceI := convertToCNY(priceInfos[i].PricePerMonth, priceInfos[i].Currency)
		priceJ := convertToCNY(priceInfos[j].PricePerMonth, priceInfos[j].Currency)
		return priceI < priceJ
	})

	// 计算价格范围
	var minHour, maxHour, minMonth, maxMonth float64
	for i, price := range priceInfos {
		hourPrice := convertToCNY(price.PricePerHour, price.Currency)
		monthPrice := convertToCNY(price.PricePerMonth, price.Currency)

		if i == 0 {
			minHour = hourPrice
			maxHour = hourPrice
			minMonth = monthPrice
			maxMonth = monthPrice
		} else {
			if hourPrice < minHour {
				minHour = hourPrice
			}
			if hourPrice > maxHour {
				maxHour = hourPrice
			}
			if monthPrice < minMonth {
				minMonth = monthPrice
			}
			if monthPrice > maxMonth {
				maxMonth = monthPrice
			}
		}
	}

	return &domain.PriceComparison{
		TemplateType: templateType,
		Options:      priceInfos,
		BestOption:   &priceInfos[0], // 价格最低的选项
		PriceRange: domain.PriceRange{
			MinPerHour:  minHour,
			MaxPerHour:  maxHour,
			MinPerMonth: minMonth,
			MaxPerMonth: maxMonth,
		},
	}, nil
}

// convertToCNY 将价格转换为人民币（简化版本，实际应该使用实时汇率）
func convertToCNY(amount float64, currency string) float64 {
	if currency == "CNY" {
		return amount
	}
	// 简化的汇率转换（USD to CNY，实际应该使用实时汇率）
	if currency == "USD" {
		return amount * 7.2 // 假设 1 USD = 7.2 CNY
	}
	return amount
}
