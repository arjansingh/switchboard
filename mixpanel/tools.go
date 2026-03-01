package mixpanel

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Segmentation ────────────────────────────────────────────────
	{
		Name: "mixpanel_query_segmentation", Description: "Query event data segmented and filtered by properties",
		Parameters: map[string]string{"event": "Event name", "from_date": "Start date yyyy-mm-dd", "to_date": "End date yyyy-mm-dd", "on": "Property expression to segment by", "unit": "Time granularity: minute/hour/day/month", "where": "Filter expression", "limit": "Max property values", "type": "Aggregation: general/unique/average"},
		Required:   []string{"event", "from_date", "to_date"},
	},
	{
		Name: "mixpanel_query_segmentation_numeric", Description: "Bucket a numeric property into ranges for distribution analysis",
		Parameters: map[string]string{"event": "Event name", "from_date": "Start date yyyy-mm-dd", "to_date": "End date yyyy-mm-dd", "on": "Numeric property expression to bucket", "buckets": "Number of buckets", "where": "Filter expression", "type": "Aggregation: general/unique/average", "unit": "Time granularity: minute/hour/day/month"},
		Required:   []string{"event", "from_date", "to_date", "on"},
	},
	{
		Name: "mixpanel_query_segmentation_sum", Description: "Sum a numeric expression across events",
		Parameters: map[string]string{"event": "Event name", "from_date": "Start date yyyy-mm-dd", "to_date": "End date yyyy-mm-dd", "on": "Numeric property expression to sum", "where": "Filter expression", "unit": "Time granularity: minute/hour/day/month"},
		Required:   []string{"event", "from_date", "to_date", "on"},
	},
	{
		Name: "mixpanel_query_segmentation_average", Description: "Average a numeric expression across events",
		Parameters: map[string]string{"event": "Event name", "from_date": "Start date yyyy-mm-dd", "to_date": "End date yyyy-mm-dd", "on": "Numeric property expression to average", "where": "Filter expression", "unit": "Time granularity: minute/hour/day/month"},
		Required:   []string{"event", "from_date", "to_date", "on"},
	},

	// ── Funnels ─────────────────────────────────────────────────────
	{
		Name:       "mixpanel_list_funnels", Description: "List all saved funnels in the project",
		Parameters: map[string]string{},
	},
	{
		Name: "mixpanel_query_funnel", Description: "Query conversion data for a saved funnel",
		Parameters: map[string]string{"funnel_id": "Funnel ID from list_funnels", "from_date": "Start date yyyy-mm-dd", "to_date": "End date yyyy-mm-dd", "length": "Completion window max 90 days", "length_unit": "day/hour/minute/second", "unit": "Interval: day/week/month", "on": "Property expression to segment by", "where": "Filter expression", "limit": "Top values max 10000"},
		Required:   []string{"funnel_id", "from_date", "to_date"},
	},

	// ── Retention ───────────────────────────────────────────────────
	{
		Name: "mixpanel_query_retention", Description: "Run a retention cohort analysis",
		Parameters: map[string]string{"from_date": "Start date yyyy-mm-dd", "to_date": "End date yyyy-mm-dd", "retention_type": "birth or compounded", "born_event": "Required when retention_type=birth", "event": "Return event", "born_where": "Filter for born events", "where": "Filter for return events", "interval": "Bucket size max 90", "interval_count": "Number of buckets", "unit": "day/week/month", "on": "Property segmentation", "limit": "Max property values"},
		Required:   []string{"from_date", "to_date"},
	},
	{
		Name: "mixpanel_query_frequency", Description: "Run a frequency/addiction report",
		Parameters: map[string]string{"from_date": "Start date yyyy-mm-dd", "to_date": "End date yyyy-mm-dd", "unit": "Overall period: day/week/month", "addiction_unit": "Granularity: hour/day", "event": "Event name", "where": "Filter expression", "on": "Property segmentation", "limit": "Max property values"},
		Required:   []string{"from_date", "to_date", "unit", "addiction_unit"},
	},

	// ── Insights + JQL ──────────────────────────────────────────────
	{
		Name:       "mixpanel_query_insight", Description: "Query a saved Insights report by bookmark ID",
		Parameters: map[string]string{"bookmark_id": "Saved report ID"},
		Required:   []string{"bookmark_id"},
	},
	{
		Name:       "mixpanel_query_jql", Description: "Execute a custom JQL script against Mixpanel data",
		Parameters: map[string]string{"script": "JQL JavaScript script body", "params": "JSON object of script parameters"},
		Required:   []string{"script"},
	},

	// ── Event Breakdown ─────────────────────────────────────────────
	{
		Name: "mixpanel_aggregate_events", Description: "Get aggregate event counts over time",
		Parameters: map[string]string{"event": "JSON array of event names", "type": "general/unique/average", "unit": "minute/hour/day/week/month", "from_date": "Start date yyyy-mm-dd", "to_date": "End date yyyy-mm-dd", "interval": "Number of units alternative to date range"},
		Required:   []string{"event", "type", "unit"},
	},
	{
		Name:       "mixpanel_top_events_today", Description: "Get today's most common events with occurrence counts",
		Parameters: map[string]string{"type": "general/unique/average", "limit": "Max events"},
		Required:   []string{"type"},
	},
	{
		Name:       "mixpanel_top_event_names", Description: "Get most common event names over the last 31 days",
		Parameters: map[string]string{"type": "general/unique/average", "limit": "Max events"},
		Required:   []string{"type"},
	},
	{
		Name: "mixpanel_event_property_values", Description: "Get aggregated property value data for a specific event",
		Parameters: map[string]string{"event": "Event name", "name": "Property name", "type": "general/unique/average", "unit": "minute/hour/day/week/month", "from_date": "Start date yyyy-mm-dd", "to_date": "End date yyyy-mm-dd", "interval": "Number of units alternative to date range", "where": "Filter expression", "limit": "Max property values"},
		Required:   []string{"event", "name", "type", "unit"},
	},
	{
		Name:       "mixpanel_top_event_properties", Description: "Get the top property names for an event",
		Parameters: map[string]string{"event": "Event name"},
		Required:   []string{"event"},
	},
	{
		Name:       "mixpanel_top_property_values", Description: "Get the top values for a specific event property",
		Parameters: map[string]string{"event": "Event name", "name": "Property name", "limit": "Max values"},
		Required:   []string{"event", "name"},
	},

	// ── Raw Export ──────────────────────────────────────────────────
	{
		Name: "mixpanel_export_events", Description: "Download raw event data as newline-delimited JSON",
		Parameters: map[string]string{"from_date": "Start date yyyy-mm-dd", "to_date": "End date yyyy-mm-dd", "event": "JSON array of event names to filter", "where": "Filter expression", "limit": "Max events max 100000"},
		Required:   []string{"from_date", "to_date"},
	},

	// ── Activity Feed ───────────────────────────────────────────────
	{
		Name:       "mixpanel_query_activity", Description: "Get the event activity stream for specific users",
		Parameters: map[string]string{"distinct_ids": "JSON array of user distinct_ids", "from_date": "Start date yyyy-mm-dd", "to_date": "End date yyyy-mm-dd"},
		Required:   []string{"distinct_ids", "from_date", "to_date"},
	},

	// ── Profiles ────────────────────────────────────────────────────
	{
		Name:       "mixpanel_query_profiles", Description: "Query user or group profiles with filters",
		Parameters: map[string]string{"where": "Filter expression", "output_properties": "JSON array of properties to return", "session_id": "Pagination token", "page": "Page number", "distinct_id": "Single user lookup", "filter_by_cohort": "JSON object with cohort ID"},
	},

	// ── Cohorts ─────────────────────────────────────────────────────
	{
		Name:       "mixpanel_list_cohorts", Description: "List all saved cohorts in the project",
		Parameters: map[string]string{},
	},

	// ── Annotations ─────────────────────────────────────────────────
	{
		Name:       "mixpanel_list_annotations", Description: "List annotations in the project",
		Parameters: map[string]string{"from_date": "Filter start date", "to_date": "Filter end date"},
	},
	{
		Name:       "mixpanel_create_annotation", Description: "Create an annotation marking a point in time",
		Parameters: map[string]string{"date": "Timestamp YYYY-MM-DD HH:mm:ss", "description": "Annotation text"},
		Required:   []string{"date", "description"},
	},
	{
		Name:       "mixpanel_delete_annotation", Description: "Delete an annotation by ID",
		Parameters: map[string]string{"annotation_id": "Annotation ID"},
		Required:   []string{"annotation_id"},
	},

	// ── Lexicon Schemas ─────────────────────────────────────────────
	{
		Name:       "mixpanel_list_schemas", Description: "List all data dictionary schemas in the project",
		Parameters: map[string]string{},
	},
	{
		Name:       "mixpanel_list_schemas_by_entity", Description: "List schemas for a specific entity type",
		Parameters: map[string]string{"entity": "Entity type: event/profile/group"},
		Required:   []string{"entity"},
	},
	{
		Name:       "mixpanel_get_schema", Description: "Get the schema for a specific entity type and name",
		Parameters: map[string]string{"entity": "Entity type", "name": "Entity name"},
		Required:   []string{"entity", "name"},
	},
	{
		Name:       "mixpanel_create_schemas", Description: "Bulk upload/replace schemas",
		Parameters: map[string]string{"schemas": "JSON array of schema objects"},
		Required:   []string{"schemas"},
	},
	{
		Name:       "mixpanel_create_schema", Description: "Create or replace a single schema for a specific entity",
		Parameters: map[string]string{"entity": "Entity type", "name": "Entity name", "schema": "JSON schema object"},
		Required:   []string{"entity", "name", "schema"},
	},
	{
		Name:       "mixpanel_delete_all_schemas", Description: "Delete all schemas from the project",
		Parameters: map[string]string{},
	},
	{
		Name:       "mixpanel_delete_schemas_by_entity", Description: "Delete all schemas for a specific entity type",
		Parameters: map[string]string{"entity": "Entity type"},
		Required:   []string{"entity"},
	},
	{
		Name:       "mixpanel_delete_schema", Description: "Delete a specific schema by entity type and name",
		Parameters: map[string]string{"entity": "Entity type", "name": "Entity name"},
		Required:   []string{"entity", "name"},
	},
}

