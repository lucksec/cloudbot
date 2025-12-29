package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lucksec/cloudbot/internal/config"
	"github.com/lucksec/cloudbot/internal/domain"
	"gopkg.in/ini.v1"
)

// ProjectRepository 项目管理仓库接口
type ProjectRepository interface {
	// CreateProject 创建新项目
	CreateProject(name string) (*domain.Project, error)

	// GetProject 获取项目信息
	GetProject(name string) (*domain.Project, error)

	// ListProjects 列出所有项目
	ListProjects() ([]*domain.Project, error)

	// DeleteProject 删除项目
	DeleteProject(name string) error

	// AddScenario 添加场景到项目
	AddScenario(projectName string, scenario *domain.Scenario) error

	// GetScenario 获取场景信息
	GetScenario(projectName, scenarioID string) (*domain.Scenario, error)

	// ListScenarios 列出项目的所有场景
	ListScenarios(projectName string) ([]*domain.Scenario, error)

	// DeleteScenario 删除场景
	DeleteScenario(projectName, scenarioID string) error

	// UpdateScenario 更新场景信息
	UpdateScenario(projectName string, scenario *domain.Scenario) error
}

// projectRepository 项目仓库实现
type projectRepository struct {
	config *config.Config
}

// NewProjectRepository 创建项目仓库实例
func NewProjectRepository(cfg *config.Config) ProjectRepository {
	return &projectRepository{
		config: cfg,
	}
}

// CreateProject 创建新项目
func (r *projectRepository) CreateProject(name string) (*domain.Project, error) {
	projectPath := filepath.Join(r.config.ProjectDir, name)

	// 检查项目是否已存在
	if _, err := os.Stat(projectPath); err == nil {
		return nil, fmt.Errorf("项目 %s 已存在", name)
	}

	// 创建项目目录
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		return nil, fmt.Errorf("创建项目目录失败: %w", err)
	}

	// 创建项目配置
	project := &domain.Project{
		Name:      name,
		Path:      projectPath,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Scenarios: []domain.Scenario{},
	}

	// 保存项目配置
	if err := r.saveProjectConfig(project); err != nil {
		os.RemoveAll(projectPath)
		return nil, fmt.Errorf("保存项目配置失败: %w", err)
	}

	return project, nil
}

// GetProject 获取项目信息
func (r *projectRepository) GetProject(name string) (*domain.Project, error) {
	projectPath := filepath.Join(r.config.ProjectDir, name)

	// 检查项目是否存在
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("项目 %s 不存在", name)
	}

	// 加载项目配置
	cfg, err := config.LoadProjectConfig(projectPath)
	if err != nil {
		return nil, fmt.Errorf("加载项目配置失败: %w", err)
	}

	// 解析项目信息
	project := &domain.Project{
		Name: name,
		Path: projectPath,
	}

	// 从配置文件中读取项目信息
	if section := cfg.Section("project"); section != nil {
		if createdAtStr := section.Key("created_at").String(); createdAtStr != "" {
			if t, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
				project.CreatedAt = t
			}
		}
		if updatedAtStr := section.Key("updated_at").String(); updatedAtStr != "" {
			if t, err := time.Parse(time.RFC3339, updatedAtStr); err == nil {
				project.UpdatedAt = t
			}
		}
	}

	// 加载场景列表（避免递归调用，直接读取目录）
	entries, err := os.ReadDir(project.Path)
	if err == nil {
		var scenarios []*domain.Scenario
		for _, entry := range entries {
			if !entry.IsDir() || entry.Name() == ".git" {
				continue
			}

			scenarioPath := filepath.Join(project.Path, entry.Name())
			metadataPath := filepath.Join(scenarioPath, ".scenario.json")

			// 尝试读取场景元数据
			if data, err := os.ReadFile(metadataPath); err == nil {
				var scenario domain.Scenario
				if err := json.Unmarshal(data, &scenario); err == nil {
					scenario.Path = scenarioPath
					scenarios = append(scenarios, &scenario)
				}
			}
		}

		// 将指针切片转换为值切片
		project.Scenarios = make([]domain.Scenario, len(scenarios))
		for i, s := range scenarios {
			project.Scenarios[i] = *s
		}
	}

	return project, nil
}

// ListProjects 列出所有项目
func (r *projectRepository) ListProjects() ([]*domain.Project, error) {
	entries, err := os.ReadDir(r.config.ProjectDir)
	if err != nil {
		return nil, fmt.Errorf("读取项目目录失败: %w", err)
	}

	var projects []*domain.Project
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		project, err := r.GetProject(entry.Name())
		if err != nil {
			continue // 跳过无法读取的项目
		}
		projects = append(projects, project)
	}

	return projects, nil
}

