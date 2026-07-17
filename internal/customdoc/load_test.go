package customdoc

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFileURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "swagger docs.json")
	if err := os.WriteFile(path, []byte(`{"swagger":"2.0","paths":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	source := (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()

	loaded, err := Load(context.Background(), source, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Source != filepath.Clean(path) {
		t.Fatalf("source = %q, want %q", loaded.Source, filepath.Clean(path))
	}
	if !strings.Contains(string(loaded.Data), `"swagger":"2.0"`) {
		t.Fatalf("data = %s", loaded.Data)
	}
}

func TestDisplaySourceRedactsURLSecrets(t *testing.T) {
	got := DisplaySource("https://user:password@example.com/openapi.json?token=secret#section")
	want := "https://example.com/openapi.json?<redacted>"
	if got != want {
		t.Fatalf("DisplaySource() = %q, want %q", got, want)
	}
}
