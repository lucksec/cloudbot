package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/lucksec/cloudbot/internal/config"
	"github.com/lucksec/cloudbot/internal/credentials"
	"github.com/lucksec/cloudbot/internal/domain"
	"github.com/lucksec/cloudbot/internal/logger"
	"github.com/lucksec/cloudbot/internal/repository"
	"github.com/lucksec/cloudbot/internal/service"
	"github.com/spf13/cobra"
)

var (
	cfg *config.Config
)

func main() {
	// 加载配置
	var err error
	cfg, err = config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志系统
	logConfig := &logger.Config{
		Level:         logger.ParseLevel(cfg.Log.Level),
		EnableConsole: cfg.Log.EnableConsole,
		EnableFile:    cfg.Log.EnableFile,
		LogDir:        cfg.Log.LogDir,
		LogFile:       cfg.Log.LogFile,
	}

	log, err := logger.InitLogger(logConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化日志系统失败: %v\n", err)
		os.Exit(1)
	}

	log.Info("meta-matrix 启动")
	log.Debug("配置加载成功: WorkDir=%s, TemplateDir=%s, ProjectDir=%s",
		cfg.WorkDir, cfg.TemplateDir, cfg.ProjectDir)

	// 初始化服务
	projectRepo := repository.NewProjectRepository(cfg)
	templateRepo := repository.NewTemplateRepository(cfg)
	priceRepo := repository.NewPriceRepository(cfg)
	terraformSvc := service.NewTerraformService(cfg)
	projectSvc := service.NewProjectService(projectRepo, templateRepo, terraformSvc)

	// 创建动态价格查询器并注入到价格仓库
	priceFetcher := service.NewTerraformPriceFetcher(cfg, templateRepo, terraformSvc)
	if priceRepoWithFetcher, ok := priceRepo.(interface{ SetPriceFetcher(repository.PriceFetcher) }); ok {
		priceRepoWithFetcher.SetPriceFetcher(priceFetcher)
	}

	priceSvc := service.NewPriceService(priceRepo)

	// 创建价格优化器（从凭据管理器获取 AccessKey）
	credManager := credentials.GetDefaultManager()
	var priceOptimizer service.AliyunPriceOptimizer
	if credManager.HasCredentials(credentials.ProviderAliyun) {
		aliyunCreds, err := credManager.GetCredentials(credentials.ProviderAliyun)
		if err == nil && aliyunCreds != nil {
			priceOptimizer = service.NewAliyunPriceOptimizer(cfg, aliyunCreds.AccessKey, aliyunCreds.SecretKey)
		}
	}

	// 创建价格优化服务（即使没有 AccessKey 也创建，内部会处理错误）
	priceOptimizerSvc := service.NewPriceOptimizerService(cfg, priceOptimizer)

	// 创建根命令
	rootCmd := &cobra.Command{
		Use:   "meta-matrix",
		Short: "meta-matrix 是一个基于 IaC 的云资源编排工具",
		Long: `meta-matrix 是一个基于 Infrastructure as Code (IaC) 理念开发的云资源编排工具。
通过 Terraform 模板，可以一键部署不同云服务商、不同地区的云资源。`,
	}

	// 添加项目命令组
	projectCmd := &cobra.Command{
		Use:   "project",
		Short: "项目管理命令",
	}
	projectCmd.AddCommand(createProjectCmd(projectSvc))
	projectCmd.AddCommand(listProjectsCmd(projectSvc))
	projectCmd.AddCommand(deleteProjectCmd(projectSvc))
	projectCmd.AddCommand(initProjectCmd(projectSvc))
	rootCmd.AddCommand(projectCmd)

	// 添加场景命令组
	scenarioCmd := &cobra.Command{
		Use:   "scenario",
		Short: "场景管理命令",
	}
	scenarioCmd.AddCommand(createScenarioCmd(projectSvc, priceSvc, priceOptimizerSvc))
	scenarioCmd.AddCommand(createDynamicScenarioCmd(projectSvc, priceSvc, priceOptimizerSvc))
	scenarioCmd.AddCommand(listScenariosCmd(projectSvc))
	scenarioCmd.AddCommand(deployScenarioCmd(projectSvc))
	scenarioCmd.AddCommand(destroyScenarioCmd(projectSvc))
	scenarioCmd.AddCommand(statusScenariosCmd(projectSvc))
	rootCmd.AddCommand(scenarioCmd)

	// 添加模板命令组（模板管理相关）
	templateCmd := &cobra.Command{
		Use:   "template",
		Short: "模板管理命令",
	}
	templateCmd.AddCommand(listTemplatesCmd(templateRepo))
	rootCmd.AddCommand(templateCmd)

	// 添加价格命令组（价格比对和优化）
	priceCmd := &cobra.Command{
		Use:   "price",
		Short: "价格比对和优化命令",
	}
	priceCmd.AddCommand(comparePriceCmd(priceSvc))
	priceCmd.AddCommand(listPriceCmd(priceSvc))
	// 添加最优配置查找命令
	priceCmd.AddCommand(findOptimalCmd(priceOptimizerSvc))
	priceCmd.AddCommand(listRegionPricesCmd(priceOptimizerSvc))
	rootCmd.AddCommand(priceCmd)

	// 添加交互式控制台命令
	rootCmd.AddCommand(newConsoleCmd(projectSvc, templateRepo))

	// 添加凭据管理命令组
	rootCmd.AddCommand(credentialCmd())

	// 设置自动补全
	setupCompletion(rootCmd)

	// 设置动态补全
	setupDynamicCompletion(rootCmd, projectSvc, templateRepo)

	// 执行命令
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "执行命令失败: %v\n", err)
		os.Exit(1)
	}
}

