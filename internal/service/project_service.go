package service

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/meta-matrix/meta-matrix/internal/credentials"
	"github.com/meta-matrix/meta-matrix/internal/domain"
	"github.com/meta-matrix/meta-matrix/internal/logger"
	"github.com/meta-matrix/meta-matrix/internal/repository"
)

// ProjectService 项目服务接口
type ProjectService interface {
	// CreateProject 创建新项目
	CreateProject(ctx context.Context, name string) (*domain.Project, error)

	// GetProject 获取项目信息
	GetProject(ctx context.Context, name string) (*domain.Project, error)

	// ListProjects 列出所有项目
	ListProjects(ctx context.Context) ([]*domain.Project, error)

	// DeleteProject 删除项目
	DeleteProject(ctx context.Context, name string) error

	// CreateScenario 从模板创建场景
	// region: 区域（可选，aliyun-proxy 模板：bj/sh/hhht/wlcb/zjk）
	CreateScenario(ctx context.Context, projectName, provider, templateName string, region string) (*domain.Scenario, error)

	// CreateScenarioWithOptions 从模板创建场景（支持动态模板生成）
	// instanceType: 实例类型（用于动态模板生成）
	// scenarioType: 场景类型 (proxy, task-executor)（用于动态模板生成）
	// options: 其他选项（如 node_count 等）
	CreateScenarioWithOptions(ctx context.Context, projectName, provider, templateName string, region, instanceType, scenarioType string, options map[string]interface{}) (*domain.Scenario, error)

	// GetScenario 获取场景信息
	GetScenario(ctx context.Context, projectName, scenarioID string) (*domain.Scenario, error)

	// ListScenarios 列出项目的所有场景
	ListScenarios(ctx context.Context, projectName string) ([]*domain.Scenario, error)

	// DeleteScenario 删除场景
	DeleteScenario(ctx context.Context, projectName, scenarioID string) error

	// DeployScenario 部署场景
	// nodeCount: 节点数量（可选，0 表示使用默认值）
	// toolName: 工具名称（可选，对应 OSS 中的程序路径）
	// toolArgs: 工具参数（可选，空格分隔的参数字符串）
	// region: 区域（可选，aliyun-proxy 模板：bj/sh/hhht/wlcb/zjk，不指定则按顺序启动）
	DeployScenario(ctx context.Context, projectName, scenarioID string, autoApprove bool, nodeCount int, toolName, toolArgs string, region string) error

	// DestroyScenario 销毁场景
	DestroyScenario(ctx context.Context, projectName, scenarioID string, autoApprove bool) error

	// GetProjectStatus 获取项目的云资源状态列表（云资源验证）
	// 返回每个场景及其当前 Terraform 状态中的资源列表
	GetProjectStatus(ctx context.Context, projectName string) ([]ScenarioStatus, error)

	// GetScenarioStatus 获取指定场景的云资源状态（云资源验证）
	// 返回单个场景及其当前 Terraform 状态中的资源列表
	GetScenarioStatus(ctx context.Context, projectName, scenarioID string) (*ScenarioStatus, error)

	// InitProject 初始化项目（预先执行所有场景的 Terraform 初始化）
	// 用于提前完成 backend 初始化和 provider 插件下载，避免首次部署时等待较久
	InitProject(ctx context.Context, name string) error
}

// ScenarioStatus 场景云资源状态
// 用于对外返回每个场景当前在云端的实际资源情况
type ScenarioStatus struct {
	Scenario  *domain.Scenario    // 场景元信息
	Resources []string            // Terraform state 中的资源列表
	Instances []ECSInstanceDetail // 实例详细信息
}

// projectService 项目服务实现
type projectService struct {
	projectRepo        repository.ProjectRepository
	templateRepo       repository.TemplateRepository
	terraformSvc       TerraformService
	dynamicTemplateSvc DynamicTemplateService // 动态模板服务
}

// NewProjectService 创建项目服务实例
func NewProjectService(
	projectRepo repository.ProjectRepository,
	templateRepo repository.TemplateRepository,
	terraformSvc TerraformService,
) ProjectService {
	// 创建动态模板服务（如果凭据管理器可用）
	var dynamicTemplateSvc DynamicTemplateService
	if credManager := credentials.GetDefaultManager(); credManager != nil {
		dynamicTemplateSvc = NewDynamicTemplateService(credManager)
	}

	return &projectService{
		projectRepo:        projectRepo,
		templateRepo:       templateRepo,
		terraformSvc:       terraformSvc,
		dynamicTemplateSvc: dynamicTemplateSvc,
	}
}

// CreateProject 创建新项目
func (s *projectService) CreateProject(ctx context.Context, name string) (*domain.Project, error) {
	if name == "" {
		return nil, fmt.Errorf("项目名称不能为空")
	}

	return s.projectRepo.CreateProject(name)
}

// GetProject 获取项目信息
func (s *projectService) GetProject(ctx context.Context, name string) (*domain.Project, error) {
	return s.projectRepo.GetProject(name)
}

// ListProjects 列出所有项目
func (s *projectService) ListProjects(ctx context.Context) ([]*domain.Project, error) {
	return s.projectRepo.ListProjects()
}

// DeleteProject 删除项目
func (s *projectService) DeleteProject(ctx context.Context, name string) error {
	// 检查项目是否存在
	_, err := s.projectRepo.GetProject(name)
	if err != nil {
		return err
	}

	// 检查是否有已部署的场景
	scenarios, err := s.projectRepo.ListScenarios(name)
	if err == nil {
		for _, scenario := range scenarios {
			if scenario.Status == "deployed" {
				return fmt.Errorf("项目 %s 包含已部署的场景 %s，请先销毁场景", name, scenario.ID)
			}
		}
	}

	return s.projectRepo.DeleteProject(name)
}

// CreateScenario 从模板创建场景
// 支持静态模板和动态模板生成
func (s *projectService) CreateScenario(ctx context.Context, projectName, provider, templateName string, region string) (*domain.Scenario, error) {
	return s.CreateScenarioWithOptions(ctx, projectName, provider, templateName, region, "", "", nil)
}

