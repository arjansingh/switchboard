package mixpanel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Constructor tests ---

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "mixpanel", i.Name())
}

// --- Configure tests ---

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{
		"service_account_username": "test-user",
		"service_account_secret":   "test-secret",
		"project_id":               "12345",
	})
	assert.NoError(t, err)
}

func TestConfigure_MissingUsername(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{
		"service_account_secret": "test-secret",
		"project_id":             "12345",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "service_account_username is required")
}

func TestConfigure_MissingSecret(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{
		"service_account_username": "test-user",
		"project_id":               "12345",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "service_account_secret is required")
}

func TestConfigure_MissingProjectID(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{
		"service_account_username": "test-user",
		"service_account_secret":   "test-secret",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "project_id is required")
}

func TestConfigure_CustomBaseURL(t *testing.T) {
	m := &mixpanel{client: &http.Client{}, baseURL: "mixpanel"}
	err := m.Configure(mcp.Credentials{
		"service_account_username": "test-user",
		"service_account_secret":   "test-secret",
		"project_id":               "12345",
		"base_url":                 "eu.mixpanel",
	})
	assert.NoError(t, err)
	assert.Equal(t, "eu.mixpanel", m.baseURL)
	assert.Equal(t, "https://eu.mixpanel.com/api/query", m.queryBase)
	assert.Equal(t, "https://data.eu.mixpanel.com/api/2.0", m.exportBase)
	assert.Equal(t, "https://eu.mixpanel.com/api/app/projects/12345", m.appBase)
}

func TestConfigure_DefaultBaseURL(t *testing.T) {
	m := &mixpanel{client: &http.Client{}, baseURL: "mixpanel"}
	err := m.Configure(mcp.Credentials{
		"service_account_username": "test-user",
		"service_account_secret":   "test-secret",
		"project_id":               "99",
	})
	assert.NoError(t, err)
	assert.Equal(t, "mixpanel", m.baseURL)
	assert.Equal(t, "https://mixpanel.com/api/query", m.queryBase)
	assert.Equal(t, "https://data.mixpanel.com/api/2.0", m.exportBase)
	assert.Equal(t, "https://mixpanel.com/api/app/projects/99", m.appBase)
}

// --- Tools tests ---

func TestTools(t *testing.T) {
	i := New()
	tools := i.Tools()
	assert.NotEmpty(t, tools)

	for _, tool := range tools {
		assert.NotEmpty(t, tool.Name, "tool has empty name")
		assert.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
	}
}

func TestTools_AllHaveMixpanelPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "mixpanel_", "tool %s missing mixpanel_ prefix", tool.Name)
	}
}

func TestTools_NoDuplicateNames(t *testing.T) {
	i := New()
	seen := make(map[string]bool)
	for _, tool := range i.Tools() {
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		seen[tool.Name] = true
	}
}

// --- Execute tests ---

func TestExecute_UnknownTool(t *testing.T) {
	m := &mixpanel{
		username:  "test",
		secret:    "test",
		projectID: "1",
		client:    &http.Client{},
		baseURL:   "mixpanel",
	}
	result, err := m.Execute(context.Background(), "mixpanel_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

// --- Dispatch parity tests ---

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %s has no dispatch handler", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	i := New()
	toolNames := make(map[string]bool)
	for _, tool := range i.Tools() {
		toolNames[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, toolNames[name], "dispatch handler %s has no tool definition", name)
	}
}

// --- Healthy tests ---

func TestHealthy_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/api/query/events/top")
		assert.Equal(t, "general", r.URL.Query().Get("type"))
		assert.Equal(t, "1", r.URL.Query().Get("limit"))
		_, _ = w.Write([]byte(`[{"event":"pageview","amount":100}]`))
	}))
	defer ts.Close()

	m := &mixpanel{
		username:  "test-user",
		secret:    "test-secret",
		projectID: "12345",
		client:    ts.Client(),
		queryBase: ts.URL + "/api/query",
	}
	assert.True(t, m.Healthy(context.Background()))
}

