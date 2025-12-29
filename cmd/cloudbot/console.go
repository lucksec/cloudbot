package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	prompt "github.com/c-bata/go-prompt"
	"github.com/lucksec/cloudbot/internal/credentials"
	"github.com/lucksec/cloudbot/internal/repository"
	"github.com/lucksec/cloudbot/internal/service"
	"github.com/spf13/cobra"
)

// console 表示交互式控制台结构体
// 使用 go-prompt 提供带 Tab 补全的 REPL（读取-执行-输出）循环
type console struct {
	projectSvc   service.ProjectService        // 项目服务
	templateRepo repository.TemplateRepository // 模板仓库
}

// newConsoleCmd 创建控制台命令
// 用户执行 `meta-matrix console` 即可进入交互式控制台
func newConsoleCmd(projectSvc service.ProjectService, templateRepo repository.TemplateRepository) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "console",
		Short: "进入交互式控制台",
		Long: `进入交互式控制台，对项目、场景、模板进行统一管理。

示例:
  meta-matrix console

进入控制台后，可使用命令:
  help                         显示帮助
  project list                 列出项目
  project create <name>        创建项目
  scenario list <project>      列出项目场景
  scenario create <project> <provider> <template>
                               创建场景
  scenario deploy <project> <scenario-id> [node-count] [tool-name] [tool-args...]
                               部署场景（支持节点数量和工具参数）
  scenario destroy <project> <scenario-id>
                               销毁场景
  template list                列出模板
  exit / quit                  退出控制台`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := &console{
				projectSvc:   projectSvc,
				templateRepo: templateRepo,
			}
			return c.run()
		},
	}

	return cmd
}

// run 启动交互式控制台主循环（带 Tab 补全）
func (c *console) run() error {
	c.printWelcome()

	// 使用 go-prompt 提供交互式输入和 Tab 补全
	p := prompt.New(
		c.executor,                           // 输入执行函数
		c.completer,                          // 补全函数
		prompt.OptionPrefix("meta-matrix> "), // 提示符
		prompt.OptionTitle("meta-matrix console"),           // 标题
		prompt.OptionSuggestionBGColor(prompt.DarkGray),     // 建议背景色
		prompt.OptionSuggestionTextColor(prompt.White),      // 建议文字颜色
		prompt.OptionSelectedSuggestionBGColor(prompt.Blue), // 选中建议背景色
		prompt.OptionSelectedSuggestionTextColor(prompt.White),
	)

	// Run 会阻塞，直到用户退出（Ctrl+D/Ctrl+C）
	p.Run()
	fmt.Println("\n已退出控制台。")
	return nil
}

// executor 执行单行命令
func (c *console) executor(in string) {
	line := strings.TrimSpace(in)
	if line == "" {
		return
	}
	if err := c.handleCommand(line); err != nil {
		fmt.Printf("错误: %v\n", err)
	}
}

// completer 提供 Tab 补全
func (c *console) completer(d prompt.Document) []prompt.Suggest {
	text := d.TextBeforeCursor()
	parts := strings.Fields(text)

	// 如果正在输入第一个单词（顶级命令）
	if len(parts) == 0 {
		return c.topLevelSuggestions("")
	}

	// 当前正在输入的 token
	current := ""
	if d.TextBeforeCursor() == "" || strings.HasSuffix(d.TextBeforeCursor(), " ") {
		// 刚输入了空格，当前 token 为空，下一个参数
		current = ""
	} else {
		current = parts[len(parts)-1]
	}

	// 检查是否在输入标志（以 - 开头）
	if strings.HasPrefix(current, "-") {
		// 根据前面的命令提供标志补全
		if len(parts) >= 2 {
			switch parts[0] {
			case "scenario":
				if len(parts) >= 3 && parts[1] == "deploy" {
					// scenario deploy 的标志
					flags := []prompt.Suggest{
						{Text: "--region", Description: "指定区域（aliyun-proxy: bj/sh/hhht/wlcb/zjk）"},
						{Text: "-r", Description: "指定区域（简写）"},
						{Text: "--node", Description: "指定节点数量"},
						{Text: "-n", Description: "指定节点数量（简写）"},
						{Text: "--interactive", Description: "交互式模式"},
						{Text: "-i", Description: "交互式模式（简写）"},
						{Text: "--auto-approve", Description: "自动批准（默认）"},
						{Text: "-y", Description: "自动批准（简写）"},
					}
					var res []prompt.Suggest
					for _, f := range flags {
						if strings.HasPrefix(f.Text, current) {
							res = append(res, f)
						}
					}
					return res
				}
			}
		}
	}

	switch parts[0] {
	case "project":
		return c.completeProject(parts[1:], current)
	case "scenario":
		return c.completeScenario(parts[1:], current)
	case "template":
		return c.completeTemplate(parts[1:], current)
	case "price":
		return c.completePrice(parts[1:], current)
	case "credential":
		return c.completeCredential(parts[1:], current)
	default:
		// 顶级命令补全
		if len(parts) == 1 {
			return c.topLevelSuggestions(current)
		}
	}

	return []prompt.Suggest{}
}

