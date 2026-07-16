package openapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadFixture(t *testing.T) []Endpoint {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "sample-openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	endpoints, err := BuildIndex(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(endpoints) < 25 {
		t.Fatalf("fixture too small: %d endpoints", len(endpoints))
	}
	return endpoints
}

func TestBuildIndexFlattensPathsAndSchemaNames(t *testing.T) {
	endpoints := loadFixture(t)
	var listUsers *Endpoint
	for i := range endpoints {
		if endpoints[i].OperationID == "listUsers" {
			listUsers = &endpoints[i]
			break
		}
	}
	if listUsers == nil {
		t.Fatal("listUsers not found")
	}
	if listUsers.Method != "GET" || listUsers.Path != "/users" {
		t.Fatalf("unexpected endpoint: %+v", listUsers)
	}
	if len(listUsers.SchemaNames) == 0 || listUsers.SchemaNames[0] != "UserList" {
		t.Fatalf("expected UserList schema, got %v", listUsers.SchemaNames)
	}
}

func TestSearchEmptyQueryRejected(t *testing.T) {
	endpoints := loadFixture(t)
	_, err := SearchWindow(endpoints, SearchOptions{})
	if err == nil {
		t.Fatal("expected empty query rejection")
	}
	if !strings.Contains(err.Error(), "keywords") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSearchORDefaultAndMultiField(t *testing.T) {
	endpoints := loadFixture(t)
	// path match
	pathHit, err := SearchWindow(endpoints, SearchOptions{Keywords: []string{"/orders"}})
	if err != nil {
		t.Fatal(err)
	}
	if pathHit.Total < 1 {
		t.Fatal("expected path match")
	}
	// operationId (exact-ish full token should rank first; subword recall may add more)
	opHit, err := SearchWindow(endpoints, SearchOptions{Keywords: []string{"listUsers"}})
	if err != nil {
		t.Fatal(err)
	}
	if opHit.Total < 1 {
		t.Fatalf("operationId total=%d", opHit.Total)
	}
	if opHit.Items[0].OperationID != "listUsers" {
		t.Fatalf("expected listUsers first, got %+v", opHit.Items[0])
	}
	// tags
	tagHit, err := SearchWindow(endpoints, SearchOptions{Keywords: []string{"Order"}})
	if err != nil {
		t.Fatal(err)
	}
	if tagHit.Total < 1 {
		t.Fatal("expected tag match")
	}
	// description
	descHit, err := SearchWindow(endpoints, SearchOptions{Keywords: []string{"tenant"}})
	if err != nil {
		t.Fatal(err)
	}
	if descHit.Total < 1 {
		t.Fatal("expected description match")
	}
	// schema names
	schemaHit, err := SearchWindow(endpoints, SearchOptions{Keywords: []string{"CatalogItem"}})
	if err != nil {
		t.Fatal(err)
	}
	if schemaHit.Total < 1 {
		t.Fatal("expected schema name match")
	}
	// OR: either keyword
	orHit, err := SearchWindow(endpoints, SearchOptions{Keywords: []string{"listUsers", "getOrder"}})
	if err != nil {
		t.Fatal(err)
	}
	if orHit.Total < 2 {
		t.Fatalf("OR total=%d want >=2", orHit.Total)
	}
	// AND
	andHit, err := SearchWindow(endpoints, SearchOptions{
		Keywords: []string{"user", "list"},
		Mode:     "and",
	})
	if err != nil {
		t.Fatal(err)
	}
	if andHit.Total < 1 {
		t.Fatal("expected AND match")
	}
}

func TestSearchWindowBoundsAndTruncation(t *testing.T) {
	endpoints := loadFixture(t)
	window, err := SearchWindow(endpoints, SearchOptions{Keywords: []string{"item"}})
	if err != nil {
		t.Fatal(err)
	}
	if window.Limit != DefaultSearchLimit {
		t.Fatalf("default limit=%d", window.Limit)
	}
	if window.Total <= DefaultSearchLimit {
		t.Fatalf("fixture should exceed default window, total=%d", window.Total)
	}
	if window.Showing != DefaultSearchLimit {
		t.Fatalf("showing=%d", window.Showing)
	}
	if !window.Truncated {
		t.Fatal("expected truncated")
	}

	limited, err := SearchWindow(endpoints, SearchOptions{Keywords: []string{"item"}, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if limited.Showing != 5 || !limited.Truncated || limited.Total <= 5 {
		t.Fatalf("unexpected window: %+v", limited)
	}

	_, err = SearchWindow(endpoints, SearchOptions{Keywords: []string{"item"}, Limit: 51})
	if err == nil {
		t.Fatal("expected limit>50 rejection")
	}
	_, err = SearchWindow(endpoints, SearchOptions{Keywords: []string{"item"}, Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFormatSearchResultsMarkdownShape(t *testing.T) {
	endpoints := loadFixture(t)
	window, err := SearchWindow(endpoints, SearchOptions{Keywords: []string{"user"}})
	if err != nil {
		t.Fatal(err)
	}
	md := FormatSearchResults(window, "")
	for _, needle := range []string{
		"共找到",
		"当前展示",
		"| 方法 | 路径 | 接口名称 | Tags |",
		"|---|---|---|---|",
	} {
		if !strings.Contains(md, needle) {
			t.Fatalf("missing %q in:\n%s", needle, md)
		}
	}
	if window.Total == 1 {
		if !strings.Contains(md, "唯一接口") {
			t.Fatal("expected unique guidance")
		}
	} else if !strings.Contains(md, "多个候选") && !strings.Contains(md, "截断") {
		t.Fatalf("expected multi/truncated guidance:\n%s", md)
	}

	empty := FormatSearchResults(SearchResultWindow{}, "")
	if !strings.Contains(empty, "未找到") {
		t.Fatalf("empty format: %s", empty)
	}

	stale := FormatSearchResults(window, "stale warning")
	if !strings.Contains(stale, "> 警告: stale warning") {
		t.Fatalf("missing stale warning:\n%s", stale)
	}
}

func TestSearchMethodOnly(t *testing.T) {
	endpoints := loadFixture(t)
	window, err := SearchWindow(endpoints, SearchOptions{Method: "post"})
	if err != nil {
		t.Fatal(err)
	}
	if window.Total < 1 {
		t.Fatal("expected POST matches")
	}
	for _, item := range window.Items {
		if item.Method != "POST" {
			t.Fatalf("non-POST item: %+v", item)
		}
	}
}
