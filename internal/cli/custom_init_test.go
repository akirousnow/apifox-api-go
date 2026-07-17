package cli_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/akirousnow/apifox-api-go/internal/binding"
	"github.com/akirousnow/apifox-api-go/internal/cli"
)

const customSwagger2Fixture = `{
  "swagger": "2.0",
  "info": {"title": "Admin Scaffold API", "version": "1.0"},
  "paths": {
    "/api/v1/admin/admin-users": {
      "post": {
        "summary": "Create admin user",
        "consumes": ["application/json"],
        "produces": ["application/json"],
        "parameters": [
          {
            "name": "body",
            "in": "body",
            "required": true,
            "schema": {"$ref": "#/definitions/CreateAdminUserRequest"}
          }
        ],
        "responses": {
          "200": {
            "description": "OK",
            "schema": {"$ref": "#/definitions/AdminUserEnvelope"}
          }
        }
      }
    }
  },
  "definitions": {
    "CreateAdminUserRequest": {
      "type": "object",
      "required": ["username"],
      "properties": {
        "username": {"type": "string"},
        "enabled": {"type": "boolean"}
      }
    },
    "AdminUserEnvelope": {
      "type": "object",
      "required": ["data"],
      "properties": {
        "data": {"$ref": "#/definitions/AdminUserResponse"}
      }
    },
    "AdminUserResponse": {
      "type": "object",
      "required": ["id"],
      "properties": {"id": {"type": "integer"}}
    }
  }
}`

func TestInitCustomLocalSwagger2MakesGetTypesAvailable(t *testing.T) {
	dependencies, initOutput := initCustomSwagger2Fixture(t)

	if !strings.Contains(initOutput, "已缓存自定义接口文档") {
		t.Fatalf("init stdout missing custom snapshot confirmation:\n%s", initOutput)
	}

	var getOut bytes.Buffer
	var getErr bytes.Buffer
	dependencies.Streams = cli.Streams{Out: &getOut, Err: &getErr}
	if err := cli.Execute(context.Background(), dependencies, []string{
		"get", "POST", "/api/v1/admin/admin-users",
	}); err != nil {
		t.Fatalf("get failed: %v\nstderr=%s", err, getErr.String())
	}

	output := getOut.String()
	for _, expected := range []string{
		"export type CreateadminuserRequestBody = CreateAdminUserRequest;",
		"export type CreateadminuserResponse = AdminUserEnvelope;",
		"export interface CreateAdminUserRequest",
		"username: string;",
		"export interface AdminUserEnvelope",
		"data: AdminUserResponse;",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("get output missing %q:\n%s", expected, output)
		}
	}
}

func TestInitCustomLocalSwagger2MakesSearchAvailable(t *testing.T) {
	dependencies, _ := initCustomSwagger2Fixture(t)

	var searchOut bytes.Buffer
	var searchErr bytes.Buffer
	dependencies.Streams = cli.Streams{Out: &searchOut, Err: &searchErr}
	if err := cli.Execute(context.Background(), dependencies, []string{
		"search", "Create", "admin", "--mode", "and",
	}); err != nil {
		t.Fatalf("search failed: %v\nstderr=%s", err, searchErr.String())
	}

	output := searchOut.String()
	if !strings.Contains(output, "POST") || !strings.Contains(output, "/api/v1/admin/admin-users") {
		t.Fatalf("search did not return the custom operation:\n%s", output)
	}
}

func TestInitCustomLocalSwagger2MakesFieldSearchAvailable(t *testing.T) {
	dependencies, _ := initCustomSwagger2Fixture(t)

	var searchOut bytes.Buffer
	var searchErr bytes.Buffer
	dependencies.Streams = cli.Streams{Out: &searchOut, Err: &searchErr}
	if err := cli.Execute(context.Background(), dependencies, []string{
		"search-fields", "username",
	}); err != nil {
		t.Fatalf("search-fields failed: %v\nstderr=%s", err, searchErr.String())
	}

	output := searchOut.String()
	if !strings.Contains(output, "/api/v1/admin/admin-users") || !strings.Contains(output, "body.username") {
		t.Fatalf("search-fields did not return the normalized body field:\n%s", output)
	}
}

