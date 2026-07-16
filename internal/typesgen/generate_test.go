package typesgen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sampleOpenAPI() json.RawMessage {
	return json.RawMessage(`{
  "openapi": "3.0.1",
  "paths": {
    "/users/{id}": {
      "parameters": [
        {
          "name": "id",
          "in": "path",
          "required": true,
          "description": "user id",
          "schema": { "type": "string" }
        }
      ],
      "get": {
        "operationId": "getUser",
        "summary": "Get user",
        "parameters": [
          {
            "name": "verbose",
            "in": "query",
            "required": false,
            "description": "include details",
            "schema": { "type": "boolean" }
          },
          {
            "name": "X-Trace",
            "in": "header",
            "required": false,
            "schema": { "type": "string" }
          },
          {
            "name": "session",
            "in": "cookie",
            "required": false,
            "schema": { "type": "string" }
          }
        ],
        "responses": {
          "200": {
            "description": "ok",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/User" }
              }
            }
          }
        }
      },
      "post": {
        "operationId": "createUser",
        "summary": "Create user",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/UserInput" }
            }
          }
        },
        "responses": {
          "201": {
            "description": "created",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/User" }
              }
            }
          }
        }
      },
      "delete": {
        "operationId": "deleteUser",
        "summary": "Delete user",
        "responses": {
          "204": {
            "description": "no content"
          }
        }
      }
    },
    "/health": {
      "get": {
        "operationId": "healthCheck",
        "summary": "Health",
        "responses": {
          "200": {
            "description": "ok",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "status": { "type": "string", "enum": ["up", "down"] },
                    "meta": {
                      "type": "object",
                      "properties": {
                        "version": { "type": "string", "description": "build version" }
                      }
                    },
                    "tags": {
                      "type": "array",
                      "items": { "type": "string" }
                    },
                    "nullableName": { "type": "string", "nullable": true }
                  },
                  "required": ["status"]
                }
              }
            }
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "User": {
        "type": "object",
        "properties": {
          "id": { "type": "string", "description": "primary key" },
          "name": { "type": "string" },
          "profile": { "$ref": "#/components/schemas/Profile" }
        },
        "required": ["id", "name"]
      },
      "UserInput": {
        "type": "object",
        "properties": {
          "name": { "type": "string" },
          "profile": { "$ref": "#/components/schemas/Profile" }
        },
        "required": ["name"]
      },
      "Profile": {
        "type": "object",
        "properties": {
          "bio": { "type": "string" },
          "age": { "type": "integer", "nullable": true }
        }
      }
    }
  }
}`)
}

func TestGetOperationContext_MissingPath(t *testing.T) {
	_, err := GetOperationContext(sampleOpenAPI(), "get", "/missing")
	if err == nil || !strings.Contains(err.Error(), "未找到接口路径") {
		t.Fatalf("expected missing path error, got %v", err)
	}
}

func TestGetOperationContext_InvalidMethod(t *testing.T) {
	_, err := GetOperationContext(sampleOpenAPI(), "FOO", "/health")
	if err == nil || !strings.Contains(err.Error(), "无效的 HTTP method") {
		t.Fatalf("expected invalid method error, got %v", err)
	}
}

func TestGetOperationContext_MethodNotOnPath(t *testing.T) {
	_, err := GetOperationContext(sampleOpenAPI(), "put", "/users/{id}")
	if err == nil || !strings.Contains(err.Error(), "该路径可用方法") {
		t.Fatalf("expected available methods error, got %v", err)
	}
}

func TestGenTypesForOperation_ParametersAndRefs(t *testing.T) {
	output, err := GenTypesForOperation(sampleOpenAPI(), "get", "/users/{id}")
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, output, "// ====== 请求参数 ======")
	assertContains(t, output, "// Query 参数（URL 问号后）")
	assertContains(t, output, "// Path 参数（URL 路径占位）")
	assertContains(t, output, "// Header 参数（请求头）")
	assertContains(t, output, "// Cookie 参数")
	assertContains(t, output, "export interface GetUserQuery")
	assertContains(t, output, "export interface GetUserPathParams")
	assertContains(t, output, "export type GetUserResponse = User;")
	assertContains(t, output, "// ====== 关联类型定义 ======")
	assertContains(t, output, "export interface User {")
	assertContains(t, output, "export interface Profile {")
	// User before Profile (BFS enqueue order: User first, then Profile)
	userIdx := strings.Index(output, "export interface User {")
	profileIdx := strings.Index(output, "export interface Profile {")
	if userIdx < 0 || profileIdx < 0 || userIdx > profileIdx {
		t.Fatalf("expected User before Profile in BFS order")
	}
	// property order for User: id, name, profile (scope to User interface body)
	userStart := strings.Index(output, "export interface User {")
	userEnd := strings.Index(output[userStart:], "export interface Profile {")
	if userStart < 0 || userEnd < 0 {
		t.Fatalf("missing User/Profile interfaces")
	}
	userBody := output[userStart : userStart+userEnd]
	idIdx := strings.Index(userBody, "id:")
	nameIdx := strings.Index(userBody, "name:")
	profileFieldIdx := strings.Index(userBody, "profile")
	if !(idIdx >= 0 && nameIdx >= 0 && profileFieldIdx >= 0 && idIdx < nameIdx && nameIdx < profileFieldIdx) {
		t.Fatalf("expected ordered properties id, name, profile in User; idxs id=%d name=%d profile=%d body=%q", idIdx, nameIdx, profileFieldIdx, userBody)
	}
}