// createProjectCmd 创建项目命令
func createProjectCmd(projectSvc service.ProjectService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "创建新项目",
		Long:  "创建一个新的项目。项目名称只能包含字母、数字、连字符和下划线。",
		Example: `  # 创建名为 my-project 的项目
  meta-matrix project create my-project`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			project, err := projectSvc.CreateProject(context.Background(), name)
			if err != nil {
				return err
			}
			fmt.Printf("项目 %s 创建成功\n", project.Name)
			fmt.Printf("路径: %s\n", project.Path)
			return nil
		},
	}
	return cmd
}

// listProjectsCmd 列出项目命令
func listProjectsCmd(projectSvc service.ProjectService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "列出所有项目",
		RunE: func(cmd *cobra.Command, args []string) error {
			projects, err := projectSvc.ListProjects(context.Background())
			if err != nil {
				return err
			}

			if len(projects) == 0 {
				fmt.Println("没有找到项目")
				return nil
			}

			fmt.Println("项目列表:")
			for _, project := range projects {
				fmt.Printf("  - %s (%d 个场景)\n", project.Name, len(project.Scenarios))
			}
			return nil
		},
	}
	return cmd
}

// deleteProjectCmd 删除项目命令
func deleteProjectCmd(projectSvc service.ProjectService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "删除项目",
		Long:  "删除指定的项目。注意：如果项目包含已部署的场景，需要先销毁场景才能删除项目。",
		Example: `  # 删除项目 my-project
  meta-matrix project delete my-project`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := projectSvc.DeleteProject(context.Background(), name); err != nil {
				return err
			}
			fmt.Printf("项目 %s 删除成功\n", name)
			return nil
		},
	}
	return cmd
}

// initProjectCmd 初始化项目命令
// 用于提前对项目下所有场景执行 Terraform 初始化 (terraform init)，
// 把 backend 初始化和 provider 插件下载在项目级别统一完成，避免首次部署时等待较久。
func initProjectCmd(projectSvc service.ProjectService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init <name>",
		Short: "初始化项目（预先执行所有场景的 Terraform 初始化）",
		Long: `初始化指定项目下的所有场景，对每个场景目录执行 Terraform 初始化 (terraform init)。

通过预热的方式，将以下耗时步骤提前完成：
  - Initializing the backend...
  - Initializing provider plugins...
  - 下载 aliyun/alicloud、hashicorp/random 等 provider 插件

这样后续执行场景部署 (scenario deploy) 时，就不需要再次长时间等待初始化步骤。`,
		Example: `  # 初始化项目 my-project（预加载所有场景的 provider）
  meta-matrix project init my-project`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			fmt.Printf("开始初始化项目 %s 下的所有场景...\n", name)
			if err := projectSvc.InitProject(context.Background(), name); err != nil {
				return err
			}

			fmt.Println("项目初始化完成。后续部署将跳过 provider 初始化的等待时间。")
			return nil
		},
	}
	return cmd
}

