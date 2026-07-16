package binding

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ResolveCurrentModuleOptions controls Current Module resolution for search/get.
type ResolveCurrentModuleOptions struct {
	CWD          string
	HomeDir      string
	ModuleIDs    []int
	// ModuleIDFlag is an optional one-shot override from --moduleId.
	// nil means the flag was not provided.
	ModuleIDFlag *int
}

// ResolveCurrentModule returns the Module to use for a single command.
// A nil *int result means the Apifox default Module (moduleIds=[]).
// ModuleIDFlag is a one-shot override and never rewrites .current-module.
func ResolveCurrentModule(options ResolveCurrentModuleOptions) (*int, error) {
	moduleIDs := options.ModuleIDs
	if moduleIDs == nil {
		moduleIDs = []int{}
	}

	if len(moduleIDs) == 0 {
		if options.ModuleIDFlag != nil {
			return nil, fmt.Errorf(
				"当前项目只使用默认模块，不接受 --moduleId。收到的 --moduleId=%d。",
				*options.ModuleIDFlag,
			)
		}
		return nil, nil
	}

	if options.ModuleIDFlag != nil {
		validated, err := ValidateModuleID(*options.ModuleIDFlag)
		if err != nil {
			return nil, err
		}
		if !containsModuleID(moduleIDs, validated) {
			return nil, fmt.Errorf(
				"--moduleId %d 不在当前项目绑定的 moduleIds [%s] 内。",
				validated,
				joinModuleIDs(moduleIDs),
			)
		}
		return intPtr(validated), nil
	}

	if len(moduleIDs) == 1 {
		return intPtr(moduleIDs[0]), nil
	}

	cwd := options.CWD
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}
	homeDir := options.HomeDir
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return nil, err
		}
	}

	fileValue, err := findCurrentModuleFile(cwd, homeDir)
	if err != nil {
		return nil, err
	}
	if fileValue != "" {
		parsed, parseErr := strconv.Atoi(fileValue)
		if parseErr != nil || parsed <= 0 {
			return nil, fmt.Errorf(
				".current-module 文件内容无效: %s。请用 `apifox-api module <moduleId>` 重置。",
				fileValue,
			)
		}
		validated, validateErr := ValidateModuleID(parsed)
		if validateErr != nil {
			return nil, validateErr
		}
		if !containsModuleID(moduleIDs, validated) {
			return nil, fmt.Errorf(
				".current-module 指向的 moduleId %d 不在当前项目绑定的 moduleIds [%s] 内。请用 `apifox-api module <moduleId>` 切换。",
				validated,
				joinModuleIDs(moduleIDs),
			)
		}
		return intPtr(validated), nil
	}

	return nil, fmt.Errorf(
		"当前项目绑定了多个 module，但未指定当前 module。\n绑定的 moduleIds: [%s]\n请先选择一个 module：\n  apifox-api module <moduleId>\n或在命令中临时指定：\n  --moduleId <moduleId>",
		joinModuleIDs(moduleIDs),
	)
}

// ModulesForRefresh returns every Module that an explicit refresh should update.
// This is independent of Current Module. Empty moduleIds means one default-module unit
// represented as a single nil entry in the optional sense — callers should treat
// len==0 as "refresh the default Module once". This function returns a copy of
// moduleIDs (possibly empty).
func ModulesForRefresh(moduleIDs []int) []int {
	if len(moduleIDs) == 0 {
		return []int{}
	}
	out := make([]int, len(moduleIDs))
	copy(out, moduleIDs)
	return out
}

// FormatModuleIDs formats bound module IDs for user-facing output.
func FormatModuleIDs(moduleIDs []int) string {
	if len(moduleIDs) == 0 {
		return "[]（默认模块）"
	}
	return "[" + joinModuleIDs(moduleIDs) + "]"
}

// SummariseModuleID formats a resolved Current Module for display.
// nil means the Apifox default Module.
func SummariseModuleID(moduleID *int) string {
	if moduleID == nil {
		return "默认模块"
	}
	return fmt.Sprintf("moduleId=%d", *moduleID)
}

func findCurrentModuleFile(cwd string, homeDir string) (string, error) {
	workspaceKey, err := NormaliseWorkspaceKey(cwd)
	if err != nil {
		return "", err
	}
	homeKey, err := NormaliseWorkspaceKey(homeDir)
	if err != nil {
		return "", err
	}
	for _, candidateKey := range AncestorKeys(workspaceKey, homeKey) {
		candidatePath := filepath.Join(candidateKey, CurrentModuleFileName)
		data, readErr := os.ReadFile(candidatePath)
		if readErr != nil {
			if errors.Is(readErr, fs.ErrNotExist) || os.IsNotExist(readErr) {
				continue
			}
			return "", readErr
		}
		return strings.TrimSpace(string(data)), nil
	}
	return "", nil
}

func containsModuleID(moduleIDs []int, target int) bool {
	for _, moduleID := range moduleIDs {
		if moduleID == target {
			return true
		}
	}
	return false
}

func joinModuleIDs(moduleIDs []int) string {
	parts := make([]string, len(moduleIDs))
	for i, moduleID := range moduleIDs {
		parts[i] = strconv.Itoa(moduleID)
	}
	return strings.Join(parts, ", ")
}

func intPtr(value int) *int {
	return &value
}

// ReadCurrentModuleFile walks up from cwd toward home and returns the trimmed
// contents of the first .current-module file found. Empty string means not found.
func ReadCurrentModuleFile(cwd string, homeDir string) (string, error) {
	return findCurrentModuleFile(cwd, homeDir)
}

// ReadCurrentModuleFile walks up from cwd toward home and returns trimmed .current-module contents.
