package mixpanel

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	mcp "github.com/daltoniam/switchboard"
)

// --- Segmentation handlers ---

func querySegmentation(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	params := queryEncode(map[string]string{
		"event":     argStr(args, "event"),
		"from_date": argStr(args, "from_date"),
		"to_date":   argStr(args, "to_date"),
		"on":        argStr(args, "on"),
		"unit":      argStr(args, "unit"),
		"where":     argStr(args, "where"),
		"type":      argStr(args, "type"),
	})
	if v := argInt(args, "limit"); v > 0 {
		params += "&limit=" + strconv.Itoa(v)
	}
	data, err := m.query(ctx, "GET", "/segmentation"+params, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func querySegmentationNumeric(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	params := queryEncode(map[string]string{
		"event":     argStr(args, "event"),
		"from_date": argStr(args, "from_date"),
		"to_date":   argStr(args, "to_date"),
		"on":        argStr(args, "on"),
		"where":     argStr(args, "where"),
		"type":      argStr(args, "type"),
		"unit":      argStr(args, "unit"),
	})
	if v := argInt(args, "buckets"); v > 0 {
		params += "&buckets=" + strconv.Itoa(v)
	}
	data, err := m.query(ctx, "GET", "/segmentation/numeric"+params, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func querySegmentationSum(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	params := queryEncode(map[string]string{
		"event":     argStr(args, "event"),
		"from_date": argStr(args, "from_date"),
		"to_date":   argStr(args, "to_date"),
		"on":        argStr(args, "on"),
		"where":     argStr(args, "where"),
		"unit":      argStr(args, "unit"),
	})
	data, err := m.query(ctx, "GET", "/segmentation/sum"+params, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func querySegmentationAverage(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	params := queryEncode(map[string]string{
		"event":     argStr(args, "event"),
		"from_date": argStr(args, "from_date"),
		"to_date":   argStr(args, "to_date"),
		"on":        argStr(args, "on"),
		"where":     argStr(args, "where"),
		"unit":      argStr(args, "unit"),
	})
	data, err := m.query(ctx, "GET", "/segmentation/average"+params, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// --- Funnel handlers ---

func listFunnels(ctx context.Context, m *mixpanel, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := m.query(ctx, "GET", "/funnels/list", nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func queryFunnel(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	params := queryEncode(map[string]string{
		"from_date":   argStr(args, "from_date"),
		"to_date":     argStr(args, "to_date"),
		"length_unit": argStr(args, "length_unit"),
		"unit":        argStr(args, "unit"),
		"on":          argStr(args, "on"),
		"where":       argStr(args, "where"),
	})
	if v := argInt(args, "funnel_id"); v > 0 {
		params += "&funnel_id=" + strconv.Itoa(v)
	}
	if v := argInt(args, "length"); v > 0 {
		params += "&length=" + strconv.Itoa(v)
	}
	if v := argInt(args, "limit"); v > 0 {
		params += "&limit=" + strconv.Itoa(v)
	}
	data, err := m.query(ctx, "GET", "/funnels"+params, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// --- Retention handlers ---

func queryRetention(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	params := queryEncode(map[string]string{
		"from_date":      argStr(args, "from_date"),
		"to_date":        argStr(args, "to_date"),
		"retention_type": argStr(args, "retention_type"),
		"born_event":     argStr(args, "born_event"),
		"event":          argStr(args, "event"),
		"born_where":     argStr(args, "born_where"),
		"where":          argStr(args, "where"),
		"unit":           argStr(args, "unit"),
		"on":             argStr(args, "on"),
	})
	if v := argInt(args, "interval"); v > 0 {
		params += "&interval=" + strconv.Itoa(v)
	}
	if v := argInt(args, "interval_count"); v > 0 {
		params += "&interval_count=" + strconv.Itoa(v)
	}
	if v := argInt(args, "limit"); v > 0 {
		params += "&limit=" + strconv.Itoa(v)
	}
	data, err := m.query(ctx, "GET", "/retention"+params, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func queryFrequency(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	params := queryEncode(map[string]string{
		"from_date":      argStr(args, "from_date"),
		"to_date":        argStr(args, "to_date"),
		"unit":           argStr(args, "unit"),
		"addiction_unit": argStr(args, "addiction_unit"),
		"event":          argStr(args, "event"),
		"where":          argStr(args, "where"),
		"on":             argStr(args, "on"),
	})
	if v := argInt(args, "limit"); v > 0 {
		params += "&limit=" + strconv.Itoa(v)
	}
	data, err := m.query(ctx, "GET", "/retention/addiction"+params, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// --- Insights + JQL handlers ---

func queryInsight(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	params := ""
	if v := argInt(args, "bookmark_id"); v > 0 {
		params = "?bookmark_id=" + strconv.Itoa(v)
	}
	data, err := m.query(ctx, "GET", "/insights"+params, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func queryJQL(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"script": argStr(args, "script"),
	}
	if v := argStr(args, "params"); v != "" {
		var p any
		if err := json.Unmarshal([]byte(v), &p); err != nil {
			return errResult(fmt.Errorf("invalid JSON for params: %w", err))
		}
		body["params"] = p
	}
	data, err := m.query(ctx, "POST", "/jql", body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