// createScenarioCmd 创建场景命令
func createScenarioCmd(projectSvc service.ProjectService, priceSvc service.PriceService, priceOptimizerSvc service.PriceOptimizerService) *cobra.Command {
	var useOptimal bool
	cmd := &cobra.Command{
		Use:   "create <project> <provider> <template> [region]",
		Short: "从模板创建场景",
		Long: `从模板库复制模板到项目中创建新场景。

参数说明:
  project   项目名称
  provider  云服务商 (aliyun, tencent, aws, vultr)
  template  模板名称

使用 --optimal 标志可以自动查找并应用最低价格的区域和实例类型配置（仅支持阿里云）。
需要配置环境变量 ALICLOUD_ACCESS_KEY 和 ALICLOUD_SECRET_KEY。

示例:
  # 使用阿里云 ECS 模板创建场景
  meta-matrix scenario create my-project aliyun ecs
  
  # 使用价格优化自动选择最低价格配置
  meta-matrix scenario create my-project aliyun ecs --optimal
  
  # 使用腾讯云文件服务器模板创建场景
  meta-matrix scenario create my-project tencent file`,
		Example: `  # 创建阿里云 ECS 场景
  meta-matrix scenario create my-project aliyun ecs
  
  # 创建场景并自动应用最优价格配置
  meta-matrix scenario create my-project aliyun ecs --optimal
  
  # 创建 aliyun-proxy 场景（指定区域）
  meta-matrix scenario create my-project aliyun aliyun-proxy bj
  meta-matrix scenario create my-project aliyun aliyun-proxy sh
  
  # 创建腾讯云文件服务器场景
  meta-matrix scenario create my-project tencent file`,
		Args: cobra.RangeArgs(3, 4),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectName := args[0]
			provider := args[1]
			templateName := args[2]
			region := ""
			if len(args) >= 4 {
				region = args[3]
			}

			// 如果启用了价格优化，查找最优配置
			var optimalConfig *service.OptimalInstanceConfig
			if useOptimal && provider == "aliyun" && priceOptimizerSvc != nil {
				fmt.Println("正在查找最优价格配置...")
				optimal, err := priceOptimizerSvc.FindOptimalConfig(context.Background(), provider, templateName, nil, nil)
				if err == nil && optimal != nil {
					optimalConfig = optimal
					fmt.Printf("✨ 找到最优配置:\n")
					fmt.Printf("  区域: %s\n", optimal.Region)
					fmt.Printf("  实例类型: %s\n", optimal.InstanceType)
					fmt.Printf("  价格: %.4f CNY/小时 (%.2f CNY/月)\n", optimal.Price, optimal.PricePerMonth)
				} else {
					fmt.Printf("⚠️  价格优化查询失败: %v，将使用默认配置\n", err)
				}
			}

			scenario, err := projectSvc.CreateScenario(context.Background(), projectName, provider, templateName, region)
			if err != nil {
				return err
			}

			fmt.Printf("\n场景创建成功\n")
			fmt.Printf("ID: %s\n", scenario.ID)
			fmt.Printf("名称: %s\n", scenario.Name)
			fmt.Printf("模板: %s\n", scenario.Template)
			if region != "" {
				fmt.Printf("区域: %s\n", region)
			}
			fmt.Printf("路径: %s\n", scenario.Path)

			// 如果找到了最优配置，自动写入 terraform.tfvars
			if optimalConfig != nil {
				tfvarsPath := fmt.Sprintf("%s/terraform.tfvars", scenario.Path)
				tfvarsContent := fmt.Sprintf("# 自动生成的最优价格配置\n")
				tfvarsContent += fmt.Sprintf("region = \"%s\"\n", optimalConfig.Region)
				tfvarsContent += fmt.Sprintf("instance_type = \"%s\"\n", optimalConfig.InstanceType)
				tfvarsContent += fmt.Sprintf("# 价格: %.4f CNY/小时 (%.2f CNY/月)\n",
					optimalConfig.Price, optimalConfig.PricePerMonth)

				if err := os.WriteFile(tfvarsPath, []byte(tfvarsContent), 0644); err == nil {
					fmt.Printf("\n✨ 已自动应用最优价格配置到 %s\n", tfvarsPath)
					fmt.Printf("  区域: %s\n", optimalConfig.Region)
					fmt.Printf("  实例类型: %s\n", optimalConfig.InstanceType)
					fmt.Printf("  价格: %.4f CNY/小时 (%.2f CNY/月)\n",
						optimalConfig.Price, optimalConfig.PricePerMonth)
				} else {
					fmt.Printf("\n💡 价格优化建议（需要手动应用）:\n")
					fmt.Printf("  编辑 %s/terraform.tfvars 文件添加:\n", scenario.Path)
					fmt.Printf("  region = \"%s\"\n", optimalConfig.Region)
					fmt.Printf("  instance_type = \"%s\"\n", optimalConfig.InstanceType)
				}
			}

			// 显示价格信息和建议
			price, err := priceSvc.GetPrice(context.Background(), provider, templateName)
			if err == nil {
				monthPriceCNY := price.PricePerMonth
				if price.Currency == "USD" {
					monthPriceCNY = price.PricePerMonth * 7.2 // 简化汇率
				}
				fmt.Printf("\n💰 价格信息:\n")
				fmt.Printf("  当前方案: %.2f %s/月 (%.4f %s/小时) ≈ %.2f CNY/月\n",
					price.PricePerMonth, price.Currency,
					price.PricePerHour, price.Currency,
					monthPriceCNY)
				fmt.Printf("  规格: %s\n", price.Spec)

				// 尝试获取同类型的最优方案建议
				templateType := getTemplateType(templateName)
				if templateType != "" {
					bestOption, err := priceSvc.GetBestOption(context.Background(), templateType)
					if err == nil && bestOption != nil {
						bestMonthPriceCNY := bestOption.PricePerMonth
						if bestOption.Currency == "USD" {
							bestMonthPriceCNY = bestOption.PricePerMonth * 7.2
						}
						if bestOption.Provider != provider || bestOption.Template != templateName {
							fmt.Printf("\n💡 价格优化建议:\n")
							fmt.Printf("  最优方案: %s/%s (%s)\n", bestOption.Provider, bestOption.Template, bestOption.Spec)
							fmt.Printf("  价格: %.2f %s/月 ≈ %.2f CNY/月\n",
								bestOption.PricePerMonth, bestOption.Currency, bestMonthPriceCNY)
							if monthPriceCNY > bestMonthPriceCNY {
								saving := monthPriceCNY - bestMonthPriceCNY
								fmt.Printf("  可节省: %.2f CNY/月 (%.1f%%)\n",
									saving, (saving/monthPriceCNY)*100)
							}
							fmt.Printf("  使用命令查看详细比对: meta-matrix price compare %s\n", templateType)
						}
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&useOptimal, "optimal", "o", false, "自动查找并应用最低价格配置（仅支持阿里云，需要配置 ALICLOUD_ACCESS_KEY 和 ALICLOUD_SECRET_KEY）")
	return cmd
}

// createDynamicScenarioCmd 创建动态场景命令
func createDynamicScenarioCmd(projectSvc service.ProjectService, priceSvc service.PriceService, priceOptimizerSvc service.PriceOptimizerService) *cobra.Command {
	var instanceType string
	var nodeCount int
	var useOptimal bool

	cmd := &cobra.Command{
		Use:   "create-dynamic <project> <provider> <scenario-type> [region]",
		Short: "动态生成并创建场景",
		Long: `通过云服务商API动态获取可用区域和实例类型，动态生成Terraform模板并创建场景。

参数说明:
  project        项目名称
  provider       云服务商 (aliyun, tencent, aws, huaweicloud)
  scenario-type  场景类型 (proxy, task-executor)
  region         区域（可选，不指定则自动选择最优区域）

支持的场景类型:
  - proxy: 代理服务器场景（Shadowsocks代理）
  - task-executor: 工具执行场景（从OSS下载并执行工具）

需要配置云服务商凭据，使用 credential set 命令或环境变量。

示例:
  # 动态创建阿里云代理场景（自动选择区域和实例类型）
  meta-matrix scenario create-dynamic my-project aliyun proxy
  
  # 动态创建阿里云代理场景（指定区域）
  meta-matrix scenario create-dynamic my-project aliyun proxy cn-beijing
  
  # 动态创建工具执行场景
  meta-matrix scenario create-dynamic my-project aliyun task-executor cn-shanghai
  
  # 使用最优价格配置
  meta-matrix scenario create-dynamic my-project aliyun proxy --optimal
  
  # 指定实例类型和节点数
  meta-matrix scenario create-dynamic my-project aliyun proxy cn-beijing --instance-type ecs.t6-c1m1.small --node-count 5`,
		Example: `  # 动态创建代理场景
  meta-matrix scenario create-dynamic my-project aliyun proxy
  
  # 动态创建工具执行场景
  meta-matrix scenario create-dynamic my-project aliyun task-executor cn-shanghai
  
  # 使用最优价格配置
  meta-matrix scenario create-dynamic my-project aliyun proxy --optimal`,
		Args: cobra.RangeArgs(3, 4),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectName := args[0]
			provider := args[1]
			scenarioType := args[2]
			region := ""
			if len(args) >= 4 {
				region = args[3]
			}

			// 验证场景类型
			if scenarioType != "proxy" && scenarioType != "task-executor" {
				return fmt.Errorf("无效的场景类型: %s，支持的类型: proxy, task-executor", scenarioType)
			}

			// 获取动态模板服务
			credManager := credentials.GetDefaultManager()
			if !credManager.HasCredentials(credentials.Provider(provider)) {
				return fmt.Errorf("未配置 %s 的凭据，请先运行: meta-matrix credential set %s", provider, provider)
			}

			dynamicTemplateSvc := service.NewDynamicTemplateService(credManager)

			// 如果启用了价格优化，查找最优配置
			var selectedRegion string
			var selectedInstanceType string

			if useOptimal && provider == "aliyun" && priceOptimizerSvc != nil {
				fmt.Println("正在查找最优价格配置...")
				optimal, err := priceOptimizerSvc.FindOptimalConfig(context.Background(), provider, scenarioType, nil, nil)
				if err == nil && optimal != nil {
					selectedRegion = optimal.Region
					selectedInstanceType = optimal.InstanceType
					fmt.Printf("✨ 找到最优配置:\n")
					fmt.Printf("  区域: %s\n", optimal.Region)
					fmt.Printf("  实例类型: %s\n", optimal.InstanceType)
					fmt.Printf("  价格: %.4f CNY/小时 (%.2f CNY/月)\n", optimal.Price, optimal.PricePerMonth)
				} else {
					fmt.Printf("⚠️  价格优化查询失败: %v，将使用默认配置\n", err)
				}
			}

			// 如果未指定区域，尝试获取可用区域
			if region == "" && selectedRegion == "" {
				fmt.Println("正在获取可用区域...")
				regions, err := dynamicTemplateSvc.GetAvailableRegions(context.Background(), provider)
				if err == nil && len(regions) > 0 {
					// 选择第一个可用区域
					selectedRegion = regions[0].ID
					fmt.Printf("自动选择区域: %s\n", regions[0].DisplayName)
				} else {
					// 使用默认区域
					if provider == "aliyun" {
						selectedRegion = "cn-beijing"
					} else if provider == "tencent" {
						selectedRegion = "ap-shanghai"
					}
					fmt.Printf("使用默认区域: %s\n", selectedRegion)
				}
			} else if region != "" {
				selectedRegion = region
			}

			// 如果未指定实例类型，尝试获取可用实例类型
			if instanceType == "" && selectedInstanceType == "" {
				fmt.Println("正在获取可用实例类型...")
				instanceTypes, err := dynamicTemplateSvc.GetAvailableInstanceTypes(context.Background(), provider, selectedRegion)
				if err == nil && len(instanceTypes) > 0 {
					// 选择第一个可用实例类型
					selectedInstanceType = instanceTypes[0].ID
					fmt.Printf("自动选择实例类型: %s\n", instanceTypes[0].ID)
				} else {
					// 使用默认实例类型
					if provider == "aliyun" {
						selectedInstanceType = "ecs.t6-c1m1.small"
					} else if provider == "tencent" {
						selectedInstanceType = "S5.SMALL1"
					}
					fmt.Printf("使用默认实例类型: %s\n", selectedInstanceType)
				}
			} else if instanceType != "" {
				selectedInstanceType = instanceType
			}

			// 构建选项
			options := make(map[string]interface{})
			if nodeCount > 0 {
				options["node_count"] = nodeCount
			}

			// 创建场景（使用动态模板）
			// 需要将 ProjectService 转换为支持 CreateScenarioWithOptions 的类型
			// 这里我们直接调用动态模板服务生成模板，然后创建场景
			fmt.Printf("\n正在生成动态模板...\n")
			fmt.Printf("  场景类型: %s\n", scenarioType)
			fmt.Printf("  云服务商: %s\n", provider)
			fmt.Printf("  区域: %s\n", selectedRegion)
			fmt.Printf("  实例类型: %s\n", selectedInstanceType)

			// 使用 CreateScenarioWithOptions 创建场景
			scenario, err := projectSvc.CreateScenarioWithOptions(context.Background(), projectName, provider, "", selectedRegion, selectedInstanceType, scenarioType, options)
			if err != nil {
				return fmt.Errorf("创建场景失败: %w", err)
			}

			fmt.Printf("\n✨ 动态场景创建成功\n")
			fmt.Printf("ID: %s\n", scenario.ID)
			fmt.Printf("名称: %s\n", scenario.Name)
			fmt.Printf("模板: %s (动态生成)\n", scenario.Template)
			fmt.Printf("区域: %s\n", selectedRegion)
			fmt.Printf("实例类型: %s\n", selectedInstanceType)
			fmt.Printf("路径: %s\n", scenario.Path)
			fmt.Printf("\n提示: 模板已动态生成，可以直接使用 terraform init 和 terraform apply 部署\n")

			return nil
		},
	}

	cmd.Flags().StringVar(&instanceType, "instance-type", "", "指定实例类型")
	cmd.Flags().IntVar(&nodeCount, "node-count", 0, "节点数量（仅对proxy场景有效）")
	cmd.Flags().BoolVarP(&useOptimal, "optimal", "o", false, "自动查找并应用最低价格配置（仅支持阿里云）")
	return cmd
}

// listScenariosCmd 列出场景命令
func listScenariosCmd(projectSvc service.ProjectService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <project>",
		Short: "列出项目的所有场景",
		Long:  "列出指定项目的所有场景，包括场景ID、状态和模板信息。",
		Example: `  # 列出项目 my-project 的所有场景
  meta-matrix scenario list my-project`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectName := args[0]
			scenarios, err := projectSvc.ListScenarios(context.Background(), projectName)
			if err != nil {
				return err
			}

			if len(scenarios) == 0 {
				fmt.Printf("项目 %s 没有场景\n", projectName)
				return nil
			}

			fmt.Printf("项目 %s 的场景列表:\n", projectName)
			for _, scenario := range scenarios {
				fmt.Printf("  - %s [%s] - %s\n", scenario.ID, scenario.Status, scenario.Template)
			}
			return nil
		},
	}
	return cmd
}