// CreateScenarioWithOptions 从模板创建场景（支持动态模板生成）
// instanceType: 实例类型（用于动态模板生成）
// scenarioType: 场景类型 (proxy, task-executor)（用于动态模板生成）
// options: 其他选项（如 node_count 等）
func (s *projectService) CreateScenarioWithOptions(ctx context.Context, projectName, provider, templateName string, region, instanceType, scenarioType string, options map[string]interface{}) (*domain.Scenario, error) {
	// 检查项目是否存在
	_, err := s.projectRepo.GetProject(projectName)
	if err != nil {
		return nil, fmt.Errorf("项目不存在: %w", err)
	}

	// 判断是否为动态模板生成（通过 scenarioType 参数）
	useDynamicTemplate := scenarioType != "" && (scenarioType == "proxy" || scenarioType == "task-executor")

	var actualProvider string
	var actualTemplateName string
	var scenario *domain.Scenario
	var scenarioPath string

	if useDynamicTemplate {
		// 使用动态模板生成
		if s.dynamicTemplateSvc == nil {
			return nil, fmt.Errorf("动态模板服务未初始化，请配置云服务商凭据")
		}

		// 生成场景 UUID
		scenarioID := uuid.New().String()

		// 创建场景
		scenario = &domain.Scenario{
			ID:       scenarioID,
			Name:     fmt.Sprintf("%s-%s-%s", provider, scenarioType, region),
			Template: fmt.Sprintf("%s/%s-dynamic", provider, scenarioType),
			Status:   "pending",
		}

		// 获取项目路径
		project, _ := s.projectRepo.GetProject(projectName)
		scenarioPath = fmt.Sprintf("%s/%s", project.Path, scenarioID)

		// 生成并保存动态模板
		if err := s.dynamicTemplateSvc.GenerateAndSaveTemplate(ctx, scenarioType, provider, region, instanceType, scenarioPath, options); err != nil {
			return nil, fmt.Errorf("生成动态模板失败: %w", err)
		}
	} else {
		// 使用静态模板（原有逻辑）
		actualProvider = provider
		actualTemplateName = templateName

		if provider == "aliyun" && templateName == "aliyun-proxy" {
			// 对于 aliyun-proxy 模板，必须指定区域
			if region == "" {
				return nil, fmt.Errorf("aliyun-proxy 模板必须指定区域，支持的区域: bj, sh, hhht, wlcb, zjk")
			}

			// 区域名称到模板路径的映射
			regionMap := map[string]string{
				"bj":   "aliyun-proxy/zone-node/ss-libev-node-bj",
				"sh":   "aliyun-proxy/zone-node/ss-libev-node-sh",
				"hhht": "aliyun-proxy/zone-node/ss-libev-node-hhht",
				"wlcb": "aliyun-proxy/zone-node/ss-libev-node-wlcb",
				"zjk":  "aliyun-proxy/zone-node/ss-libev-node-zjk",
			}

			// 验证区域是否有效
			templatePath, ok := regionMap[region]
			if !ok {
				return nil, fmt.Errorf("无效的区域: %s，支持的区域: bj, sh, hhht, wlcb, zjk", region)
			}
			actualTemplateName = templatePath
		}

		// 检查模板是否存在
		_, err = s.templateRepo.GetTemplate(actualProvider, actualTemplateName)
		if err != nil {
			return nil, fmt.Errorf("模板不存在: %w", err)
		}

		// 生成场景 UUID
		scenarioID := uuid.New().String()

		// 创建场景
		scenario = &domain.Scenario{
			ID:       scenarioID,
			Name:     fmt.Sprintf("%s-%s", provider, templateName),
			Template: fmt.Sprintf("%s/%s", actualProvider, actualTemplateName),
			Status:   "pending",
		}

		// 如果指定了区域，在名称中添加区域标识
		if provider == "aliyun" && templateName == "aliyun-proxy" && region != "" {
			scenario.Name = fmt.Sprintf("%s-%s-%s", provider, templateName, region)
		}

		// 获取项目路径
		project, _ := s.projectRepo.GetProject(projectName)
		scenarioPath = fmt.Sprintf("%s/%s", project.Path, scenarioID)

		// 复制模板到场景目录
		if err := s.templateRepo.CopyTemplate(actualProvider, actualTemplateName, scenarioPath); err != nil {
			return nil, fmt.Errorf("复制模板失败: %w", err)
		}
	}

	// 保存场景
	if err := s.projectRepo.AddScenario(projectName, scenario); err != nil {
		return nil, fmt.Errorf("保存场景失败: %w", err)
	}

	return scenario, nil
}

// GetScenario 获取场景信息
func (s *projectService) GetScenario(ctx context.Context, projectName, scenarioID string) (*domain.Scenario, error) {
	return s.projectRepo.GetScenario(projectName, scenarioID)
}

// ListScenarios 列出项目的所有场景
func (s *projectService) ListScenarios(ctx context.Context, projectName string) ([]*domain.Scenario, error) {
	return s.projectRepo.ListScenarios(projectName)
}

// DeleteScenario 删除场景
func (s *projectService) DeleteScenario(ctx context.Context, projectName, scenarioID string) error {
	// 检查场景是否存在
	scenario, err := s.projectRepo.GetScenario(projectName, scenarioID)
	if err != nil {
		return err
	}

	// 如果场景已部署，需要先销毁
	if scenario.Status == "deployed" {
		return fmt.Errorf("场景 %s 已部署，请先销毁场景", scenarioID)
	}

	return s.projectRepo.DeleteScenario(projectName, scenarioID)
}

