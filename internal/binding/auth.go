package binding

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// AuthFingerprint returns the first 16 hex chars of SHA-256(authKey).
func AuthFingerprint(authKey string) string {
	sum := sha256.Sum256([]byte(authKey))
	return hex.EncodeToString(sum[:])[:16]
}

// ResolveRuntimeAuthKey implements runtime precedence: env → binding → global.
func ResolveRuntimeAuthKey(env map[string]string, storedAuthKey string, globalAuthKey string) (authKey string, fingerprint string, usedEnv bool, err error) {
	if envKey := strings.TrimSpace(env[EnvAuthKey]); envKey != "" {
		return envKey, AuthFingerprint(envKey), true, nil
	}
	if stored := strings.TrimSpace(storedAuthKey); stored != "" {
		return stored, AuthFingerprint(stored), false, nil
	}
	if global := strings.TrimSpace(globalAuthKey); global != "" {
		return global, AuthFingerprint(global), false, nil
	}
	return "", "", false, fmt.Errorf("%s", missingAuthKeyMessage)
}

const missingAuthKeyMessage = `未配置 Apifox Auth Key。可选配置方式（优先级从高到低）：
  1. 环境变量 APIFOX_AUTH_KEY（覆盖一切）
  2. 在当前目录运行 ` + "`apifox-api init <projectId> --authKey <token>`" + `（仅对当前项目生效）
  3. 运行 ` + "`apifox-api config set-auth-key <token>`" + `（全局默认，所有项目共享）`

// ResolveInitAuthKey implements init precedence:
// flag → env → exact existing binding → global (prefetch only, never persisted).
func ResolveInitAuthKey(flagAuthKey string, env map[string]string, existingBinding *RegistryBinding, globalAuthKey string) InitAuthKeyResolution {
	var resolution InitAuthKeyResolution

	if flag := strings.TrimSpace(flagAuthKey); flag != "" {
		resolution.PersistAuthKey = flag
		resolution.PrefetchAuthKey = flag
		resolution.PrefetchFingerprint = AuthFingerprint(flag)
		resolution.PrefetchSource = "命令行参数"
		return resolution
	}

	if envKey := strings.TrimSpace(env[EnvAuthKey]); envKey != "" {
		resolution.PersistAuthKey = envKey
		resolution.PrefetchAuthKey = envKey
		resolution.PrefetchFingerprint = AuthFingerprint(envKey)
		resolution.PrefetchSource = "环境变量"
		return resolution
	}

	if existingBinding != nil {
		if stored := strings.TrimSpace(existingBinding.AuthKey); stored != "" {
			resolution.PersistAuthKey = stored
			resolution.PrefetchAuthKey = stored
			resolution.PrefetchFingerprint = AuthFingerprint(stored)
			resolution.PrefetchSource = "已有绑定"
			return resolution
		}
	}

	if global := strings.TrimSpace(globalAuthKey); global != "" {
		// Global key is for prefetch only — never written into the binding.
		resolution.PrefetchAuthKey = global
		resolution.PrefetchFingerprint = AuthFingerprint(global)
		resolution.PrefetchSource = "全局默认"
		return resolution
	}

	return resolution
}
