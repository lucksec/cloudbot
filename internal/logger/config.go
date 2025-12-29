package logger

// Config 日志配置
type Config struct {
	// Level 日志级别：DEBUG, INFO, WARN, ERROR
	Level LogLevel
	
	// EnableConsole 是否启用控制台输出
	EnableConsole bool
	
	// EnableFile 是否启用文件输出
	EnableFile bool
	
	// LogDir 日志目录
	LogDir string
	
	// LogFile 日志文件名（如果为空，则使用默认格式：cloudbot-YYYY-MM-DD.log）
	LogFile string
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Level:         INFO,
		EnableConsole: true,
		EnableFile:   true,
		LogDir:       "logs",
		LogFile:      "", // 使用默认格式
	}
}

