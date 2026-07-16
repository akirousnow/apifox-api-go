package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/akirousnow/apifox-api-go/internal/binding"
	"github.com/akirousnow/apifox-api-go/internal/openapi"
	"github.com/akirousnow/apifox-api-go/internal/snapshot"
)

func newSearchCommand(dependencies Dependencies) *cobra.Command {
	var moduleIDFlag int
	var moduleIDFlagSet bool
	var methodFlag string
	var modeFlag string
	var limitFlag int
	var jsonFlag bool

	command := &cobra.Command{
		Use:   "search [keywords...]",
		Short: "Search OpenAPI endpoints offline from the local Snapshot cache",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearchCommand(dependencies, cmd, searchCommandInput{
				keywords:        args,
				method:          methodFlag,
				mode:            modeFlag,
				limit:           limitFlag,
				jsonOutput:      jsonFlag,
				moduleIDFlag:    moduleIDFlag,
				moduleIDFlagSet: moduleIDFlagSet,
			})
		},
	}
	command.Flags().StringVar(&methodFlag, "method", "", "filter by HTTP method (GET/POST/...)")
	command.Flags().StringVar(&modeFlag, "mode", "or", "keyword combine mode: or | and")
	command.Flags().IntVar(&limitFlag, "limit", 0, "result window size (default 20, max 50)")
	command.Flags().BoolVar(&jsonFlag, "json", false, "emit machine-readable JSON (no strategy prose)")
	command.Flags().IntVar(&moduleIDFlag, "moduleId", 0, "one-shot Current Module override (does not rewrite .current-module)")
	command.PreRun = func(cmd *cobra.Command, args []string) {
		moduleIDFlagSet = cmd.Flags().Changed("moduleId")
	}
	return command
}

type searchCommandInput struct {
	keywords        []string
	method          string
	mode            string
	limit           int
	jsonOutput      bool
	moduleIDFlag    int
	moduleIDFlagSet bool
}

func runSearchCommand(dependencies Dependencies, cmd *cobra.Command, input searchCommandInput) error {
	cwd := dependencies.CWD
	homeDir := dependencies.HomeDir
	env := dependencies.Env
	if env == nil {
		env = map[string]string{}
	}

	// Reject completely empty search at the command layer (matches TypeScript).
	hasKeyword := false
	for _, keyword := range input.keywords {
		if strings.TrimSpace(keyword) != "" {
			hasKeyword = true
			break
		}
	}
	if !hasKeyword && strings.TrimSpace(input.method) == "" {
		return fmt.Errorf("apifox-api search 失败: 请提供 keywords 或 --method，避免返回整个模块的接口。")
	}

	// Validate method early so invalid values fail before cache/network work.
	if _, err := openapi.ValidateHTTPMethod(input.method); err != nil {
		return fmt.Errorf("apifox-api search 失败: %w", err)
	}

	resolved, err := binding.ResolveBinding(binding.ResolveOptions{
		CWD:     cwd,
		HomeDir: homeDir,
		Env:     env,
	})
	if err != nil {
		return fmt.Errorf("apifox-api search 失败: %w", err)
	}

	moduleOptions := binding.ResolveCurrentModuleOptions{
		CWD:       cwd,
		HomeDir:   homeDir,
		ModuleIDs: resolved.ModuleIDs,
	}
	if input.moduleIDFlagSet {
		flagValue := input.moduleIDFlag
		moduleOptions.ModuleIDFlag = &flagValue
	}

	currentModule, err := binding.ResolveCurrentModule(moduleOptions)
	if err != nil {
		return fmt.Errorf("apifox-api search 失败: %w", err)
	}

	authFingerprint := binding.AuthFingerprint(resolved.AuthKey)
	allowStale := true
	loadResult, err := snapshot.LoadModuleSnapshot(snapshot.LoadOptions{
		WorkspaceDir:      resolved.WorkspaceDir,
		ProjectID:         resolved.ProjectID,
		AuthKey:           resolved.AuthKey,
		AuthFingerprint:   authFingerprint,
		ModuleID:          currentModule,
		Env:               env,
		AllowStaleOnError: &allowStale,
		Context:           cmd.Context(),
		FetchFunc:         dependencies.FetchFunc,
	})
	if err != nil {
		return fmt.Errorf("apifox-api search 失败: %w", err)
	}

	endpoints, err := openapi.BuildIndex(loadResult.Data)
	if err != nil {
		return fmt.Errorf("apifox-api search 失败: 无法解析 OpenAPI 快照: %w", err)
	}

	window, err := openapi.SearchWindow(endpoints, openapi.SearchOptions{
		Keywords: input.keywords,
		Mode:     input.mode,
		Method:   input.method,
		Limit:    input.limit,
	})
	if err != nil {
		return fmt.Errorf("apifox-api search 失败: %w", err)
	}

	if loadResult.Warning != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "警告: %s\n", loadResult.Warning)
	}

	if input.jsonOutput {
		payload, err := openapi.FormatSearchJSON(window, loadResult.ModuleID, loadResult.Stale)
		if err != nil {
			return fmt.Errorf("apifox-api search 失败: %w", err)
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), string(payload))
		return err
	}

	// Stale warning already went to stderr; keep markdown free of warning banners.
	markdown := openapi.FormatSearchResults(window, "")
	if !strings.HasSuffix(markdown, "\n") {
		markdown += "\n"
	}
	_, err = fmt.Fprint(cmd.OutOrStdout(), markdown)
	return err
}