func TestHealthy_Failure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer ts.Close()

	m := &mixpanel{
		username:  "test-user",
		secret:    "bad-secret",
		projectID: "12345",
		client:    ts.Client(),
		queryBase: ts.URL + "/api/query",
	}
	assert.False(t, m.Healthy(context.Background()))
}

// --- HTTP helper tests ---

func TestQuery_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		// Verify Basic Auth
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "test-user", user)
		assert.Equal(t, "test-secret", pass)
		// Verify project_id appended
		assert.Equal(t, "12345", r.URL.Query().Get("project_id"))
		assert.Contains(t, r.URL.Path, "/api/query/insights")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"count":42}]}`))
	}))
	defer ts.Close()

	m := &mixpanel{
		username:  "test-user",
		secret:    "test-secret",
		projectID: "12345",
		client:    ts.Client(),
		baseURL:   "mixpanel",
		queryBase: ts.URL + "/api/query",
	}
	data, err := m.query(context.Background(), "GET", "/insights", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "42")
}

func TestQuery_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer ts.Close()

	m := &mixpanel{
		username:  "test-user",
		secret:    "bad-secret",
		projectID: "12345",
		client:    ts.Client(),
		queryBase: ts.URL + "/api/query",
	}
	_, err := m.query(context.Background(), "GET", "/insights", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mixpanel API error (403)")
}

func TestQuery_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	m := &mixpanel{
		username:  "test-user",
		secret:    "test-secret",
		projectID: "12345",
		client:    ts.Client(),
		queryBase: ts.URL + "/api/query",
	}
	data, err := m.query(context.Background(), "DELETE", "/something", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestQuery_POST(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "val", body["key"])
		_, _ = w.Write([]byte(`{"created":true}`))
	}))
	defer ts.Close()

	m := &mixpanel{
		username:  "test-user",
		secret:    "test-secret",
		projectID: "12345",
		client:    ts.Client(),
		queryBase: ts.URL + "/api/query",
	}
	data, err := m.query(context.Background(), "POST", "/create", map[string]string{"key": "val"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "created")
}

func TestQuery_PathWithExistingQueryParams(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// project_id should be appended with &
		assert.Equal(t, "12345", r.URL.Query().Get("project_id"))
		assert.Equal(t, "bar", r.URL.Query().Get("foo"))
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	m := &mixpanel{
		username:  "test-user",
		secret:    "test-secret",
		projectID: "12345",
		client:    ts.Client(),
		queryBase: ts.URL + "/api/query",
	}
	data, err := m.query(context.Background(), "GET", "/insights?foo=bar", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "ok")
}

func TestExport_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "test-user", user)
		assert.Equal(t, "test-secret", pass)
		assert.Contains(t, r.URL.Path, "/api/2.0/export")
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte("{\"event\":\"pageview\"}\n{\"event\":\"click\"}\n"))
	}))
	defer ts.Close()

	m := &mixpanel{
		username:   "test-user",
		secret:     "test-secret",
		projectID:  "12345",
		client:     ts.Client(),
		exportBase: ts.URL + "/api/2.0",
	}
	data, err := m.export(context.Background(), "/export?from_date=2024-01-01&to_date=2024-01-02")
	require.NoError(t, err)
	assert.Contains(t, string(data), "pageview")
	assert.Contains(t, string(data), "click")
}

func TestApp_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "test-user", user)
		assert.Equal(t, "test-secret", pass)
		// appBase already includes /projects/{projectID}
		assert.Contains(t, r.URL.Path, "/api/app/projects/12345/custom-events")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"name":"signup"}]}`))
	}))
	defer ts.Close()

	m := &mixpanel{
		username:  "test-user",
		secret:    "test-secret",
		projectID: "12345",
		client:    ts.Client(),
		appBase:   ts.URL + "/api/app/projects/12345",
	}
	data, err := m.app(context.Background(), "GET", "/custom-events", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "signup")
}