func TestInitCustomURLCachesSwagger2ForOfflineGet(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requestCount.Add(1)
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(customSwagger2Fixture))
	}))

	homeDir := t.TempDir()
	workspace := filepath.Join(homeDir, "app")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	dependencies := cli.Dependencies{CWD: workspace, HomeDir: homeDir, Env: map[string]string{}}
	var initOut bytes.Buffer
	var initErr bytes.Buffer
	dependencies.Streams = cli.Streams{Out: &initOut, Err: &initErr}
	if err := cli.Execute(context.Background(), dependencies, []string{
		"init", "--custom", server.URL + "/swagger-docs.json",
	}); err != nil {
		t.Fatalf("init URL failed: %v\nstderr=%s", err, initErr.String())
	}
	server.Close()

	if requestCount.Load() != 1 {
		t.Fatalf("custom URL requests = %d, want 1", requestCount.Load())
	}
	var getOut bytes.Buffer
	var getErr bytes.Buffer
	dependencies.Streams = cli.Streams{Out: &getOut, Err: &getErr}
	if err := cli.Execute(context.Background(), dependencies, []string{
		"get", "POST", "/api/v1/admin/admin-users",
	}); err != nil {
		t.Fatalf("offline get failed: %v\nstderr=%s", err, getErr.String())
	}
	if !strings.Contains(getOut.String(), "export type CreateadminuserResponse = AdminUserEnvelope;") {
		t.Fatalf("offline get did not use cached URL document:\n%s", getOut.String())
	}
}

func TestRefreshCustomURLReplacesSnapshotWithoutAuth(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		count := requestCount.Add(1)
		document := customSwagger2Fixture
		if count > 1 {
			document = strings.Replace(document, "Create admin user", "Create refreshed admin user", 1)
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(document))
	}))
	defer server.Close()

	homeDir := t.TempDir()
	workspace := filepath.Join(homeDir, "app")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	dependencies := cli.Dependencies{CWD: workspace, HomeDir: homeDir, Env: map[string]string{}}
	dependencies.Streams = cli.Streams{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	if err := cli.Execute(context.Background(), dependencies, []string{
		"init", "--custom", server.URL + "/swagger-docs.json",
	}); err != nil {
		t.Fatalf("init URL failed: %v", err)
	}

	var refreshOut bytes.Buffer
	var refreshErr bytes.Buffer
	dependencies.Streams = cli.Streams{Out: &refreshOut, Err: &refreshErr}
	if err := cli.Execute(context.Background(), dependencies, []string{"refresh"}); err != nil {
		t.Fatalf("refresh failed: %v\nstderr=%s", err, refreshErr.String())
	}
	if requestCount.Load() != 2 {
		t.Fatalf("custom URL requests = %d, want 2 after refresh", requestCount.Load())
	}

	var searchOut bytes.Buffer
	dependencies.Streams = cli.Streams{Out: &searchOut, Err: &bytes.Buffer{}}
	if err := cli.Execute(context.Background(), dependencies, []string{"search", "refreshed"}); err != nil {
		t.Fatalf("search refreshed document failed: %v", err)
	}
	if !strings.Contains(searchOut.String(), "Create refreshed admin user") {
		t.Fatalf("search did not see refreshed custom document:\n%s", searchOut.String())
	}
}

func TestInitCustomRejectsDocumentWithoutPaths(t *testing.T) {
	homeDir := t.TempDir()
	workspace := filepath.Join(homeDir, "app")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	documentPath := filepath.Join(workspace, "invalid.json")
	if err := os.WriteFile(documentPath, []byte(`{
  "swagger": "2.0",
  "info": {"title": "Missing paths", "version": "1.0"}
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	err := cli.Execute(context.Background(), cli.Dependencies{
		Streams: cli.Streams{Out: &bytes.Buffer{}, Err: &stderr},
		CWD:     workspace,
		HomeDir: homeDir,
		Env:     map[string]string{},
	}, []string{"init", "--custom", documentPath})
	if err == nil || !strings.Contains(err.Error(), "缺少 paths") {
		t.Fatalf("expected missing paths error, got err=%v stderr=%s", err, stderr.String())
	}

	registry, readErr := binding.ReadGlobalRegistry(homeDir)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(registry.Bindings) != 0 {
		t.Fatalf("invalid custom document must not create binding: %+v", registry.Bindings)
	}
}

func initCustomSwagger2Fixture(t *testing.T) (cli.Dependencies, string) {
	t.Helper()
	homeDir := t.TempDir()
	workspace := filepath.Join(homeDir, "app")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}

	documentPath := filepath.Join(workspace, "swagger-docs.json")
	if err := os.WriteFile(documentPath, []byte(customSwagger2Fixture), 0o644); err != nil {
		t.Fatal(err)
	}

	dependencies := cli.Dependencies{
		CWD:     workspace,
		HomeDir: homeDir,
		Env:     map[string]string{},
	}
	var initOut bytes.Buffer
	var initErr bytes.Buffer
	dependencies.Streams = cli.Streams{Out: &initOut, Err: &initErr}
	if err := cli.Execute(context.Background(), dependencies, []string{"init", "--custom", documentPath}); err != nil {
		t.Fatalf("init --custom failed: %v\nstderr=%s", err, initErr.String())
	}
	return dependencies, initOut.String()
}
