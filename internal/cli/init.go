package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"apifox-api/go-version/internal/binding"
)

const initUsage = `用法: apifox-api init <projectId> [--moduleIds 5,8,12] [--authKey <token>]

说明:
- 把当前工作目录绑定到一个 Apifox projectId，写入全局注册表 ~/.apifox-api.json。
- --moduleIds：逗号分隔的正整数，绑定多个模块；省略表示只使用默认模块。
- --authKey：Apifox Access Token，会存入全局注册表；省略时回退 APIFOX_AUTH_KEY 环境变量。`

func newInitCommand(dependencies Dependencies) *cobra.Command {
	var moduleIDsFlag string
	var authKeyFlag string

	command := &cobra.Command{
		Use:   "init <projectId>",
		Short: "Bind the current workspace directory to an Apifox project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInitCommand(dependencies, args[0], moduleIDsFlag, authKeyFlag, cmd)
		},
	}

	command.Flags().StringVar(&moduleIDsFlag, "moduleIds", "", "comma-separated positive module IDs")
	command.Flags().StringVar(&authKeyFlag, "authKey", "", "Apifox Access Token stored on this binding")
	command.SetUsageTemplate(initUsage + "\n")
	return command
}

func runInitCommand(dependencies Dependencies, projectIDArg string, moduleIDsRaw string, authKeyFlag string, cmd *cobra.Command) error {
	cwd := dependencies.CWD
	if cwd == "" {
		return fmt.Errorf("cwd is required")
	}
	homeDir := dependencies.HomeDir
	if homeDir == "" {
		return fmt.Errorf("homeDir is required")
	}
	env := dependencies.Env
	if env == nil {
		env = map[string]string{}
	}

	projectID, err := binding.ValidateProjectID(projectIDArg)
	if err != nil {
		return fmt.Errorf("apifox-api init 失败: %w\n\n%s", err, initUsage)
	}

	moduleIDs, err := binding.ParseModuleIDs(moduleIDsRaw)
	if err != nil {
		return fmt.Errorf("apifox-api init 失败: %w\n\n%s", err, initUsage)
	}

	auth, err := binding.ResolveInitAuthKeyForCWD(authKeyFlag, env, cwd, homeDir)
	if err != nil {
		return fmt.Errorf("写入全局注册表失败: %w\n\n%s", err, initUsage)
	}

	legacy, err := binding.ReadLegacyBindingForMigration(cwd)
	if err != nil {
		return fmt.Errorf("写入全局注册表失败: %w\n\n%s", err, initUsage)
	}

	upsertOptions := binding.UpsertOptions{
		CWD:       cwd,
		HomeDir:   homeDir,
		ProjectID: projectID,
		ModuleIDs: moduleIDs,
	}
	if auth.PersistAuthKey != "" {
		upsertOptions.AuthKey = auth.PersistAuthKey
	}
	if legacy != nil && legacy.ProjectName != "" {
		upsertOptions.ProjectName = legacy.ProjectName
	}

	upsert, err := binding.UpsertBinding(upsertOptions)
	if err != nil {
		return fmt.Errorf("写入全局注册表失败: %w\n\n%s", err, initUsage)
	}

	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()

	lines := []string{
		"已写入 Apifox Project Binding。",
		fmt.Sprintf("workspace: %s", upsert.WorkspaceKey),
		fmt.Sprintf("registry: %s", upsert.RegistryPath),
		fmt.Sprintf("projectId: %s", projectID),
	}
	if len(moduleIDs) == 0 {
		lines = append(lines, "moduleIds: []（默认模块）")
	} else {
		parts := make([]string, len(moduleIDs))
		for i, id := range moduleIDs {
			parts[i] = fmt.Sprintf("%d", id)
		}
		lines = append(lines, fmt.Sprintf("moduleIds: [%s]", strings.Join(parts, ", ")))
	}

	if auth.PrefetchAuthKey != "" && auth.PersistAuthKey != "" {
		lines = append(lines, fmt.Sprintf(
			"authKey: 已配置（来源=%s, fingerprint=%s）",
			auth.PrefetchSource, auth.PrefetchFingerprint,
		))
	} else if auth.PrefetchAuthKey != "" {
		lines = append(lines, fmt.Sprintf(
			"authKey: 已配置（来源=全局默认，仅用于本次拉取，不写入 binding, fingerprint=%s）",
			auth.PrefetchFingerprint,
		))
	} else {
		lines = append(lines, "authKey: 未配置（运行时需通过 APIFOX_AUTH_KEY 提供）")
	}

	if upsert.PreviousBinding != nil {
		previousModuleIDs := "[]（默认模块）"
		if len(upsert.PreviousBinding.ModuleIDs) > 0 {
			parts := make([]string, len(upsert.PreviousBinding.ModuleIDs))
			for i, id := range upsert.PreviousBinding.ModuleIDs {
				parts[i] = fmt.Sprintf("%d", id)
			}
			previousModuleIDs = "[" + strings.Join(parts, ", ") + "]"
		}
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf(
			"已覆盖原有绑定: projectId=%s, moduleIds=%s",
			upsert.PreviousBinding.ProjectID, previousModuleIDs,
		))
	}

	// Network prefetch is intentionally deferred to a later issue (remote export).
	// Issue 02 still reports when prefetch would be skipped or would use global-only key.
	if auth.PrefetchAuthKey != "" {
		lines = append(lines, "")
		lines = append(lines, "首次拉取接口文档快照已跳过（网络导出尚未接入）；可稍后运行 `apifox-api refresh`。")
	} else {
		lines = append(lines, "")
		lines = append(lines, "未配置 authKey，已跳过首次拉取。运行时需通过 APIFOX_AUTH_KEY 提供 token。")
	}

	if len(moduleIDs) > 1 {
		firstModuleID := moduleIDs[0]
		if err := binding.WriteCurrentModuleFile(upsert.WorkspaceKey, firstModuleID); err != nil {
			fmt.Fprintf(errOut, "写入 .current-module 失败: %v\n", err)
		} else {
			lines = append(lines, "")
			lines = append(lines, fmt.Sprintf(
				"已生成 %s（当前 module=%d）。",
				filepath.Join(upsert.WorkspaceKey, binding.CurrentModuleFileName),
				firstModuleID,
			))
			lines = append(lines, "切换当前 module: apifox-api module <moduleId>")
		}
	}

	if legacy != nil {
		legacyPath := filepath.Join(cwd, binding.LegacyBindingFileName)
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("检测到旧版工作区绑定 %s (projectId=%s)。", legacyPath, legacy.ProjectID))
		lines = append(lines, "新版绑定已迁移到全局注册表，旧文件不再生效，建议手动删除。")
	}

	_, err = fmt.Fprintln(out, strings.Join(lines, "\n"))
	return err
}
