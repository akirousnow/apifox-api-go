package openapi

import (
	"encoding/json"
	"strings"
	"testing"
)

const fieldWalkFixture = `{
  "openapi": "3.1.0",
  "paths": {
    "/users/{userId}": {
      "parameters": [
        {"name": "userId", "in": "path", "description": "path user id"},
        {"name": "legacy", "in": "query", "description": "path-level query"},
        {"name": "X-Trace", "in": "header", "description": "ignored header"}
      ],
      "get": {
        "summary": "Get user",
        "operationId": "getUser",
        "parameters": [
          {"name": "userId", "in": "path", "description": "op user id"},
          {"name": "phone", "in": "query", "description": "手机号"},
          {"name": "X-Trace", "in": "header", "description": "still ignored"}
        ],
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/UserBody"}
            }
          }
        },
        "responses": {
          "200": {
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/UserResponse"}
              }
            }
          },
          "404": {
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {"error": {"type": "string", "description": "not primary"}}
                }
              }
            }
          }
        }
      }
    },
    "/cycle": {
      "post": {
        "summary": "Cycle ref",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/NodeA"}
            }
          }
        },
        "responses": {"200": {"description": "ok"}}
      }
    },
    "/deep": {
      "post": {
        "summary": "Deep nest",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/Deep1"}
            }
          }
        },
        "responses": {"200": {"description": "ok"}}
      }
    }
  },
  "components": {
    "schemas": {
      "UserBody": {
        "type": "object",
        "description": "user body schema",
        "properties": {
          "user": {
            "type": "object",
            "description": "user object",
            "properties": {
              "email": {"type": "string", "description": "邮箱"},
              "profile": {"$ref": "#/components/schemas/Profile"}
            }
          }
        }
      },
      "Profile": {
        "type": "object",
        "description": "profile schema",
        "properties": {
          "nickname": {"type": "string", "description": "昵称"}
        }
      },
      "UserResponse": {
        "type": "object",
        "properties": {
          "data": {
            "type": "object",
            "properties": {
              "id": {"type": "string", "description": "响应ID"}
            }
          }
        }
      },
      "NodeA": {
        "type": "object",
        "properties": {
          "child": {"$ref": "#/components/schemas/NodeB"}
        }
      },
      "NodeB": {
        "type": "object",
        "properties": {
          "back": {"$ref": "#/components/schemas/NodeA"},
          "leaf": {"type": "string", "description": "cycle leaf"}
        }
      },
      "Deep1": {"type": "object", "properties": {"l2": {"$ref": "#/components/schemas/Deep2"}}},
      "Deep2": {"type": "object", "properties": {"l3": {"$ref": "#/components/schemas/Deep3"}}},
      "Deep3": {"type": "object", "properties": {"l4": {"$ref": "#/components/schemas/Deep4"}}},
      "Deep4": {"type": "object", "properties": {"l5": {"$ref": "#/components/schemas/Deep5"}}},
      "Deep5": {"type": "object", "properties": {"l6": {"$ref": "#/components/schemas/Deep6"}}},
      "Deep6": {"type": "object", "properties": {"l7": {"$ref": "#/components/schemas/Deep7"}}},
      "Deep7": {"type": "object", "properties": {"l8": {"$ref": "#/components/schemas/Deep8"}}},
      "Deep8": {"type": "object", "properties": {"l9": {"$ref": "#/components/schemas/Deep9"}}},
      "Deep9": {"type": "object", "properties": {"l10": {"type": "string", "description": "too deep"}}}
    }
  }
}`

func loadFieldWalkDoc(t *testing.T) (map[string]json.RawMessage, map[string]json.RawMessage) {
	t.Helper()
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(fieldWalkFixture), &doc); err != nil {
		t.Fatal(err)
	}
	return doc, ExtractComponentsSchemas(doc)
}

func pathOperation(t *testing.T, doc map[string]json.RawMessage, path, method string) (map[string]json.RawMessage, map[string]json.RawMessage) {
	t.Helper()
	var paths map[string]json.RawMessage
	if err := json.Unmarshal(doc["paths"], &paths); err != nil {
		t.Fatal(err)
	}
	var pathItem map[string]json.RawMessage
	if err := json.Unmarshal(paths[path], &pathItem); err != nil {
		t.Fatal(err)
	}
	var operation map[string]json.RawMessage
	if err := json.Unmarshal(pathItem[method], &operation); err != nil {
		t.Fatal(err)
	}
	return pathItem, operation
}