// DeployScenario 部署场景
// 支持通过 nodeCount 参数覆盖 Terraform 模板中的节点数量（如 node_count 变量）
// 当 nodeCount <= 0 时，不传递覆盖变量，沿用模板默认/随机逻辑
// toolName 和 toolArgs 用于 task-executor-spot 模板，指定从 OSS 获取的执行程序和参数
// region 用于 aliyun-proxy 模板，指定区域（bj/sh/hhht/wlcb/zjk），如果为空则按顺序启动
func (s *projectService) DeployScenario(ctx context.Context, projectName, scenarioID string, autoApprove bool, nodeCount int, toolName, toolArgs string, region string) error {
	log := logger.GetLogger()
	log.Info("开始部署场景: project=%s, scenario=%s, nodeCount=%d, toolName=%s, region=%s",
		projectName, scenarioID, nodeCount, toolName, region)

	// 获取场景信息
	scenario, err := s.projectRepo.GetScenario(projectName, scenarioID)
	if err != nil {
		log.Error("获取场景信息失败: project=%s, scenario=%s, error=%v", projectName, scenarioID, err)
		return err
	}

	// 处理 aliyun-proxy 模板的区域选择
	// 如果场景模板路径已经包含区域信息（如 aliyun/aliyun-proxy/zone-node/ss-libev-node-xx），
	// 说明创建时已经指定了区域，直接使用该区域部署，忽略部署时的 region 参数
	if strings.Contains(scenario.Template, "aliyun/aliyun-proxy/zone-node/ss-libev-node-") {
		// 从模板路径中已经确定了区域，直接部署
		log.Info("场景已指定区域（模板路径: %s），使用模板中的区域进行部署", scenario.Template)
		return s.deployAliyunProxyScenario(ctx, projectName, scenarioID, autoApprove, nodeCount, toolName, toolArgs, scenario)
	}

	// 如果模板是 aliyun/aliyun-proxy（没有指定具体区域），则使用旧的逻辑（按顺序启动所有区域）
	// 这种情况不应该存在，因为创建时必须指定区域
	if strings.Contains(scenario.Template, "aliyun/aliyun-proxy") && !strings.Contains(scenario.Template, "zone-node") {
		log.Warn("场景模板未指定区域，使用旧的逻辑按顺序启动所有区域")
		return s.deployAliyunProxyWithRegion(ctx, projectName, scenarioID, autoApprove, nodeCount, toolName, toolArgs, scenario, region)
	}

	// 构建可选的 Terraform 变量
	vars := make(map[string]string)

	// 根据模板类型，从凭据管理器获取并传递云服务商凭据
	// 使用精确匹配，避免为不需要的模板传递变量
	templatePath := scenario.Template

	// 腾讯云模板：所有模板都需要 tencentcloud_secret_id 和 tencentcloud_secret_key
	if strings.HasPrefix(templatePath, "tencent/") {
		credManager := credentials.GetDefaultManager()
		if credManager.HasCredentials(credentials.ProviderTencent) {
			creds, err := credManager.GetCredentials(credentials.ProviderTencent)
			if err == nil && creds != nil {
				vars["tencentcloud_secret_id"] = creds.AccessKey
				vars["tencentcloud_secret_key"] = creds.SecretKey

				// 如果用户指定了区域，使用用户指定的；否则尝试查找有配额的区域
				if creds.Region != "" {
					vars["region"] = creds.Region
				} else {
					// 尝试查找有抢占式实例配额的区域和实例类型
					// 查询 S5 实例族在各区域的可用性
					availability, err := QuerySpotInstanceAvailability(ctx, GetTencentRegions(), "S5")
					if err == nil && len(availability) > 0 {
						// 优先选择国内区域（按顺序：上海、南京、广州、北京、成都、重庆）
						domesticRegions := []string{"ap-shanghai", "ap-nanjing", "ap-guangzhou", "ap-beijing", "ap-chengdu", "ap-chongqing"}
						for _, dr := range domesticRegions {
							for _, av := range availability {
								if av.Region == dr && av.Available {
									vars["region"] = av.Region
									// 如果找到可用的实例类型，也可以设置
									if av.InstanceType != "" {
										log.Info("自动选择腾讯云区域和实例类型: region=%s, instance_type=%s", av.Region, av.InstanceType)
									} else {
										log.Info("自动选择腾讯云区域: %s", av.Region)
									}
									goto regionFound
								}
							}
						}
						// 如果没有国内区域，使用第一个可用区域
						vars["region"] = availability[0].Region
						log.Info("自动选择腾讯云区域: %s", availability[0].Region)
					regionFound:
					} else {
						// 如果查询失败，尝试使用 FindBestTencentRegion
						bestRegion, err := FindBestTencentRegion(ctx, "S5.SMALL1")
						if err == nil && bestRegion != "" {
							vars["region"] = bestRegion
							log.Info("自动选择腾讯云区域: %s", bestRegion)
						} else {
							// 如果查找失败，使用默认区域
							vars["region"] = "ap-beijing"
						}
					}
				}
			}
		}
	}

	// 阿里云模板：根据模板类型传递不同的凭据
	if strings.HasPrefix(templatePath, "aliyun/") {
		credManager := credentials.GetDefaultManager()
		if credManager.HasCredentials(credentials.ProviderAliyun) {
			creds, err := credManager.GetCredentials(credentials.ProviderAliyun)
			if err == nil && creds != nil {
				// aliyun-proxy 模板需要 access_key 和 secret_key
				if strings.Contains(templatePath, "aliyun-proxy") {
					vars["access_key"] = creds.AccessKey
					vars["secret_key"] = creds.SecretKey
				}
				// 所有模板都可以传递 region
				if creds.Region != "" {
					vars["region"] = creds.Region
				}
			}
		}
		// task-executor-spot 需要传递 OSS 凭据（oss_access_key_id/oss_access_key_secret）
		// 这些会在下面单独处理
	}

	// 华为云模板：所有模板都需要 access_key 和 secret_key
	if strings.HasPrefix(templatePath, "huaweicloud/") {
		credManager := credentials.GetDefaultManager()
		if credManager.HasCredentials(credentials.ProviderHuaweicloud) {
			creds, err := credManager.GetCredentials(credentials.ProviderHuaweicloud)
			if err == nil && creds != nil {
				vars["access_key"] = creds.AccessKey
				vars["secret_key"] = creds.SecretKey
				if creds.Region != "" {
					vars["region"] = creds.Region
				}
			}
		}
	}

	if nodeCount > 0 {
		// 目前仅对支持 node_count 的模板传递该变量，避免其他模板报 "未定义变量" 错误
		// 例如 aliyun/aliyun-proxy/zone-node/ss-libev-node-bj
		if strings.Contains(scenario.Template, "aliyun/aliyun-proxy") ||
			strings.Contains(scenario.Template, "huaweicloud/huaweicloud-proxy") ||
			strings.Contains(scenario.Template, "tencent/tencent-proxy") ||
			strings.Contains(scenario.Template, "tencent/tencent-proxy-postpaid") {
			vars["node_count"] = strconv.Itoa(nodeCount)
		}
	}

	// 如果提供了工具名称，设置 task-executor-spot 模板的相关变量
	if toolName != "" {
		// 检查是否为 task-executor-spot 模板
		if strings.Contains(scenario.Template, "task-executor-spot") {
			// 直接使用工具名，让模板自动在存储桶中查找
			// 模板会尝试多个路径：工具名、programs/工具名、tools/工具名、bin/工具名
			vars["program_oss_path"] = toolName
			if toolArgs != "" {
				vars["execution_args"] = toolArgs
			}

			// 传递项目名称和场景ID，用于结果路径组织
			vars["project_name"] = projectName
			vars["scenario_id"] = scenarioID

			// 设置工具存储桶（优先使用环境变量，否则使用默认值）
			toolBucket := os.Getenv("TOOL_OSS_BUCKET")
			if toolBucket == "" {
				// 默认使用 aliyuncloudtools
				toolBucket = "aliyuncloudtools"
			}
			vars["tool_oss_bucket"] = toolBucket
			log.Info("使用工具存储桶: %s", toolBucket)
		}
	}

	// 对于腾讯云抢占式实例，如果节点数 > 1，直接使用跨区域分散部署
	// 这样可以避免单区域配额不足导致的部分成功问题
	if strings.Contains(scenario.Template, "tencent/") {
		// 获取实际节点数
		actualNodeCount := nodeCount
		if actualNodeCount <= 0 {
			if nodeCountStr, ok := vars["node_count"]; ok && nodeCountStr != "" {
				if parsed, err := strconv.Atoi(nodeCountStr); err == nil && parsed > 0 {
					actualNodeCount = parsed
				}
			}
			if actualNodeCount <= 0 {
				actualNodeCount = 3 // 默认值
			}
		}

		// 如果节点数 > 1，且启用了抢占式实例，直接使用跨区域分散部署
		// 注意：按量计费版本（tencent-proxy-postpaid）不需要跨区域分散部署，配额通常充足
		if actualNodeCount > 1 && !strings.Contains(scenario.Template, "tencent-proxy-postpaid") {
			// 检查是否启用了抢占式实例（通过检查变量或模板）
			enableSpot := true // 默认启用
			if spotEnabled, ok := vars["enable_spot"]; ok {
				enableSpot = spotEnabled != "false"
			}

			if enableSpot {
				log.Info("腾讯云抢占式实例多节点部署，使用跨区域分散部署策略: nodeCount=%d", actualNodeCount)
				// 获取国内区域列表
				domesticRegions := GetDomesticRegions()
				// 如果用户指定了区域，将其放在第一位；否则使用所有国内区域
				regions := domesticRegions
				if vars["region"] != "" {
					// 将用户指定的区域放在第一位
					regions = []string{vars["region"]}
					for _, r := range domesticRegions {
						if r != vars["region"] {
							regions = append(regions, r)
						}
					}
				}
				return s.deployAcrossMultipleRegions(ctx, projectName, scenarioID, autoApprove, actualNodeCount, toolName, toolArgs, scenario, vars, regions)
			}
		}
	}

	// 初始化 Terraform
	if err := s.terraformSvc.Init(ctx, scenario.Path); err != nil {
		return fmt.Errorf("初始化 Terraform 失败: %w", err)
	}

	// 验证配置
	if err := s.terraformSvc.Validate(ctx, scenario.Path); err != nil {
		return fmt.Errorf("验证 Terraform 配置失败: %w", err)
	}

	// 执行 plan
	if err := s.terraformSvc.Plan(ctx, scenario.Path, vars); err != nil {
		// 如果是腾讯云且是配额错误，尝试其他区域
		if strings.Contains(scenario.Template, "tencent/") &&
			(strings.Contains(err.Error(), "LimitExceeded.SpotQuota") ||
				strings.Contains(err.Error(), "配额不足")) {
			return s.retryDeployWithDifferentRegions(ctx, projectName, scenarioID, autoApprove, nodeCount, toolName, toolArgs, scenario, vars, err)
		}
		return fmt.Errorf("Terraform plan 失败: %w", err)
	}

	// 执行 apply
	if err := s.terraformSvc.Apply(ctx, scenario.Path, autoApprove, vars); err != nil {
		// 如果是腾讯云且是配额错误，尝试其他区域
		if strings.Contains(scenario.Template, "tencent/") &&
			(strings.Contains(err.Error(), "LimitExceeded.SpotQuota") ||
				strings.Contains(err.Error(), "配额不足")) {
			return s.retryDeployWithDifferentRegions(ctx, projectName, scenarioID, autoApprove, nodeCount, toolName, toolArgs, scenario, vars, err)
		}
		return fmt.Errorf("Terraform apply 失败: %w", err)
	}

	// 更新场景状态
	scenario.Status = "deployed"
	if err := s.projectRepo.UpdateScenario(projectName, scenario); err != nil {
		log.Error("更新场景状态失败: project=%s, scenario=%s, error=%v", projectName, scenarioID, err)
		return fmt.Errorf("更新场景状态失败: %w", err)
	}

	log.Info("场景部署成功: project=%s, scenario=%s", projectName, scenarioID)
	return nil
}

