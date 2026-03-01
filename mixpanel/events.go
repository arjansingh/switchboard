package mixpanel

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

// --- Event Breakdown handlers (Query API) ---

// aggregateEvents queries GET /events for aggregate event counts over time.
func aggregateEvents(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	params := queryEncode(map[string]string{
		"event":     argStr(args, "event"),
		"type":      argStr(args, "type"),
		"unit":      argStr(args, "unit"),
		"from_date": argStr(args, "from_date"),
		"to_date":   argStr(args, "to_date"),
	})
	if v := argInt(args, "interval"); v > 0 {
		params += "&interval=" + strconv.Itoa(v)
	}
	data, err := m.query(ctx, "GET", "/events"+params, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// topEventsToday queries GET /events/top for today's most common events.
func topEventsToday(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	params := queryEncode(map[string]string{
		"type": argStr(args, "type"),
	})
	if v := argInt(args, "limit"); v > 0 {
		params += "&limit=" + strconv.Itoa(v)
	}
	data, err := m.query(ctx, "GET", "/events/top"+params, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// topEventNames queries GET /events/names for the most common event names.
func topEventNames(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	params := queryEncode(map[string]string{
		"type": argStr(args, "type"),
	})
	if v := argInt(args, "limit"); v > 0 {
		params += "&limit=" + strconv.Itoa(v)
	}
	data, err := m.query(ctx, "GET", "/events/names"+params, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// eventPropertyValues queries GET /events/properties for aggregated property value data.
func eventPropertyValues(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	params := queryEncode(map[string]string{
		"event":     argStr(args, "event"),
		"name":      argStr(args, "name"),
		"type":      argStr(args, "type"),
		"unit":      argStr(args, "unit"),
		"from_date": argStr(args, "from_date"),
		"to_date":   argStr(args, "to_date"),
		"where":     argStr(args, "where"),
	})
	if v := argInt(args, "interval"); v > 0 {
		params += "&interval=" + strconv.Itoa(v)
	}
	if v := argInt(args, "limit"); v > 0 {
		params += "&limit=" + strconv.Itoa(v)
	}
	data, err := m.query(ctx, "GET", "/events/properties"+params, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// topEventProperties queries GET /events/properties/top for the top property names of an event.
func topEventProperties(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	params := queryEncode(map[string]string{
		"event": argStr(args, "event"),
	})
	data, err := m.query(ctx, "GET", "/events/properties/top"+params, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// topPropertyValues queries GET /events/properties/values for the top values of an event property.
func topPropertyValues(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	params := queryEncode(map[string]string{
		"event": argStr(args, "event"),
		"name":  argStr(args, "name"),
	})
	if v := argInt(args, "limit"); v > 0 {
		params += "&limit=" + strconv.Itoa(v)
	}
	data, err := m.query(ctx, "GET", "/events/properties/values"+params, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// --- Raw Export handler (Export API) ---

// exportEvents queries GET /export via the Export API and converts JSONL to a JSON array.
func exportEvents(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	params := queryEncode(map[string]string{
		"from_date": argStr(args, "from_date"),
		"to_date":   argStr(args, "to_date"),
		"event":     argStr(args, "event"),
		"where":     argStr(args, "where"),
	})
	if v := argInt(args, "limit"); v > 0 {
		params += "&limit=" + strconv.Itoa(v)
	}
	raw, err := m.export(ctx, "/export"+params)
	if err != nil {
		return errResult(err)
	}
	// Convert JSONL to JSON array
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	var events []json.RawMessage
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		events = append(events, json.RawMessage(line))
	}
	if events == nil {
		events = []json.RawMessage{}
	}
	result, err := json.Marshal(events)
	if err != nil {
		return errResult(err)
	}
	return rawResult(result)
}

// --- Activity Feed handler (Query API) ---

// queryActivity queries GET /stream/query for user event activity.
func queryActivity(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	params := queryEncode(map[string]string{
		"distinct_ids": argStr(args, "distinct_ids"),
		"from_date":    argStr(args, "from_date"),
		"to_date":      argStr(args, "to_date"),
	})
	data, err := m.query(ctx, "GET", "/stream/query"+params, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// --- Profiles handler (Query API, POST) ---

// queryProfiles queries POST /engage for user or group profiles.
func queryProfiles(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{}
	if v := argStr(args, "where"); v != "" {
		body["where"] = v
	}
	if v := argStr(args, "output_properties"); v != "" {
		var props any
		if err := json.Unmarshal([]byte(v), &props); err != nil {
			return errResult(fmt.Errorf("invalid JSON for output_properties: %w", err))
		}
		body["output_properties"] = props
	}
	if v := argStr(args, "session_id"); v != "" {
		body["session_id"] = v
	}
	if v := argInt(args, "page"); v > 0 {
		body["page"] = v
	}
	if v := argStr(args, "distinct_id"); v != "" {
		body["distinct_id"] = v
	}
	if v := argStr(args, "filter_by_cohort"); v != "" {
		var cohort any
		if err := json.Unmarshal([]byte(v), &cohort); err != nil {
			return errResult(fmt.Errorf("invalid JSON for filter_by_cohort: %w", err))
		}
		body["filter_by_cohort"] = cohort
	}
	data, err := m.query(ctx, "POST", "/engage", body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// --- Cohorts handler (Query API, POST) ---

// listCohorts queries POST /cohorts/list for all saved cohorts.
func listCohorts(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	data, err := m.query(ctx, "GET", "/cohorts/list", nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