// statusScenariosCmd 获取项目云资源状态命令
// 用于进行云资源验证，查看每个场景在云端实际创建的资源列表
func statusScenariosCmd(projectSvc service.ProjectService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status <project> [scenario-id]",
		Short: "查看项目或指定场景的云资源状态",
		Long: `查看指定项目下所有场景或指定场景的云资源状态（云资源验证）。

通过读取 Terraform state，获取每个场景在云端实际创建的资源列表，用于判断场景是否真正启动成功。

输出信息包括:
  - 场景 ID
  - 场景状态（pending/deployed/destroyed）
  - 使用的模板
  - Terraform state 中资源数量
  - 资源名称列表（可用于排查问题）
  - 实例详细信息（ECS/EC2 等）`,
		Example: `  # 查看项目 my-project 的云资源状态
  meta-matrix scenario status my-project
  
  # 查看指定场景的云资源状态
  meta-matrix scenario status my-project <scenario-id>`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectName := args[0]

			// 如果提供了场景ID，只查询指定场景
			if len(args) == 2 {
				scenarioID := args[1]
				st, err := projectSvc.GetScenarioStatus(context.Background(), projectName, scenarioID)
				if err != nil {
					return err
				}

				printScenarioStatus(st)
				return nil
			}

			// 否则查询项目下所有场景
			statusList, err := projectSvc.GetProjectStatus(context.Background(), projectName)
			if err != nil {
				return err
			}

			if len(statusList) == 0 {
				fmt.Printf("项目 %s 暂无场景。\n", projectName)
				return nil
			}

			fmt.Printf("项目 %s 的云资源状态:\n", projectName)
			for _, st := range statusList {
				printScenarioStatus(&st)
			}

			return nil
		},
	}
	return cmd
}

