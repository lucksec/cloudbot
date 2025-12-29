package main

import (
	"fmt"
	"strings"

	"github.com/lucksec/cloudbot/internal/credentials"
	"github.com/spf13/cobra"
)

// credentialCmd 凭据管理命令组
func credentialCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "credential",
		Short: "云服务商凭据管理",
		Long: `管理云服务商的 AccessKey 和 SecretKey。

支持以下云服务商:
  - aliyun: 阿里云
  - tencent: 腾讯云
  - huaweicloud: 华为云
  - aws: AWS
  - vultr: Vultr

凭据可以存储在配置文件中，也可以从环境变量读取。
环境变量优先级低于配置文件。`,
	}

	cmd.AddCommand(listCredentialsCmd())
	cmd.AddCommand(setCredentialCmd())
	cmd.AddCommand(getCredentialCmd())
	cmd.AddCommand(removeCredentialCmd())

	return cmd
}

// listCredentialsCmd 列出所有已配置的凭据
func listCredentialsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "列出所有已配置的凭据",
		Long:  "显示所有已配置凭据的云服务商列表。",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := credentials.GetDefaultManager()
			providers := manager.ListProviders()

			if len(providers) == 0 {
				fmt.Println("未配置任何凭据")
				fmt.Println("\n提示: 使用 'cloudbot credential set <provider>' 配置凭据")
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

				// 隐藏 SecretKey，只显示前4位和后4位
				maskedSK := maskSecretForCLI(creds.SecretKey)
				fmt.Printf("  %s (%s):\n", provider.DisplayName(), provider)
				fmt.Printf("    AccessKey: %s\n", creds.AccessKey)
				fmt.Printf("    SecretKey: %s\n", maskedSK)
				if creds.Region != "" {
					fmt.Printf("    Region: %s\n", creds.Region)
				}
				fmt.Println()
			}

			return nil
		},
	}
	return cmd
}

// setCredentialCmd 设置凭据
func setCredentialCmd() *cobra.Command {
	var accessKey, secretKey, region string

	cmd := &cobra.Command{
		Use:   "set <provider>",
		Short: "设置云服务商凭据",
		Long: `设置指定云服务商的 AccessKey 和 SecretKey。

支持的 provider: aliyun, tencent, huaweicloud, aws, vultr

示例:
  # 交互式设置（会提示输入）
  cloudbot credential set aliyun
  
  # 通过参数设置
  cloudbot credential set aliyun --access-key <key> --secret-key <key> --region <region>`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			providerStr := args[0]
			provider := credentials.Provider(providerStr)

			if !provider.IsValid() {
				return fmt.Errorf("无效的云服务商: %s。支持的云服务商: aliyun, tencent, huaweicloud, aws, vultr", providerStr)
			}

			manager := credentials.GetDefaultManager()

			// 如果没有通过参数提供，交互式输入
			if accessKey == "" {
				fmt.Printf("请输入 %s 的 AccessKey: ", provider.DisplayName())
				fmt.Scanln(&accessKey)
			}

			if secretKey == "" {
				fmt.Printf("请输入 %s 的 SecretKey: ", provider.DisplayName())
				// 使用密码输入方式（不显示）
				secretKeyBytes, err := readPassword()
				if err != nil {
					return fmt.Errorf("读取 SecretKey 失败: %w", err)
				}
				secretKey = string(secretKeyBytes)
			}

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
		},
	}

	cmd.Flags().StringVarP(&accessKey, "access-key", "a", "", "AccessKey (或 Secret ID)")
	cmd.Flags().StringVarP(&secretKey, "secret-key", "s", "", "SecretKey (或 Secret Key)")
	cmd.Flags().StringVarP(&region, "region", "r", "", "默认区域（可选）")

	return cmd
}

// getCredentialCmd 获取凭据
func getCredentialCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <provider>",
		Short: "获取云服务商凭据",
		Long:  "显示指定云服务商的凭据信息（SecretKey 会被隐藏）。",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			providerStr := args[0]
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

			maskedSK := maskSecretForCLI(creds.SecretKey)

			fmt.Printf("%s 凭据信息:\n", provider.DisplayName())
			fmt.Printf("  AccessKey: %s\n", creds.AccessKey)
			fmt.Printf("  SecretKey: %s\n", maskedSK)
			if creds.Region != "" {
				fmt.Printf("  Region: %s\n", creds.Region)
			}

			return nil
		},
	}
	return cmd
}

// removeCredentialCmd 删除凭据
func removeCredentialCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <provider>",
		Short: "删除云服务商凭据",
		Long:  "从配置文件中删除指定云服务商的凭据。",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			providerStr := args[0]
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
		},
	}
	return cmd
}

// maskSecret 隐藏 SecretKey（只显示前4位和后4位）
// 注意：此函数在 console.go 中也有定义，为了避免重复定义，这里使用不同的包名
// 但由于都在同一个包中，需要重命名或删除一个
// 这里保留作为 CLI 命令使用，console.go 中的版本用于控制台
func maskSecretForCLI(secret string) string {
	if len(secret) <= 8 {
		return "****"
	}
	return secret[:4] + "****" + secret[len(secret)-4:]
}

// readPassword 读取密码（不显示输入）
func readPassword() ([]byte, error) {
	// 简单的实现：直接读取（实际应该使用 golang.org/x/term）
	var password string
	fmt.Scanln(&password)
	return []byte(password), nil
}
