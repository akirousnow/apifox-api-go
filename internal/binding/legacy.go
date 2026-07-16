package binding

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ReadLegacyBindingForMigration reads the old workspace-local .apifox-mcp.json if present.
// Invalid or missing files return (nil, nil) so init can continue.
func ReadLegacyBindingForMigration(cwd string) (*LegacyBinding, error) {
	legacyPath := filepath.Join(cwd, LegacyBindingFileName)
	data, err := os.ReadFile(legacyPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, nil
	}
	projectIDRaw, ok := raw["projectId"]
	if !ok {
		return nil, nil
	}
	var projectID string
	if err := json.Unmarshal(projectIDRaw, &projectID); err != nil {
		return nil, nil
	}
	validated, err := ValidateProjectID(projectID)
	if err != nil {
		return nil, nil
	}
	legacy := &LegacyBinding{ProjectID: validated}
	if nameRaw, ok := raw["projectName"]; ok {
		var projectName string
		if err := json.Unmarshal(nameRaw, &projectName); err == nil {
			if trimmed := strings.TrimSpace(projectName); trimmed != "" {
				legacy.ProjectName = trimmed
			}
		}
	}
	return legacy, nil
}

// WriteCurrentModuleFile writes `.current-module` under bindingRoot with "<id>\n".
func WriteCurrentModuleFile(bindingRoot string, moduleID int) error {
	validated, err := ValidateModuleID(moduleID)
	if err != nil {
		return err
	}
	moduleFilePath := filepath.Join(bindingRoot, CurrentModuleFileName)
	return os.WriteFile(moduleFilePath, []byte(fmt.Sprintf("%d\n", validated)), 0o644)
}
