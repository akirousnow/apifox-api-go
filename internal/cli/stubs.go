package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"apifox-api/go-version/internal/binding"
)

func newModuleAwareStubCommand(dependencies Dependencies, use string, short string) *cobra.Command {
	var moduleIDFlag int
	var moduleIDFlagSet bool

	command := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModuleAwareStub(dependencies, cmd, moduleIDFlag, moduleIDFlagSet, use)
		},
	}
	command.Flags().IntVar(&moduleIDFlag, "moduleId", 0, "one-shot Current Module override (does not rewrite .current-module)")
	command.PreRun = func(cmd *cobra.Command, args []string) {
		moduleIDFlagSet = cmd.Flags().Changed("moduleId")
	}
	return command
}

func runModuleAwareStub(
	dependencies Dependencies,
	cmd *cobra.Command,
	moduleIDFlag int,
	moduleIDFlagSet bool,
	commandName string,
) error {
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
		return fmt.Errorf("apifox-api %s 失败: %w", commandName, err)
	}

	options := binding.ResolveCurrentModuleOptions{
		CWD:       cwd,
		HomeDir:   homeDir,
		ModuleIDs: resolved.ModuleIDs,
	}
	if moduleIDFlagSet {
		flagValue := moduleIDFlag
		options.ModuleIDFlag = &flagValue
	}

	currentModule, err := binding.ResolveCurrentModule(options)
	if err != nil {
		return fmt.Errorf("apifox-api %s 失败: %w", commandName, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "ok module=%s\n", binding.SummariseModuleID(currentModule))
	return nil
}
