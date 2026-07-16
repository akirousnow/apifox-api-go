package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"apifox-api/go-version/internal/binding"
)

const moduleUsage = `用法: apifox-api module [moduleId]

说明:
- 无参：打印当前 module 与全部绑定的 moduleIds。
- 有参：把 <moduleId> 写入绑定根的 .current-module，完成切换。
- <moduleId> 必须是正整数，且在绑定的 moduleIds 内。`

func newModuleCommand(dependencies Dependencies) *cobra.Command {
	command := &cobra.Command{
		Use:   "module [moduleId]",
		Short: "Show or switch the Current Module for the resolved Project Binding",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModuleCommand(dependencies, args, cmd)
		},
	}
	command.SetUsageTemplate(moduleUsage + "\n")
	return command
}

func runModuleCommand(dependencies Dependencies, args []string, cmd *cobra.Command) error {
	out := cmd.OutOrStdout()

	var targetModuleID *int
	if len(args) == 1 {
		trimmed := strings.TrimSpace(args[0])
		if trimmed == "" {
			return fmt.Errorf("apifox-api module 失败: moduleId 不能为空。\n\n%s", moduleUsage)
		}
		parsed, err := strconv.Atoi(trimmed)
		if err != nil || parsed <= 0 || strconv.Itoa(parsed) != trimmed {
			return fmt.Errorf("apifox-api module 失败: moduleId 必须是正整数，收到 “%s”。\n\n%s", args[0], moduleUsage)
		}
		targetModuleID = &parsed
	}

	cwd := dependencies.CWD
	homeDir := dependencies.HomeDir
	env := dependencies.Env
	if env == nil {
		env = map[string]string{}
	}

	resolved, err := binding.ResolveBinding(binding.ResolveOptions{
		CWD:     cwd,
		HomeDir: homeDir,
		Env:     env,
	})
	if err != nil {
		return fmt.Errorf("apifox-api module 失败: %w", err)
	}

	if targetModuleID == nil {
		currentModule, resolveErr := binding.ResolveCurrentModule(binding.ResolveCurrentModuleOptions{
			CWD:       cwd,
			HomeDir:   homeDir,
			ModuleIDs: resolved.ModuleIDs,
		})
		if resolveErr != nil {
			// Keep bound module list on stdout for recovery; error text goes via returned err → stderr once.
			fmt.Fprintf(out, "绑定的 moduleIds: %s\n", binding.FormatModuleIDs(resolved.ModuleIDs))
			return fmt.Errorf("apifox-api module 失败: %w", resolveErr)
		}
		fmt.Fprintf(out, "当前 module: %s\n", binding.SummariseModuleID(currentModule))
		fmt.Fprintf(out, "绑定的 moduleIds: %s\n", binding.FormatModuleIDs(resolved.ModuleIDs))
		return nil
	}

	if !moduleIDInList(resolved.ModuleIDs, *targetModuleID) {
		return fmt.Errorf(
			"apifox-api module 失败: moduleId %d 不在绑定的 moduleIds %s 内。",
			*targetModuleID,
			binding.FormatModuleIDs(resolved.ModuleIDs),
		)
	}

	if err := binding.WriteCurrentModuleFile(resolved.WorkspaceDir, *targetModuleID); err != nil {
		return fmt.Errorf("apifox-api module 失败: 写入 .current-module 失败: %w", err)
	}

	fmt.Fprintf(out, "已切换当前 module 为 %d。\n", *targetModuleID)
	return nil
}

func moduleIDInList(moduleIDs []int, target int) bool {
	for _, moduleID := range moduleIDs {
		if moduleID == target {
			return true
		}
	}
	return false
}
