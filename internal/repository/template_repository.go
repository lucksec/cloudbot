package repository

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/meta-matrix/meta-matrix/internal/config"
	"github.com/meta-matrix/meta-matrix/internal/domain"
)

// TemplateRepository 模板仓库接口
type TemplateRepository interface {
	// ListTemplates 列出所有模板
	ListTemplates() ([]*domain.Template, error)

	// GetTemplate 获取模板信息
	GetTemplate(provider, name string) (*domain.Template, error)

	// GetTemplatePath 获取模板路径
	GetTemplatePath(provider, name string) (string, error)

	// CopyTemplate 复制模板到目标目录
	CopyTemplate(provider, name, destPath string) error
}

// templateRepository 模板仓库实现
type templateRepository struct {
	config *config.Config
}

// NewTemplateRepository 创建模板仓库实例
func NewTemplateRepository(cfg *config.Config) TemplateRepository {
	return &templateRepository{
		config: cfg,
	}
}

// ListTemplates 列出所有模板
func (r *templateRepository) ListTemplates() ([]*domain.Template, error) {
	templateDir := r.config.TemplateDir

	// 读取模板目录
	entries, err := os.ReadDir(templateDir)
	if err != nil {
		return nil, fmt.Errorf("读取模板目录失败: %w", err)
	}

	var templates []*domain.Template

	// 遍历云服务商目录
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		provider := entry.Name()
		providerPath := filepath.Join(templateDir, provider)

		// 跳过非云服务商目录（如 img, README.md 等）
		if !isProvider(provider) {
			continue
		}

		// 读取该云服务商下的模板
		templateEntries, err := os.ReadDir(providerPath)
		if err != nil {
			continue
		}

		for _, templateEntry := range templateEntries {
			if !templateEntry.IsDir() {
				continue
			}

			templateName := templateEntry.Name()
			templatePath := filepath.Join(providerPath, templateName)

			// 递归查找 main.tf 文件（支持嵌套结构）
			mainTfPath, err := r.findMainTf(templatePath)
			if err != nil {
				// 如果没有找到 main.tf，尝试查找子目录中的模板
				subTemplates := r.findSubTemplates(provider, templatePath, templateName)
				templates = append(templates, subTemplates...)
				continue
			}

			// 如果找到 main.tf，使用包含 main.tf 的目录作为模板路径
			actualTemplatePath := filepath.Dir(mainTfPath)

			// 读取模板文件列表
			files, err := r.getTemplateFiles(actualTemplatePath)
			if err != nil {
				continue
			}

			// 读取描述信息（优先从根目录读取）
			description := r.getTemplateDescription(templatePath)
			if description == "" {
				description = r.getTemplateDescription(actualTemplatePath)
			}

			template := &domain.Template{
				Provider:    provider,
				Name:        templateName,
				Path:        actualTemplatePath,
				Description: description,
				Files:       files,
			}

			templates = append(templates, template)
		}
	}

	return templates, nil
}

// GetTemplate 获取模板信息
func (r *templateRepository) GetTemplate(provider, name string) (*domain.Template, error) {
	templatePath := filepath.Join(r.config.TemplateDir, provider, name)

	// 检查模板是否存在
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("模板 %s/%s 不存在", provider, name)
	}

	// 递归查找 main.tf 文件
	mainTfPath, err := r.findMainTf(templatePath)
	if err != nil {
		return nil, fmt.Errorf("模板 %s/%s 不是有效的 Terraform 模板: %w", provider, name, err)
	}

	// 使用包含 main.tf 的目录作为模板路径
	actualTemplatePath := filepath.Dir(mainTfPath)

	// 读取模板文件列表
	files, err := r.getTemplateFiles(actualTemplatePath)
	if err != nil {
		return nil, fmt.Errorf("读取模板文件列表失败: %w", err)
	}

	// 读取描述信息（优先从根目录读取）
	description := r.getTemplateDescription(templatePath)
	if description == "" {
		description = r.getTemplateDescription(actualTemplatePath)
	}

	return &domain.Template{
		Provider:    provider,
		Name:        name,
		Path:        actualTemplatePath,
		Description: description,
		Files:       files,
	}, nil
}