func TestGenTypesForAllOperationsOnPath_OrderAndSharedRefs(t *testing.T) {
	output, err := GenTypesForAllOperationsOnPath(sampleOpenAPI(), "/users/{id}")
	if err != nil {
		t.Fatal(err)
	}
	// method separators ordered GET, POST, DELETE
	getSep := strings.Index(output, "// ═════════ GET /users/{id} ═════════")
	postSep := strings.Index(output, "// ═════════ POST /users/{id} ═════════")
	deleteSep := strings.Index(output, "// ═════════ DELETE /users/{id} ═════════")
	if getSep < 0 || postSep < 0 || deleteSep < 0 {
		t.Fatalf("missing method separators:\n%s", output)
	}
	if !(getSep < postSep && postSep < deleteSep) {
		t.Fatalf("methods not ordered GET/POST/DELETE")
	}
	// shared User/Profile emitted once
	if strings.Count(output, "export interface User {") != 1 {
		t.Fatalf("User should be emitted once, got %d", strings.Count(output, "export interface User {"))
	}
	if strings.Count(output, "export interface Profile {") != 1 {
		t.Fatalf("Profile should be emitted once")
	}
	assertContains(t, output, "export type CreateUserRequestBody = UserInput;")
	assertContains(t, output, "export type CreateUserResponse = User;")
	assertContains(t, output, "// 未找到 2xx JSON 响应，未生成 Response 类型。")
}

func TestGenTypesForOperation_InlineEnumNullableAndMultiline(t *testing.T) {
	output, err := GenTypesForOperation(sampleOpenAPI(), "GET", "/health")
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, output, `"up" | "down"`)
	assertContains(t, output, "Array<string>")
	assertContains(t, output, "string | null")
	if !strings.Contains(output, "version") {
		t.Fatalf("expected version field in output:\n%s", output)
	}
	assertContains(t, output, "export type HealthCheckResponse")
}

func TestGenTypesForOperation_OrderingStability(t *testing.T) {
	first, err := GenTypesForOperation(sampleOpenAPI(), "get", "/users/{id}")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		next, err := GenTypesForOperation(sampleOpenAPI(), "get", "/users/{id}")
		if err != nil {
			t.Fatal(err)
		}
		if next != first {
			t.Fatalf("non-deterministic output on iteration %d", i)
		}
	}
}

func TestGenTypes_GoldenFiles(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
		all    bool
		file   string
	}{
		{name: "single-get-user", method: "get", path: "/users/{id}", file: "single_get_user.ts"},
		{name: "path-only-users", path: "/users/{id}", all: true, file: "path_only_users.ts"},
		{name: "health", method: "get", path: "/health", file: "health.ts"},
	}

	goldenDir := filepath.Join("testdata", "goldens")
	if err := os.MkdirAll(goldenDir, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var output string
			var err error
			if testCase.all {
				output, err = GenTypesForAllOperationsOnPath(sampleOpenAPI(), testCase.path)
			} else {
				output, err = GenTypesForOperation(sampleOpenAPI(), testCase.method, testCase.path)
			}
			if err != nil {
				t.Fatal(err)
			}
			goldenPath := filepath.Join(goldenDir, testCase.file)
			if os.Getenv("UPDATE_GOLDEN") == "1" {
				if err := os.WriteFile(goldenPath, []byte(output+"\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			expectedBytes, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden %s: %v (set UPDATE_GOLDEN=1 to create)", goldenPath, err)
			}
			expected := strings.TrimSuffix(string(expectedBytes), "\n")
			if output != expected {
				t.Fatalf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", testCase.file, output, expected)
			}
		})
	}
}

func assertContains(t *testing.T, haystack string, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("expected output to contain %q\n--- output ---\n%s", needle, haystack)
	}
}