func TestExport_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
	}))
	defer ts.Close()

	m := &mixpanel{
		username:   "test-user",
		secret:     "test-secret",
		projectID:  "12345",
		client:     ts.Client(),
		exportBase: ts.URL + "/api/2.0",
	}
	_, err := m.export(context.Background(), "/export?from_date=2024-01-01&to_date=2024-01-02")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mixpanel API error (429)")
}

func TestApp_POST(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "deploy v2", body["description"])
		_, _ = w.Write([]byte(`{"id":1,"description":"deploy v2"}`))
	}))
	defer ts.Close()

	m := &mixpanel{
		username:  "test-user",
		secret:    "test-secret",
		projectID: "12345",
		client:    ts.Client(),
		appBase:   ts.URL + "/api/app/projects/12345",
	}
	data, err := m.app(context.Background(), "POST", "/annotations", map[string]string{"description": "deploy v2"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "deploy v2")
}

// --- Result helper tests ---

func TestRawResult(t *testing.T) {
	data := json.RawMessage(`{"key":"value"}`)
	result, err := rawResult(data)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, `{"key":"value"}`, result.Data)
}

func TestRawResult_FromBytes(t *testing.T) {
	data := []byte("{\"event\":\"pageview\"}\n{\"event\":\"click\"}\n")
	result, err := rawResult(data)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "pageview")
}

func TestErrResult(t *testing.T) {
	result, err := errResult(fmt.Errorf("test error"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "test error", result.Data)
}

// --- Argument helper tests ---

func TestArgStr(t *testing.T) {
	assert.Equal(t, "val", argStr(map[string]any{"k": "val"}, "k"))
	assert.Empty(t, argStr(map[string]any{}, "k"))
}

func TestArgInt(t *testing.T) {
	assert.Equal(t, 42, argInt(map[string]any{"n": float64(42)}, "n"))
	assert.Equal(t, 42, argInt(map[string]any{"n": 42}, "n"))
	assert.Equal(t, 42, argInt(map[string]any{"n": "42"}, "n"))
	assert.Equal(t, 0, argInt(map[string]any{}, "n"))
}

func TestArgBool(t *testing.T) {
	assert.True(t, argBool(map[string]any{"b": true}, "b"))
	assert.False(t, argBool(map[string]any{"b": false}, "b"))
	assert.True(t, argBool(map[string]any{"b": "true"}, "b"))
	assert.False(t, argBool(map[string]any{}, "b"))
}

func TestQueryEncode(t *testing.T) {
	t.Run("with values", func(t *testing.T) {
		result := queryEncode(map[string]string{"key": "val", "empty": ""})
		assert.Contains(t, result, "key=val")
		assert.NotContains(t, result, "empty")
		assert.True(t, result[0] == '?')
	})

	t.Run("all empty", func(t *testing.T) {
		result := queryEncode(map[string]string{"empty": ""})
		assert.Empty(t, result)
	})
}

// --- Analytics handler tests ---

func newTestMixpanel(ts *httptest.Server) *mixpanel {
	return &mixpanel{
		username:  "u",
		secret:    "s",
		projectID: "1",
		client:    ts.Client(),
		queryBase: ts.URL + "/api/query",
	}
}

func TestQuerySegmentation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/query/segmentation")
		assert.Equal(t, "signup", r.URL.Query().Get("event"))
		assert.Equal(t, "2024-01-01", r.URL.Query().Get("from_date"))
		assert.Equal(t, "2024-01-31", r.URL.Query().Get("to_date"))
		_, _ = w.Write([]byte(`{"data":{"series":["2024-01-01"],"values":{"signup":{"2024-01-01":42}}}}`))
	}))
	defer ts.Close()

	m := newTestMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_query_segmentation", map[string]any{
		"event": "signup", "from_date": "2024-01-01", "to_date": "2024-01-31",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "42")
}