// deployAliyunProxyWithRegion 处理 aliyun-proxy 模板的区域选择逻辑
func (s *projectService) deployAliyunProxyWithRegion(ctx context.Context, projectName, scenarioID string, autoApprove bool, nodeCount int, toolName, toolArgs string, scenario *domain.Scenario, region string) error {
	log := logger.GetLogger()

	// 检查场景模板路径是否已经包含区域信息
	// 如果模板路径是 aliyun/aliyun-proxy/zone-node/ss-libev-node-xx 格式，说明创建时已经指定了区域
	if strings.Contains(scenario.Template, "aliyun/aliyun-proxy/zone-node/ss-libev-node-") {
		// 从模板路径中提取区域信息
		templatePath := scenario.Template
		var extractedRegion string
		if strings.Contains(templatePath, "ss-libev-node-bj") {
			extractedRegion = "bj"
		} else if strings.Contains(templatePath, "ss-libev-node-sh") {
			extractedRegion = "sh"
		} else if strings.Contains(templatePath, "ss-libev-node-hhht") {
			extractedRegion = "hhht"
		} else if strings.Contains(templatePath, "ss-libev-node-wlcb") {
			extractedRegion = "wlcb"
		} else if strings.Contains(templatePath, "ss-libev-node-zjk") {
			extractedRegion = "zjk"
		}

		// 如果从模板路径中提取到了区域，且部署时没有指定区域，直接使用模板中的区域
		if extractedRegion != "" && region == "" {
			log.Info("场景已指定区域: %s，使用模板中的区域进行部署", extractedRegion)
			return s.deployAliyunProxyScenario(ctx, projectName, scenarioID, autoApprove, nodeCount, toolName, toolArgs, scenario)
		}

		// 如果部署时指定了区域，但模板中已经有区域，使用部署时指定的区域（覆盖）
		if extractedRegion != "" && region != "" && region != extractedRegion {
			log.Warn("场景模板区域 (%s) 与部署指定区域 (%s) 不一致，使用部署指定的区域", extractedRegion, region)
			// 需要更新场景的模板路径
			regionMap := map[string]string{
				"bj":   "aliyun/aliyun-proxy/zone-node/ss-libev-node-bj",
				"sh":   "aliyun/aliyun-proxy/zone-node/ss-libev-node-sh",
				"hhht": "aliyun/aliyun-proxy/zone-node/ss-libev-node-hhht",
				"wlcb": "aliyun/aliyun-proxy/zone-node/ss-libev-node-wlcb",
				"zjk":  "aliyun/aliyun-proxy/zone-node/ss-libev-node-zjk",
			}
			if newTemplatePath, ok := regionMap[region]; ok {
				scenario.Template = newTemplatePath
				// 需要重新复制模板文件（这里简化处理，直接使用新的模板路径部署）
				log.Info("切换到区域: %s，模板路径: %s", region, newTemplatePath)
				return s.deployAliyunProxyScenario(ctx, projectName, scenarioID, autoApprove, nodeCount, toolName, toolArgs, scenario)
			}
		}

		// 如果区域匹配，直接部署
		if extractedRegion != "" && region == extractedRegion {
			return s.deployAliyunProxyScenario(ctx, projectName, scenarioID, autoApprove, nodeCount, toolName, toolArgs, scenario)
		}
	}

	// 区域名称到模板路径的映射
	regionMap := map[string]string{
		"bj":   "aliyun/aliyun-proxy/zone-node/ss-libev-node-bj",
		"sh":   "aliyun/aliyun-proxy/zone-node/ss-libev-node-sh",
		"hhht": "aliyun/aliyun-proxy/zone-node/ss-libev-node-hhht",
		"wlcb": "aliyun/aliyun-proxy/zone-node/ss-libev-node-wlcb",
		"zjk":  "aliyun/aliyun-proxy/zone-node/ss-libev-node-zjk",
	}

	// 区域顺序（如果不指定区域，按此顺序启动）
	regionOrder := []string{"bj", "sh", "hhht", "wlcb", "zjk"}

	// 如果指定了区域，只使用该区域
	if region != "" {
		templatePath, ok := regionMap[region]
		if !ok {
			return fmt.Errorf("无效的区域: %s，支持的区域: bj, sh, hhht, wlcb, zjk", region)
		}

		// 更新场景的模板路径
		scenario.Template = templatePath
		log.Info("使用指定区域: %s，模板路径: %s", region, templatePath)

		// 使用基础部署方法
		return s.deployAliyunProxyScenario(ctx, projectName, scenarioID, autoApprove, nodeCount, toolName, toolArgs, scenario)
	}

	// 未指定区域，按顺序启动所有区域
	log.Info("未指定区域，按顺序启动所有区域: %v", regionOrder)

	var lastErr error
	successCount := 0

	// 获取项目路径
	project, err := s.projectRepo.GetProject(projectName)
	if err != nil {
		return fmt.Errorf("获取项目失败: %w", err)
	}

	for _, reg := range regionOrder {
		templatePath := regionMap[reg]
		log.Info("尝试启动区域: %s，模板路径: %s", reg, templatePath)

		// 为每个区域创建新的场景
		regionScenarioID := scenario.ID + "-" + reg
		regionScenarioPath := fmt.Sprintf("%s/%s", project.Path, regionScenarioID)

		// 复制区域模板到场景目录
		provider := "aliyun"
		templateName := strings.TrimPrefix(templatePath, "aliyun/")
		if err := s.templateRepo.CopyTemplate(provider, templateName, regionScenarioPath); err != nil {
			log.Warn("区域 %s 复制模板失败: %v", reg, err)
			lastErr = err
			continue
		}

		// 创建场景对象
		regionScenario := &domain.Scenario{
			ID:       regionScenarioID,
			Name:     fmt.Sprintf("%s-%s", scenario.Name, reg),
			Template: templatePath,
			Status:   "pending",
			Path:     regionScenarioPath,
		}

		// 保存场景到数据库
		if err := s.projectRepo.AddScenario(projectName, regionScenario); err != nil {
			log.Warn("区域 %s 保存场景失败: %v", reg, err)
			lastErr = err
			continue
		}

		// 部署该区域
		err := s.deployAliyunProxyScenario(ctx, projectName, regionScenarioID, autoApprove, nodeCount, toolName, toolArgs, regionScenario)
		if err != nil {
			log.Warn("区域 %s 部署失败: %v", reg, err)
			lastErr = err
			continue
		}

		successCount++
		log.Info("区域 %s 部署成功", reg)
	}

	if successCount == 0 {
		return fmt.Errorf("所有区域部署失败，最后一个错误: %w", lastErr)
	}

	log.Info("区域部署完成: 成功 %d/%d", successCount, len(regionOrder))
	return nil
}

