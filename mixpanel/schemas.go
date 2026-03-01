package mixpanel

import (
	"context"
	"encoding/json"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// --- Annotation handlers ---

func listAnnotations(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	params := queryEncode(map[string]string{
		"from_date": argStr(args, "from_date"),
		"to_date":   argStr(args, "to_date"),
	})
	data, err := m.app(ctx, "GET", "/annotations"+params, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createAnnotation(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]string{
		"date":        argStr(args, "date"),
		"description": argStr(args, "description"),
	}
	data, err := m.app(ctx, "POST", "/annotations", body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteAnnotation(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	id := argPath(args, "annotation_id")
	data, err := m.app(ctx, "DELETE", "/annotations/"+id, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

// --- Lexicon Schema handlers ---

func listSchemas(ctx context.Context, m *mixpanel, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := m.app(ctx, "GET", "/schemas", nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listSchemasByEntity(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	entity := argPath(args, "entity")
	data, err := m.app(ctx, "GET", "/schemas/"+entity, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getSchema(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	entity := argPath(args, "entity")
	name := argPath(args, "name")
	data, err := m.app(ctx, "GET", "/schemas/"+entity+"/"+name, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createSchemas(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	var schemas any
	if err := json.Unmarshal([]byte(argStr(args, "schemas")), &schemas); err != nil {
		return errResult(fmt.Errorf("invalid JSON for schemas: %w", err))
	}
	data, err := m.app(ctx, "POST", "/schemas", schemas)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createSchema(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	entity := argPath(args, "entity")
	name := argPath(args, "name")
	var schema any
	if err := json.Unmarshal([]byte(argStr(args, "schema")), &schema); err != nil {
		return errResult(fmt.Errorf("invalid JSON for schema: %w", err))
	}
	data, err := m.app(ctx, "POST", "/schemas/"+entity+"/"+name, schema)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteAllSchemas(ctx context.Context, m *mixpanel, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := m.app(ctx, "DELETE", "/schemas", nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteSchemasByEntity(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	entity := argPath(args, "entity")
	data, err := m.app(ctx, "DELETE", "/schemas/"+entity, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func deleteSchema(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	entity := argPath(args, "entity")
	name := argPath(args, "name")
	data, err := m.app(ctx, "DELETE", "/schemas/"+entity+"/"+name, nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
