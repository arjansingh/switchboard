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
		queryBase: ts.URL + "/api/query/",
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
		queryBase: ts.URL + "/api/query/",
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