// deployAliyunProxyScenario 部署单个 aliyun-proxy 场景
func (s *projectService) deployAliyunProxyScenario(ctx context.Context, projectName, scenarioID string, autoApprove bool, nodeCount int, toolName, toolArgs string, scenario *domain.Scenario) error {
	log := logger.GetLogger()

	// 构建可选的 Terraform 变量
	vars := make(map[string]string)

	// 获取阿里云凭据
	credManager := credentials.GetDefaultManager()
	if credManager.HasCredentials(credentials.ProviderAliyun) {
		creds, err := credManager.GetCredentials(credentials.ProviderAliyun)
		if err == nil && creds != nil {
			vars["access_key"] = creds.AccessKey
			vars["secret_key"] = creds.SecretKey
			// 从模板路径中提取区域信息
			if strings.Contains(scenario.Template, "ss-libev-node-bj") {
				vars["region"] = "cn-beijing"
			} else if strings.Contains(scenario.Template, "ss-libev-node-sh") {
				vars["region"] = "cn-shanghai"
			} else if strings.Contains(scenario.Template, "ss-libev-node-hhht") {
				vars["region"] = "cn-huhehaote"
			} else if strings.Contains(scenario.Template, "ss-libev-node-wlcb") {
				vars["region"] = "cn-wulanchabu"
			} else if strings.Contains(scenario.Template, "ss-libev-node-zjk") {
				vars["region"] = "cn-zhangjiakou"
			}
		}
	}

	// 设置节点数量
	if nodeCount > 0 {
		vars["node_count"] = strconv.Itoa(nodeCount)
	}

	// 初始化 Terraform
	if err := s.terraformSvc.Init(ctx, scenario.Path); err != nil {
		return fmt.Errorf("初始化 Terraform 失败: %w", err)
	}

	// 验证配置
	if err := s.terraformSvc.Validate(ctx, scenario.Path); err != nil {
		return fmt.Errorf("验证 Terraform 配置失败: %w", err)
	}

	// 执行 plan
	if err := s.terraformSvc.Plan(ctx, scenario.Path, vars); err != nil {
		return fmt.Errorf("Terraform plan 失败: %w", err)
	}

	// 执行 apply
	if err := s.terraformSvc.Apply(ctx, scenario.Path, autoApprove, vars); err != nil {
		return fmt.Errorf("Terraform apply 失败: %w", err)
	}

	// 更新场景状态
	scenario.Status = "deployed"
	if err := s.projectRepo.UpdateScenario(projectName, scenario); err != nil {
		log.Error("更新场景状态失败: project=%s, scenario=%s, error=%v", projectName, scenarioID, err)
		return fmt.Errorf("更新场景状态失败: %w", err)
	}

	log.Info("场景部署成功: project=%s, scenario=%s", projectName, scenarioID)
	return nil
}

