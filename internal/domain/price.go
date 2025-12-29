package domain

// PriceInfo 表示一个模板的价格信息
type PriceInfo struct {
	Provider    string  `json:"provider"`     // 云服务商：aliyun, tencent, aws, vultr
	Template    string  `json:"template"`      // 模板名称
	Region      string  `json:"region"`       // 区域（可选，如 cn-beijing, us-east-1）
	PricePerHour float64 `json:"price_per_hour"` // 每小时价格（单位：元/小时）
	PricePerMonth float64 `json:"price_per_month"` // 每月价格（单位：元/月）
	Currency    string  `json:"currency"`      // 货币单位（CNY, USD）
	Spec        string  `json:"spec"`         // 规格描述（如 1核2G, 2核4G）
	UpdatedAt   string  `json:"updated_at"`    // 价格更新时间
}

// PriceComparison 表示价格比对结果
type PriceComparison struct {
	TemplateType string      `json:"template_type"` // 模板类型（如 ecs, proxy）
	Options      []PriceInfo `json:"options"`       // 可选方案列表（按价格排序）
	BestOption   *PriceInfo  `json:"best_option"`   // 最优方案（价格最低）
	PriceRange   PriceRange  `json:"price_range"`   // 价格范围
}

// PriceRange 价格范围
type PriceRange struct {
	MinPerHour  float64 `json:"min_per_hour"`  // 最低每小时价格
	MaxPerHour  float64 `json:"max_per_hour"`   // 最高每小时价格
	MinPerMonth float64 `json:"min_per_month"` // 最低每月价格
	MaxPerMonth float64 `json:"max_per_month"` // 最高每月价格
}