// printScenarioStatus 打印场景状态信息
func printScenarioStatus(st *service.ScenarioStatus) {
	sc := st.Scenario
	resCount := len(st.Resources)

	fmt.Printf("\n场景: %s\n", sc.ID)
	fmt.Printf("  状态: %s\n", sc.Status)
	fmt.Printf("  模板: %s\n", sc.Template)
	fmt.Printf("  云资源数量: %d\n", resCount)
	if resCount > 0 {
		fmt.Println("  资源列表:")
		for _, r := range st.Resources {
			fmt.Printf("    - %s\n", r)
		}
	} else {
		fmt.Println("  资源列表: (未在 Terraform 状态中发现资源，可能未部署或部署失败)")
	}

	// 显示实例详细信息（ECS/EC2 等）
	if len(st.Instances) > 0 {
		fmt.Println("  实例详情:")
		for _, ins := range st.Instances {
			fmt.Printf("    - %s\n", ins.Name)
			if ins.ID != "" {
				fmt.Printf("      ID: %s\n", ins.ID)
			}
			if ins.Region != "" {
				fmt.Printf("      区域: %s\n", ins.Region)
			}
			if ins.InstanceType != "" {
				fmt.Printf("      规格: %s\n", ins.InstanceType)
			}
			if ins.Status != "" {
				fmt.Printf("      状态: %s\n", ins.Status)
			}
			if len(ins.PublicIPs) > 0 {
				fmt.Printf("      公网 IP: %s\n", strings.Join(ins.PublicIPs, ", "))
			}
			if len(ins.PrivateIPs) > 0 {
				fmt.Printf("      私网 IP: %s\n", strings.Join(ins.PrivateIPs, ", "))
			}
		}
	}
}