func TestQuerySegmentation_WithLimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "10", r.URL.Query().Get("limit"))
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer ts.Close()

	m := newTestMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_query_segmentation", map[string]any{
		"event": "signup", "from_date": "2024-01-01", "to_date": "2024-01-31", "limit": float64(10),
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestQuerySegmentationNumeric(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/query/segmentation/numeric")
		assert.Equal(t, "purchase", r.URL.Query().Get("event"))
		assert.Equal(t, "properties[\"amount\"]", r.URL.Query().Get("on"))
		assert.Equal(t, "5", r.URL.Query().Get("buckets"))
		_, _ = w.Write([]byte(`{"data":{"series":["2024-01-01"],"values":{"0-100":{"2024-01-01":10}}}}`))
	}))
	defer ts.Close()

	m := newTestMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_query_segmentation_numeric", map[string]any{
		"event": "purchase", "from_date": "2024-01-01", "to_date": "2024-01-31",
		"on": `properties["amount"]`, "buckets": float64(5),
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "0-100")
}

func TestQuerySegmentationSum(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/query/segmentation/sum")
		assert.Equal(t, "purchase", r.URL.Query().Get("event"))
		assert.Equal(t, "properties[\"revenue\"]", r.URL.Query().Get("on"))
		_, _ = w.Write([]byte(`{"data":{"series":["2024-01-01"],"values":{"total":{"2024-01-01":9999}}}}`))
	}))
	defer ts.Close()

	m := newTestMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_query_segmentation_sum", map[string]any{
		"event": "purchase", "from_date": "2024-01-01", "to_date": "2024-01-31",
		"on": `properties["revenue"]`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "9999")
}

func TestQuerySegmentationAverage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/query/segmentation/average")
		assert.Equal(t, "purchase", r.URL.Query().Get("event"))
		assert.Equal(t, "properties[\"price\"]", r.URL.Query().Get("on"))
		_, _ = w.Write([]byte(`{"data":{"series":["2024-01-01"],"values":{"avg":{"2024-01-01":25.5}}}}`))
	}))
	defer ts.Close()

	m := newTestMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_query_segmentation_average", map[string]any{
		"event": "purchase", "from_date": "2024-01-01", "to_date": "2024-01-31",
		"on": `properties["price"]`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "25.5")
}

func TestListFunnels(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/query/funnels/list")
		_, _ = w.Write([]byte(`[{"funnel_id":123,"name":"Onboarding"}]`))
	}))
	defer ts.Close()

	m := newTestMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_list_funnels", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Onboarding")
}

func TestQueryFunnel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/query/funnels")
		assert.Equal(t, "123", r.URL.Query().Get("funnel_id"))
		assert.Equal(t, "2024-01-01", r.URL.Query().Get("from_date"))
		_, _ = w.Write([]byte(`{"meta":{"dates":["2024-01-01"]},"data":{"steps":[{"count":100}]}}`))
	}))
	defer ts.Close()

	m := newTestMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_query_funnel", map[string]any{
		"funnel_id": float64(123), "from_date": "2024-01-01", "to_date": "2024-01-31",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "100")
}

func TestQueryFunnel_WithLengthAndLimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "14", r.URL.Query().Get("length"))
		assert.Equal(t, "50", r.URL.Query().Get("limit"))
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer ts.Close()

	m := newTestMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_query_funnel", map[string]any{
		"funnel_id": float64(1), "from_date": "2024-01-01", "to_date": "2024-01-31",
		"length": float64(14), "limit": float64(50),
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestQueryRetention(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/query/retention")
		assert.Equal(t, "2024-01-01", r.URL.Query().Get("from_date"))
		assert.Equal(t, "2024-01-31", r.URL.Query().Get("to_date"))
		_, _ = w.Write([]byte(`{"data":[{"date":"2024-01-01","counts":[100,50,30]}]}`))
	}))
	defer ts.Close()

	m := newTestMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_query_retention", map[string]any{
		"from_date": "2024-01-01", "to_date": "2024-01-31",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "100")
}

