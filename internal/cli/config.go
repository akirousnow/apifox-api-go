package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/akirousnow/apifox-api-go/internal/binding"
)

const configUsage = `用法: apifox-api config set-auth-key <token>

说明:
- 设置全局默认 Apifox Auth Key。
- 所有未单独配置 authKey 的项目都会回退到这个全局 token。
- <token> 不能为空或纯空白。`

func newConfigCommand(dependencies Dependencies) *cobra.Command {
	configCommand := &cobra.Command{
		Use:   "config",
		Short: "Manage global apifox-api configuration",
	}

	setAuthKeyCommand := &cobra.Command{
		Use:   "set-auth-key <token>",
		Short: "Set the Global Auth Key in ~/.apifox-api.json",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetAuthKey(dependencies, args[0], cmd)
		},
	}

	configCommand.AddCommand(setAuthKeyCommand)
	return configCommand
}

func runSetAuthKey(dependencies Dependencies, token string, cmd *cobra.Command) error {
	homeDir := dependencies.HomeDir
	if homeDir == "" {
		return fmt.Errorf("homeDir is required")
	}

	result, err := binding.SetGlobalAuthKey(homeDir, token)
	if err != nil {
		return fmt.Errorf("apifox-api config 失败: %w", err)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "已设置全局默认 Apifox Auth Key。")
	if result.HasPrevious {
		fmt.Fprintf(out, "  之前: %s\n", result.PreviousFingerprint)
	} else {
		fmt.Fprintln(out, "  之前: （无）")
	}
	fmt.Fprintf(out, "  现在: %s\n", result.NextFingerprint)
	fmt.Fprintf(out, "  写入: %s\n", result.RegistryPath)
	fmt.Fprintln(out, "所有未单独配置 authKey 的项目都会回退到这个全局 token。")
	return nil
}