// deployScenarioCmd 部署场景命令
func deployScenarioCmd(projectSvc service.ProjectService) *cobra.Command {
	var autoApprove bool
	var nodeCount int

	cmd := &cobra.Command{
		Use:   "deploy <project> <scenario-id> [node-count] [tool-name] [tool-args...]",
		Short: "部署场景",
		Long: `执行 Terraform apply 部署场景，实际创建云服务器。

部署过程:
  1. 初始化 Terraform (terraform init)
  2. 验证配置 (terraform validate)
  3. 预览变更 (terraform plan)
  4. 应用变更 (terraform apply)

参数说明:
  project      项目名称
  scenario-id  场景 ID
  node-count   节点数量（可选，仅对支持 node_count 的模板生效）
  tool-name    工具名称（可选，对应 OSS 中的程序路径，如 programs/gogo.sh）
  tool-args    工具参数（可选，空格分隔的参数列表）

部署前请确保:
  - 已配置云服务商凭据
  - 账户余额充足
  - 网络连接正常
  - 如需使用工具执行，需配置 OSS 相关变量

注意: 默认会自动批准（--auto-approve），如需交互式确认请使用 --interactive 标志。`,
		Example: `  # 自动部署（默认行为，跳过确认）
  meta-matrix scenario deploy my-project <scenario-id>
  
  # 交互式部署（会显示 plan 并询问确认）
  meta-matrix scenario deploy my-project <scenario-id> --interactive

  # 指定节点数量（覆盖模板中的 node_count）
  meta-matrix scenario deploy my-project <scenario-id> --node 10

  # 部署并执行工具（task-executor-spot 模板）
  meta-matrix scenario deploy my-project <scenario-id> 1 gogo -o -p - -i 10.1.79.254
  
  # 使用 --node 标志指定节点数量
  meta-matrix scenario deploy my-project <scenario-id> --node 5 gogo -o -p - -i 10.1.79.254
  
  # 指定区域（aliyun-proxy 模板）
  meta-matrix scenario deploy my-project <scenario-id> --region bj
  meta-matrix scenario deploy my-project <scenario-id> --region sh --node 10`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectName := args[0]
			scenarioID := args[1]

			// 解析参数：支持两种格式
			// 格式1: deploy <project> <scenario-id> [node-count] [tool-name] [tool-args...]
			// 格式2: deploy <project> <scenario-id> --node <count> [tool-name] [tool-args...]
			var parsedNodeCount int
			var toolName string
			var toolArgs []string

			argIdx := 2

			// 如果使用 --node 标志，优先使用标志值
			if nodeCount > 0 {
				parsedNodeCount = nodeCount
			} else if argIdx < len(args) {
				// 尝试解析第3个参数为节点数量
				if parsed, err := strconv.Atoi(args[argIdx]); err == nil && parsed > 0 {
					parsedNodeCount = parsed
					argIdx++
				}
			}

			// 解析工具名称和参数
			if argIdx < len(args) {
				toolName = args[argIdx]
				argIdx++
				if argIdx < len(args) {
					toolArgs = args[argIdx:]
				}
			}

			// 如果设置了 --interactive，覆盖 autoApprove 为 false
			interactive, _ := cmd.Flags().GetBool("interactive")
			if interactive {
				autoApprove = false
			}

			// 构建工具参数字符串
			toolArgsStr := strings.Join(toolArgs, " ")

			// 区域参数传空字符串，因为区域在创建场景时已确定
			if err := projectSvc.DeployScenario(context.Background(), projectName, scenarioID, autoApprove, parsedNodeCount, toolName, toolArgsStr, ""); err != nil {
				return err
			}

			fmt.Printf("场景 %s 部署成功", scenarioID)
			if parsedNodeCount > 0 {
				fmt.Printf("，节点数量: %d", parsedNodeCount)
			}
			if toolName != "" {
				fmt.Printf("，工具: %s", toolName)
				if toolArgsStr != "" {
					fmt.Printf("，参数: %s", toolArgsStr)
				}
			}
			fmt.Println()
			return nil
		},
	}

	cmd.Flags().BoolVarP(&autoApprove, "auto-approve", "y", true, "自动批准，跳过确认（默认启用）")
	cmd.Flags().BoolP("interactive", "i", false, "交互式模式，显示 plan 并询问确认（会覆盖 --auto-approve）")
	cmd.Flags().IntVarP(&nodeCount, "node", "n", 0, "指定节点数量（覆盖模板中的 node_count，0 表示使用默认/随机值）")
	return cmd
}