// topLevelSuggestions 顶级命令补全
func (c *console) topLevelSuggestions(current string) []prompt.Suggest {
	cmds := []prompt.Suggest{
		{Text: "help", Description: "显示帮助"},
		{Text: "project", Description: "项目管理命令"},
		{Text: "scenario", Description: "场景管理命令"},
		{Text: "template", Description: "模板管理命令"},
		{Text: "credential", Description: "凭据管理命令"},
		{Text: "exit", Description: "退出控制台"},
		{Text: "quit", Description: "退出控制台"},
	}
	var res []prompt.Suggest
	for _, s := range cmds {
		if strings.HasPrefix(s.Text, current) {
			res = append(res, s)
		}
	}
	return res
}

// completeProject project 子命令补全
func (c *console) completeProject(args []string, current string) []prompt.Suggest {
	if len(args) == 0 {
		// 补全 project 子命令
		subs := []prompt.Suggest{
			{Text: "list", Description: "列出所有项目"},
			{Text: "create", Description: "创建新项目"},
			{Text: "delete", Description: "删除项目"},
		}
		var res []prompt.Suggest
		for _, s := range subs {
			if strings.HasPrefix(s.Text, current) {
				res = append(res, s)
			}
		}
		return res
	}

	switch args[0] {
	case "list":
		// project list 无更多参数
		return []prompt.Suggest{}
	case "create":
		// project create <name>，不做名称补全
		return []prompt.Suggest{}
	case "delete":
		// project delete <name>，补全项目名
		if len(args) == 1 {
			return c.completeProjectNames(current)
		}
		return []prompt.Suggest{}
	default:
		return []prompt.Suggest{}
	}
}

// completeScenario scenario 子命令补全
func (c *console) completeScenario(args []string, current string) []prompt.Suggest {
	if len(args) == 0 {
		// 补全 scenario 子命令
		subs := []prompt.Suggest{
			{Text: "list", Description: "列出项目的所有场景"},
			{Text: "create", Description: "从模板创建场景"},
			{Text: "deploy", Description: "部署场景"},
			{Text: "destroy", Description: "销毁场景"},
			{Text: "status", Description: "查看项目所有场景的云资源状态"},
		}
		var res []prompt.Suggest
		for _, s := range subs {
			if strings.HasPrefix(s.Text, current) {
				res = append(res, s)
			}
		}
		return res
	}

	switch args[0] {
	case "list":
		// scenario list <project>
		if len(args) == 1 {
			return c.completeProjectNames(current)
		}
	case "create":
		// scenario create <project> <provider> <template>
		if len(args) == 1 {
			// 补全项目名
			return c.completeProjectNames(current)
		}
		if len(args) == 2 {
			// 补全 provider
			return c.completeProviders(current)
		}
		if len(args) == 3 {
			// 补全 template（基于 provider）
			// args[1] 是项目名，args[2] 是 provider
			return c.completeTemplates(args[2], current)
		}
	case "deploy":
		// scenario deploy <project> <scenario-id> [node-count] [tool-name] [tool-args...] [--node]
		if len(args) == 1 {
			// 补全项目名
			return c.completeProjectNames(current)
		}
		if len(args) == 2 {
			// 补全场景 ID
			return c.completeScenarioIDs(args[1], current)
		}
		// 检查是否在输入标志
		if strings.HasPrefix(current, "-") {
			// 补全标志
			flags := []prompt.Suggest{
				{Text: "--node", Description: "指定节点数量"},
				{Text: "-n", Description: "指定节点数量（简写）"},
				{Text: "--interactive", Description: "交互式模式"},
				{Text: "-i", Description: "交互式模式（简写）"},
				{Text: "--auto-approve", Description: "自动批准（默认）"},
				{Text: "-y", Description: "自动批准（简写）"},
			}
			var res []prompt.Suggest
			for _, f := range flags {
				if strings.HasPrefix(f.Text, current) {
					res = append(res, f)
				}
			}
			return res
		}
		// 如果当前为空（刚输入空格），提供标志提示
		if current == "" && len(args) >= 2 {
			flags := []prompt.Suggest{
				{Text: "--node", Description: "指定节点数量"},
				{Text: "-n", Description: "指定节点数量（简写）"},
				{Text: "--interactive", Description: "交互式模式"},
				{Text: "-i", Description: "交互式模式（简写）"},
			}
			return flags
		}
		// 第三个参数为节点数量，第四个为工具名，后续为工具参数，通常不需要补全
	case "destroy":
		// scenario destroy <project> <scenario-id>
		if len(args) == 1 {
			// 补全项目名
			return c.completeProjectNames(current)
		}
		if len(args) == 2 {
			// 补全场景 ID
			return c.completeScenarioIDs(args[1], current)
		}
	case "status":
		// scenario status <project> [scenario-id]
		if len(args) == 1 {
			return c.completeProjectNames(current)
		}
		if len(args) == 2 {
			// 补全场景 ID
			return c.completeScenarioIDs(args[1], current)
		}
	}

	return []prompt.Suggest{}
}