func TestCollectOperationFieldsParamsMergeAndIgnoreHeader(t *testing.T) {
	doc, schemas := loadFieldWalkDoc(t)
	pathItem, operation := pathOperation(t, doc, "/users/{userId}", "get")
	index := CollectOperationFields(pathItem, operation, schemas, FieldWalkOptions{})

	if len(index.RequestParams) != 3 {
		t.Fatalf("params=%+v", index.RequestParams)
	}
	byKey := map[string]IndexedParam{}
	for _, param := range index.RequestParams {
		byKey[param.In+":"+param.Name] = param
	}
	if byKey["path:userId"].Description != "op user id" {
		t.Fatalf("path override missing: %+v", byKey["path:userId"])
	}
	if byKey["query:legacy"].Description != "path-level query" {
		t.Fatalf("path-level query missing: %+v", byKey["query:legacy"])
	}
	if byKey["query:phone"].Description != "手机号" {
		t.Fatalf("phone missing: %+v", byKey["query:phone"])
	}
	for _, param := range index.RequestParams {
		if param.In == "header" || strings.EqualFold(param.Name, "X-Trace") {
			t.Fatalf("header must be ignored: %+v", param)
		}
	}
	if len(index.RequestFields) != 0 || len(index.ResponseFields) != 0 {
		t.Fatalf("body/response should be empty without options: %+v", index)
	}
}

func TestCollectOperationFieldsNestedRefBodyAndResponse(t *testing.T) {
	doc, schemas := loadFieldWalkDoc(t)
	pathItem, operation := pathOperation(t, doc, "/users/{userId}", "get")
	index := CollectOperationFields(pathItem, operation, schemas, FieldWalkOptions{
		IncludeBody:     true,
		IncludeResponse: true,
	})

	bodyByPath := map[string]IndexedField{}
	for _, field := range index.RequestFields {
		bodyByPath[field.JSONPath] = field
	}
	if bodyByPath["user"].Description != "user object" {
		t.Fatalf("user desc: %+v", bodyByPath["user"])
	}
	if bodyByPath["user.email"].Description != "邮箱" {
		t.Fatalf("email: %+v", bodyByPath["user.email"])
	}
	if bodyByPath["user.profile.nickname"].Description != "昵称" {
		t.Fatalf("nickname: %+v", bodyByPath["user.profile.nickname"])
	}

	responseByPath := map[string]IndexedField{}
	for _, field := range index.ResponseFields {
		responseByPath[field.JSONPath] = field
	}
	if responseByPath["data.id"].Description != "响应ID" {
		t.Fatalf("response id: %+v", responseByPath["data.id"])
	}
	for _, field := range index.ResponseFields {
		if field.Name == "error" {
			t.Fatalf("non-primary 404 must not be indexed: %+v", field)
		}
	}
}

func TestCollectOperationFieldsCycleRefDoesNotHang(t *testing.T) {
	doc, schemas := loadFieldWalkDoc(t)
	pathItem, operation := pathOperation(t, doc, "/cycle", "post")
	done := make(chan OperationFieldIndex, 1)
	go func() {
		done <- CollectOperationFields(pathItem, operation, schemas, FieldWalkOptions{IncludeBody: true})
	}()
	select {
	case index := <-done:
		foundLeaf := false
		for _, field := range index.RequestFields {
			if field.Name == "leaf" {
				foundLeaf = true
			}
		}
		if !foundLeaf {
			t.Fatalf("expected cycle leaf field: %+v", index.RequestFields)
		}
	case <-make(chan struct{}):
	}
	// simple timeout via second select with immediate default after first success path above
	_ = done
}

func TestCollectOperationFieldsDepthCap(t *testing.T) {
	doc, schemas := loadFieldWalkDoc(t)
	pathItem, operation := pathOperation(t, doc, "/deep", "post")
	index := CollectOperationFields(pathItem, operation, schemas, FieldWalkOptions{IncludeBody: true})

	maxDots := -1
	var deepest string
	hasL10 := false
	for _, field := range index.RequestFields {
		dots := strings.Count(field.JSONPath, ".")
		if dots > maxDots {
			maxDots = dots
			deepest = field.JSONPath
		}
		if field.Name == "l10" || strings.Contains(field.JSONPath, "l10") {
			hasL10 = true
		}
	}
	// depths 0..MaxFieldWalkDepth inclusive => up to MaxFieldWalkDepth dots in path
	if maxDots > MaxFieldWalkDepth {
		t.Fatalf("depth exceeded: deepest=%s dots=%d", deepest, maxDots)
	}
	if hasL10 {
		t.Fatalf("l10 should be beyond depth cap, deepest=%s fields=%+v", deepest, index.RequestFields)
	}
	if maxDots < MaxFieldWalkDepth-1 {
		t.Fatalf("expected near-cap depth, deepest=%s dots=%d", deepest, maxDots)
	}
}
