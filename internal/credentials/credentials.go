package credentials

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/ini.v1"
)

// Provider 云服务商类型
type Provider string

const (
	ProviderAliyun      Provider = "aliyun"
	ProviderTencent     Provider = "tencent"
	ProviderHuaweicloud Provider = "huaweicloud"
	ProviderAWS         Provider = "aws"
	ProviderVultr       Provider = "vultr"
)

// Credentials 云服务商凭据
type Credentials struct {
	AccessKey string
	SecretKey string
	Region    string // 可选：默认区域
}

// CredentialManager 凭据管理器接口
type CredentialManager interface {
	// GetCredentials 获取指定云服务商的凭据
	GetCredentials(provider Provider) (*Credentials, error)
	
	// SetCredentials 设置指定云服务商的凭据
	SetCredentials(provider Provider, creds *Credentials) error
	
	// HasCredentials 检查是否已配置凭据
	HasCredentials(provider Provider) bool
	
	// ListProviders 列出所有已配置凭据的云服务商
	ListProviders() []Provider
	
	// RemoveCredentials 删除指定云服务商的凭据
	RemoveCredentials(provider Provider) error
}

// credentialManager 凭据管理器实现
type credentialManager struct {
	configPath string
	mu         sync.RWMutex
	creds      map[Provider]*Credentials
}

var defaultManager CredentialManager
var once sync.Once

// NewCredentialManager 创建凭据管理器实例
func NewCredentialManager(configPath string) (CredentialManager, error) {
	manager := &credentialManager{
		configPath: configPath,
		creds:      make(map[Provider]*Credentials),
	}
	
	// 加载配置
	if err := manager.load(); err != nil {
		return nil, fmt.Errorf("加载凭据配置失败: %w", err)
	}
	
	return manager, nil
}

// GetDefaultManager 获取默认凭据管理器
func GetDefaultManager() CredentialManager {
	once.Do(func() {
		// 确定默认配置文件路径（优先使用当前目录的 .redc.ini）
		var configPath string
		configPaths := []string{
			".redc.ini",
			filepath.Join(os.Getenv("HOME"), ".meta-matrix", ".redc.ini"),
		}
		
		// 查找已存在的配置文件
		for _, path := range configPaths {
			if _, err := os.Stat(path); err == nil {
				configPath = path
				break
			}
		}
		
		// 如果配置文件不存在，使用第一个路径作为默认路径（会在保存时创建）
		if configPath == "" {
			configPath = configPaths[0] // 使用 ".redc.ini"
		}
		
		// 创建管理器（即使文件不存在也会创建，保存时会自动创建文件）
		manager, err := NewCredentialManager(configPath)
		if err != nil {
			// 如果创建失败，创建一个基本的管理器（至少可以保存）
			defaultManager = &credentialManager{
				configPath: configPath,
				creds:      make(map[Provider]*Credentials),
			}
		} else {
			defaultManager = manager
		}
	})
	
	return defaultManager
}

// load 从配置文件加载凭据
func (m *credentialManager) load() error {
	if m.configPath == "" {
		// 如果没有配置文件，只从环境变量加载
		return m.loadFromEnv()
	}
	
	// 检查配置文件是否存在
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		// 文件不存在，只从环境变量加载
		return m.loadFromEnv()
	}
	
	// 加载 INI 文件
	cfg, err := ini.Load(m.configPath)
	if err != nil {
		// 如果加载失败，尝试从环境变量加载
		return m.loadFromEnv()
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 加载各云服务商的凭据
	providers := []Provider{
		ProviderAliyun,
		ProviderTencent,
		ProviderHuaweicloud,
		ProviderAWS,
		ProviderVultr,
	}
	
	for _, provider := range providers {
		sectionName := string(provider)
		section := cfg.Section(sectionName)
		
		// 腾讯云使用 secret_id，其他云服务商使用 access_key（保持向后兼容）
		var accessKey string
		if provider == ProviderTencent {
			// 优先读取 secret_id，如果没有则读取 access_key（向后兼容）
			accessKey = section.Key("secret_id").String()
			if accessKey == "" {
				accessKey = section.Key("access_key").String()
			}
		} else {
			accessKey = section.Key("access_key").String()
		}
		
		secretKey := section.Key("secret_key").String()
		region := section.Key("region").String()
		
		// 如果配置文件中没有，尝试从环境变量获取
		if accessKey == "" {
			accessKey = m.getEnvAccessKey(provider)
		}
		if secretKey == "" {
			secretKey = m.getEnvSecretKey(provider)
		}
		if region == "" {
			region = m.getEnvRegion(provider)
		}
		
		if accessKey != "" && secretKey != "" {
			m.creds[provider] = &Credentials{
				AccessKey: accessKey,
				SecretKey: secretKey,
				Region:    region,
			}
		}
	}
	
	return nil
}