// destroyScenarioCmd 销毁场景命令
func destroyScenarioCmd(projectSvc service.ProjectService) *cobra.Command {
	var autoApprove bool

	cmd := &cobra.Command{
		Use:   "destroy <project> <scenario-id>",
		Short: "销毁场景",
		Long: `执行 Terraform destroy 销毁场景，删除所有已创建的资源。

警告: 此操作会删除所有已创建的云资源，包括:
  - 云服务器实例
  - VPC 和子网
  - 安全组
  - 其他相关资源

此操作不可逆，请谨慎操作。`,
		Example: `  # 交互式销毁（会询问确认）
  meta-matrix scenario destroy my-project <scenario-id>
  
  # 自动销毁（跳过确认）
  meta-matrix scenario destroy my-project <scenario-id> --auto-approve`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectName := args[0]
			scenarioID := args[1]

			if err := projectSvc.DestroyScenario(context.Background(), projectName, scenarioID, autoApprove); err != nil {
				return err
			}

			fmt.Printf("场景 %s 销毁成功\n", scenarioID)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&autoApprove, "auto-approve", "y", false, "自动批准，跳过确认")
	return cmd
}

// listTemplatesCmd 列出模板命令
func listTemplatesCmd(templateRepo repository.TemplateRepository) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "列出所有可用模板",
		RunE: func(cmd *cobra.Command, args []string) error {
			templates, err := templateRepo.ListTemplates()
			if err != nil {
				return err
			}

			if len(templates) == 0 {
				fmt.Println("没有找到模板")
				return nil
			}

			// 按云服务商分组显示
			providerMap := make(map[string][]*domain.Template)
			for _, template := range templates {
				providerMap[template.Provider] = append(providerMap[template.Provider], template)
			}

			fmt.Println("可用模板:")
			for provider, tmpls := range providerMap {
				fmt.Printf("\n%s:\n", provider)
				for _, tmpl := range tmpls {
					fmt.Printf("  - %s", tmpl.Name)
					if tmpl.Description != "" {
						fmt.Printf(": %s", tmpl.Description)
					}
					fmt.Println()
				}
			}
			return nil
		},
	}
	return cmd
}

// comparePriceCmd 价格比对命令
func comparePriceCmd(priceSvc service.PriceService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compare <template-type>",
		Short: "比对指定类型模板的价格",
		Long: `比对指定类型模板在不同云服务商之间的价格，找出最优方案。

支持的模板类型:
  - ecs: ECS 云服务器
  - proxy: 代理服务器
  - ec2: AWS EC2 实例
  - vps: VPS 服务器`,
		Example: `  # 比对 ECS 类型模板的价格
  meta-matrix price compare ecs
  
  # 比对代理服务器类型模板的价格
  meta-matrix price compare proxy`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			templateType := args[0]

			comparison, err := priceSvc.ComparePrices(context.Background(), templateType)
			if err != nil {
				return err
			}

			fmt.Printf("模板类型: %s\n", comparison.TemplateType)
			fmt.Printf("价格范围: %.2f - %.2f CNY/月 (%.4f - %.4f CNY/小时)\n",
				comparison.PriceRange.MinPerMonth,
				comparison.PriceRange.MaxPerMonth,
				comparison.PriceRange.MinPerHour,
				comparison.PriceRange.MaxPerHour)
			fmt.Println()

			if comparison.BestOption != nil {
				best := comparison.BestOption
				fmt.Printf("✨ 最优方案: %s/%s\n", best.Provider, best.Template)
				fmt.Printf("   规格: %s\n", best.Spec)
				fmt.Printf("   区域: %s\n", best.Region)
				fmt.Printf("   价格: %.2f %s/月 (%.4f %s/小时)\n",
					best.PricePerMonth, best.Currency,
					best.PricePerHour, best.Currency)
				fmt.Println()
			}

			fmt.Println("所有可选方案（按价格从低到高）:")
			for i, option := range comparison.Options {
				marker := "  "
				if i == 0 {
					marker = "⭐ "
				}
				fmt.Printf("%s%d. %s/%s (%s)\n", marker, i+1, option.Provider, option.Template, option.Spec)
				fmt.Printf("     价格: %.2f %s/月 (%.4f %s/小时)\n",
					option.PricePerMonth, option.Currency,
					option.PricePerHour, option.Currency)
				fmt.Printf("     区域: %s\n", option.Region)
				fmt.Println()
			}

			return nil
		},
	}
	return cmd
}