// retryDeployWithDifferentRegions 使用不同区域重试部署（用于腾讯云配额不足的情况）
// 支持跨区域分散部署：当某个区域配额不足时，将节点分散到多个区域
func (s *projectService) retryDeployWithDifferentRegions(ctx context.Context, projectName, scenarioID string, autoApprove bool, nodeCount int, toolName, toolArgs string, scenario *domain.Scenario, originalVars map[string]string, originalErr error) error {
	log := logger.GetLogger()

	// 获取实际节点数：如果 nodeCount 为 0，尝试从 vars 获取，否则使用默认值 3
	actualNodeCount := nodeCount
	if actualNodeCount <= 0 {
		if nodeCountStr, ok := originalVars["node_count"]; ok && nodeCountStr != "" {
			if parsed, err := strconv.Atoi(nodeCountStr); err == nil && parsed > 0 {
				actualNodeCount = parsed
			}
		}
		// 如果仍然为 0，使用默认值 3（与模板默认值一致）
		if actualNodeCount <= 0 {
			actualNodeCount = 3
		}
	}

	log.Warn("检测到配额不足错误，尝试跨区域分散部署: error=%v, nodeCount=%d", originalErr, actualNodeCount)

	// 只使用国内区域（排除国外区域）
	domesticRegions := GetDomesticRegions()

	// 获取当前使用的区域
	currentRegion := originalVars["region"]
	if currentRegion == "" {
		currentRegion = "ap-beijing"
	}

	// 移除当前区域，从其他区域开始尝试
	availableRegions := []string{}
	for _, region := range domesticRegions {
		if region != currentRegion {
			availableRegions = append(availableRegions, region)
		}
	}

	// 如果节点数量较多（>1）且有多个可用区域，尝试分散部署
	// 否则，尝试在单个区域部署所有节点
	if actualNodeCount > 1 && len(availableRegions) > 1 {
		return s.deployAcrossMultipleRegions(ctx, projectName, scenarioID, autoApprove, actualNodeCount, toolName, toolArgs, scenario, originalVars, availableRegions)
	}

	// 单区域部署：尝试其他区域
	for _, region := range availableRegions {
		log.Info("尝试使用区域: %s", region)

		// 复制变量并更新区域
		newVars := make(map[string]string)
		for k, v := range originalVars {
			newVars[k] = v
		}
		newVars["region"] = region

		// 重新初始化 Terraform
		if err := s.terraformSvc.Init(ctx, scenario.Path); err != nil {
			log.Warn("重新初始化失败，跳过区域 %s: %v", region, err)
			continue
		}

		// 重新验证
		if err := s.terraformSvc.Validate(ctx, scenario.Path); err != nil {
			log.Warn("验证失败，跳过区域 %s: %v", region, err)
			continue
		}

		// 重新执行 plan
		if err := s.terraformSvc.Plan(ctx, scenario.Path, newVars); err != nil {
			// 如果还是配额错误，继续尝试下一个区域
			if strings.Contains(err.Error(), "LimitExceeded.SpotQuota") ||
				strings.Contains(err.Error(), "配额不足") {
				log.Warn("区域 %s 配额不足，继续尝试其他区域", region)
				continue
			}
			log.Warn("Plan 失败，跳过区域 %s: %v", region, err)
			continue
		}

		// 重新执行 apply
		if err := s.terraformSvc.Apply(ctx, scenario.Path, autoApprove, newVars); err != nil {
			// 如果还是配额错误，继续尝试下一个区域
			if strings.Contains(err.Error(), "LimitExceeded.SpotQuota") ||
				strings.Contains(err.Error(), "配额不足") {
				log.Warn("区域 %s 配额不足，继续尝试其他区域", region)
				continue
			}
			// 其他错误，返回
			return fmt.Errorf("Terraform apply 失败 (区域: %s): %w", region, err)
		}

		// 成功！更新场景状态
		scenario.Status = "deployed"
		if err := s.projectRepo.UpdateScenario(projectName, scenario); err != nil {
			log.Error("更新场景状态失败: project=%s, scenario=%s, error=%v", projectName, scenarioID, err)
			return fmt.Errorf("更新场景状态失败: %w", err)
		}

		log.Info("场景部署成功 (使用区域: %s): project=%s, scenario=%s", region, projectName, scenarioID)
		return nil
	}

	// 所有区域都尝试失败
	return fmt.Errorf("所有国内区域都配额不足，部署失败。原始错误: %w", originalErr)
}