// loadFromEnv 从环境变量加载凭据
func (m *credentialManager) loadFromEnv() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	providers := []struct {
		provider Provider
		envAK    string
		envSK    string
		envRegion string
	}{
		{ProviderAliyun, "ALICLOUD_ACCESS_KEY", "ALICLOUD_SECRET_KEY", "ALICLOUD_REGION"},
		{ProviderTencent, "TENCENTCLOUD_SECRET_ID", "TENCENTCLOUD_SECRET_KEY", "TENCENTCLOUD_REGION"},
		{ProviderHuaweicloud, "HUAWEICLOUD_ACCESS_KEY", "HUAWEICLOUD_SECRET_KEY", "HUAWEICLOUD_REGION"},
		{ProviderAWS, "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_REGION"},
		{ProviderVultr, "VULTR_API_KEY", "VULTR_API_KEY", "VULTR_REGION"},
	}
	
	for _, p := range providers {
		accessKey := os.Getenv(p.envAK)
		secretKey := os.Getenv(p.envSK)
		region := os.Getenv(p.envRegion)
		
		if accessKey != "" && secretKey != "" {
			m.creds[p.provider] = &Credentials{
				AccessKey: accessKey,
				SecretKey: secretKey,
				Region:    region,
			}
		}
	}
	
	return nil
}

// getEnvAccessKey 从环境变量获取 AccessKey
func (m *credentialManager) getEnvAccessKey(provider Provider) string {
	envMap := map[Provider]string{
		ProviderAliyun:      "ALICLOUD_ACCESS_KEY",
		ProviderTencent:     "TENCENTCLOUD_SECRET_ID",
		ProviderHuaweicloud: "HUAWEICLOUD_ACCESS_KEY",
		ProviderAWS:         "AWS_ACCESS_KEY_ID",
		ProviderVultr:       "VULTR_API_KEY",
	}
	
	if envKey, ok := envMap[provider]; ok {
		return os.Getenv(envKey)
	}
	return ""
}

// getEnvSecretKey 从环境变量获取 SecretKey
func (m *credentialManager) getEnvSecretKey(provider Provider) string {
	envMap := map[Provider]string{
		ProviderAliyun:      "ALICLOUD_SECRET_KEY",
		ProviderTencent:     "TENCENTCLOUD_SECRET_KEY",
		ProviderHuaweicloud: "HUAWEICLOUD_SECRET_KEY",
		ProviderAWS:         "AWS_SECRET_ACCESS_KEY",
		ProviderVultr:       "VULTR_API_KEY",
	}
	
	if envKey, ok := envMap[provider]; ok {
		return os.Getenv(envKey)
	}
	return ""
}

// getEnvRegion 从环境变量获取 Region
func (m *credentialManager) getEnvRegion(provider Provider) string {
	envMap := map[Provider]string{
		ProviderAliyun:      "ALICLOUD_REGION",
		ProviderTencent:     "TENCENTCLOUD_REGION",
		ProviderHuaweicloud: "HUAWEICLOUD_REGION",
		ProviderAWS:         "AWS_REGION",
		ProviderVultr:       "VULTR_REGION",
	}
	
	if envKey, ok := envMap[provider]; ok {
		return os.Getenv(envKey)
	}
	return ""
}

