package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/akirousnow/apifox-api-go/internal/binding"
	"github.com/akirousnow/apifox-api-go/internal/snapshot"
)

func newRefreshCommand(dependencies Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "refresh",
		Short: "Force-refresh OpenAPI Snapshot caches for every bound Module",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRefreshCommand(dependencies, cmd)
		},
	}
}

func runRefreshCommand(dependencies Dependencies, cmd *cobra.Command) error {
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
		return fmt.Errorf("apifox-api refresh 失败: %w", err)
	}

	authKey := strings.TrimSpace(resolved.AuthKey)
	if authKey == "" {
		return fmt.Errorf("apifox-api refresh 失败: 缺少 Auth Key，无法从远程拉取 OpenAPI 快照。")
	}

	moduleIDs := binding.ModulesForRefresh(resolved.ModuleIDs)
	authFingerprint := binding.AuthFingerprint(authKey)
	allowStale := false

	results, err := snapshot.LoadAllModuleSnapshots(
		resolved.WorkspaceDir,
		resolved.ProjectID,
		authKey,
		authFingerprint,
		moduleIDs,
		snapshot.LoadOptions{
			Env:               env,
			ForceRefresh:      true,
			AllowStaleOnError: &allowStale,
			Context:           cmd.Context(),
			FetchFunc:         dependencies.FetchFunc,
		},
	)
	if err != nil {
		return fmt.Errorf("apifox-api refresh 失败: %w", err)
	}

	for _, result := range results {
		moduleLabel := binding.SummariseModuleID(result.ModuleID)
		fmt.Fprintf(cmd.OutOrStdout(), "已刷新 %s → %s\n", moduleLabel, result.CachePath)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "refresh 完成，共 %d 个 module。\n", len(results))
	return nil
}