// completeTemplate template 子命令补全
func (c *console) completeTemplate(args []string, current string) []prompt.Suggest {
	if len(args) == 0 {
		subs := []prompt.Suggest{
			{Text: "list", Description: "列出所有可用模板"},
		}
		var res []prompt.Suggest
		for _, s := range subs {
			if strings.HasPrefix(s.Text, current) {
				res = append(res, s)
			}
		}
		return res
	}
	// template list 无额外参数
	return []prompt.Suggest{}
}

// completePrice price 子命令补全
func (c *console) completePrice(args []string, current string) []prompt.Suggest {
	if len(args) == 0 {
		subs := []prompt.Suggest{
			{Text: "list", Description: "列出所有价格信息"},
			{Text: "compare", Description: "比对指定类型模板的价格"},
			{Text: "optimal", Description: "获取最优价格配置"},
			{Text: "regions", Description: "获取所有地区的价格"},
		}
		var res []prompt.Suggest
		for _, s := range subs {
			if strings.HasPrefix(s.Text, current) {
				res = append(res, s)
			}
		}
		return res
	}

	switch args[0] {
	case "list":
		// price list 无额外参数
		return []prompt.Suggest{}
	case "compare":
		// price compare <template-type>
		// 模板类型通常不需要补全，但可以提供一些常见类型
		if len(args) == 1 {
			types := []prompt.Suggest{
				{Text: "ecs", Description: "ECS 云服务器"},
				{Text: "proxy", Description: "代理服务器"},
				{Text: "ec2", Description: "AWS EC2 实例"},
			}
			var res []prompt.Suggest
			for _, t := range types {
				if strings.HasPrefix(t.Text, current) {
					res = append(res, t)
				}
			}
			return res
		}
	case "optimal", "regions":
		// price optimal <provider> <template>
		// price regions <provider> <template>
		if len(args) == 1 {
			// 补全 provider
			return c.completeProviders(current)
		}
		if len(args) == 2 {
			// 补全 template（基于 provider）
			return c.completeTemplates(args[1], current)
		}
	}

	return []prompt.Suggest{}
}

// completeCredential credential 子命令补全
func (c *console) completeCredential(args []string, current string) []prompt.Suggest {
	if len(args) == 0 {
		subs := []prompt.Suggest{
			{Text: "list", Description: "列出所有已配置的凭据"},
			{Text: "set", Description: "设置云服务商凭据"},
			{Text: "get", Description: "获取云服务商凭据"},
			{Text: "remove", Description: "删除云服务商凭据"},
		}
		var res []prompt.Suggest
		for _, s := range subs {
			if strings.HasPrefix(s.Text, current) {
				res = append(res, s)
			}
		}
		return res
	}

	switch args[0] {
	case "set", "get", "remove":
		// 补全云服务商
		if len(args) == 1 {
			providers := []prompt.Suggest{
				{Text: "aliyun", Description: "阿里云"},
				{Text: "tencent", Description: "腾讯云"},
				{Text: "huaweicloud", Description: "华为云"},
				{Text: "aws", Description: "AWS"},
				{Text: "vultr", Description: "Vultr"},
			}
			var res []prompt.Suggest
			for _, p := range providers {
				if strings.HasPrefix(p.Text, current) {
					res = append(res, p)
				}
			}
			return res
		}
	case "list":
		// list 无额外参数
		return []prompt.Suggest{}
	}

	return []prompt.Suggest{}
}

// completeProjectNames 动态补全项目名
func (c *console) completeProjectNames(current string) []prompt.Suggest {
	projects, err := c.projectSvc.ListProjects(context.Background())
	if err != nil {
		return []prompt.Suggest{}
	}
	var res []prompt.Suggest
	for _, p := range projects {
		if strings.HasPrefix(p.Name, current) {
			desc := fmt.Sprintf("%d 个场景", len(p.Scenarios))
			res = append(res, prompt.Suggest{Text: p.Name, Description: desc})
		}
	}
	return res
}

