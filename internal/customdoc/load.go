// Package customdoc loads user-provided OpenAPI documents from custom sources.
package customdoc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const maxDocumentBytes = 50 << 20

// Loaded is a custom document together with its canonical, persistable source.
type Loaded struct {
	Source string
	Data   []byte
}

// Load reads a custom document. Relative file paths resolve from baseDir.
func Load(ctx context.Context, source string, baseDir string) (Loaded, error) {
	if err := ctx.Err(); err != nil {
		return Loaded{}, err
	}

	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return Loaded{}, fmt.Errorf("--custom 不能为空，请提供 HTTP(S) URL 或本地文件路径。")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return Loaded{}, fmt.Errorf("解析自定义接口文档地址失败: %w", err)
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme == "http" || parsed.Scheme == "https" {
		parsed.Fragment = ""
		return loadHTTP(ctx, parsed)
	}

	path := trimmed
	if parsed.Scheme == "file" {
		if parsed.Host != "" && parsed.Host != "localhost" {
			return Loaded{}, fmt.Errorf("file URL 只支持本机路径，收到 host=%s。", parsed.Host)
		}
		path, err = url.PathUnescape(parsed.Path)
		if err != nil {
			return Loaded{}, fmt.Errorf("解析 file URL 失败: %w", err)
		}
		path = filepath.FromSlash(path)
		if runtime.GOOS == "windows" && len(path) >= 3 && path[0] == filepath.Separator && path[2] == ':' {
			path = path[1:]
		}
	} else if parsed.Scheme != "" && strings.Contains(trimmed, "://") {
		return Loaded{}, fmt.Errorf("--custom 只支持 HTTP(S) URL、file URL 或本地文件路径。")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(baseDir, path)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return Loaded{}, fmt.Errorf("解析自定义接口文档路径失败: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return Loaded{}, fmt.Errorf("读取自定义接口文档失败: %s: %w", path, err)
	}
	if info.IsDir() {
		return Loaded{}, fmt.Errorf("读取自定义接口文档失败: %s 是目录，不是文件。", path)
	}
	if info.Size() > maxDocumentBytes {
		return Loaded{}, fmt.Errorf("自定义接口文档超过 %d 字节上限。", maxDocumentBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Loaded{}, fmt.Errorf("读取自定义接口文档失败: %s: %w", path, err)
	}
	if len(data) > maxDocumentBytes {
		return Loaded{}, fmt.Errorf("自定义接口文档超过 %d 字节上限。", maxDocumentBytes)
	}
	return Loaded{Source: filepath.Clean(path), Data: data}, nil
}

func loadHTTP(ctx context.Context, source *url.URL) (Loaded, error) {
	client := &http.Client{
		Timeout: 60 * time.Second,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("自定义接口文档 URL 重定向超过 10 次")
			}
			return nil
		},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source.String(), nil)
	if err != nil {
		return Loaded{}, fmt.Errorf("创建自定义接口文档请求失败: %w", err)
	}
	request.Header.Set("Accept", "application/json, application/openapi+json")
	response, err := client.Do(request)
	if err != nil {
		if urlError, ok := err.(*url.Error); ok {
			err = urlError.Err
		}
		return Loaded{}, fmt.Errorf("读取自定义接口文档 URL 失败: %s: %w", DisplaySource(source.String()), err)
	}
	defer response.Body.Close()

	data, err := io.ReadAll(io.LimitReader(response.Body, maxDocumentBytes+1))
	if err != nil {
		return Loaded{}, fmt.Errorf("读取自定义接口文档 URL 失败: %w", err)
	}
	if len(data) > maxDocumentBytes {
		return Loaded{}, fmt.Errorf("自定义接口文档超过 %d 字节上限。", maxDocumentBytes)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return Loaded{}, fmt.Errorf("读取自定义接口文档 URL 失败: HTTP %d", response.StatusCode)
	}
	return Loaded{Source: source.String(), Data: data}, nil
}

// DisplaySource removes URL credentials and query values before user-facing output.
func DisplaySource(source string) string {
	parsed, err := url.Parse(source)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return source
	}
	hadQuery := parsed.RawQuery != ""
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	display := parsed.String()
	if hadQuery {
		display += "?<redacted>"
	}
	return display
}
