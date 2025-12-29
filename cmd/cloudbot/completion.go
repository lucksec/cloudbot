package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/meta-matrix/meta-matrix/internal/repository"
	"github.com/meta-matrix/meta-matrix/internal/service"
	"github.com/spf13/cobra"
)

// 动态补全函数

// completeProjects 补全项目名称列表
func completeProjects(projectSvc service.ProjectService) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		projects, err := projectSvc.ListProjects(context.Background())
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		var completions []string
		for _, project := range projects {
			if strings.HasPrefix(project.Name, toComplete) {
				completions = append(completions, project.Name)
			}
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	}
}

// completeScenarios 补全场景ID列表
func completeScenarios(projectSvc service.ProjectService) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		// 第一个参数是项目名
		if len(args) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		projectName := args[0]
		scenarios, err := projectSvc.ListScenarios(context.Background(), projectName)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		var completions []string
		for _, scenario := range scenarios {
			if strings.HasPrefix(scenario.ID, toComplete) {
				// 显示格式：ID [状态] - 模板
				completions = append(completions, fmt.Sprintf("%s\t[%s] %s", scenario.ID, scenario.Status, scenario.Template))
			}
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	}
}

// completeProviders 补全云服务商列表
func completeProviders(templateRepo repository.TemplateRepository) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		templates, err := templateRepo.ListTemplates()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		providers := make(map[string]bool)
		for _, template := range templates {
			if strings.HasPrefix(template.Provider, toComplete) {
				providers[template.Provider] = true
			}
		}

		var completions []string
		for provider := range providers {
			completions = append(completions, provider)
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	}
}

// completeTemplates 补全模板名称列表
func completeTemplates(templateRepo repository.TemplateRepository) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		// 第一个参数是 provider
		if len(args) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		provider := args[0]
		templates, err := templateRepo.ListTemplates()
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		var completions []string
		for _, template := range templates {
			if template.Provider == provider && strings.HasPrefix(template.Name, toComplete) {
				desc := template.Description
				if desc == "" {
					desc = "模板"
				}
				completions = append(completions, fmt.Sprintf("%s\t%s", template.Name, desc))
			}
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	}
}

// setupCompletion 设置自动补全命令
func setupCompletion(rootCmd *cobra.Command) {
	// 添加 completion 命令
	completionCmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "生成自动补全脚本",
		Long: `生成指定 shell 的自动补全脚本。

支持的 shell: bash, zsh, fish, powershell

安装方法:

Bash:
  $ source <(meta-matrix completion bash)
  
  # 或添加到 ~/.bashrc
  $ echo 'source <(meta-matrix completion bash)' >> ~/.bashrc

Zsh:
  $ source <(meta-matrix completion zsh)
  
  # 或添加到 ~/.zshrc
  $ echo 'source <(meta-matrix completion zsh)' >> ~/.zshrc

Fish:
  $ meta-matrix completion fish | source
  
  # 或添加到 ~/.config/fish/completions/meta-matrix.fish
  $ meta-matrix completion fish > ~/.config/fish/completions/meta-matrix.fish

PowerShell:
  $ meta-matrix completion powershell | Out-String | Invoke-Expression
  
  # 或添加到 PowerShell profile
  $ meta-matrix completion powershell >> $PROFILE
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.ExactValidArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			switch args[0] {
			case "bash":
				rootCmd.GenBashCompletion(os.Stdout)
			case "zsh":
				rootCmd.GenZshCompletion(os.Stdout)
			case "fish":
				rootCmd.GenFishCompletion(os.Stdout, true)
			case "powershell":
				rootCmd.GenPowerShellCompletion(os.Stdout)
			}
		},
	}

	rootCmd.AddCommand(completionCmd)
}
