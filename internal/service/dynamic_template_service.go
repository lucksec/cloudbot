package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lucksec/cloudbot/internal/credentials"
	"github.com/lucksec/cloudbot/internal/logger"
)

// DynamicTemplateService 动态模板服务
// 用于动态获取可用区域和实例类型，并生成模板
type DynamicTemplateService interface {
	// GetAvailableRegions 获取可用区域列表
	GetAvailableRegions(ctx context.Context, provider string) ([]Region, error)

	// GetAvailableInstanceTypes 获取指定区域的可用实例类型
	GetAvailableInstanceTypes(ctx context.Context, provider, region string) ([]InstanceType, error)

	// GenerateAndSaveTemplate 生成并保存模板到指定目录
	GenerateAndSaveTemplate(ctx context.Context, scenario, provider, region, instanceType, destPath string, options map[string]interface{}) error
}

// dynamicTemplateService 动态模板服务实现
type dynamicTemplateService struct {
	credManager credentials.CredentialManager
	templateGen TemplateGenerator
}

// NewDynamicTemplateService 创建动态模板服务
func NewDynamicTemplateService(credManager credentials.CredentialManager) DynamicTemplateService {
	return &dynamicTemplateService{
		credManager: credManager,
		templateGen: NewTemplateGenerator(credManager),
	}
}

// GetAvailableRegions 获取可用区域列表
func (s *dynamicTemplateService) GetAvailableRegions(ctx context.Context, provider string) ([]Region, error) {
	providerEnum := credentials.Provider(provider)
	if !s.credManager.HasCredentials(providerEnum) {
		return nil, fmt.Errorf("未配置 %s 的凭据", provider)
	}

	creds, err := s.credManager.GetCredentials(providerEnum)
	if err != nil {
		return nil, fmt.Errorf("获取 %s 凭据失败: %w", provider, err)
	}

	client, err := NewCloudProviderClient(provider, creds.AccessKey, creds.SecretKey)
	if err != nil {
		return nil, fmt.Errorf("创建云服务商客户端失败: %w", err)
	}

	return client.GetAvailableRegions(ctx)
}

// GetAvailableInstanceTypes 获取指定区域的可用实例类型
func (s *dynamicTemplateService) GetAvailableInstanceTypes(ctx context.Context, provider, region string) ([]InstanceType, error) {
	providerEnum := credentials.Provider(provider)
	if !s.credManager.HasCredentials(providerEnum) {
		return nil, fmt.Errorf("未配置 %s 的凭据", provider)
	}

	creds, err := s.credManager.GetCredentials(providerEnum)
	if err != nil {
		return nil, fmt.Errorf("获取 %s 凭据失败: %w", provider, err)
	}

	client, err := NewCloudProviderClient(provider, creds.AccessKey, creds.SecretKey)
	if err != nil {
		return nil, fmt.Errorf("创建云服务商客户端失败: %w", err)
	}

	return client.GetAvailableInstanceTypes(ctx, region)
}

// GenerateAndSaveTemplate 生成并保存模板到指定目录
func (s *dynamicTemplateService) GenerateAndSaveTemplate(ctx context.Context, scenario, provider, region, instanceType, destPath string, options map[string]interface{}) error {
	log := logger.GetLogger()
	log.Info("开始生成动态模板: scenario=%s, provider=%s, region=%s, instanceType=%s",
		scenario, provider, region, instanceType)

	// 生成模板
	templatePath, err := s.templateGen.GenerateTemplate(ctx, scenario, provider, region, instanceType, options)
	if err != nil {
		return fmt.Errorf("生成模板失败: %w", err)
	}

	// 确保目标目录存在
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	// 复制模板文件到目标目录
	if err := s.copyTemplateFiles(templatePath, destPath); err != nil {
		return fmt.Errorf("复制模板文件失败: %w", err)
	}

	log.Info("动态模板生成成功: destPath=%s", destPath)
	return nil
}

// copyTemplateFiles 复制模板文件
func (s *dynamicTemplateService) copyTemplateFiles(srcDir, destDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("读取源目录失败: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		srcPath := filepath.Join(srcDir, entry.Name())
		destPath := filepath.Join(destDir, entry.Name())

		data, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("读取文件 %s 失败: %w", srcPath, err)
		}

		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return fmt.Errorf("写入文件 %s 失败: %w", destPath, err)
		}
	}

	return nil
}