// deployAcrossMultipleRegions 跨多个区域分散部署节点
// 将节点分散到多个区域，直到所有节点都部署完成
func (s *projectService) deployAcrossMultipleRegions(ctx context.Context, projectName, scenarioID string, autoApprove bool, totalNodeCount int, toolName, toolArgs string, scenario *domain.Scenario, baseVars map[string]string, regions []string) error {
	log := logger.GetLogger()
	log.Info("开始跨区域分散部署: 总节点数=%d, 可用区域数=%d", totalNodeCount, len(regions))

	remainingNodes := totalNodeCount
	regionIndex := 0
	deployedRegions := []string{}

	// 计算每个区域分配的节点数（尽量平均分配）
	nodesPerRegion := totalNodeCount / len(regions)
	if nodesPerRegion < 1 {
		nodesPerRegion = 1
	}

	for remainingNodes > 0 && regionIndex < len(regions) {
		region := regions[regionIndex]
		// 计算当前区域应该部署的节点数
		// 策略：每个区域只部署 1 个节点，避免配额不足
		// 这样可以最大化利用各区域的配额，提高成功率
		nodesToDeploy := 1

		log.Info("尝试在区域 %s 部署 %d 个节点 (剩余 %d 个节点)", region, nodesToDeploy, remainingNodes)

		// 复制变量并更新区域和节点数
		newVars := make(map[string]string)
		for k, v := range baseVars {
			newVars[k] = v
		}
		newVars["region"] = region
		newVars["node_count"] = strconv.Itoa(nodesToDeploy)

		// 重新初始化 Terraform
		if err := s.terraformSvc.Init(ctx, scenario.Path); err != nil {
			log.Warn("区域 %s 初始化失败，跳过: %v", region, err)
			regionIndex++
			continue
		}

		// 重新验证
		if err := s.terraformSvc.Validate(ctx, scenario.Path); err != nil {
			log.Warn("区域 %s 验证失败，跳过: %v", region, err)
			regionIndex++
			continue
		}

		// 执行 plan
		if err := s.terraformSvc.Plan(ctx, scenario.Path, newVars); err != nil {
			if strings.Contains(err.Error(), "LimitExceeded.SpotQuota") ||
				strings.Contains(err.Error(), "配额不足") {
				log.Warn("区域 %s 配额不足，尝试下一个区域", region)
				regionIndex++
				continue
			}
			log.Warn("区域 %s Plan 失败，跳过: %v", region, err)
			regionIndex++
			continue
		}

		// 执行 apply
		if err := s.terraformSvc.Apply(ctx, scenario.Path, autoApprove, newVars); err != nil {
			if strings.Contains(err.Error(), "LimitExceeded.SpotQuota") ||
				strings.Contains(err.Error(), "配额不足") {
				log.Warn("区域 %s 配额不足，尝试下一个区域", region)
				regionIndex++
				continue
			}
			// 其他错误，记录但继续尝试其他区域
			log.Warn("区域 %s Apply 失败，尝试下一个区域: %v", region, err)
			regionIndex++
			continue
		}

		// 成功部署
		log.Info("成功在区域 %s 部署 %d 个节点", region, nodesToDeploy)
		deployedRegions = append(deployedRegions, fmt.Sprintf("%s(%d)", region, nodesToDeploy))
		remainingNodes -= nodesToDeploy

		// 如果所有节点都已部署，退出循环
		if remainingNodes <= 0 {
			break
		}

		// 移动到下一个区域
		regionIndex++
	}

	if remainingNodes > 0 {
		return fmt.Errorf("跨区域部署未完成: 剩余 %d 个节点未部署。已部署区域: %v", remainingNodes, deployedRegions)
	}

	// 更新场景状态
	scenario.Status = "deployed"
	if err := s.projectRepo.UpdateScenario(projectName, scenario); err != nil {
		log.Error("更新场景状态失败: project=%s, scenario=%s, error=%v", projectName, scenarioID, err)
		return fmt.Errorf("更新场景状态失败: %w", err)
	}

	log.Info("跨区域部署成功: 总节点数=%d, 部署区域=%v", totalNodeCount, deployedRegions)
	return nil
}