// listPriceCmd 列出所有价格信息命令
func listPriceCmd(priceSvc service.PriceService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "列出所有模板的价格信息",
		Long:  "列出所有已配置的模板价格信息，包括云服务商、模板名称、规格和价格。",
		Example: `  # 列出所有价格信息
  meta-matrix price list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			prices, err := priceSvc.ListPrices(context.Background())
			if err != nil {
				return err
			}

			if len(prices) == 0 {
				fmt.Println("没有找到价格信息")
				return nil
			}

			// 按云服务商分组显示
			providerMap := make(map[string][]*domain.PriceInfo)
			for _, price := range prices {
				providerMap[price.Provider] = append(providerMap[price.Provider], price)
			}

			fmt.Println("价格信息列表:")
			for provider, priceList := range providerMap {
				fmt.Printf("\n%s:\n", provider)
				for _, price := range priceList {
					monthPriceCNY := price.PricePerMonth
					if price.Currency == "USD" {
						monthPriceCNY = price.PricePerMonth * 7.2 // 简化汇率
					}
					fmt.Printf("  - %s (%s)\n", price.Template, price.Spec)
					fmt.Printf("    价格: %.2f %s/月 (%.4f %s/小时) ≈ %.2f CNY/月\n",
						price.PricePerMonth, price.Currency,
						price.PricePerHour, price.Currency,
						monthPriceCNY)
					fmt.Printf("    区域: %s\n", price.Region)
				}
			}

			return nil
		},
	}
	return cmd
}

// getTemplateType 根据模板名称推断模板类型
func getTemplateType(templateName string) string {
	// 简单的类型推断逻辑
	if contains(templateName, "ecs") {
		return "ecs"
	}
	if contains(templateName, "proxy") {
		return "proxy"
	}
	if contains(templateName, "ec2") {
		return "ec2"
	}
	if contains(templateName, "vps") {
		return "vps"
	}
	return ""
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// findOptimalCmd 查找最优价格配置命令
func findOptimalCmd(priceOptimizerSvc service.PriceOptimizerService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "optimal <provider> <template>",
		Short: "查找最低价格的区域和实例类型配置",
		Long: `通过调用云服务商 API 查询实时价格，找出最低价格的区域和实例类型配置。

支持的云服务商:
  - aliyun: 使用阿里云 DescribePrice API

需要配置环境变量:
  - ALICLOUD_ACCESS_KEY: 阿里云 AccessKey ID
  - ALICLOUD_SECRET_KEY: 阿里云 SecretKey`,
		Example: `  # 查找阿里云 ECS 的最优配置
  meta-matrix price optimal aliyun ecs
  
  # 查找指定实例类型的最优配置
  meta-matrix price optimal aliyun ecs --instance-types ecs.t5-lc1m1.small,ecs.t5-lc1m2.small`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider := args[0]
			template := args[1]

			instanceTypes, _ := cmd.Flags().GetStringSlice("instance-types")
			regions, _ := cmd.Flags().GetStringSlice("regions")

			optimal, err := priceOptimizerSvc.FindOptimalConfig(context.Background(), provider, template, instanceTypes, regions)
			if err != nil {
				return fmt.Errorf("查找最优配置失败: %w", err)
			}

			fmt.Printf("✨ 最优价格配置:\n\n")
			fmt.Printf("  云服务商: %s\n", provider)
			fmt.Printf("  模板: %s\n", template)
			fmt.Printf("  区域: %s\n", optimal.Region)
			fmt.Printf("  实例类型: %s\n", optimal.InstanceType)
			fmt.Printf("  价格: %.4f %s/小时\n", optimal.Price, optimal.Currency)
			fmt.Printf("  月价格: %.2f %s/月\n", optimal.PricePerMonth, optimal.Currency)
			fmt.Printf("\n使用方式:\n")
			fmt.Printf("  terraform apply -var=\"region=%s\" -var=\"instance_type=%s\"\n",
				optimal.Region, optimal.InstanceType)

			return nil
		},
	}

	cmd.Flags().StringSlice("instance-types", nil, "要比较的实例类型列表（逗号分隔）")
	cmd.Flags().StringSlice("regions", nil, "要比较的区域列表（逗号分隔）")
	return cmd
}

// listRegionPricesCmd 列出各区域价格并标注最低价
func listRegionPricesCmd(priceOptimizerSvc service.PriceOptimizerService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "regions <provider> <template>",
		Short: "获取各区域价格，标注最低价",
		Long: `通过云服务商 API 获取各区域实时价格，按价格排序并标注最低价。

支持的云服务商:
  - aliyun: 使用阿里云 DescribePrice API

需要环境变量:
  - ALICLOUD_ACCESS_KEY: 阿里云 AccessKey ID
  - ALICLOUD_SECRET_KEY: 阿里云 SecretKey`,
		Example: `  # 列出阿里云 ECS 在常用区域的价格
  meta-matrix price regions aliyun ecs

  # 指定实例类型和区域
  meta-matrix price regions aliyun ecs \
    --instance-types ecs.t5-lc1m1.small,ecs.t5-lc1m2.small \
    --regions cn-beijing,cn-shanghai,cn-hangzhou`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider := args[0]
			template := args[1]

			instanceTypes, _ := cmd.Flags().GetStringSlice("instance-types")
			regions, _ := cmd.Flags().GetStringSlice("regions")

			if priceOptimizerSvc == nil {
				return fmt.Errorf("价格优化器未初始化，请配置 ALICLOUD_ACCESS_KEY 和 ALICLOUD_SECRET_KEY")
			}

			prices, err := priceOptimizerSvc.ListRegionPrices(context.Background(), provider, template, instanceTypes, regions)
			if err != nil {
				return err
			}

			if len(prices) == 0 {
				fmt.Println("未找到价格信息")
				return nil
			}

			fmt.Printf("各区域价格（按小时计费，已按价格升序排序）：\n\n")
			for i, p := range prices {
				marker := "  "
				if i == 0 {
					marker = "⭐ " // 最低价标记
				}
				fmt.Printf("%s%s / %s\n", marker, p.Region, p.InstanceType)
				fmt.Printf("   价格: %.4f %s/小时 (%.2f %s/月)\n", p.PricePerHour, p.Currency, p.PricePerMonth, p.Currency)
				if i == 0 {
					fmt.Printf("   -> 最低价\n")
				}
				fmt.Println()
			}

			fmt.Printf("提示: 可在部署时使用 -var=\"region=<region>\" -var=\"instance_type=<type>\" 应用最低价配置。\n")
			return nil
		},
	}

	cmd.Flags().StringSlice("instance-types", nil, "要比较的实例类型列表（逗号分隔）")
	cmd.Flags().StringSlice("regions", nil, "要比较的区域列表（逗号分隔）")
	return cmd
}