// GetCredentials 获取指定云服务商的凭据
func (m *credentialManager) GetCredentials(provider Provider) (*Credentials, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	creds, ok := m.creds[provider]
	if !ok {
		// 尝试从环境变量获取
		accessKey := m.getEnvAccessKey(provider)
		secretKey := m.getEnvSecretKey(provider)
		region := m.getEnvRegion(provider)
		
		if accessKey != "" && secretKey != "" {
			return &Credentials{
				AccessKey: accessKey,
				SecretKey: secretKey,
				Region:    region,
			}, nil
		}
		
		return nil, fmt.Errorf("未找到 %s 的凭据配置", provider)
	}
	
	return creds, nil
}

// SetCredentials 设置指定云服务商的凭据
func (m *credentialManager) SetCredentials(provider Provider, creds *Credentials) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 更新内存中的凭据
	m.creds[provider] = creds
	
	// 如果没有配置文件路径，使用默认路径
	if m.configPath == "" {
		m.configPath = ".redc.ini"
	}
	
	// 保存到配置文件
	return m.save()
}

// HasCredentials 检查是否已配置凭据
func (m *credentialManager) HasCredentials(provider Provider) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if _, ok := m.creds[provider]; ok {
		return true
	}
	
	// 检查环境变量
	accessKey := m.getEnvAccessKey(provider)
	secretKey := m.getEnvSecretKey(provider)
	return accessKey != "" && secretKey != ""
}

// ListProviders 列出所有已配置凭据的云服务商
func (m *credentialManager) ListProviders() []Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var providers []Provider
	for provider := range m.creds {
		providers = append(providers, provider)
	}
	
	// 也检查环境变量中的凭据
	allProviders := []Provider{
		ProviderAliyun,
		ProviderTencent,
		ProviderHuaweicloud,
		ProviderAWS,
		ProviderVultr,
	}
	
	for _, provider := range allProviders {
		if !m.hasProvider(providers, provider) {
			if m.HasCredentials(provider) {
				providers = append(providers, provider)
			}
		}
	}
	
	return providers
}

// hasProvider 检查列表中是否包含指定 provider
func (m *credentialManager) hasProvider(list []Provider, provider Provider) bool {
	for _, p := range list {
		if p == provider {
			return true
		}
	}
	return false
}

// RemoveCredentials 删除指定云服务商的凭据
func (m *credentialManager) RemoveCredentials(provider Provider) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	delete(m.creds, provider)
	
	// 如果没有配置文件路径，使用默认路径
	if m.configPath == "" {
		m.configPath = ".redc.ini"
	}
	
	// 保存到配置文件
	return m.save()
}

// save 保存凭据到配置文件
func (m *credentialManager) save() error {
	// 如果没有配置文件路径，使用默认路径
	if m.configPath == "" {
		m.configPath = ".redc.ini"
	}
	
	// 确保目录存在
	dir := filepath.Dir(m.configPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建配置目录失败: %w", err)
		}
	}
	
	// 加载现有配置（如果存在）
	var cfg *ini.File
	if _, err := os.Stat(m.configPath); err == nil {
		// 文件存在，加载它
		loadedCfg, err := ini.Load(m.configPath)
		if err != nil {
			// 如果加载失败，创建新配置
			cfg = ini.Empty()
		} else {
			cfg = loadedCfg
		}
	} else {
		// 文件不存在，创建新配置
		cfg = ini.Empty()
	}
	
	// 更新各云服务商的凭据
	for provider, creds := range m.creds {
		sectionName := string(provider)
		section := cfg.Section(sectionName)
		
		// 腾讯云使用 secret_id，其他云服务商使用 access_key
		if provider == ProviderTencent {
			section.Key("secret_id").SetValue(creds.AccessKey)
		} else {
			section.Key("access_key").SetValue(creds.AccessKey)
		}
		section.Key("secret_key").SetValue(creds.SecretKey)
		if creds.Region != "" {
			section.Key("region").SetValue(creds.Region)
		}
	}
	
	// 保存到文件
	return cfg.SaveTo(m.configPath)
}