// completeScenarioIDs 动态补全场景 ID
func (c *console) completeScenarioIDs(projectName, current string) []prompt.Suggest {
	if projectName == "" {
		return []prompt.Suggest{}
	}
	scenarios, err := c.projectSvc.ListScenarios(context.Background(), projectName)
	if err != nil {
		return []prompt.Suggest{}
	}
	var res []prompt.Suggest
	for _, s := range scenarios {
		if strings.HasPrefix(s.ID, current) {
			desc := fmt.Sprintf("[%s] %s", s.Status, s.Template)
			res = append(res, prompt.Suggest{Text: s.ID, Description: desc})
		}
	}
	return res
}

// completeProviders 动态补全 provider
func (c *console) completeProviders(current string) []prompt.Suggest {
	templates, err := c.templateRepo.ListTemplates()
	if err != nil {
		return []prompt.Suggest{}
	}
	seen := make(map[string]bool)
	var res []prompt.Suggest
	for _, t := range templates {
		if seen[t.Provider] {
			continue
		}
		seen[t.Provider] = true
		if strings.HasPrefix(t.Provider, current) {
			res = append(res, prompt.Suggest{Text: t.Provider, Description: "云服务商"})
		}
	}
	return res
}

// completeTemplates 动态补全模板名称
func (c *console) completeTemplates(provider, current string) []prompt.Suggest {
	if provider == "" {
		return []prompt.Suggest{}
	}
	templates, err := c.templateRepo.ListTemplates()
	if err != nil {
		return []prompt.Suggest{}
	}
	var res []prompt.Suggest
	for _, t := range templates {
		if t.Provider != provider {
			continue
		}
		if strings.HasPrefix(t.Name, current) {
			desc := t.Description
			if desc == "" {
				desc = "模板"
			}
			res = append(res, prompt.Suggest{Text: t.Name, Description: desc})
		}
	}
	return res
}

// printWelcome 打印欢迎信息和基础命令提示
func (c *console) printWelcome() {
	fmt.Println("╔═════════════════════════════════════════════════════════╗")
	fmt.Println("║           Meta-Matrix 交互式控制台 v1.0.0               ║")
	fmt.Println("╚═════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("提示: 输入 'help' 查看可用命令，输入 'exit' 或 'quit' 退出")
	fmt.Println("      按 Tab 键自动补全命令和参数")
	fmt.Println()
}

// handleCommand 解析并处理一条命令
func (c *console) handleCommand(line string) error {
	// 支持用空格分隔的简单命令
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return nil
	}

	switch parts[0] {
	case "help", "h", "?":
		c.printHelp()
		return nil
	case "exit", "quit", "q":
		fmt.Println("退出控制台。")
		os.Exit(0)
	case "project":
		return c.handleProjectCommand(parts[1:])
	case "scenario":
		return c.handleScenarioCommand(parts[1:])
	case "template":
		return c.handleTemplateCommand(parts[1:])
	case "credential":
		return c.handleCredentialCommand(parts[1:])
	default:
		fmt.Println("未知命令。输入 'help' 查看支持的命令。")
		return nil
	}
	return nil
}

// handleProjectCommand 处理项目相关命令
func (c *console) handleProjectCommand(args []string) error {
	if len(args) == 0 {
		fmt.Println("用法: project [list|create <name>|delete <name>]")
		return nil
	}

	switch args[0] {
	case "list":
		return c.cmdProjectList()
	case "create":
		if len(args) < 2 {
			fmt.Println("用法: project create <name>")
			return nil
		}
		return c.cmdProjectCreate(args[1])
	case "delete":
		if len(args) < 2 {
			fmt.Println("用法: project delete <name>")
			return nil
		}
		return c.cmdProjectDelete(args[1])
	default:
		fmt.Println("未知 project 子命令。支持: list, create, delete")
		return nil
	}
}

// handleScenarioCommand 处理场景相关命令
func (c *console) handleScenarioCommand(args []string) error {
	if len(args) == 0 {
		fmt.Println("用法: scenario [list|create|deploy|destroy|status] ...")
		return nil
	}

	switch args[0] {
	case "list":
		if len(args) < 2 {
			fmt.Println("用法: scenario list <project>")
			return nil
		}
		return c.cmdScenarioList(args[1])
	case "create":
		if len(args) < 4 {
			fmt.Println("用法: scenario create <project> <provider> <template> [region]")
			fmt.Println("      对于 aliyun-proxy 模板，可以指定区域: bj, sh, hhht, wlcb, zjk")
			return nil
		}
		region := ""
		if len(args) >= 5 {
			region = args[4]
		}
		return c.cmdScenarioCreate(args[1], args[2], args[3], region)
	case "deploy":
		if len(args) < 3 {
			fmt.Println("用法: scenario deploy <project> <scenario-id> [node-count] [tool-name] [tool-args...]")
			return nil
		}
		// 支持参数：deploy <project> <scenario-id> [node-count] [tool-name] [tool-args...]
		// 将所有剩余参数传递给 cmdScenarioDeploy
		if len(args) >= 4 {
			return c.cmdScenarioDeploy(args[1], args[2], args[3:]...)
		}
		return c.cmdScenarioDeploy(args[1], args[2])
	case "destroy":
		if len(args) < 3 {
			fmt.Println("用法: scenario destroy <project> <scenario-id>")
			return nil
		}
		return c.cmdScenarioDestroy(args[1], args[2])
	case "status":
		if len(args) < 2 {
			fmt.Println("用法: scenario status <project> [scenario-id]")
			return nil
		}
		// 如果提供了场景ID，查询指定场景；否则查询项目下所有场景
		if len(args) >= 3 {
			return c.cmdScenarioStatusByID(args[1], args[2])
		}
		return c.cmdScenarioStatus(args[1])
	default:
		fmt.Println("未知 scenario 子命令。支持: list, create, deploy, destroy")
		return nil
	}
}