// DestroyScenario 销毁场景
func (s *projectService) DestroyScenario(ctx context.Context, projectName, scenarioID string, autoApprove bool) error {
	log := logger.GetLogger()
	log.Info("开始销毁场景: project=%s, scenario=%s", projectName, scenarioID)

	// 获取场景信息
	scenario, err := s.projectRepo.GetScenario(projectName, scenarioID)
	if err != nil {
		log.Error("获取场景信息失败: project=%s, scenario=%s, error=%v", projectName, scenarioID, err)
		return err
	}

	// 构建可选的 Terraform 变量（用于传递云服务商凭据）
	vars := make(map[string]string)

	// 根据模板类型，从凭据管理器获取并传递云服务商凭据
	// 使用精确匹配，避免为不需要的模板传递变量
	templatePath := scenario.Template

	// 腾讯云模板：所有模板都需要 tencentcloud_secret_id 和 tencentcloud_secret_key
	if strings.HasPrefix(templatePath, "tencent/") {
		credManager := credentials.GetDefaultManager()
		if credManager.HasCredentials(credentials.ProviderTencent) {
			creds, err := credManager.GetCredentials(credentials.ProviderTencent)
			if err == nil && creds != nil {
				vars["tencentcloud_secret_id"] = creds.AccessKey
				vars["tencentcloud_secret_key"] = creds.SecretKey
				log.Debug("已传递腾讯云凭据到 Terraform destroy")
			}
		}
	}

	// 阿里云模板：根据模板类型传递不同的凭据
	if strings.HasPrefix(templatePath, "aliyun/") {
		credManager := credentials.GetDefaultManager()
		if credManager.HasCredentials(credentials.ProviderAliyun) {
			creds, err := credManager.GetCredentials(credentials.ProviderAliyun)
			if err == nil && creds != nil {
				// aliyun-proxy 模板需要 access_key 和 secret_key
				if strings.Contains(templatePath, "aliyun-proxy") {
					vars["access_key"] = creds.AccessKey
					vars["secret_key"] = creds.SecretKey
					log.Debug("已传递阿里云凭据到 Terraform destroy")
				}
			}
		}
		// task-executor-spot 在销毁时也需要 OSS 凭据（如果有的话）
		if strings.Contains(templatePath, "task-executor-spot") {
			credManager := credentials.GetDefaultManager()
			if credManager.HasCredentials(credentials.ProviderAliyun) {
				creds, err := credManager.GetCredentials(credentials.ProviderAliyun)
				if err == nil && creds != nil {
					vars["oss_access_key_id"] = creds.AccessKey
					vars["oss_access_key_secret"] = creds.SecretKey
					log.Debug("已传递 OSS 凭据到 Terraform destroy")
				}
			}
		}
	}

	// 华为云模板：所有模板都需要 access_key 和 secret_key
	if strings.HasPrefix(templatePath, "huaweicloud/") {
		credManager := credentials.GetDefaultManager()
		if credManager.HasCredentials(credentials.ProviderHuaweicloud) {
			creds, err := credManager.GetCredentials(credentials.ProviderHuaweicloud)
			if err == nil && creds != nil {
				vars["access_key"] = creds.AccessKey
				vars["secret_key"] = creds.SecretKey
				log.Debug("已传递华为云凭据到 Terraform destroy")
			}
		}
	}

	// 执行 destroy
	if err := s.terraformSvc.Destroy(ctx, scenario.Path, autoApprove, vars); err != nil {
		log.Error("Terraform destroy 失败: project=%s, scenario=%s, error=%v", projectName, scenarioID, err)
		return fmt.Errorf("Terraform destroy 失败: %w", err)
	}

	// 更新场景状态
	scenario.Status = "destroyed"
	if err := s.projectRepo.UpdateScenario(projectName, scenario); err != nil {
		log.Error("更新场景状态失败: project=%s, scenario=%s, error=%v", projectName, scenarioID, err)
		return fmt.Errorf("更新场景状态失败: %w", err)
	}

	log.Info("场景销毁成功: project=%s, scenario=%s", projectName, scenarioID)
	return nil
}

// GetProjectStatus 获取项目的云资源状态列表（云资源验证）
// 会遍历项目下的所有场景，调用 Terraform state list 获取云资源列表
func (s *projectService) GetProjectStatus(ctx context.Context, projectName string) ([]ScenarioStatus, error) {
	// 确认项目存在
	if _, err := s.projectRepo.GetProject(projectName); err != nil {
		return nil, fmt.Errorf("项目不存在: %w", err)
	}

	// 获取项目所有场景
	scenarios, err := s.projectRepo.ListScenarios(projectName)
	if err != nil {
		return nil, fmt.Errorf("获取场景列表失败: %w", err)
	}

	var result []ScenarioStatus

	for _, sc := range scenarios {
		var resources []string
		var instances []ECSInstanceDetail

		// 调用 Terraform state list 获取云端资源列表
		// 如果 state 不存在或命令失败，不视为致命错误，只记录为空
		if sc.Path != "" {
			if rs, err := s.terraformSvc.StateList(ctx, sc.Path); err == nil {
				resources = rs
			}
			if inst, err := s.terraformSvc.ShowInstances(ctx, sc.Path); err == nil {
				instances = inst
			}
		}

		result = append(result, ScenarioStatus{
			Scenario:  sc,
			Resources: resources,
			Instances: instances,
		})
	}

	return result, nil
}

// GetScenarioStatus 获取指定场景的云资源状态（云资源验证）
func (s *projectService) GetScenarioStatus(ctx context.Context, projectName, scenarioID string) (*ScenarioStatus, error) {
	// 获取场景信息
	scenario, err := s.projectRepo.GetScenario(projectName, scenarioID)
	if err != nil {
		return nil, fmt.Errorf("场景不存在: %w", err)
	}

	var resources []string
	var instances []ECSInstanceDetail

	// 调用 Terraform state list 获取云端资源列表
	// 如果 state 不存在或命令失败，不视为致命错误，只记录为空
	if scenario.Path != "" {
		if rs, err := s.terraformSvc.StateList(ctx, scenario.Path); err == nil {
			resources = rs
		}
		if inst, err := s.terraformSvc.ShowInstances(ctx, scenario.Path); err == nil {
			instances = inst
		}
	}

	return &ScenarioStatus{
		Scenario:  scenario,
		Resources: resources,
		Instances: instances,
	}, nil
}

// InitProject 初始化项目（预先执行所有场景的 Terraform 初始化）
// 会对项目下所有场景执行 terraform init，用于提前完成 provider 下载等初始化工作
func (s *projectService) InitProject(ctx context.Context, name string) error {
	// 确认项目存在
	project, err := s.projectRepo.GetProject(name)
	if err != nil {
		return fmt.Errorf("项目不存在: %w", err)
	}

	// 获取项目所有场景
	scenarios, err := s.projectRepo.ListScenarios(name)
	if err != nil {
		return fmt.Errorf("获取场景列表失败: %w", err)
	}

	if len(scenarios) == 0 {
		// 没有场景时直接返回
		return nil
	}

	for _, sc := range scenarios {
		if sc.Path == "" {
			continue
		}

		fmt.Printf("正在初始化场景 %s (项目: %s, 模板: %s)...\n", sc.ID, project.Name, sc.Template)

		// 在每个场景目录执行 terraform init
		if err := s.terraformSvc.Init(ctx, sc.Path); err != nil {
			return fmt.Errorf("初始化场景 %s (项目 %s) 失败: %w", sc.ID, project.Name, err)
		}
	}

	return nil
}