func TestQueryRetention_WithIntervalAndLimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "7", r.URL.Query().Get("interval"))
		assert.Equal(t, "4", r.URL.Query().Get("interval_count"))
		assert.Equal(t, "25", r.URL.Query().Get("limit"))
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer ts.Close()

	m := newTestMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_query_retention", map[string]any{
		"from_date": "2024-01-01", "to_date": "2024-01-31",
		"interval": float64(7), "interval_count": float64(4), "limit": float64(25),
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestQueryFrequency(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/query/retention/addiction")
		assert.Equal(t, "2024-01-01", r.URL.Query().Get("from_date"))
		assert.Equal(t, "week", r.URL.Query().Get("unit"))
		assert.Equal(t, "day", r.URL.Query().Get("addiction_unit"))
		_, _ = w.Write([]byte(`{"data":{"all":{"1":500,"2":200}}}`))
	}))
	defer ts.Close()

	m := newTestMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_query_frequency", map[string]any{
		"from_date": "2024-01-01", "to_date": "2024-01-31", "unit": "week", "addiction_unit": "day",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "500")
}

func TestQueryFrequency_WithLimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "10", r.URL.Query().Get("limit"))
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer ts.Close()

	m := newTestMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_query_frequency", map[string]any{
		"from_date": "2024-01-01", "to_date": "2024-01-31", "unit": "week", "addiction_unit": "day",
		"limit": float64(10),
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestQueryInsight(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/query/insights")
		assert.Equal(t, "456", r.URL.Query().Get("bookmark_id"))
		_, _ = w.Write([]byte(`{"results":{"series":{"2024-01-01":{"count":88}}}}`))
	}))
	defer ts.Close()

	m := newTestMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_query_insight", map[string]any{
		"bookmark_id": float64(456),
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "88")
}

func TestQueryJQL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/api/query/jql")

		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "function main(){return Events({})}", body["script"])
		assert.NotNil(t, body["params"])

		_, _ = w.Write([]byte(`[{"name":"signup","count":77}]`))
	}))
	defer ts.Close()

	m := newTestMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_query_jql", map[string]any{
		"script": "function main(){return Events({})}",
		"params": `{"limit":10}`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "77")
}

func TestQueryJQL_NoParams(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "function main(){return Events({})}", body["script"])
		_, ok := body["params"]
		assert.False(t, ok, "params should not be present when empty")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	m := newTestMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_query_jql", map[string]any{
		"script": "function main(){return Events({})}",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestQueryJQL_InvalidParams(t *testing.T) {
	m := &mixpanel{
		username:  "u",
		secret:    "s",
		projectID: "1",
		client:    &http.Client{},
		queryBase: "http://localhost/api/query",
	}
	result, err := m.Execute(context.Background(), "mixpanel_query_jql", map[string]any{
		"script": "function main(){return Events({})}",
		"params": "not valid json{{{",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid JSON for params")
}

// --- Event Breakdown handler tests ---

func TestAggregateEvents(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/query/events")
		assert.Equal(t, `["pageview","click"]`, r.URL.Query().Get("event"))
		assert.Equal(t, "general", r.URL.Query().Get("type"))
		assert.Equal(t, "day", r.URL.Query().Get("unit"))
		_, _ = w.Write([]byte(`{"data":{"series":["2024-01-01"],"values":{"pageview":{"2024-01-01":100}}}}`))
	}))
	defer ts.Close()

	m := newTestMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_aggregate_events", map[string]any{
		"event": `["pageview","click"]`, "type": "general", "unit": "day",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "100")
}

func TestAggregateEvents_WithInterval(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "7", r.URL.Query().Get("interval"))
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer ts.Close()

	m := newTestMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_aggregate_events", map[string]any{
		"event": `["pageview"]`, "type": "general", "unit": "day", "interval": float64(7),
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestTopEventsToday(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/query/events/top")
		assert.Equal(t, "general", r.URL.Query().Get("type"))
		_, _ = w.Write([]byte(`{"events":[{"event":"pageview","amount":500}]}`))
	}))
	defer ts.Close()

	m := newTestMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_top_events_today", map[string]any{
		"type": "general",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "500")
}

