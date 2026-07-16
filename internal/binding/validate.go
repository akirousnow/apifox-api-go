package binding

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var projectIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// ValidateProjectID trims and validates an Apifox project id.
func ValidateProjectID(projectID string) (string, error) {
	value := strings.TrimSpace(projectID)
	if value == "" {
		return "", fmt.Errorf("请提供 Apifox projectId！")
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return "", fmt.Errorf("请提供 Apifox projectId，而不是 Apifox 分享链接 URL。")
	}
	if !projectIDPattern.MatchString(value) {
		return "", fmt.Errorf("无效的 Apifox projectId，只能包含字母、数字、下划线或短横线。")
	}
	return value, nil
}

// ValidateModuleID ensures moduleId is a positive integer.
func ValidateModuleID(moduleID int) (int, error) {
	if moduleID <= 0 {
		return 0, fmt.Errorf("无效的 moduleId：%d。moduleId 必须是正整数。", moduleID)
	}
	return moduleID, nil
}

// ParseModuleIDs parses a comma-separated list of positive integers.
// Empty / omitted input means the Apifox default Module ([]).
func ParseModuleIDs(rawModuleIDs string) ([]int, error) {
	trimmed := strings.TrimSpace(rawModuleIDs)
	if trimmed == "" {
		return []int{}, nil
	}

	tokens := strings.Split(trimmed, ",")
	moduleIDs := make([]int, 0, len(tokens))
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		parsed, err := strconv.Atoi(token)
		if err != nil || parsed <= 0 || strconv.Itoa(parsed) != token {
			return nil, fmt.Errorf("无效的 moduleId：%s。--moduleIds 必须是逗号分隔的正整数。", token)
		}
		validated, err := ValidateModuleID(parsed)
		if err != nil {
			return nil, err
		}
		moduleIDs = append(moduleIDs, validated)
	}
	return moduleIDs, nil
}