// GetTemplatePath 获取模板路径
func (r *templateRepository) GetTemplatePath(provider, name string) (string, error) {
	templatePath := filepath.Join(r.config.TemplateDir, provider, name)

	// 检查模板是否存在
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		return "", fmt.Errorf("模板 %s/%s 不存在", provider, name)
	}

	return templatePath, nil
}

// CopyTemplate 复制模板到目标目录
func (r *templateRepository) CopyTemplate(provider, name, destPath string) error {
	template, err := r.GetTemplate(provider, name)
	if err != nil {
		return err
	}

	// 创建目标目录
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	// 复制模板文件
	for _, file := range template.Files {
		srcPath := filepath.Join(template.Path, file)
		dstPath := filepath.Join(destPath, file)

		// 读取源文件
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("读取文件 %s 失败: %w", srcPath, err)
		}

		// 确保目标目录存在
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			return fmt.Errorf("创建目标目录失败: %w", err)
		}

		// 写入目标文件
		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			return fmt.Errorf("写入文件 %s 失败: %w", dstPath, err)
		}
	}

	return nil
}

// getTemplateFiles 获取模板文件列表
func (r *templateRepository) getTemplateFiles(templatePath string) ([]string, error) {
	var files []string

	err := filepath.Walk(templatePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过目录
		if info.IsDir() {
			return nil
		}

		// 跳过隐藏文件和特定文件
		if strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// 计算相对路径
		relPath, err := filepath.Rel(templatePath, path)
		if err != nil {
			return err
		}

		files = append(files, relPath)
		return nil
	})

	return files, err
}

// getTemplateDescription 获取模板描述
func (r *templateRepository) getTemplateDescription(templatePath string) string {
	// 尝试读取 readme.md
	readmePath := filepath.Join(templatePath, "readme.md")
	if _, err := os.Stat(readmePath); err == nil {
		data, err := os.ReadFile(readmePath)
		if err == nil {
			// 返回第一行作为描述
			lines := strings.Split(string(data), "\n")
			if len(lines) > 0 && lines[0] != "" {
				return strings.TrimSpace(lines[0])
			}
		}
	}

	return ""
}

// findMainTf 递归查找 main.tf 文件
func (r *templateRepository) findMainTf(rootPath string) (string, error) {
	var mainTfPath string
	var found bool

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 查找 main.tf 文件
		if !info.IsDir() && info.Name() == "main.tf" {
			mainTfPath = path
			found = true
			return filepath.SkipAll // 找到后停止遍历
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	if !found {
		return "", fmt.Errorf("未找到 main.tf 文件")
	}

	return mainTfPath, nil
}

// findSubTemplates 查找子目录中的模板（用于处理嵌套结构）
func (r *templateRepository) findSubTemplates(provider, rootPath, baseName string) []*domain.Template {
	var templates []*domain.Template

	// 遍历子目录，查找包含 main.tf 的目录
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// 如果是 main.tf 文件
		if !info.IsDir() && info.Name() == "main.tf" {
			// 获取包含 main.tf 的目录
			templateDir := filepath.Dir(path)

			// 计算相对于根路径的相对路径作为模板名称
			relPath, err := filepath.Rel(rootPath, templateDir)
			if err != nil {
				return nil
			}

			// 构建模板名称：baseName-subPath
			templateName := baseName
			if relPath != "." {
				// 将路径转换为模板名称，例如 zone-node/ss-libev-node-bj -> aliyun-proxy-zone-node-ss-libev-node-bj
				parts := strings.Split(relPath, string(filepath.Separator))
				templateName = baseName + "-" + strings.Join(parts, "-")
			}

			// 读取模板文件列表
			files, err := r.getTemplateFiles(templateDir)
			if err != nil {
				return nil
			}

			// 读取描述信息
			description := r.getTemplateDescription(rootPath)
			if description == "" {
				description = r.getTemplateDescription(templateDir)
			}

			template := &domain.Template{
				Provider:    provider,
				Name:        templateName,
				Path:        templateDir,
				Description: description,
				Files:       files,
			}

			templates = append(templates, template)
		}

		return nil
	})

	if err != nil {
		return templates
	}

	return templates
}

// isProvider 判断是否为云服务商目录
func isProvider(name string) bool {
	providers := []string{"aliyun", "tencent", "aws", "vultr", "ecs", "huaweicloud"}
	for _, p := range providers {
		if name == p {
			return true
		}
	}
	return false
}