func TestTopEventsToday_WithLimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "5", r.URL.Query().Get("limit"))
		_, _ = w.Write([]byte(`{"events":[]}`))
	}))
	defer ts.Close()

	m := newTestMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_top_events_today", map[string]any{
		"type": "general", "limit": float64(5),
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestTopEventNames(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/query/events/names")
		assert.Equal(t, "general", r.URL.Query().Get("type"))
		_, _ = w.Write([]byte(`["pageview","click","signup"]`))
	}))
	defer ts.Close()

	m := newTestMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_top_event_names", map[string]any{
		"type": "general",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "signup")
}

func TestEventPropertyValues(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/query/events/properties")
		assert.Equal(t, "pageview", r.URL.Query().Get("event"))
		assert.Equal(t, "browser", r.URL.Query().Get("name"))
		assert.Equal(t, "general", r.URL.Query().Get("type"))
		assert.Equal(t, "day", r.URL.Query().Get("unit"))
		_, _ = w.Write([]byte(`{"data":{"values":{"Chrome":{"2024-01-01":300}}}}`))
	}))
	defer ts.Close()

	m := newTestMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_event_property_values", map[string]any{
		"event": "pageview", "name": "browser", "type": "general", "unit": "day",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Chrome")
}

func TestTopEventProperties(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/query/events/properties/top")
		assert.Equal(t, "pageview", r.URL.Query().Get("event"))
		_, _ = w.Write([]byte(`{"browser":{"count":1000},"os":{"count":800}}`))
	}))
	defer ts.Close()

	m := newTestMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_top_event_properties", map[string]any{
		"event": "pageview",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "browser")
}

func TestTopPropertyValues(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/query/events/properties/values")
		assert.Equal(t, "pageview", r.URL.Query().Get("event"))
		assert.Equal(t, "browser", r.URL.Query().Get("name"))
		_, _ = w.Write([]byte(`["Chrome","Firefox","Safari"]`))
	}))
	defer ts.Close()

	m := newTestMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_top_property_values", map[string]any{
		"event": "pageview", "name": "browser",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Chrome")
}

func TestTopPropertyValues_WithLimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "3", r.URL.Query().Get("limit"))
		_, _ = w.Write([]byte(`["Chrome","Firefox","Safari"]`))
	}))
	defer ts.Close()

	m := newTestMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_top_property_values", map[string]any{
		"event": "pageview", "name": "browser", "limit": float64(3),
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

// --- Export handler tests ---

func TestExportEvents(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/api/2.0/export")
		_, _ = w.Write([]byte("{\"event\":\"pageview\"}\n{\"event\":\"click\"}\n"))
	}))
	defer ts.Close()

	m := &mixpanel{username: "u", secret: "s", projectID: "1", client: ts.Client(), exportBase: ts.URL + "/api/2.0"}
	result, err := m.Execute(context.Background(), "mixpanel_export_events", map[string]any{
		"from_date": "2024-01-01", "to_date": "2024-01-02",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	// Should be JSON array, not JSONL
	var events []json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(result.Data), &events))
	assert.Len(t, events, 2)
}

func TestExportEvents_Empty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(""))
	}))
	defer ts.Close()

	m := &mixpanel{username: "u", secret: "s", projectID: "1", client: ts.Client(), exportBase: ts.URL + "/api/2.0"}
	result, err := m.Execute(context.Background(), "mixpanel_export_events", map[string]any{
		"from_date": "2024-01-01", "to_date": "2024-01-02",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "[]", result.Data)
}

func TestExportEvents_WithLimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "100", r.URL.Query().Get("limit"))
		_, _ = w.Write([]byte("{\"event\":\"pageview\"}\n"))
	}))
	defer ts.Close()

	m := &mixpanel{username: "u", secret: "s", projectID: "1", client: ts.Client(), exportBase: ts.URL + "/api/2.0"}
	result, err := m.Execute(context.Background(), "mixpanel_export_events", map[string]any{
		"from_date": "2024-01-01", "to_date": "2024-01-02", "limit": float64(100),
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	var events []json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(result.Data), &events))
	assert.Len(t, events, 1)
}

