package openapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestValidateHTTPMethod(t *testing.T) {
	ok, err := ValidateHTTPMethod("get")
	if err != nil || ok != "GET" {
		t.Fatalf("valid get: %q %v", ok, err)
	}
	_, err = ValidateHTTPMethod("FETCH")
	if err == nil {
		t.Fatal("expected invalid method error")
	}
	if !strings.Contains(err.Error(), "必须是合法 HTTP 方法") {
		t.Fatalf("unexpected error: %v", err)
	}
	// Valid method with zero matches is not an error (checked via SearchWindow).
	window, err := SearchWindow(loadFixture(t), SearchOptions{Method: "TRACE"})
	if err != nil {
		t.Fatal(err)
	}
	if window.Total != 0 {
		t.Fatalf("TRACE should be empty in fixture, total=%d", window.Total)
	}
}

func TestTokenizeSubwords(t *testing.T) {
	cases := []struct {
		input string
		want  []string
	}{
		{"listUsers", []string{"listusers", "list", "users"}},
		{"CreateOrder", []string{"createorder", "create", "order"}},
		{"user_id", []string{"user_id", "user", "id"}},
		{"user-profile", []string{"user-profile", "user", "profile"}},
		{"用户登录", []string{"用户登录"}},
	}
	for _, testCase := range cases {
		tokens := Tokenize(testCase.input)
		tokenSet := map[string]struct{}{}
		for _, token := range tokens {
			tokenSet[token] = struct{}{}
		}
		for _, expected := range testCase.want {
			if _, ok := tokenSet[expected]; !ok {
				t.Fatalf("Tokenize(%q) missing %q; got %v", testCase.input, expected, tokens)
			}
		}
	}
}

func TestCJKExactNoFuzzy(t *testing.T) {
	endpoints := []Endpoint{
		{Method: "POST", Path: "/auth/login", Summary: "用户登录", OperationID: "loginUser"},
	}
	hit, err := SearchWindow(endpoints, SearchOptions{Keywords: []string{"用户"}})
	if err != nil {
		t.Fatal(err)
	}
	if hit.Total != 1 {
		t.Fatalf("CJK exact total=%d", hit.Total)
	}
	// Near-miss / unrelated CJK must not match via shared characters alone.
	miss, err := SearchWindow(endpoints, SearchOptions{Keywords: []string{"支付"}})
	if err != nil {
		t.Fatal(err)
	}
	if miss.Total != 0 {
		t.Fatalf("CJK must not fuzzy/partial-expand multi-char, total=%d", miss.Total)
	}
	// Typo that shares first rune still should not match multi-char needle.
	miss2, err := SearchWindow(endpoints, SearchOptions{Keywords: []string{"用戸登录"}})
	if err != nil {
		t.Fatal(err)
	}
	if miss2.Total != 0 {
		t.Fatalf("CJK multi-char typo must not match, total=%d", miss2.Total)
	}
}

func TestASCIIFuzzyLongTokens(t *testing.T) {
	endpoints := []Endpoint{
		{Method: "GET", Path: "/users", Summary: "List users", OperationID: "listUsers"},
	}
	// listUsrs is edit-distance 1 from listUsers / listusers token.
	window, err := SearchWindow(endpoints, SearchOptions{Keywords: []string{"listUsrs"}})
	if err != nil {
		t.Fatal(err)
	}
	if window.Total != 1 {
		t.Fatalf("expected ASCII fuzzy hit, total=%d", window.Total)
	}
	// Short tokens (<=3) must not fuzzy.
	shortMiss, err := SearchWindow(endpoints, SearchOptions{Keywords: []string{"usr"}})
	if err != nil {
		t.Fatal(err)
	}
	// "usr" may still substring-match "users"; use a non-substring short typo.
	shortMiss, err = SearchWindow(endpoints, SearchOptions{Keywords: []string{"xyz"}})
	if err != nil {
		t.Fatal(err)
	}
	if shortMiss.Total != 0 {
		t.Fatalf("expected no match for xyz, total=%d", shortMiss.Total)
	}
}