// handleTemplateCommand 处理模板相关命令
func (c *console) handleTemplateCommand(args []string) error {
	if len(args) == 0 {
		fmt.Println("用法: template [list]")
		return nil
	}

	switch args[0] {
	case "list":
		return c.cmdTemplateList()
	default:
		fmt.Println("未知 template 子命令。支持: list")
		return nil
	}
}

// handleCredentialCommand 处理凭据相关命令
func (c *console) handleCredentialCommand(args []string) error {
	if len(args) == 0 {
		fmt.Println("用法: credential [list|set|get|remove] ...")
		return nil
	}

	switch args[0] {
	case "list":
		return c.cmdCredentialList()
	case "set":
		if len(args) < 2 {
			fmt.Println("用法: credential set <provider>")
			return nil
		}
		return c.cmdCredentialSet(args[1])
	case "get":
		if len(args) < 2 {
			fmt.Println("用法: credential get <provider>")
			return nil
		}
		return c.cmdCredentialGet(args[1])
	case "remove":
		if len(args) < 2 {
			fmt.Println("用法: credential remove <provider>")
			return nil
		}
		return c.cmdCredentialRemove(args[1])
	default:
		fmt.Println("未知 credential 子命令。支持: list, set, get, remove")
		return nil
	}
}

// cmdCredentialList 列出所有凭据
func (c *console) cmdCredentialList() error {
	manager := credentials.GetDefaultManager()
	providers := manager.ListProviders()

	if len(providers) == 0 {
		fmt.Println("未配置任何凭据")
		fmt.Println("\n提示: 使用 'credential set <provider>' 配置凭据")
		return nil
	}

	fmt.Println("已配置的云服务商凭据:")
	fmt.Println()

	for _, provider := range providers {
		creds, err := manager.GetCredentials(provider)
		if err != nil {
			fmt.Printf("  %s (%s): 获取失败 - %v\n", provider.DisplayName(), provider, err)
			continue
		}

		maskedSK := maskSecret(creds.SecretKey)
		fmt.Printf("  %s (%s):\n", provider.DisplayName(), provider)
		fmt.Printf("    AccessKey: %s\n", creds.AccessKey)
		fmt.Printf("    SecretKey: %s\n", maskedSK)
		if creds.Region != "" {
			fmt.Printf("    Region: %s\n", creds.Region)
		}
		fmt.Println()
	}

	return nil
}

// cmdCredentialSet 设置凭据
func (c *console) cmdCredentialSet(providerStr string) error {
	provider := credentials.Provider(providerStr)
	if !provider.IsValid() {
		return fmt.Errorf("无效的云服务商: %s。支持的云服务商: aliyun, tencent, huaweicloud, aws, vultr", providerStr)
	}

	manager := credentials.GetDefaultManager()

	var accessKey, secretKey, region string
	fmt.Printf("请输入 %s 的 AccessKey: ", provider.DisplayName())
	fmt.Scanln(&accessKey)

	fmt.Printf("请输入 %s 的 SecretKey: ", provider.DisplayName())
	fmt.Scanln(&secretKey)

	fmt.Printf("请输入默认区域（可选，直接回车跳过）: ")
	fmt.Scanln(&region)

	if accessKey == "" || secretKey == "" {
		return fmt.Errorf("AccessKey 和 SecretKey 不能为空")
	}

	creds := &credentials.Credentials{
		AccessKey: accessKey,
		SecretKey: secretKey,
		Region:    region,
	}

	if err := manager.SetCredentials(provider, creds); err != nil {
		return fmt.Errorf("设置凭据失败: %w", err)
	}

	fmt.Printf("%s 凭据设置成功\n", provider.DisplayName())
	return nil
}

