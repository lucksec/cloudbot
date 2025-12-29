package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/ini.v1"
)

// Config 应用配置
type Config struct {
	// 工作目录
	WorkDir string
	
	// 模板目录
	TemplateDir string
	
	// 项目目录
	ProjectDir string
	
	// Terraform 配置
	Terraform TerraformConfig
	
	// 日志配置
	Log LogConfig
	
	// 凭据配置文件路径
	CredentialConfigPath string
}

// TerraformConfig Terraform 相关配置
type TerraformConfig struct {
	// Terraform 可执行文件路径
	ExecPath string
	
	// 默认工作目录
	WorkDir string
}

// LogConfig 日志配置
type LogConfig struct {
	// 日志级别：DEBUG, INFO, WARN, ERROR
	Level string
	
	// 是否启用控制台输出
	EnableConsole bool
	
	// 是否启用文件输出
	EnableFile bool
	
	// 日志目录
	LogDir string
	
	// 日志文件名（如果为空，则使用默认格式）
	LogFile string
}

// LoadConfig 加载配置文件
func LoadConfig() (*Config, error) {
	// 确定配置文件路径
	var configPath string
	configPaths := []string{".redc.ini", "$HOME/.cloudbot/.redc.ini"}
	for _, path := range configPaths {
		if path[0] == '$' {
			if path == "$HOME/.cloudbot/.redc.ini" {
				homeDir := os.Getenv("HOME")
				if homeDir != "" {
					path = filepath.Join(homeDir, ".cloudbot", ".redc.ini")
				} else {
					continue
				}
			}
		}
		if _, err := os.Stat(path); err == nil {
			configPath = path
			break
		}
	}
	
	config := &Config{
		WorkDir:     ".",
		TemplateDir: "./redc-templates",
		ProjectDir:  "./projects",
		Terraform: TerraformConfig{
			ExecPath: "terraform",
			WorkDir:  ".",
		},
		Log: LogConfig{
			Level:         "INFO",
			EnableConsole: true,
			EnableFile:   true,
			LogDir:       "logs",
			LogFile:      "",
		},
		CredentialConfigPath: configPath,
	}
	
	// 尝试读取配置文件
	var cfgFile *ini.File
	
	for _, configPath := range configPaths {
		if configPath[0] == '$' {
			// 处理环境变量
			if configPath == "$HOME/.cloudbot/.redc.ini" {
				homeDir := os.Getenv("HOME")
				if homeDir != "" {
					configPath = filepath.Join(homeDir, ".cloudbot", ".redc.ini")
				} else {
					continue
				}
			}
		}
		
		if _, err := os.Stat(configPath); err == nil {
			cfgFile, err = ini.Load(configPath)
			if err == nil {
				break
			}
		}
	}
	
	// 如果成功加载配置文件，读取配置值
	if cfgFile != nil {
		if section := cfgFile.Section("default"); section != nil {
			if workDir := section.Key("work_dir").String(); workDir != "" {
				config.WorkDir = workDir
			}
			if templateDir := section.Key("template_dir").String(); templateDir != "" {
				config.TemplateDir = templateDir
			}
			if projectDir := section.Key("project_dir").String(); projectDir != "" {
				config.ProjectDir = projectDir
			}
		}
		
		if section := cfgFile.Section("terraform"); section != nil {
			if execPath := section.Key("exec_path").String(); execPath != "" {
				config.Terraform.ExecPath = execPath
			}
			if workDir := section.Key("work_dir").String(); workDir != "" {
				config.Terraform.WorkDir = workDir
			}
		}
		
		if section := cfgFile.Section("log"); section != nil {
			if level := section.Key("level").String(); level != "" {
				config.Log.Level = level
			}
			if enableConsole := section.Key("enable_console").String(); enableConsole != "" {
				config.Log.EnableConsole = enableConsole == "true" || enableConsole == "1"
			}
			if enableFile := section.Key("enable_file").String(); enableFile != "" {
				config.Log.EnableFile = enableFile == "true" || enableFile == "1"
			}
			if logDir := section.Key("log_dir").String(); logDir != "" {
				config.Log.LogDir = logDir
			}
			if logFile := section.Key("log_file").String(); logFile != "" {
				config.Log.LogFile = logFile
			}
		}
	}
	
	// 确保目录存在
	if err := ensureDirs(config); err != nil {
		return nil, fmt.Errorf("创建目录失败: %w", err)
	}
	
	return config, nil
}

// ensureDirs 确保必要的目录存在
func ensureDirs(config *Config) error {
	dirs := []string{
		config.ProjectDir,
		config.TemplateDir,
	}
	
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录 %s 失败: %w", dir, err)
		}
	}
	
	return nil
}

// LoadProjectConfig 加载项目配置文件
func LoadProjectConfig(projectPath string) (*ini.File, error) {
	configPath := filepath.Join(projectPath, "project.ini")
	
	// 如果文件不存在，创建默认配置
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfg := ini.Empty()
		if err := cfg.SaveTo(configPath); err != nil {
			return nil, fmt.Errorf("创建项目配置失败: %w", err)
		}
	}
	
	return ini.Load(configPath)
}

// SaveProjectConfig 保存项目配置文件
func SaveProjectConfig(projectPath string, cfg *ini.File) error {
	configPath := filepath.Join(projectPath, "project.ini")
	return cfg.SaveTo(configPath)
}