func TestMultiKeywordOrderIndependent(t *testing.T) {
	endpoints := []Endpoint{
		{Method: "GET", Path: "/users", Summary: "List users", OperationID: "listUsers", Tags: []string{"User"}},
		{Method: "POST", Path: "/orders", Summary: "Create order", OperationID: "createOrder", Tags: []string{"Order"}},
		{Method: "GET", Path: "/users/orders", Summary: "User orders", OperationID: "listUserOrders", Tags: []string{"User", "Order"}},
	}
	first, err := SearchWindow(endpoints, SearchOptions{Keywords: []string{"user", "order"}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := SearchWindow(endpoints, SearchOptions{Keywords: []string{"order", "user"}})
	if err != nil {
		t.Fatal(err)
	}
	if first.Total != second.Total {
		t.Fatalf("totals differ: %d vs %d", first.Total, second.Total)
	}
	for index := range first.Items {
		if first.Items[index].Method != second.Items[index].Method || first.Items[index].Path != second.Items[index].Path {
			t.Fatalf("order changed at %d: %+v vs %+v", index, first.Items[index], second.Items[index])
		}
	}
}

func TestANDModeRequiresAllKeywords(t *testing.T) {
	endpoints := []Endpoint{
		{Method: "GET", Path: "/users", Summary: "List users", OperationID: "listUsers"},
		{Method: "GET", Path: "/orders", Summary: "List orders", OperationID: "listOrders"},
		{Method: "GET", Path: "/users/orders", Summary: "User orders", OperationID: "listUserOrders"},
	}
	andHit, err := SearchWindow(endpoints, SearchOptions{
		Keywords: []string{"user", "order"},
		Mode:     "and",
	})
	if err != nil {
		t.Fatal(err)
	}
	if andHit.Total != 1 || andHit.Items[0].Path != "/users/orders" {
		t.Fatalf("AND unexpected: total=%d items=%+v", andHit.Total, andHit.Items)
	}
}

func TestTieBreakPathThenMethod(t *testing.T) {
	endpoints := []Endpoint{
		{Method: "POST", Path: "/alpha", Summary: "same", OperationID: "a"},
		{Method: "GET", Path: "/alpha", Summary: "same", OperationID: "b"},
		{Method: "GET", Path: "/beta", Summary: "same", OperationID: "c"},
	}
	window, err := SearchWindow(endpoints, SearchOptions{Keywords: []string{"same"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(window.Items) != 3 {
		t.Fatalf("expected 3, got %d", len(window.Items))
	}
	// Equal scores → path ASC → method ASC
	want := []string{"GET /alpha", "POST /alpha", "GET /beta"}
	for index, expected := range want {
		got := window.Items[index].Method + " " + window.Items[index].Path
		if got != expected {
			t.Fatalf("tie-break index %d: got %s want %s", index, got, expected)
		}
	}
}

func TestFormatSearchJSONSchema(t *testing.T) {
	window := SearchResultWindow{
		Total:     1,
		Showing:   1,
		Truncated: false,
		Limit:     20,
		Items: []Endpoint{
			{Method: "GET", Path: "/x", Summary: "X", Tags: []string{"t"}, OperationID: "getX"},
		},
	}
	moduleID := 42
	raw, err := FormatSearchJSON(window, &moduleID, true)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"total", "showing", "truncated", "limit", "module", "stale", "items"} {
		if _, ok := doc[key]; !ok {
			t.Fatalf("missing key %s in %s", key, string(raw))
		}
	}
	if doc["stale"] != true {
		t.Fatalf("stale=%v", doc["stale"])
	}
	if int(doc["module"].(float64)) != 42 {
		t.Fatalf("module=%v", doc["module"])
	}
	items := doc["items"].([]any)
	item := items[0].(map[string]any)
	for _, key := range []string{"method", "path", "summary", "tags", "operationId"} {
		if _, ok := item[key]; !ok {
			t.Fatalf("missing item key %s", key)
		}
	}
	// No strategy prose keys.
	for _, banned := range []string{"guidance", "message", "strategy", "warning"} {
		if _, ok := doc[banned]; ok {
			t.Fatalf("unexpected prose field %s", banned)
		}
	}
	// null module
	rawNull, err := FormatSearchJSON(window, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	var docNull map[string]any
	if err := json.Unmarshal(rawNull, &docNull); err != nil {
		t.Fatal(err)
	}
	if docNull["module"] != nil {
		t.Fatalf("module should be null, got %v", docNull["module"])
	}
}

func TestGoldenCorpusFile(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "golden_corpus.json"))
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Endpoints []Endpoint `json:"endpoints"`
		Cases     []struct {
			Name     string   `json:"name"`
			Keywords []string `json:"keywords"`
			Mode     string   `json:"mode"`
			Method   string   `json:"method"`
			Want     []string `json:"want"` // "METHOD path"
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	for _, testCase := range corpus.Cases {
		t.Run(testCase.Name, func(t *testing.T) {
			window, err := SearchWindow(corpus.Endpoints, SearchOptions{
				Keywords: testCase.Keywords,
				Mode:     testCase.Mode,
				Method:   testCase.Method,
			})
			if err != nil {
				t.Fatal(err)
			}
			got := make([]string, 0, len(window.Items))
			for _, item := range window.Items {
				got = append(got, item.Method+" "+item.Path)
			}
			if !reflect.DeepEqual(got, testCase.Want) {
				t.Fatalf("got %v want %v", got, testCase.Want)
			}
		})
	}
}