// cmdCredentialGet 获取凭据
func (c *console) cmdCredentialGet(providerStr string) error {
	provider := credentials.Provider(providerStr)
	if !provider.IsValid() {
		return fmt.Errorf("无效的云服务商: %s", providerStr)
	}

	manager := credentials.GetDefaultManager()

	if !manager.HasCredentials(provider) {
		return fmt.Errorf("未配置 %s 的凭据", provider.DisplayName())
	}

	creds, err := manager.GetCredentials(provider)
	if err != nil {
		return fmt.Errorf("获取凭据失败: %w", err)
	}

	maskedSK := maskSecret(creds.SecretKey)

	fmt.Printf("%s 凭据信息:\n", provider.DisplayName())
	fmt.Printf("  AccessKey: %s\n", creds.AccessKey)
	fmt.Printf("  SecretKey: %s\n", maskedSK)
	if creds.Region != "" {
		fmt.Printf("  Region: %s\n", creds.Region)
	}

	return nil
}

// cmdCredentialRemove 删除凭据
func (c *console) cmdCredentialRemove(providerStr string) error {
	provider := credentials.Provider(providerStr)
	if !provider.IsValid() {
		return fmt.Errorf("无效的云服务商: %s", providerStr)
	}

	manager := credentials.GetDefaultManager()

	if !manager.HasCredentials(provider) {
		return fmt.Errorf("未配置 %s 的凭据", provider.DisplayName())
	}

	fmt.Printf("确认删除 %s 的凭据? (yes/no): ", provider.DisplayName())
	var confirm string
	fmt.Scanln(&confirm)

	if strings.ToLower(confirm) != "yes" && strings.ToLower(confirm) != "y" {
		fmt.Println("已取消")
		return nil
	}

	if err := manager.RemoveCredentials(provider); err != nil {
		return fmt.Errorf("删除凭据失败: %w", err)
	}

	fmt.Printf("%s 凭据已删除\n", provider.DisplayName())
	return nil
}

// maskSecret 隐藏 SecretKey
func maskSecret(secret string) string {
	if len(secret) <= 8 {
		return "****"
	}
	return secret[:4] + "****" + secret[len(secret)-4:]
}

// cmdProjectList 列出所有项目
func (c *console) cmdProjectList() error {
	projects, err := c.projectSvc.ListProjects(context.Background())
	if err != nil {
		return fmt.Errorf("获取项目列表失败: %w", err)
	}

	if len(projects) == 0 {
		fmt.Println("当前没有项目。可以使用 'project create <name>' 创建新项目。")
		return nil
	}

	fmt.Println("项目列表:")
	for _, p := range projects {
		fmt.Printf("  - %s (%d 个场景)\n", p.Name, len(p.Scenarios))
	}
	return nil
}

// cmdProjectCreate 创建新项目
func (c *console) cmdProjectCreate(name string) error {
	project, err := c.projectSvc.CreateProject(context.Background(), name)
	if err != nil {
		return fmt.Errorf("创建项目失败: %w", err)
	}

	fmt.Printf("项目创建成功:\n")
	fmt.Printf("  名称: %s\n", project.Name)
	fmt.Printf("  路径: %s\n", project.Path)
	fmt.Println("提示: 可以使用 'scenario create' 创建场景。")
	return nil
}

// cmdProjectDelete 删除项目
func (c *console) cmdProjectDelete(name string) error {
	// 先检查项目是否存在
	_, err := c.projectSvc.GetProject(context.Background(), name)
	if err != nil {
		return fmt.Errorf("项目不存在: %w", err)
	}

	// 确认删除
	fmt.Printf("警告: 即将删除项目 %s 及其所有场景和数据。\n", name)
	fmt.Print("确认删除? (yes/no): ")

	var confirmLine string
	if _, err := fmt.Scanln(&confirmLine); err != nil {
		return fmt.Errorf("读取确认输入失败: %w", err)
	}
	confirm := strings.TrimSpace(strings.ToLower(confirmLine))
	if confirm != "yes" && confirm != "y" {
		fmt.Println("已取消删除操作。")
		return nil
	}

	if err := c.projectSvc.DeleteProject(context.Background(), name); err != nil {
		return fmt.Errorf("删除项目失败: %w", err)
	}

	fmt.Printf("项目 %s 已删除。\n", name)
	return nil
}

// cmdScenarioList 列出项目的场景
func (c *console) cmdScenarioList(projectName string) error {
	scenarios, err := c.projectSvc.ListScenarios(context.Background(), projectName)
	if err != nil {
		return fmt.Errorf("获取场景列表失败: %w", err)
	}

	if len(scenarios) == 0 {
		fmt.Printf("项目 %s 暂无场景。可以使用 'scenario create %s <provider> <template>' 创建。\n", projectName, projectName)
		return nil
	}

	fmt.Printf("项目 %s 的场景列表:\n", projectName)
	for _, s := range scenarios {
		fmt.Printf("  - %s [%s] - %s\n", s.ID, s.Status, s.Template)
	}
	return nil
}

