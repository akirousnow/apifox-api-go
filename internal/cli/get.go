package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/akirousnow/apifox-api-go/internal/binding"
	"github.com/akirousnow/apifox-api-go/internal/openapi"
	"github.com/akirousnow/apifox-api-go/internal/snapshot"
	"github.com/akirousnow/apifox-api-go/internal/typesgen"
)

const getUsage = `用法: apifox-api get <method> <path> [--moduleId N]
       apifox-api get <path> [--method METHOD] [--moduleId N]
       apifox-api get <path> [--moduleId N]

说明:
- 给出 <method> + <path>：为单个接口生成 TypeScript 类型。
- 给出 <path> --method <method>：同上，推荐写法。
- 只给出 <path>：为该路径下所有 HTTP method 各生成一组类型，并去重共享的关联类型。
- --moduleId：临时使用指定 module 的接口快照，不影响当前 module 设置。`

func newGetCommand(dependencies Dependencies) *cobra.Command {
	var moduleIDFlag int
	var moduleIDFlagSet bool
	var methodFlag string

	command := &cobra.Command{
		Use:   "get [method] [path]",
		Short: "Generate TypeScript types for an OpenAPI operation from the local Snapshot",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGetCommand(dependencies, cmd, getCommandInput{
				args:            args,
				methodFlag:      methodFlag,
				moduleIDFlag:    moduleIDFlag,
				moduleIDFlagSet: moduleIDFlagSet,
			})
		},
	}
	command.Flags().StringVar(&methodFlag, "method", "", "HTTP method (GET/POST/...); alternative to positional method")
	command.Flags().IntVar(&moduleIDFlag, "moduleId", 0, "one-shot Current Module override (does not rewrite .current-module)")
	command.PreRun = func(cmd *cobra.Command, args []string) {
		moduleIDFlagSet = cmd.Flags().Changed("moduleId")
	}
	return command
}

type getCommandInput struct {
	args            []string
	methodFlag      string
	moduleIDFlag    int
	moduleIDFlagSet bool
}

type parsedGetArgs struct {
	method string
	path   string
}

func parseGetArgs(positional []string, methodFlag string) (parsedGetArgs, error) {
	if len(positional) == 0 {
		return parsedGetArgs{}, fmt.Errorf("%s", getUsage)
	}
	if len(positional) > 2 {
		extra := strings.Join(positional[2:], " ")
		return parsedGetArgs{}, fmt.Errorf("get 只能接受 <method> <path> 两个位置参数，多余参数: %s", extra)
	}

	methodFlag = strings.TrimSpace(methodFlag)

	if len(positional) == 1 {
		// path-only, optional --method
		if methodFlag != "" {
			normalized, err := openapi.ValidateHTTPMethod(methodFlag)
			if err != nil {
				return parsedGetArgs{}, err
			}
			return parsedGetArgs{method: normalized, path: positional[0]}, nil
		}
		return parsedGetArgs{path: positional[0]}, nil
	}

	// two positionals: method path
	if methodFlag != "" {
		return parsedGetArgs{}, fmt.Errorf("不能同时提供位置 method 与 --method。")
	}
	normalized, err := openapi.ValidateHTTPMethod(positional[0])
	if err != nil {
		// Match typesgen Chinese error for invalid method when using positional form.
		// ValidateHTTPMethod already has Chinese text for --method style; for positional
		// legacy form TS says "无效的 HTTP method: X". Prefer openapi validation message
		// for consistency with search, but typesgen also validates.
		return parsedGetArgs{}, err
	}
	return parsedGetArgs{method: normalized, path: positional[1]}, nil
}

func runGetCommand(dependencies Dependencies, cmd *cobra.Command, input getCommandInput) error {
	cwd := dependencies.CWD
	homeDir := dependencies.HomeDir
	env := dependencies.Env
	if env == nil {
		env = map[string]string{}
	}

	parsed, err := parseGetArgs(input.args, input.methodFlag)
	if err != nil {
		if strings.Contains(err.Error(), "用法:") {
			return fmt.Errorf("%s", err.Error())
		}
		return fmt.Errorf("apifox-api get 失败: %w\n\n%s", err, getUsage)
	}

	resolved, err := binding.ResolveBinding(binding.ResolveOptions{
		CWD:     cwd,
		HomeDir: homeDir,
		Env:     env,
	})
	if err != nil {
		return err
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
		return err
	}

	authFingerprint := resolved.AuthFingerprint
	allowStale := true
	loadResult, err := snapshot.LoadModuleSnapshot(snapshot.LoadOptions{
		WorkspaceDir:      resolved.WorkspaceDir,
		ProjectID:         resolved.ProjectID,
		AuthKey:           resolved.AuthKey,
		AuthFingerprint:   authFingerprint,
		CustomSource:      resolved.CustomSource,
		ModuleID:          currentModule,
		Env:               env,
		AllowStaleOnError: &allowStale,
		Context:           cmd.Context(),
		FetchFunc:         dependencies.FetchFunc,
	})
	if err != nil {
		return err
	}

	var typesCode string
	if parsed.method != "" {
		typesCode, err = typesgen.GenTypesForOperation(loadResult.Data, parsed.method, parsed.path)
	} else {
		typesCode, err = typesgen.GenTypesForAllOperationsOnPath(loadResult.Data, parsed.path)
	}
	if err != nil {
		message := err.Error()
		if strings.Contains(message, "没有可用的 HTTP method") {
			return fmt.Errorf(
				"未找到接口 %s 下的可用方法，可先运行 `apifox-api search <关键词>` 获取完整的 method + path",
				parsed.path,
			)
		}
		if strings.Contains(message, "未找到") {
			if parsed.method != "" {
				return fmt.Errorf(
					"未找到接口 %s %s，可先运行 `apifox-api search <关键词>` 获取完整的 method + path",
					strings.ToUpper(parsed.method),
					parsed.path,
				)
			}
			return fmt.Errorf("%s", message)
		}
		return fmt.Errorf("apifox-api get 失败: %s", message)
	}

	// Stale warning goes to stderr; stdout is pure TypeScript.
	if loadResult.Warning != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "警告: %s\n", loadResult.Warning)
	}

	if !strings.HasSuffix(typesCode, "\n") {
		typesCode += "\n"
	}
	_, err = fmt.Fprint(cmd.OutOrStdout(), typesCode)
	return err
}
