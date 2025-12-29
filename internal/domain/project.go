package domain

import "time"

// Project 表示一个项目
type Project struct {
	Name        string    `json:"name"`         // 项目名称
	Path        string    `json:"path"`         // 项目路径
	CreatedAt   time.Time `json:"created_at"`   // 创建时间
	UpdatedAt   time.Time `json:"updated_at"`   // 更新时间
	Scenarios   []Scenario `json:"scenarios"`   // 场景列表
}

// Scenario 表示一个场景（部署实例）
type Scenario struct {
	ID          string    `json:"id"`           // UUID 标识
	Name        string    `json:"name"`         // 场景名称
	Template    string    `json:"template"`    // 模板路径（如 aliyun/ecs）
	Path        string    `json:"path"`         // 场景路径
	Status      string    `json:"status"`       // 状态：pending, deployed, destroyed
	CreatedAt   time.Time `json:"created_at"`   // 创建时间
	UpdatedAt   time.Time `json:"updated_at"`   // 更新时间
}

// Template 表示一个模板
type Template struct {
	Provider    string   `json:"provider"`      // 云服务商：aliyun, tencent, aws, vultr
	Name        string   `json:"name"`          // 模板名称
	Path        string   `json:"path"`          // 模板路径
	Description string   `json:"description"`   // 模板描述
	Files       []string `json:"files"`         // 模板文件列表
}