// cmdScenarioCreate 创建场景
func (c *console) cmdScenarioCreate(projectName, provider, templateName string, region string) error {
	scenario, err := c.projectSvc.CreateScenario(context.Background(), projectName, provider, templateName, region)
	if err != nil {
		return fmt.Errorf("创建场景失败: %w", err)
	}

	fmt.Println("场景创建成功:")
	fmt.Printf("  ID: %s\n", scenario.ID)
	fmt.Printf("  名称: %s\n", scenario.Name)
	fmt.Printf("  模板: %s\n", scenario.Template)
	if region != "" {
		fmt.Printf("  区域: %s\n", region)
	}
	fmt.Printf("  路径: %s\n", scenario.Path)
	fmt.Printf("提示: 使用 'scenario deploy %s %s' 进行部署。\n", projectName, scenario.ID)
	return nil
}

// cmdScenarioDeploy 部署场景
// 支持参数格式：scenario deploy <project> <scenario-id> [node-count] [tool-name] [tool-args...] [--node <count>]
// 注意：对于 aliyun-proxy 模板，区域在创建场景时已确定，部署时不需要指定区域
func (c *console) cmdScenarioDeploy(projectName, scenarioID string, args ...string) error {
	var nodeCount int
	var toolName string
	var toolArgs string

	argIdx := 0

	// 解析参数，支持标志和位置参数混合
	for argIdx < len(args) {
		arg := args[argIdx]

		// 忽略 --region 参数，因为区域在创建场景时已确定
		if arg == "--region" || arg == "-r" {
			argIdx++
			if argIdx < len(args) {
				// 忽略区域参数值
				argIdx++
			}
			continue
		}

		if arg == "--node" || arg == "-n" {
			argIdx++
			if argIdx < len(args) {
				if parsed, err := strconv.Atoi(args[argIdx]); err == nil && parsed > 0 {
					nodeCount = parsed
					argIdx++
				} else {
					return fmt.Errorf("--node 标志需要指定有效的节点数量")
				}
			} else {
				return fmt.Errorf("--node 标志需要指定节点数量")
			}
			continue
		}

		// 如果不是标志，按位置参数解析
		// 第一个非标志参数可能是节点数量或工具名称
		if nodeCount == 0 && toolName == "" {
			// 尝试解析为节点数量
			if parsed, err := strconv.Atoi(arg); err == nil && parsed > 0 {
				nodeCount = parsed
				argIdx++
				continue
			}
			// 如果不是数字，则作为工具名称
			toolName = arg
			argIdx++
			// 剩余参数作为工具参数
			if argIdx < len(args) {
				toolArgs = strings.Join(args[argIdx:], " ")
			}
			break
		} else if toolName == "" {
			// 如果节点数量已设置，下一个参数是工具名称
			toolName = arg
			argIdx++
			// 剩余参数作为工具参数
			if argIdx < len(args) {
				toolArgs = strings.Join(args[argIdx:], " ")
			}
			break
		} else {
			// 工具名称已设置，剩余参数都是工具参数
			toolArgs = strings.Join(args[argIdx:], " ")
			break
		}
	}

	if nodeCount > 0 {
		fmt.Printf("开始部署场景（节点数量: %d）...\n", nodeCount)
	} else {
		fmt.Println("开始部署场景...")
	}
	if toolName != "" {
		fmt.Printf("工具: %s", toolName)
		if toolArgs != "" {
			fmt.Printf("，参数: %s", toolArgs)
		}
		fmt.Println()
	}
	fmt.Println("提示: 确保已配置云服务商凭据且网络正常。")

	// 默认自动批准，避免 EOF 错误（console 中 Terraform 命令非交互式）
	// 区域参数传空字符串，因为区域在创建场景时已确定
	if err := c.projectSvc.DeployScenario(context.Background(), projectName, scenarioID, true, nodeCount, toolName, toolArgs, ""); err != nil {
		return fmt.Errorf("部署场景失败: %w", err)
	}

	fmt.Printf("场景 %s 部署成功", scenarioID)
	if nodeCount > 0 {
		fmt.Printf("，节点数量: %d", nodeCount)
	}
	if toolName != "" {
		fmt.Printf("，工具: %s", toolName)
	}
	fmt.Println()
	return nil
}