// --- Activity Feed handler tests ---

func TestQueryActivity(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/query/stream/query")
		assert.Equal(t, `["user1","user2"]`, r.URL.Query().Get("distinct_ids"))
		assert.Equal(t, "2024-01-01", r.URL.Query().Get("from_date"))
		assert.Equal(t, "2024-01-31", r.URL.Query().Get("to_date"))
		_, _ = w.Write([]byte(`{"results":{"events":[{"event":"pageview"}]}}`))
	}))
	defer ts.Close()

	m := newTestMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_query_activity", map[string]any{
		"distinct_ids": `["user1","user2"]`, "from_date": "2024-01-01", "to_date": "2024-01-31",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "pageview")
}

// --- Profiles handler tests ---

func TestQueryProfiles(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/api/query/engage")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, `properties["plan"]=="premium"`, body["where"])
		_, _ = w.Write([]byte(`{"results":[{"$distinct_id":"user1","$properties":{"plan":"premium"}}],"total":1}`))
	}))
	defer ts.Close()

	m := newTestMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_query_profiles", map[string]any{
		"where": `properties["plan"]=="premium"`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "premium")
}

func TestQueryProfiles_WithOutputProperties(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		props, ok := body["output_properties"].([]any)
		assert.True(t, ok)
		assert.Contains(t, props, "$email")
		_, _ = w.Write([]byte(`{"results":[],"total":0}`))
	}))
	defer ts.Close()

	m := newTestMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_query_profiles", map[string]any{
		"output_properties": `["$email","$name"]`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestQueryProfiles_InvalidOutputProperties(t *testing.T) {
	m := &mixpanel{
		username:  "u",
		secret:    "s",
		projectID: "1",
		client:    &http.Client{},
		queryBase: "http://localhost/api/query",
	}
	result, err := m.Execute(context.Background(), "mixpanel_query_profiles", map[string]any{
		"output_properties": "not valid json{{{",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid JSON for output_properties")
}

func TestQueryProfiles_InvalidFilterByCohort(t *testing.T) {
	m := &mixpanel{
		username:  "u",
		secret:    "s",
		projectID: "1",
		client:    &http.Client{},
		queryBase: "http://localhost/api/query",
	}
	result, err := m.Execute(context.Background(), "mixpanel_query_profiles", map[string]any{
		"filter_by_cohort": "not valid json{{{",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid JSON for filter_by_cohort")
}

func TestQueryProfiles_WithPagination(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "abc123", body["session_id"])
		assert.Equal(t, float64(2), body["page"])
		_, _ = w.Write([]byte(`{"results":[],"total":0}`))
	}))
	defer ts.Close()

	m := newTestMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_query_profiles", map[string]any{
		"session_id": "abc123", "page": float64(2),
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

// --- Cohorts handler tests ---

func TestListCohorts(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/query/cohorts/list")
		_, _ = w.Write([]byte(`[{"id":1,"name":"Power Users"},{"id":2,"name":"Churned"}]`))
	}))
	defer ts.Close()

	m := newTestMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_list_cohorts", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Power Users")
}

// --- Annotation handler tests ---

func newTestAppMixpanel(ts *httptest.Server) *mixpanel {
	return &mixpanel{
		username:  "u",
		secret:    "s",
		projectID: "1",
		client:    ts.Client(),
		appBase:   ts.URL + "/api/app/projects/1",
	}
}

func TestListAnnotations(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/app/projects/1/annotations")
		_, _ = w.Write([]byte(`{"status":"ok","results":[{"id":1,"description":"deploy"}]}`))
	}))
	defer ts.Close()

	m := newTestAppMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_list_annotations", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "deploy")
}

