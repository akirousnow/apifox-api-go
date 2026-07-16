package openapi

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildFieldIndexAndSearchByFields(t *testing.T) {
	endpoints, err := BuildFieldIndex([]byte(fieldWalkFixture))
	if err != nil {
		t.Fatal(err)
	}
	if len(endpoints) < 3 {
		t.Fatalf("endpoints=%d", len(endpoints))
	}

	var getUser *FieldEndpoint
	for i := range endpoints {
		if endpoints[i].OperationID == "getUser" {
			getUser = &endpoints[i]
			break
		}
	}
	if getUser == nil {
		t.Fatal("getUser missing")
	}
	if len(getUser.Fields.RequestParams) == 0 {
		t.Fatal("expected params")
	}
	if len(getUser.Fields.RequestFields) == 0 {
		t.Fatal("expected body fields")
	}
	if len(getUser.Fields.ResponseFields) == 0 {
		t.Fatal("expected response fields")
	}

	window, err := SearchByFields(endpoints, SearchOptions{Keywords: []string{"phone"}})
	if err != nil {
		t.Fatal(err)
	}
	if window.Total != 1 {
		t.Fatalf("phone total=%d", window.Total)
	}
	hits := FormatMatchedFields(window.Items[0].Matches)
	if !strings.Contains(hits, "query.phone") {
		t.Fatalf("hits=%s", hits)
	}
	if !strings.Contains(hits, "手机号") {
		t.Fatalf("expected description in hits: %s", hits)
	}

	window, err = SearchByFields(endpoints, SearchOptions{Keywords: []string{"email"}})
	if err != nil {
		t.Fatal(err)
	}
	if window.Total < 1 {
		t.Fatal("email should match body field")
	}
	hits = FormatMatchedFields(window.Items[0].Matches)
	if !strings.Contains(hits, "body.user.email") {
		t.Fatalf("hits=%s", hits)
	}

	window, err = SearchByFields(endpoints, SearchOptions{Keywords: []string{"响应ID"}})
	if err != nil {
		t.Fatal(err)
	}
	if window.Total < 1 {
		t.Fatal("response description should match")
	}
	hits = FormatMatchedFields(window.Items[0].Matches)
	if !strings.Contains(hits, "response.") {
		t.Fatalf("hits=%s", hits)
	}

	window, err = SearchByFields(endpoints, SearchOptions{
		Keywords: []string{"phone"},
		Method:   "POST",
	})
	if err != nil {
		t.Fatal(err)
	}
	if window.Total != 0 {
		t.Fatalf("method filter should zero phone: %d", window.Total)
	}
}

func TestSearchByFieldsRequiresKeywords(t *testing.T) {
	endpoints, err := BuildFieldIndex([]byte(fieldWalkFixture))
	if err != nil {
		t.Fatal(err)
	}
	_, err = SearchByFields(endpoints, SearchOptions{Method: "GET"})
	if err == nil {
		t.Fatal("expected keywords required error")
	}
	if !strings.Contains(err.Error(), "keywords") {
		t.Fatalf("err=%v", err)
	}
}

func TestSearchByFieldsModeAndLimit(t *testing.T) {
	endpoints, err := BuildFieldIndex([]byte(fieldWalkFixture))
	if err != nil {
		t.Fatal(err)
	}
	window, err := SearchByFields(endpoints, SearchOptions{
		Keywords: []string{"phone", "email"},
		Mode:     "and",
	})
	if err != nil {
		t.Fatal(err)
	}
	if window.Total != 1 {
		t.Fatalf("and mode total=%d", window.Total)
	}

	window, err = SearchByFields(endpoints, SearchOptions{
		Keywords: []string{"leaf"},
		Limit:    1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if window.Limit != 1 {
		t.Fatalf("limit=%d", window.Limit)
	}
}

func TestFormatFieldSearchResultsIncludesHits(t *testing.T) {
	endpoints, err := BuildFieldIndex([]byte(fieldWalkFixture))
	if err != nil {
		t.Fatal(err)
	}
	window, err := SearchByFields(endpoints, SearchOptions{Keywords: []string{"phone"}})
	if err != nil {
		t.Fatal(err)
	}
	markdown := FormatFieldSearchResults(window, "")
	if !strings.Contains(markdown, "命中字段") {
		t.Fatalf("missing column: %s", markdown)
	}
	if !strings.Contains(markdown, "query.phone") {
		t.Fatalf("missing hit: %s", markdown)
	}

	payload, err := FormatFieldSearchJSON(window, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(payload, &doc); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"total", "showing", "truncated", "limit", "module", "stale", "items"} {
		if _, ok := doc[key]; !ok {
			t.Fatalf("missing %s", key)
		}
	}
	items := doc["items"].([]any)
	if len(items) < 1 {
		t.Fatal("items empty")
	}
	item := items[0].(map[string]any)
	matches := item["matches"].([]any)
	if len(matches) < 1 {
		t.Fatal("matches empty")
	}
}