// DeleteProject 删除项目
func (r *projectRepository) DeleteProject(name string) error {
	projectPath := filepath.Join(r.config.ProjectDir, name)

	// 检查项目是否存在
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return fmt.Errorf("项目 %s 不存在", name)
	}

	// 删除项目目录
	return os.RemoveAll(projectPath)
}

// AddScenario 添加场景到项目
func (r *projectRepository) AddScenario(projectName string, scenario *domain.Scenario) error {
	project, err := r.GetProject(projectName)
	if err != nil {
		return err
	}

	// 创建场景目录
	scenarioPath := filepath.Join(project.Path, scenario.ID)
	if err := os.MkdirAll(scenarioPath, 0755); err != nil {
		return fmt.Errorf("创建场景目录失败: %w", err)
	}

	// 保存场景配置
	scenario.Path = scenarioPath
	scenario.CreatedAt = time.Now()
	scenario.UpdatedAt = time.Now()

	// 更新项目配置
	project.UpdatedAt = time.Now()
	if err := r.saveProjectConfig(project); err != nil {
		return fmt.Errorf("更新项目配置失败: %w", err)
	}

	// 保存场景元数据
	if err := r.saveScenarioMetadata(projectName, scenario); err != nil {
		return fmt.Errorf("保存场景元数据失败: %w", err)
	}

	return nil
}

// GetScenario 获取场景信息
func (r *projectRepository) GetScenario(projectName, scenarioID string) (*domain.Scenario, error) {
	project, err := r.GetProject(projectName)
	if err != nil {
		return nil, err
	}

	scenarioPath := filepath.Join(project.Path, scenarioID)

	// 检查场景目录是否存在
	if _, err := os.Stat(scenarioPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("场景 %s 不存在", scenarioID)
	}

	// 加载场景元数据
	metadataPath := filepath.Join(scenarioPath, ".scenario.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("读取场景元数据失败: %w", err)
	}

	var scenario domain.Scenario
	if err := json.Unmarshal(data, &scenario); err != nil {
		return nil, fmt.Errorf("解析场景元数据失败: %w", err)
	}

	scenario.Path = scenarioPath
	return &scenario, nil
}

// ListScenarios 列出项目的所有场景
func (r *projectRepository) ListScenarios(projectName string) ([]*domain.Scenario, error) {
	project, err := r.GetProject(projectName)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(project.Path)
	if err != nil {
		return nil, fmt.Errorf("读取项目目录失败: %w", err)
	}

	var scenarios []*domain.Scenario
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == ".git" {
			continue
		}

		scenario, err := r.GetScenario(projectName, entry.Name())
		if err != nil {
			continue // 跳过无法读取的场景
		}
		scenarios = append(scenarios, scenario)
	}

	return scenarios, nil
}

// DeleteScenario 删除场景
func (r *projectRepository) DeleteScenario(projectName, scenarioID string) error {
	project, err := r.GetProject(projectName)
	if err != nil {
		return err
	}

	scenarioPath := filepath.Join(project.Path, scenarioID)

	// 检查场景是否存在
	if _, err := os.Stat(scenarioPath); os.IsNotExist(err) {
		return fmt.Errorf("场景 %s 不存在", scenarioID)
	}

	// 删除场景目录
	return os.RemoveAll(scenarioPath)
}

// UpdateScenario 更新场景信息
func (r *projectRepository) UpdateScenario(projectName string, scenario *domain.Scenario) error {
	// 检查场景是否存在
	_, err := r.GetScenario(projectName, scenario.ID)
	if err != nil {
		return err
	}

	// 更新场景的更新时间
	scenario.UpdatedAt = time.Now()

	// 保存场景元数据
	return r.saveScenarioMetadata(projectName, scenario)
}

// saveProjectConfig 保存项目配置
func (r *projectRepository) saveProjectConfig(project *domain.Project) error {
	cfg, err := config.LoadProjectConfig(project.Path)
	if err != nil {
		cfg = ini.Empty()
	}

	section := cfg.Section("project")
	section.Key("name").SetValue(project.Name)
	section.Key("created_at").SetValue(project.CreatedAt.Format(time.RFC3339))
	section.Key("updated_at").SetValue(project.UpdatedAt.Format(time.RFC3339))

	return config.SaveProjectConfig(project.Path, cfg)
}

// saveScenarioMetadata 保存场景元数据
func (r *projectRepository) saveScenarioMetadata(projectName string, scenario *domain.Scenario) error {
	metadataPath := filepath.Join(scenario.Path, ".scenario.json")

	data, err := json.MarshalIndent(scenario, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化场景元数据失败: %w", err)
	}

	return os.WriteFile(metadataPath, data, 0644)
}
