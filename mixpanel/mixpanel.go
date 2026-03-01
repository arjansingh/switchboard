package mixpanel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

type mixpanel struct {
	username  string
	secret    string
	projectID string
	client    *http.Client
	baseURL   string // region prefix, e.g. "mixpanel", "eu.mixpanel", "in.mixpanel"

	// Precomputed base URLs (parse-don't-validate)
	queryBase  string // https://{baseURL}.com/api/query
	exportBase string // https://data.{baseURL}.com/api/2.0
	appBase    string // https://{baseURL}.com/api/app/projects/{projectID}
}

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

func New() mcp.Integration {
	return &mixpanel{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "mixpanel",
	}
}

func (m *mixpanel) Name() string { return "mixpanel" }

func (m *mixpanel) Configure(creds mcp.Credentials) error {
	m.username = creds["service_account_username"]
	m.secret = creds["service_account_secret"]
	m.projectID = creds["project_id"]

	if m.username == "" {
		return fmt.Errorf("mixpanel: service_account_username is required")
	}
	if m.secret == "" {
		return fmt.Errorf("mixpanel: service_account_secret is required")
	}
	if m.projectID == "" {
		return fmt.Errorf("mixpanel: project_id is required")
	}
	if v := creds["base_url"]; v != "" {
		m.baseURL = strings.TrimRight(v, "/")
	}

	// Compute base URLs once (parse-don't-validate)
	m.queryBase = "https://" + m.baseURL + ".com/api/query"
	m.exportBase = "https://data." + m.baseURL + ".com/api/2.0"
	m.appBase = "https://" + m.baseURL + ".com/api/app/projects/" + m.projectID

	return nil
}

func (m *mixpanel) Healthy(ctx context.Context) bool {
	_, err := m.query(ctx, "GET", "events/top?type=general&limit=1", nil)
	return err == nil
}

func (m *mixpanel) Tools() []mcp.ToolDefinition {
	return tools
}

func (m *mixpanel) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, m, args)
}

// --- HTTP helpers ---

func (m *mixpanel) doRequest(ctx context.Context, method, fullURL string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(m.username, m.secret)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("mixpanel API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

// query calls the Mixpanel Query API. It auto-appends project_id to the query params.
// path should start with "/" (e.g., "/insights" or "/insights?from_date=2024-01-01").
func (m *mixpanel) query(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	fullURL := m.queryBase + path + sep + "project_id=" + m.projectID
	return m.doRequest(ctx, method, fullURL, body)
}

// export calls the Mixpanel Export API. Returns raw bytes for JSONL response format.
// path should start with "/" and include any query params (e.g., "/export?from_date=2024-01-01").
func (m *mixpanel) export(ctx context.Context, path string) ([]byte, error) {
	fullURL := m.exportBase + path

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(m.username, m.secret)

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("mixpanel API error (%d): %s", resp.StatusCode, string(data))
	}
	return data, nil
}

// app calls the Mixpanel App API. project_id is already embedded in the appBase URL.
// path should start with "/" (e.g., "/annotations").
func (m *mixpanel) app(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	return m.doRequest(ctx, method, m.appBase+path, body)
}

// --- Result helpers ---

type handlerFunc func(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error)

func rawResult(data []byte) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: string(data)}, nil
}

func errResult(err error) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
}

// --- Argument helpers ---

func argStr(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func argInt(args map[string]any, key string) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}

func argBool(args map[string]any, key string) bool {
	switch v := args[key].(type) {
	case bool:
		return v
	case string:
		return v == "true"
	}
	return false
}

// queryEncode builds a query string from non-empty key/value pairs.
func queryEncode(params map[string]string) string {
	vals := url.Values{}
	for k, v := range params {
		if v != "" {
			vals.Set(k, v)
		}
	}
	if len(vals) == 0 {
		return ""
	}
	return "?" + vals.Encode()
}