// cmdScenarioStatus 查看项目场景的云资源状态
// 会读取 Terraform state 中的资源列表，用于云资源验证
func (c *console) cmdScenarioStatus(projectName string) error {
	fmt.Println("开始获取项目云资源状态（云资源验证）...")

	statusList, err := c.projectSvc.GetProjectStatus(context.Background(), projectName)
	if err != nil {
		return fmt.Errorf("获取项目云资源状态失败: %w", err)
	}

	if len(statusList) == 0 {
		fmt.Printf("项目 %s 暂无场景。\n", projectName)
		return nil
	}

	fmt.Printf("项目 %s 的云资源状态:\n", projectName)
	for _, st := range statusList {
		c.printScenarioStatus(&st)
	}

	return nil
}

// cmdScenarioStatusByID 查看指定场景的云资源状态
func (c *console) cmdScenarioStatusByID(projectName, scenarioID string) error {
	fmt.Println("开始获取场景云资源状态（云资源验证）...")

	st, err := c.projectSvc.GetScenarioStatus(context.Background(), projectName, scenarioID)
	if err != nil {
		return fmt.Errorf("获取场景云资源状态失败: %w", err)
	}

	c.printScenarioStatus(st)
	return nil
}

// printScenarioStatus 打印场景状态信息
func (c *console) printScenarioStatus(st *service.ScenarioStatus) {
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

// cmdScenarioDestroy 销毁场景
func (c *console) cmdScenarioDestroy(projectName, scenarioID string) error {
	fmt.Println("警告: 即将销毁场景并删除所有已创建的云资源。")
	fmt.Print("确认销毁? (yes/no): ")

	var confirmLine string
	if _, err := fmt.Scanln(&confirmLine); err != nil {
		return fmt.Errorf("读取确认输入失败: %w", err)
	}
	confirm := strings.TrimSpace(strings.ToLower(confirmLine))
	if confirm != "yes" && confirm != "y" {
		fmt.Println("已取消销毁操作。")
		return nil
	}

	if err := c.projectSvc.DestroyScenario(context.Background(), projectName, scenarioID, true); err != nil {
		return fmt.Errorf("销毁场景失败: %w", err)
	}

	fmt.Printf("场景 %s 已销毁。\n", scenarioID)
	return nil
}

// cmdTemplateList 列出模板
func (c *console) cmdTemplateList() error {
	templates, err := c.templateRepo.ListTemplates()
	if err != nil {
		return fmt.Errorf("获取模板列表失败: %w", err)
	}

	if len(templates) == 0 {
		fmt.Println("没有可用模板。")
		return nil
	}

	fmt.Println("可用模板:")
	providerMap := make(map[string][]string)
	for _, t := range templates {
		desc := t.Description
		if desc == "" {
			desc = "模板"
		}
		providerMap[t.Provider] = append(providerMap[t.Provider], fmt.Sprintf("%s - %s", t.Name, desc))
	}

	for provider, list := range providerMap {
		fmt.Printf("\n%s:\n", provider)
		for _, item := range list {
			fmt.Printf("  - %s\n", item)
		}
	}
	return nil
}

// printHelp 打印控制台内可用命令帮助
func (c *console) printHelp() {
	fmt.Println("可用命令:")
	fmt.Println("  help                          显示本帮助")
	fmt.Println("  exit | quit                   退出控制台")
	fmt.Println()
	fmt.Println("  project list                  列出所有项目")
	fmt.Println("  project create <name>         创建新项目")
	fmt.Println("  project delete <name>         删除项目")
	fmt.Println()
	fmt.Println("  scenario list <project>       列出项目的所有场景")
	fmt.Println("  scenario create <project> <provider> <template>")
	fmt.Println("                                从模板创建场景")
	fmt.Println("  scenario deploy <project> <scenario-id> [node-count] [tool-name] [tool-args...]")
	fmt.Println("                                部署场景（支持节点数量和工具参数）")
	fmt.Println("  scenario destroy <project> <scenario-id>")
	fmt.Println("                                销毁场景")
	fmt.Println("  scenario status <project> [scenario-id]")
	fmt.Println("                                查看项目或指定场景的云资源状态")
	fmt.Println()
	fmt.Println("  credential list                列出所有已配置的凭据")
	fmt.Println("  credential set <provider>      设置云服务商凭据")
	fmt.Println("  credential get <provider>      获取云服务商凭据")
	fmt.Println("  credential remove <provider>    删除云服务商凭据")
	fmt.Println()
	fmt.Println("  credential list                列出所有已配置的凭据")
	fmt.Println("  credential set <provider>      设置云服务商凭据")
	fmt.Println("  credential get <provider>      获取云服务商凭据")
	fmt.Println("  credential remove <provider>    删除云服务商凭据")
	fmt.Println()
	fmt.Println("  template list                 列出所有可用模板")
	fmt.Println()
	fmt.Println("提示: 命令与 CLI 保持一致，建议先通过 'template list' 和 'project list' 了解现有资源。")
}
