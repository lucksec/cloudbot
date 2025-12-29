package credentials

// String 返回 Provider 的字符串表示
func (p Provider) String() string {
	return string(p)
}

// DisplayName 返回 Provider 的显示名称
func (p Provider) DisplayName() string {
	names := map[Provider]string{
		ProviderAliyun:      "阿里云",
		ProviderTencent:     "腾讯云",
		ProviderHuaweicloud: "华为云",
		ProviderAWS:         "AWS",
		ProviderVultr:       "Vultr",
	}
	
	if name, ok := names[p]; ok {
		return name
	}
	return string(p)
}

// IsValid 检查 Provider 是否有效
func (p Provider) IsValid() bool {
	validProviders := []Provider{
		ProviderAliyun,
		ProviderTencent,
		ProviderHuaweicloud,
		ProviderAWS,
		ProviderVultr,
	}
	
	for _, valid := range validProviders {
		if p == valid {
			return true
		}
	}
	return false
}



