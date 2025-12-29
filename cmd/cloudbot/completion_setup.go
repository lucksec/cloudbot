package main

import (
	"github.com/lucksec/cloudbot/internal/repository"
	"github.com/lucksec/cloudbot/internal/service"
	"github.com/spf13/cobra"
)

// setupDynamicCompletion 设置动态补全
func setupDynamicCompletion(rootCmd *cobra.Command, projectSvc service.ProjectService, templateRepo repository.TemplateRepository) {
	// 项目相关命令的补全
	setupProjectCompletion(rootCmd, projectSvc)

	// 场景相关命令的补全
	setupScenarioCompletion(rootCmd, projectSvc, templateRepo)

	// 模板相关命令的补全
	setupTemplateCompletion(rootCmd, templateRepo)

	// 价格相关命令的补全
	setupPriceCompletion(rootCmd, templateRepo)
}

// setupProjectCompletion 设置项目命令的补全
func setupProjectCompletion(rootCmd *cobra.Command, projectSvc service.ProjectService) {
	// 查找 project delete 命令
	projectCmd := findCommand(rootCmd, "project")
	if projectCmd == nil {
		return
	}

	deleteCmd := findCommand(projectCmd, "delete")
	if deleteCmd != nil {
		deleteCmd.ValidArgsFunction = completeProjects(projectSvc)
	}
}

// setupPriceCompletion 设置价格命令的补全
func setupPriceCompletion(rootCmd *cobra.Command, templateRepo repository.TemplateRepository) {
	priceCmd := findCommand(rootCmd, "price")
	if priceCmd == nil {
		return
	}

	// price optimal <provider> <template>
	optimalCmd := findCommand(priceCmd, "optimal")
	if optimalCmd != nil {
		optimalCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				// 补全 provider
				return completeProviders(templateRepo)(cmd, args, toComplete)
			} else if len(args) == 1 {
				// 补全 template
				return completeTemplates(templateRepo)(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}

	// price regions <provider> <template>
	regionsCmd := findCommand(priceCmd, "regions")
	if regionsCmd != nil {
		regionsCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				// 补全 provider
				return completeProviders(templateRepo)(cmd, args, toComplete)
			} else if len(args) == 1 {
				// 补全 template
				return completeTemplates(templateRepo)(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}
}

// setupScenarioCompletion 设置场景命令的补全
func setupScenarioCompletion(rootCmd *cobra.Command, projectSvc service.ProjectService, templateRepo repository.TemplateRepository) {
	scenarioCmd := findCommand(rootCmd, "scenario")
	if scenarioCmd == nil {
		return
	}

	// scenario create 命令
	createCmd := findCommand(scenarioCmd, "create")
	if createCmd != nil {
		createCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				// 第一个参数：项目名
				return completeProjects(projectSvc)(cmd, args, toComplete)
			} else if len(args) == 1 {
				// 第二个参数：provider
				return completeProviders(templateRepo)(cmd, args, toComplete)
			} else if len(args) == 2 {
				// 第三个参数：template
				return completeTemplates(templateRepo)(cmd, args, toComplete)
			} else if len(args) == 3 {
				// 第四个参数：region（仅对 aliyun-proxy 模板）
				if args[1] == "aliyun" && args[2] == "aliyun-proxy" {
					return []string{"bj", "sh", "hhht", "wlcb", "zjk"}, cobra.ShellCompDirectiveNoFileComp
				}
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}

	// scenario list 命令
	listCmd := findCommand(scenarioCmd, "list")
	if listCmd != nil {
		listCmd.ValidArgsFunction = completeProjects(projectSvc)
	}

	// scenario deploy 命令
	deployCmd := findCommand(scenarioCmd, "deploy")
	if deployCmd != nil {
		// 第一个参数：项目名
		deployCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				// 补全项目名
				return completeProjects(projectSvc)(cmd, args, toComplete)
			} else if len(args) == 1 {
				// 补全场景ID
				return completeScenarios(projectSvc)(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}

	// scenario destroy 命令
	destroyCmd := findCommand(scenarioCmd, "destroy")
	if destroyCmd != nil {
		destroyCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				// 补全项目名
				return completeProjects(projectSvc)(cmd, args, toComplete)
			} else if len(args) == 1 {
				// 补全场景ID
				return completeScenarios(projectSvc)(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}
}

// setupTemplateCompletion 设置模板命令的补全
func setupTemplateCompletion(rootCmd *cobra.Command, templateRepo repository.TemplateRepository) {
	templateCmd := findCommand(rootCmd, "template")
	if templateCmd == nil {
		return
	}

	listCmd := findCommand(templateCmd, "list")
	if listCmd != nil {
		// template list 不需要参数补全
	}
}

// findCommand 查找命令
func findCommand(root *cobra.Command, name string) *cobra.Command {
	for _, cmd := range root.Commands() {
		if cmd.Name() == name {
			return cmd
		}
		// 递归查找子命令
		if found := findCommand(cmd, name); found != nil {
			return found
		}
	}
	return nil
}