func TestListAnnotations_WithDateFilters(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "2024-01-01", r.URL.Query().Get("from_date"))
		assert.Equal(t, "2024-01-31", r.URL.Query().Get("to_date"))
		_, _ = w.Write([]byte(`{"status":"ok","results":[]}`))
	}))
	defer ts.Close()

	m := newTestAppMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_list_annotations", map[string]any{
		"from_date": "2024-01-01",
		"to_date":   "2024-01-31",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestCreateAnnotation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "deploy v2", body["description"])
		assert.Equal(t, "2024-01-15 00:00:00", body["date"])
		_, _ = w.Write([]byte(`{"status":"ok","results":{"id":1}}`))
	}))
	defer ts.Close()

	m := newTestAppMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_create_annotation", map[string]any{
		"date": "2024-01-15 00:00:00", "description": "deploy v2",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestDeleteAnnotation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Contains(t, r.URL.Path, "/api/app/projects/1/annotations/42")
		w.WriteHeader(204)
	}))
	defer ts.Close()

	m := newTestAppMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_delete_annotation", map[string]any{
		"annotation_id": "42",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "success")
}

// --- Lexicon Schema handler tests ---

func TestListSchemas(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/app/projects/1/schemas")
		_, _ = w.Write([]byte(`{"status":"ok","results":[{"entityType":"event","name":"signup"}]}`))
	}))
	defer ts.Close()

	m := newTestAppMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_list_schemas", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "signup")
}

func TestListSchemasByEntity(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/app/projects/1/schemas/event")
		_, _ = w.Write([]byte(`{"status":"ok","results":[{"name":"pageview"}]}`))
	}))
	defer ts.Close()

	m := newTestAppMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_list_schemas_by_entity", map[string]any{
		"entity": "event",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "pageview")
}

func TestGetSchema(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/app/projects/1/schemas/event/signup")
		_, _ = w.Write([]byte(`{"status":"ok","results":{"name":"signup","description":"User signed up"}}`))
	}))
	defer ts.Close()

	m := newTestAppMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_get_schema", map[string]any{
		"entity": "event", "name": "signup",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "signup")
}

func TestCreateSchemas(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/api/app/projects/1/schemas")
		var body []any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Len(t, body, 1)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer ts.Close()

	m := newTestAppMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_create_schemas", map[string]any{
		"schemas": `[{"entityType":"event","name":"signup"}]`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestCreateSchemas_InvalidJSON(t *testing.T) {
	m := &mixpanel{username: "u", secret: "s", projectID: "1", client: &http.Client{}}
	result, err := m.Execute(context.Background(), "mixpanel_create_schemas", map[string]any{
		"schemas": "not valid json",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid JSON for schemas")
}

func TestCreateSchema(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/api/app/projects/1/schemas/event/signup")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "User signed up", body["description"])
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer ts.Close()

	m := newTestAppMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_create_schema", map[string]any{
		"entity": "event", "name": "signup", "schema": `{"description":"User signed up"}`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestCreateSchema_InvalidJSON(t *testing.T) {
	m := &mixpanel{username: "u", secret: "s", projectID: "1", client: &http.Client{}}
	result, err := m.Execute(context.Background(), "mixpanel_create_schema", map[string]any{
		"entity": "event", "name": "signup", "schema": "{bad json",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid JSON for schema")
}

func TestDeleteAllSchemas(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Contains(t, r.URL.Path, "/api/app/projects/1/schemas")
		w.WriteHeader(204)
	}))
	defer ts.Close()

	m := newTestAppMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_delete_all_schemas", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "success")
}

func TestDeleteSchemasByEntity(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Contains(t, r.URL.Path, "/api/app/projects/1/schemas/event")
		w.WriteHeader(204)
	}))
	defer ts.Close()

	m := newTestAppMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_delete_schemas_by_entity", map[string]any{
		"entity": "event",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "success")
}

func TestDeleteSchema(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Contains(t, r.URL.Path, "/api/app/projects/1/schemas/event/signup")
		w.WriteHeader(204)
	}))
	defer ts.Close()

	m := newTestAppMixpanel(ts)
	result, err := m.Execute(context.Background(), "mixpanel_delete_schema", map[string]any{
		"entity": "event", "name": "signup",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "success")
}