// --- Dispatch map ---

var dispatch = map[string]handlerFunc{
	// Segmentation
	"mixpanel_query_segmentation":         querySegmentation,
	"mixpanel_query_segmentation_numeric": querySegmentationNumeric,
	"mixpanel_query_segmentation_sum":     querySegmentationSum,
	"mixpanel_query_segmentation_average": querySegmentationAverage,

	// Funnels
	"mixpanel_list_funnels": listFunnels,
	"mixpanel_query_funnel": queryFunnel,

	// Retention
	"mixpanel_query_retention": queryRetention,
	"mixpanel_query_frequency": queryFrequency,

	// Insights + JQL
	"mixpanel_query_insight": queryInsight,
	"mixpanel_query_jql":     queryJQL,

	// Event Breakdown
	"mixpanel_aggregate_events":      aggregateEvents,
	"mixpanel_top_events_today":      topEventsToday,
	"mixpanel_top_event_names":       topEventNames,
	"mixpanel_event_property_values": eventPropertyValues,
	"mixpanel_top_event_properties":  topEventProperties,
	"mixpanel_top_property_values":   topPropertyValues,

	// Raw Export
	"mixpanel_export_events": exportEvents,

	// Activity Feed
	"mixpanel_query_activity": queryActivity,

	// Profiles
	"mixpanel_query_profiles": queryProfiles,

	// Cohorts
	"mixpanel_list_cohorts": listCohorts,

	// Annotations
	"mixpanel_list_annotations":  listAnnotations,
	"mixpanel_create_annotation": createAnnotation,
	"mixpanel_delete_annotation": deleteAnnotation,

	// Lexicon Schemas
	"mixpanel_list_schemas":             listSchemas,
	"mixpanel_list_schemas_by_entity":   listSchemasByEntity,
	"mixpanel_get_schema":               getSchema,
	"mixpanel_create_schemas":           createSchemas,
	"mixpanel_create_schema":            createSchema,
	"mixpanel_delete_all_schemas":       deleteAllSchemas,
	"mixpanel_delete_schemas_by_entity": deleteSchemasByEntity,
	"mixpanel_delete_schema":            deleteSchema,
}
