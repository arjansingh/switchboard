# Mixpanel Integration Design

## Interview Findings

- **Scope**: Full platform coverage (~31 tools)
- **Auth**: Service Account (Basic auth with username:secret)
- **Region**: Configurable base URL, default US (`mixpanel`)
- **SDK**: No Go SDK for query APIs — raw HTTP adapter
- **Closest analog**: PostHog adapter (analytics, raw HTTP, ~50 tools, configurable base URL)

## Architecture

### Raw HTTP Adapter

No maintained Go SDK exists for Mixpanel query/analytics APIs. `mixpanel/mixpanel-go` is ingestion-only.

### Three API Base URLs

Mixpanel uses three distinct API base URL patterns:

| API Family | Base URL Pattern | Purpose |
|---|---|---|
| Query API | `https://{region}.com/api/query/` | Analytics, segmentation, funnels, retention, profiles, events |
| Raw Export API | `https://data.{region}.com/api/2.0/` | Bulk event data download |
| App API | `https://{region}.com/api/app/projects/{projectId}/` | Annotations, Lexicon schemas |

Where `{region}` = `mixpanel` (US default), `eu.mixpanel` (EU), or `in.mixpanel` (India).

### Auth Model

All endpoints use HTTP Basic Auth with Service Account credentials.

```go
type mixpanel struct {
    username  string       // Service Account username
    secret    string       // Service Account secret
    projectID string       // Required for most endpoints
    client    *http.Client
    baseURL   string       // Region prefix, default "mixpanel" (→ mixpanel.com)
}
```

**Configure credentials**: `service_account_username`, `service_account_secret`, `project_id` (required). `base_url` (optional, defaults to `mixpanel`).

### HTTP Helpers

Three request methods to handle the different base URLs:

- `query(ctx, method, path, body)` → `https://{baseURL}.com/api/query/{path}?project_id={projectID}`
- `export(ctx, path, params)` → `https://data.{baseURL}.com/api/2.0/{path}`
- `app(ctx, method, path, body)` → `https://{baseURL}.com/api/app/projects/{projectID}/{path}`

All inject Basic Auth header: `Authorization: Basic base64(username:secret)`.

### Rate Limits

- Query API: 60 queries/hour, max 5 concurrent
- Export API: 60 queries/hour, 3/second, max 100 concurrent

No built-in rate limiting in the adapter (matches Switchboard convention — surface errors, don't retry).

### File Structure

```
mixpanel/
  mixpanel.go          Core struct, Configure, Healthy, Execute, HTTP helpers, arg helpers (~200 lines)
  tools.go             31 ToolDefinitions + dispatch map (~400 lines)
  analytics.go         Segmentation, funnels, retention, insights, JQL handlers (10 tools)
  events.go            Event breakdown, raw export, activity feed, profiles, cohorts (10 tools)
  schemas.go           Lexicon schemas, annotations (11 tools)
  mixpanel_test.go     All test categories
```

### Healthy Check

```go
func (m *mixpanel) Healthy(ctx context.Context) bool {
    // GET /events/top with limit=1 — lightweight, verifies auth + project access
    _, err := m.query(ctx, "GET", "events/top?type=general&limit=1", nil)
    return err == nil
}
```

---

## Complete Tool Inventory (31 tools)

### 1. Segmentation (4 tools) — `analytics.go`

#### `mixpanel_query_segmentation`

**Purpose**: Query event data segmented and filtered by properties. The core analytics query — equivalent to Mixpanel's Insights report.

**API**: `GET /segmentation`
**Base**: Query API (`{region}.com/api/query/segmentation`)

| Parameter | Type | Required | Description |
|---|---|---|---|
| `event` | string | yes | Event name to query |
| `from_date` | string | yes | Start date (yyyy-mm-dd) |
| `to_date` | string | yes | End date (yyyy-mm-dd) |
| `on` | string | no | Property expression to segment by (e.g., `properties["country"]`) |
| `unit` | string | no | Time granularity: minute/hour/day/month (default: day) |
| `where` | string | no | Filter expression (e.g., `properties["plan"] == "pro"`) |
| `limit` | int | no | Max property values (default: 60, max: 10000) |
| `type` | string | no | Aggregation: general/unique/average (default: general) |

**Response**: `{"data": {"series": [...], "values": {...}}, "legend_size": N}`

**Tests**:
- Success: mock returning series data, verify JSON passthrough
- Missing required `event` param: verify request still sent (Mixpanel returns error)
- API error (429 rate limit): verify error propagation
- With all optional params: verify query string construction

---

#### `mixpanel_query_segmentation_numeric`

**Purpose**: Bucket a numeric property into ranges. Useful for analyzing distributions (e.g., "how many users made 1-5 purchases vs 6-10?").

**API**: `GET /segmentation/numeric`
**Base**: Query API

| Parameter | Type | Required | Description |
|---|---|---|---|
| `event` | string | yes | Event name |
| `from_date` | string | yes | Start date |
| `to_date` | string | yes | End date |
| `on` | string | yes | Numeric property expression to bucket |
| `buckets` | int | no | Number of buckets |
| `where` | string | no | Filter expression |
| `type` | string | no | Aggregation type |
| `unit` | string | no | Time granularity |

**Response**: Same structure as segmentation, with numeric bucket keys in values.

**Tests**:
- Success with numeric buckets
- API error propagation

---

#### `mixpanel_query_segmentation_sum`

**Purpose**: Sum a numeric expression across events. Useful for revenue totals, aggregate counts by property value.

**API**: `GET /segmentation/sum`
**Base**: Query API

| Parameter | Type | Required | Description |
|---|---|---|---|
| `event` | string | yes | Event name |
| `from_date` | string | yes | Start date |
| `to_date` | string | yes | End date |
| `on` | string | yes | Numeric property expression to sum |
| `where` | string | no | Filter expression |
| `unit` | string | no | Time granularity |

**Response**: Same segmentation structure with summed values.

**Tests**:
- Success: verify sum data passthrough
- API error propagation

---

#### `mixpanel_query_segmentation_average`

**Purpose**: Average a numeric expression across events. Useful for "average session duration" or "average purchase amount."

**API**: `GET /segmentation/average`
**Base**: Query API

| Parameter | Type | Required | Description |
|---|---|---|---|
| `event` | string | yes | Event name |
| `from_date` | string | yes | Start date |
| `to_date` | string | yes | End date |
| `on` | string | yes | Numeric property expression to average |
| `where` | string | no | Filter expression |
| `unit` | string | no | Time granularity |

**Response**: Same segmentation structure with averaged values.

**Tests**:
- Success: verify average data passthrough
- API error propagation

---

### 2. Funnels (2 tools) — `analytics.go`

#### `mixpanel_list_funnels`

**Purpose**: List all saved funnels in the project. Returns funnel IDs and names needed to query funnel data.

**API**: `GET /funnels/list`
**Base**: Query API

| Parameter | Type | Required | Description |
|---|---|---|---|
| _(none beyond project_id)_ | | | |

**Response**: `[{"funnel_id": 7509, "name": "Signup funnel"}, ...]`

**Tests**:
- Success: verify array of funnel objects
- Empty project: verify empty array handling

---

#### `mixpanel_query_funnel`

**Purpose**: Query conversion data for a saved funnel. Returns step-by-step conversion rates, timing, and segmentation.

**API**: `GET /funnels`
**Base**: Query API

| Parameter | Type | Required | Description |
|---|---|---|---|
| `funnel_id` | int | yes | The funnel to query (from list_funnels) |
| `from_date` | string | yes | Start date |
| `to_date` | string | yes | End date |
| `length` | int | no | Completion window (max 90 days) |
| `length_unit` | string | no | Unit for length: day/hour/minute/second |
| `unit` | string | no | Interval: day/week/month |
| `on` | string | no | Property expression to segment by |
| `where` | string | no | Filter expression |
| `limit` | int | no | Top values (default: 255, max: 10000) |

**Response**: Funnel data with steps, conversion ratios, average times.

**Tests**:
- Success with funnel data
- Missing funnel_id: verify error
- With segmentation (on param)

---

### 3. Retention (2 tools) — `analytics.go`

#### `mixpanel_query_retention`

**Purpose**: Run a retention cohort analysis. Answers "of users who did X on day 1, what % came back on day N?"

**API**: `GET /retention`
**Base**: Query API

| Parameter | Type | Required | Description |
|---|---|---|---|
| `from_date` | string | yes | Start date |
| `to_date` | string | yes | End date |
| `retention_type` | string | no | birth (default) or compounded |
| `born_event` | string | conditional | Required when retention_type=birth — the "first" event |
| `event` | string | no | Return event (all events if unspecified) |
| `born_where` | string | no | Filter for born events |
| `where` | string | no | Filter for return events |
| `interval` | int | no | Bucket size (max 90 days) |
| `interval_count` | int | no | Number of buckets (default: 1) |
| `unit` | string | no | day/week/month (default: day) |
| `on` | string | no | Property segmentation |
| `limit` | int | no | Max segmentation values |

**Response**: `{"2012-01-01": {"counts": [2, 1, 2], "first": 2}, ...}`

**Tests**:
- Birth retention with born_event
- Compounded retention
- API error propagation

---

#### `mixpanel_query_frequency`

**Purpose**: Run a frequency/addiction report. Answers "how often do users perform action X per time period?"

**API**: `GET /retention/addiction`
**Base**: Query API

| Parameter | Type | Required | Description |
|---|---|---|---|
| `from_date` | string | yes | Start date |
| `to_date` | string | yes | End date |
| `unit` | string | yes | Overall period: day/week/month |
| `addiction_unit` | string | yes | Granularity: hour/day |
| `event` | string | no | Specific event (all if unspecified) |
| `where` | string | no | Filter expression |
| `on` | string | no | Property segmentation |
| `limit` | int | no | Max segmentation values |

**Response**: `{"data": {"YYYY-MM-DD": [frequency counts]}}`

**Tests**:
- Success with frequency data
- Missing required unit/addiction_unit params

---

### 4. Insights (1 tool) — `analytics.go`

#### `mixpanel_query_insight`

**Purpose**: Query a saved Insights report by bookmark ID. Returns the pre-configured report data.

**API**: `GET /insights`
**Base**: Query API

| Parameter | Type | Required | Description |
|---|---|---|---|
| `bookmark_id` | int | yes | The saved report ID |

**Response**: `{"computed_at": "...", "date_range": {...}, "headers": [...], "series": {...}}`

**Tests**:
- Success with insight data
- Invalid bookmark_id: API error propagation

---

### 5. JQL (1 tool) — `analytics.go`

#### `mixpanel_query_jql`

**Purpose**: Execute a custom JQL (JavaScript Query Language) script against Mixpanel data. The most flexible query mechanism — can express arbitrary computations.

**API**: `POST /jql`
**Base**: Query API

| Parameter | Type | Required | Description |
|---|---|---|---|
| `script` | string | yes | JQL JavaScript script body |
| `params` | string | no | JSON object of script parameters |

**Request body**: `{"script": "...", "params": {...}}`

**Response**: JSON array of query results (shape depends on the script).

**Tests**:
- Success with JQL script
- Invalid script: API error propagation
- With params JSON

---

### 6. Event Breakdown (6 tools) — `events.go`

#### `mixpanel_aggregate_events`

**Purpose**: Get aggregate event counts over time. Equivalent to a simple "how many times did event X happen per day?"

**API**: `GET /events`
**Base**: Query API

| Parameter | Type | Required | Description |
|---|---|---|---|
| `event` | string | yes | JSON array of event names (e.g., `["signup","login"]`) |
| `type` | string | yes | general/unique/average |
| `unit` | string | yes | minute/hour/day/week/month |
| `from_date` | string | conditional | Start date (required if no interval) |
| `to_date` | string | conditional | End date (required if no interval) |
| `interval` | int | conditional | Number of units (alternative to date range) |

**Response**: `{"data": {"series": [...], "values": {"event": {"date": count}}}, "legend_size": N}`

**Tests**:
- Success with date range
- Success with interval instead of date range
- Multiple events in JSON array

---

#### `mixpanel_top_events_today`

**Purpose**: Get today's most common events with occurrence counts and day-over-day change.

**API**: `GET /events/top`
**Base**: Query API

| Parameter | Type | Required | Description |
|---|---|---|---|
| `type` | string | yes | general/unique/average |
| `limit` | int | no | Max events (default: 100) |

**Response**: `{"events": [{"amount": N, "event": "name", "percent_change": F}], "type": "..."}`

**Tests**:
- Success: verify events array
- With limit parameter

---

#### `mixpanel_top_event_names`

**Purpose**: Get most common event names over the last 31 days. Useful for data discovery — "what events exist in this project?"

**API**: `GET /events/names`
**Base**: Query API

| Parameter | Type | Required | Description |
|---|---|---|---|
| `type` | string | yes | general/unique/average |
| `limit` | int | no | Max events |

**Response**: Array of event name strings.

**Tests**:
- Success: verify string array
- With limit

---

#### `mixpanel_event_property_values`

**Purpose**: Get aggregated property value data for a specific event. Answers "what are the values of property X for event Y?"

**API**: `GET /events/properties`
**Base**: Query API

| Parameter | Type | Required | Description |
|---|---|---|---|
| `event` | string | yes | Event name |
| `name` | string | yes | Property name (e.g., `properties["country"]`) |
| `type` | string | yes | general/unique/average |
| `unit` | string | yes | Time granularity |
| `from_date` | string | conditional | Start date |
| `to_date` | string | conditional | End date |
| `interval` | int | conditional | Alternative to date range |
| `where` | string | no | Filter expression |
| `limit` | int | no | Max values |

**Response**: Same series/values structure as events endpoint.

**Tests**:
- Success with property data
- Missing required params

---

#### `mixpanel_top_event_properties`

**Purpose**: Get the top property names for events in the project. Data discovery — "what properties does event X have?"

**API**: `GET /events/properties/top`
**Base**: Query API

| Parameter | Type | Required | Description |
|---|---|---|---|
| `event` | string | yes | Event name |

**Response**: JSON object with property names and their prevalence metrics.

**Tests**:
- Success: verify property list
- Unknown event: API error

---

#### `mixpanel_top_property_values`

**Purpose**: Get the top values for a specific event property. Data discovery — "what are the most common countries?"

**API**: `GET /events/properties/values`
**Base**: Query API

| Parameter | Type | Required | Description |
|---|---|---|---|
| `event` | string | yes | Event name |
| `name` | string | yes | Property name |
| `limit` | int | no | Max values |

**Response**: Array of property values.

**Tests**:
- Success: verify values array
- With limit param

---

### 7. Raw Data Export (1 tool) — `events.go`

#### `mixpanel_export_events`

**Purpose**: Download raw event data as newline-delimited JSON. For bulk analysis or data pipeline integration. Capped to prevent unbounded responses.

**API**: `GET /export`
**Base**: Export API (`data.{region}.com/api/2.0/export`) — different base URL!

| Parameter | Type | Required | Description |
|---|---|---|---|
| `from_date` | string | yes | Start date (yyyy-mm-dd, UTC) |
| `to_date` | string | yes | End date (yyyy-mm-dd, UTC) |
| `event` | string | no | JSON array of event names to filter |
| `where` | string | no | Filter expression |
| `limit` | int | no | Max events (max: 100000) |

**Response**: JSONL (newline-delimited JSON). Each line is a complete event object.

**Note**: Response is JSONL, not JSON. Handler must collect lines into a JSON array for consistent ToolResult format. Enforce a 10MB limit via io.LimitReader (matches PostHog convention).

**Tests**:
- Success: verify JSONL→JSON array conversion
- API error (429)
- With event filter and limit

---

### 8. Activity Feed (1 tool) — `events.go`

#### `mixpanel_query_activity`

**Purpose**: Get the event activity stream for specific users. User-level debugging — "what did user X do between date A and B?"

**API**: `GET /stream/query`
**Base**: Query API

| Parameter | Type | Required | Description |
|---|---|---|---|
| `distinct_ids` | string | yes | JSON array of user distinct_ids |
| `from_date` | string | yes | Start date |
| `to_date` | string | yes | End date |

**Response**: `{"status": "ok", "results": {"events": [{...}, ...]}}`

**Tests**:
- Success: verify events array
- Single user
- Multiple users in array

---

### 9. Profiles (1 tool) — `events.go`

#### `mixpanel_query_profiles`

**Purpose**: Query user or group profiles with filters. Supports pagination for large result sets.

**API**: `POST /engage`
**Base**: Query API

| Parameter | Type | Required | Description |
|---|---|---|---|
| `where` | string | no | Filter expression |
| `output_properties` | string | no | JSON array of properties to return |
| `session_id` | string | no | Pagination token from previous response |
| `page` | int | no | Page number (requires session_id if > 0) |
| `distinct_id` | string | no | Single user lookup |
| `filter_by_cohort` | string | no | JSON object with cohort ID |

**Response**: `{"page": 0, "page_size": 1000, "session_id": "...", "status": "ok", "total": N, "results": [...]}`

**Tests**:
- Success: verify profile results
- With where filter
- Pagination: verify session_id passthrough
- Single distinct_id lookup

---

### 10. Cohorts (1 tool) — `events.go`

#### `mixpanel_list_cohorts`

**Purpose**: List all saved cohorts in the project. Returns cohort metadata (name, ID, count, description).

**API**: `POST /cohorts/list`
**Base**: Query API

| Parameter | Type | Required | Description |
|---|---|---|---|
| _(none beyond project_id)_ | | | |

**Response**: `[{"count": N, "is_visible": 1, "description": "...", "created": "...", "project_id": N, "id": N, "name": "..."}]`

**Tests**:
- Success: verify cohort array
- Empty project: empty array

---

### 11. Annotations (3 tools) — `schemas.go`

#### `mixpanel_list_annotations`

**Purpose**: List annotations in the project. Annotations mark important events (deployments, launches) on Mixpanel charts.

**API**: `GET /annotations`
**Base**: App API (`{region}.com/api/app/projects/{projectId}/annotations`)

| Parameter | Type | Required | Description |
|---|---|---|---|
| `from_date` | string | no | Filter start date |
| `to_date` | string | no | Filter end date |

**Response**: `{"status": "ok", "results": [{"id": N, "date": "...", "description": "...", "user": {...}, "tags": [...]}]}`

**Tests**:
- Success: verify annotations array
- With date filters
- API error (401/403)

---

#### `mixpanel_create_annotation`

**Purpose**: Create an annotation marking a point in time (deployment, feature launch, incident). Appears on all Mixpanel charts.

**API**: `POST /annotations`
**Base**: App API

| Parameter | Type | Required | Description |
|---|---|---|---|
| `date` | string | yes | Timestamp in `YYYY-MM-DD HH:mm:ss` format |
| `description` | string | yes | Annotation text |

**Request body**: `{"date": "...", "description": "..."}`

**Response**: `{"status": "ok", "results": {"id": N, "date": "...", "description": "...", ...}}`

**Tests**:
- Success: verify created annotation with ID
- Missing date or description: verify error

---

#### `mixpanel_delete_annotation`

**Purpose**: Delete an annotation by ID.

**API**: `DELETE /annotations/{annotationId}`
**Base**: App API

| Parameter | Type | Required | Description |
|---|---|---|---|
| `annotation_id` | int | yes | The annotation to delete |

**Response**: `{"status": "ok", "results": {"id": N}}`

**Tests**:
- Success: verify deletion response
- Invalid ID: API error

---

### 12. Lexicon Schemas (8 tools) — `schemas.go`

#### `mixpanel_list_schemas`

**Purpose**: List all data dictionary schemas in the project. Schemas describe events, properties, and their metadata.

**API**: `GET /schemas`
**Base**: App API (`{region}.com/api/app/projects/{projectId}/schemas`)

| Parameter | Type | Required | Description |
|---|---|---|---|
| _(none)_ | | | |

**Response**: JSON array of schema objects (JSON Schema draft-07 format).

**Tests**:
- Success: verify schema array
- Empty project: empty array

---

#### `mixpanel_list_schemas_by_entity`

**Purpose**: List schemas for a specific entity type (e.g., "event", "profile", "group").

**API**: `GET /schemas/{entity}`
**Base**: App API

| Parameter | Type | Required | Description |
|---|---|---|---|
| `entity` | string | yes | Entity type (event/profile/group) |

**Response**: JSON array of schemas for that entity type.

**Tests**:
- Success with "event" entity
- Invalid entity type: API error

---

#### `mixpanel_get_schema`

**Purpose**: Get the schema for a specific entity type and name (e.g., the schema for event "signup").

**API**: `GET /schemas/{entity}/{name}`
**Base**: App API

| Parameter | Type | Required | Description |
|---|---|---|---|
| `entity` | string | yes | Entity type |
| `name` | string | yes | Entity name |

**Response**: Single schema object.

**Tests**:
- Success: verify schema object
- Unknown entity/name: API error (404)

---

#### `mixpanel_create_schemas`

**Purpose**: Bulk upload/replace schemas. Used to sync an external data dictionary with Mixpanel.

**API**: `POST /schemas`
**Base**: App API

| Parameter | Type | Required | Description |
|---|---|---|---|
| `schemas` | string | yes | JSON array of schema objects |

**Request body**: JSON array of schemas.

**Response**: Success/error status.

**Tests**:
- Success: verify upload
- Invalid schema format: API error
- Invalid JSON in schemas param

---

#### `mixpanel_create_schema`

**Purpose**: Create or replace a single schema for a specific entity.

**API**: `POST /schemas/{entity}/{name}`
**Base**: App API

| Parameter | Type | Required | Description |
|---|---|---|---|
| `entity` | string | yes | Entity type |
| `name` | string | yes | Entity name |
| `schema` | string | yes | JSON schema object |

**Request body**: Single schema object.

**Response**: Success/error status.

**Tests**:
- Success: verify creation
- Invalid schema JSON

---

#### `mixpanel_delete_all_schemas`

**Purpose**: Delete all schemas from the project. Destructive — clears the entire data dictionary.

**API**: `DELETE /schemas`
**Base**: App API

| Parameter | Type | Required | Description |
|---|---|---|---|
| _(none)_ | | | |

**Response**: Success status.

**Tests**:
- Success: verify deletion
- API error propagation

---

#### `mixpanel_delete_schemas_by_entity`

**Purpose**: Delete all schemas for a specific entity type.

**API**: `DELETE /schemas/{entity}`
**Base**: App API

| Parameter | Type | Required | Description |
|---|---|---|---|
| `entity` | string | yes | Entity type to clear |

**Response**: Success status.

**Tests**:
- Success: verify deletion
- Invalid entity: API error

---

#### `mixpanel_delete_schema`

**Purpose**: Delete a specific schema by entity type and name.

**API**: `DELETE /schemas/{entity}/{name}`
**Base**: App API

| Parameter | Type | Required | Description |
|---|---|---|---|
| `entity` | string | yes | Entity type |
| `name` | string | yes | Entity name |

**Response**: Success status.

**Tests**:
- Success: verify deletion
- Unknown entity/name: API error (404)

---

## Testing Requirements

Per the add-integration skill, every adapter must have:

### Mandatory Test Categories

| Category | Description |
|---|---|
| Constructor | `New()` returns valid integration, `Name()` returns `"mixpanel"` |
| Configure success | Valid credentials accepted |
| Configure failures | One test per required credential (`service_account_username`, `service_account_secret`, `project_id`) |
| Tools metadata | All 31 tools have Name + Description, all prefixed with `mixpanel_`, no duplicates |
| Dispatch parity: AllToolsCovered | Every `Tools()` entry has a handler in dispatch map |
| Dispatch parity: NoOrphanHandlers | Every dispatch key has a `ToolDefinition` |
| Execute unknown tool | Returns `IsError: true`, `"unknown tool"` in Data |
| HTTP helpers | `httptest.NewServer` for success, API errors (>=400), 204 no-content |
| Arg helpers | Type coercion: `float64→int`, `string→bool` |

### Per-Tool HTTP Tests

Each tool handler gets at least:
1. **Success case**: Mock server returns expected response, verify passthrough
2. **API error case**: Mock returns >=400 status, verify `IsError: true`
3. **Edge cases** as noted in individual tool docs above

### Export-Specific Tests

`mixpanel_export_events` needs additional:
- JSONL→JSON array conversion test
- 10MB limit enforcement test (io.LimitReader)
- Empty export (no events match)

---

## Wiring

1. Register in `cmd/server/main.go`: add `mixpanelInt "github.com/daltoniam/switchboard/mixpanel"` import and `mixpanelInt.New()` to integration list
2. Add default config in `config/config.go` → `defaultConfig()`:
   ```go
   "mixpanel": {
       Enabled:     false,
       Credentials: mcp.Credentials{
           "service_account_username": "",
           "service_account_secret":   "",
           "project_id":               "",
           "base_url":                 "",
       },
   },
   ```

---

## Sources

- [Mixpanel API Overview](https://developer.mixpanel.com/reference/overview)
- [Mixpanel Query API](https://developer.mixpanel.com/reference/query-api)
- [Segmentation Query](https://developer.mixpanel.com/reference/segmentation-query)
- [Funnels Query](https://developer.mixpanel.com/reference/funnels-query)
- [List Saved Funnels](https://developer.mixpanel.com/reference/funnels-list-saved)
- [Retention Query](https://developer.mixpanel.com/reference/retention-query)
- [Frequency Query](https://developer.mixpanel.com/reference/retention-frequency-query)
- [Insights Query](https://developer.mixpanel.com/reference/insights-query)
- [JQL Query](https://developer.mixpanel.com/reference/query-jql)
- [Cohorts List](https://developer.mixpanel.com/reference/cohorts-list)
- [Engage/Profiles Query](https://developer.mixpanel.com/reference/engage-query)
- [Activity Stream](https://developer.mixpanel.com/reference/activity-stream-query)
- [Aggregate Events](https://developer.mixpanel.com/reference/list-recent-events)
- [Top Events](https://developer.mixpanel.com/reference/query-top-events)
- [Event Properties](https://developer.mixpanel.com/reference/query-event-properties)
- [Top Event Properties](https://developer.mixpanel.com/reference/query-events-top-properties)
- [Raw Event Export](https://developer.mixpanel.com/reference/raw-event-export)
- [Annotations API](https://developer.mixpanel.com/reference/list-all-annotations-for-project)
- [Lexicon Schemas API](https://developer.mixpanel.com/reference/lexicon-schemas-api)
- [mcp-mixpanel (Python)](https://pypi.org/project/mcp-mixpanel/)
- [Composio Mixpanel Skill](https://skills.sh/composiohq/awesome-claude-skills/mixpanel-automation)

---

# Mixpanel Integration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a full-coverage Mixpanel integration adapter (31 tools) to Switchboard.

**Architecture:** Raw HTTP adapter with three base URL patterns (Query, Export, App APIs), Basic Auth via Service Account credentials, following PostHog/Sentry patterns. Configurable region prefix for US/EU/India.

**Tech Stack:** Go 1.26, net/http, encoding/json, httptest, testify

---

## Execution Framework

- **Primary model**: Bottleneck — the core adapter (`mixpanel.go`) + tool definitions (`tools.go`) must exist before any handler can be written. Phase 1 & 2 are the critical path.
- **Supporting**: Selective Excellence (PostHog is the reference, but Mixpanel's triple-base-URL pattern is the key differentiator), REPL (verify each TDD cycle before moving on)
- **Key insight**: Phases 3/4/5 (handler implementations) are fully independent and parallelizable once Phase 2 completes — this is the primary parallelization opportunity.
- **Inversion**: Failure = misrouting requests to wrong base URL (Query vs Export vs App) → Mitigate with dedicated HTTP helper methods per API family + httptest coverage for each URL pattern

## Go Proverbs Guiding This Plan

- "A little copying is better than a little dependency" — arg/result helpers duplicated per adapter
- "Accept interfaces, return structs" — `New()` returns `mcp.Integration`
- "Make the zero value useful" — defaults in `New()` (region, http client)
- "Errors are values" — `ToolResult.IsError` pattern, never swallow errors
- "Clear is better than clever" — simple query string builders over abstraction

## Workflow Contract

- [x] TDD: Write failing test before implementation (always on)
- [ ] Phase DAG: Before executing a Phase, create todos for ALL Parts with dependency edges
- [ ] Checkpoints: Pause after each PHASE for user review
- [ ] Checkpoint protocol:
  - Run `make ci` (build, vet, test-race, lint, security)
  - Update this plan with progress
  - Check for missed plan todos
  - Provide summary + what's next
- [ ] Plan updates: Update on scope change, phase done, or when blocked
- [ ] Completion check: Verify Part fully done before marking complete
- [ ] Milestone end: Run `/plan-check` at milestone completion

**DAG Protocol:**
1. Create ALL Part todos for the Phase at once (not incrementally)
2. Set blocks/blockedBy edges between dependent Parts
3. Execute in dependency order (unblocked first, parallelize independent Parts)
4. Mark in_progress before starting, completed after finishing

## Scope Summary

Add a Mixpanel integration adapter with 31 tools spanning analytics queries, event breakdown, raw export, profiles, cohorts, annotations, and lexicon schemas. The adapter uses raw HTTP with Basic Auth, three distinct API base URL patterns, and follows existing Switchboard conventions. Includes full test coverage with dispatch parity enforcement.

## File Manifest

**Files:**
- Create: `mixpanel/mixpanel.go` — core struct, Configure, HTTP helpers, arg helpers
- Create: `mixpanel/tools.go` — 31 ToolDefinitions + dispatch map
- Create: `mixpanel/analytics.go` — segmentation, funnels, retention, insights, JQL handlers
- Create: `mixpanel/events.go` — event breakdown, export, activity, profiles, cohorts handlers
- Create: `mixpanel/schemas.go` — annotations + lexicon schema handlers
- Create: `mixpanel/mixpanel_test.go` — all test categories
- Modify: `cmd/server/main.go:14-30,167-178` — add import + register integration
- Modify: `config/config.go:45-89` — add default config entry

## Existing Code to Reuse

| What | Path | How |
|------|------|-----|
| PostHog adapter (reference pattern) | `posthog/posthog.go` | Mirror struct layout, HTTP helpers, arg helpers |
| PostHog test patterns | `posthog/posthog_test.go` | Mirror test structure: constructor, configure, dispatch parity, httptest |
| Sentry org helper pattern | `sentry/sentry.go:177-183` | Reference for `proj()` helper that falls back to configured default |
| Sentry queryEncode | `sentry/sentry.go:164-175` | Copy queryEncode pattern for building query strings |

---

## Phase 1: Core Adapter Foundation

- Unblocks: everything — no handler can be written without the core struct, HTTP helpers, and test infrastructure
- Goal: Create `mixpanel/mixpanel.go` and `mixpanel/mixpanel_test.go` with full TDD coverage of the core
- Parts:
  - Part A: Core struct + Configure (TDD) — struct, New(), Name(), Configure() with validation
  - Part B: Arg + result helpers (TDD) — argStr, argInt, argBool, queryEncode, rawResult, errResult
  - Part C: HTTP helpers (TDD) — query(), export(), app() methods with Basic Auth, three base URL patterns
  - Part D: Commit
- Done when: `go test ./mixpanel/...` passes, struct + Configure + all helpers verified, three URL patterns covered
- Files: `mixpanel/mixpanel.go`, `mixpanel/mixpanel_test.go`

**Dependencies:**
- Part A: no blockers (start here)
- Part B: no blockers (parallel with A — different functions in same file)
- Part C: blocks on A (HTTP helpers reference struct fields set by Configure)
- Part D: blocks on A, B, C

### Task 1A: Core struct + Configure (TDD)

**Files:**
- Create: `mixpanel/mixpanel_test.go`
- Create: `mixpanel/mixpanel.go`

**Step 1: Write failing tests**

```go
package mixpanel

import (
	"net/http"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "mixpanel", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{
		"service_account_username": "user.123.mp-service-account",
		"service_account_secret":   "secret123",
		"project_id":               "12345",
	})
	assert.NoError(t, err)
}

func TestConfigure_MissingUsername(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{
		"service_account_username": "",
		"service_account_secret":   "secret",
		"project_id":               "123",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "service_account_username is required")
}

func TestConfigure_MissingSecret(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{
		"service_account_username": "user",
		"service_account_secret":   "",
		"project_id":               "123",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "service_account_secret is required")
}

func TestConfigure_MissingProjectID(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{
		"service_account_username": "user",
		"service_account_secret":   "secret",
		"project_id":               "",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "project_id is required")
}

func TestConfigure_CustomBaseURL(t *testing.T) {
	m := &mixpanel{client: &http.Client{}, baseURL: "mixpanel"}
	err := m.Configure(mcp.Credentials{
		"service_account_username": "user",
		"service_account_secret":   "secret",
		"project_id":               "123",
		"base_url":                 "eu.mixpanel",
	})
	assert.NoError(t, err)
	assert.Equal(t, "eu.mixpanel", m.baseURL)
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/arjansingh/code/switchboard/worktrees/mixpanel && go test ./mixpanel/... -run 'TestNew|TestConfigure' -v`
Expected: FAIL — package doesn't exist

**Step 3: Write minimal implementation**

```go
package mixpanel

import (
	"context"
	"fmt"
	"net/http"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

type mixpanel struct {
	username  string
	secret    string
	projectID string
	client    *http.Client
	baseURL   string // region prefix, e.g. "mixpanel", "eu.mixpanel", "in.mixpanel"
}

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
		m.baseURL = v
	}
	return nil
}

func (m *mixpanel) Healthy(ctx context.Context) bool {
	return false // stub — implemented after HTTP helpers
}

func (m *mixpanel) Tools() []mcp.ToolDefinition {
	return nil // stub — implemented in Phase 2
}

func (m *mixpanel) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/arjansingh/code/switchboard/worktrees/mixpanel && go test ./mixpanel/... -run 'TestNew|TestConfigure' -v`
Expected: PASS — all 6 tests green

---

### Task 1B: Arg + result helpers (TDD)

**Files:**
- Modify: `mixpanel/mixpanel_test.go` — add arg/result helper tests
- Modify: `mixpanel/mixpanel.go` — add helpers

**Step 1: Write failing tests** (append to test file)

```go
// --- arg helper tests ---

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

// --- result helper tests ---

func TestRawResult(t *testing.T) {
	data := json.RawMessage(`{"key":"value"}`)
	result, err := rawResult(data)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, `{"key":"value"}`, result.Data)
}

func TestErrResult(t *testing.T) {
	result, err := errResult(fmt.Errorf("test error"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "test error", result.Data)
}
```

**Step 2: Run test to verify they fail**

Run: `cd /Users/arjansingh/code/switchboard/worktrees/mixpanel && go test ./mixpanel/... -run 'TestArg|TestQuery|TestRawResult|TestErrResult' -v`
Expected: FAIL — functions not defined

**Step 3: Write minimal implementation** (append to mixpanel.go)

```go
// --- Result helpers ---

type handlerFunc func(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error)

func rawResult(data json.RawMessage) (*mcp.ToolResult, error) {
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
```

**Step 4: Run test to verify they pass**

Run: `cd /Users/arjansingh/code/switchboard/worktrees/mixpanel && go test ./mixpanel/... -run 'TestArg|TestQuery|TestRawResult|TestErrResult' -v`
Expected: PASS

---

### Task 1C: HTTP helpers (TDD)

**Files:**
- Modify: `mixpanel/mixpanel_test.go` — add HTTP helper tests
- Modify: `mixpanel/mixpanel.go` — add query(), export(), app() methods

**Step 1: Write failing tests** (append to test file)

```go
// --- HTTP helper tests ---

func TestQuery_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/query/segmentation")
		assert.Equal(t, "12345", r.URL.Query().Get("project_id"))
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "testuser", user)
		assert.Equal(t, "testsecret", pass)
		_, _ = w.Write([]byte(`{"data":{"series":[]}}`))
	}))
	defer ts.Close()

	m := testClient(ts)
	data, err := m.query(context.Background(), "GET", "segmentation", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "series")
}

func TestQuery_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
	}))
	defer ts.Close()

	m := testClient(ts)
	_, err := m.query(context.Background(), "GET", "segmentation", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mixpanel API error (429)")
}

func TestQuery_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	m := testClient(ts)
	data, err := m.query(context.Background(), "DELETE", "test", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestQuery_PostWithBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "test", body["script"])
		_, _ = w.Write([]byte(`[{"count":1}]`))
	}))
	defer ts.Close()

	m := testClient(ts)
	data, err := m.query(context.Background(), "POST", "jql", map[string]string{"script": "test"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "count")
}

func TestExport_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/api/2.0/export")
		assert.Equal(t, "12345", r.URL.Query().Get("project_id"))
		_, _ = w.Write([]byte(`{"event":"signup"}` + "\n" + `{"event":"login"}`))
	}))
	defer ts.Close()

	m := testExportClient(ts)
	data, err := m.export(context.Background(), "export", map[string]string{"from_date": "2024-01-01", "to_date": "2024-01-02"})
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}

func TestApp_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/api/app/projects/12345/annotations")
		_, _ = w.Write([]byte(`{"status":"ok","results":[]}`))
	}))
	defer ts.Close()

	m := testClient(ts)
	data, err := m.app(context.Background(), "GET", "annotations", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "ok")
}

// testClient creates a mixpanel instance pointing at a test server for query/app APIs.
func testClient(ts *httptest.Server) *mixpanel {
	// Replace https://{baseURL}.com with the test server URL.
	// The query() and app() methods will build URLs from baseURL.
	return &mixpanel{
		username:  "testuser",
		secret:    "testsecret",
		projectID: "12345",
		client:    ts.Client(),
		baseURL:   "mixpanel",
		queryBase: ts.URL + "/api/query/",
		appBase:   ts.URL + "/api/app/projects/",
	}
}

// testExportClient creates a mixpanel instance for export API tests.
func testExportClient(ts *httptest.Server) *mixpanel {
	return &mixpanel{
		username:   "testuser",
		secret:     "testsecret",
		projectID:  "12345",
		client:     ts.Client(),
		baseURL:    "mixpanel",
		exportBase: ts.URL + "/api/2.0/",
	}
}
```

**Step 2: Run test to verify they fail**

Run: `cd /Users/arjansingh/code/switchboard/worktrees/mixpanel && go test ./mixpanel/... -run 'TestQuery|TestExport|TestApp' -v`
Expected: FAIL — methods not defined

**Step 3: Write minimal implementation**

The HTTP helpers are the most critical code. Note the design decision: store computed base URLs for testability (testClient can override them), but compute them from `baseURL` region prefix in `Configure()`.

```go
// Add fields to struct:
type mixpanel struct {
	username   string
	secret     string
	projectID  string
	client     *http.Client
	baseURL    string // region prefix
	queryBase  string // computed: https://{baseURL}.com/api/query/
	exportBase string // computed: https://data.{baseURL}.com/api/2.0/
	appBase    string // computed: https://{baseURL}.com/api/app/projects/
}

// In Configure(), after setting baseURL, compute the base URLs:
func (m *mixpanel) Configure(creds mcp.Credentials) error {
	// ... existing validation ...
	if m.queryBase == "" {
		m.queryBase = fmt.Sprintf("https://%s.com/api/query/", m.baseURL)
	}
	if m.exportBase == "" {
		m.exportBase = fmt.Sprintf("https://data.%s.com/api/2.0/", m.baseURL)
	}
	if m.appBase == "" {
		m.appBase = fmt.Sprintf("https://%s.com/api/app/projects/", m.baseURL)
	}
	return nil
}

// HTTP helpers:

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

// query calls the Query API: GET/POST https://{region}.com/api/query/{path}
func (m *mixpanel) query(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	u := m.queryBase + path
	if !strings.Contains(u, "project_id=") {
		sep := "?"
		if strings.Contains(u, "?") {
			sep = "&"
		}
		u += sep + "project_id=" + m.projectID
	}
	return m.doRequest(ctx, method, u, body)
}

// export calls the Export API: GET https://data.{region}.com/api/2.0/{path}
func (m *mixpanel) export(ctx context.Context, path string, params map[string]string) ([]byte, error) {
	params["project_id"] = m.projectID
	u := m.exportBase + path + queryEncode(params)
	// Export returns JSONL, not JSON — use raw bytes
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
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

// app calls the App API: GET/POST/DELETE https://{region}.com/api/app/projects/{projectId}/{path}
func (m *mixpanel) app(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	u := m.appBase + m.projectID + "/" + path
	return m.doRequest(ctx, method, u, body)
}
```

**Step 4: Run test to verify they pass**

Run: `cd /Users/arjansingh/code/switchboard/worktrees/mixpanel && go test ./mixpanel/... -v`
Expected: PASS — all tests green

---

### Task 1D: Commit

**Step 1: Commit**

```bash
cd /Users/arjansingh/code/switchboard/worktrees/mixpanel
git add mixpanel/mixpanel.go mixpanel/mixpanel_test.go
git commit -m "feat(mixpanel): add core adapter with Configure, HTTP helpers, and tests"
```

---

**CHECKPOINT: Phase 1 complete. Run `go test ./mixpanel/... -v` and `go vet ./mixpanel/...`. Review before proceeding.**

---

## Phase 2: Tool Definitions + Dispatch Skeleton

- Unblocks: all handler phases (3, 4, 5) — no handler can be written without tool definitions and dispatch map
- Goal: Create `tools.go` with all 31 ToolDefinitions, dispatch map with stubs, and dispatch parity tests
- Parts:
  - Part A: Write all 31 ToolDefinitions in tools.go — see design doc "Complete Tool Inventory" for exact names, descriptions, parameters
  - Part B: Write dispatch map with all 31 entries pointing to stub handlers that return "not implemented"
  - Part C: Write dispatch parity tests + tools metadata tests + execute unknown tool test
  - Part D: Implement Healthy() now that query() exists
  - Part E: Commit
- Done when: dispatch parity tests pass, all 31 tools registered, Healthy() implemented
- Files: `mixpanel/tools.go`, `mixpanel/mixpanel.go` (Healthy, Execute), `mixpanel/mixpanel_test.go`

**Dependencies:**
- Part A: no blockers
- Part B: blocks on A (dispatch map references tool names)
- Part C: blocks on A and B
- Part D: no blockers (parallel with A/B/C)
- Part E: blocks on A, B, C, D

### Task 2A: Tool definitions

Create `mixpanel/tools.go` with all 31 `mcp.ToolDefinition` entries. Reference the design doc's Complete Tool Inventory for exact parameter names, types, required flags, and descriptions.

Tool naming convention: `mixpanel_{action}_{resource}` (e.g., `mixpanel_query_segmentation`, `mixpanel_list_funnels`).

Each tool needs:
- `Name`: exact tool name from inventory
- `Description`: 1-2 sentence purpose
- `Parameters`: JSON schema with properties, required array, and types

### Task 2B: Dispatch map + stub handlers

Add dispatch map to `tools.go`:

```go
var dispatch = map[string]handlerFunc{
	// Segmentation
	"mixpanel_query_segmentation":         stubHandler,
	"mixpanel_query_segmentation_numeric": stubHandler,
	// ... all 31 entries ...
}

func stubHandler(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: "not implemented", IsError: true}, nil
}
```

### Task 2C: Dispatch parity + metadata tests

Add to `mixpanel_test.go`:

```go
func TestTools(t *testing.T) {
	i := New()
	tools := i.Tools()
	assert.NotEmpty(t, tools)
	for _, tool := range tools {
		assert.NotEmpty(t, tool.Name)
		assert.NotEmpty(t, tool.Description)
	}
}

func TestTools_AllHaveMixpanelPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "mixpanel_")
	}
}

func TestTools_NoDuplicateNames(t *testing.T) {
	i := New()
	seen := make(map[string]bool)
	for _, tool := range i.Tools() {
		assert.False(t, seen[tool.Name], "duplicate: %s", tool.Name)
		seen[tool.Name] = true
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	m := &mixpanel{username: "u", secret: "s", projectID: "1", client: &http.Client{}, baseURL: "mixpanel"}
	result, err := m.Execute(context.Background(), "mixpanel_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

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
```

### Task 2D: Implement Healthy()

Update `mixpanel.go`:

```go
func (m *mixpanel) Healthy(ctx context.Context) bool {
	_, err := m.query(ctx, "GET", "events/top?type=general&limit=1", nil)
	return err == nil
}
```

Also update `Execute()` to use the dispatch map:

```go
func (m *mixpanel) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, m, args)
}
```

### Task 2E: Commit

```bash
git add mixpanel/tools.go mixpanel/mixpanel.go mixpanel/mixpanel_test.go
git commit -m "feat(mixpanel): add 31 tool definitions with dispatch map and parity tests"
```

---

**CHECKPOINT: Phase 2 complete. Run `go test ./mixpanel/... -v`. All dispatch parity tests should pass. Stub handlers return "not implemented." Review before proceeding.**

---

## Phase 3: Analytics Handlers (10 tools)

- Unblocks: nothing downstream (leaf phase)
- Goal: Implement all analytics query handlers in `analytics.go`
- Parts:
  - Part A: Segmentation handlers (4 tools) — `querySegmentation`, `querySegmentationNumeric`, `querySegmentationSum`, `querySegmentationAverage`
  - Part B: Funnel handlers (2 tools) — `listFunnels`, `queryFunnel`
  - Part C: Retention handlers (3 tools) — `queryRetention`, `queryFrequency`
  - Part D: Insights + JQL handlers (2 tools) — `queryInsight`, `queryJQL`
  - Part E: Commit
- Done when: all 10 analytics handlers pass httptest-based tests
- Files: `mixpanel/analytics.go`, `mixpanel/mixpanel_test.go`

**Dependencies:**
- Parts A, B, C, D: ALL independent — **parallelize via subagents**
- Part E: blocks on A, B, C, D

**Parallelization note:** Parts A-D touch different handler functions in `analytics.go` and different test functions in `mixpanel_test.go`. They can be developed in parallel by separate subagents writing to non-overlapping sections.

### Task 3A: Segmentation handlers (TDD)

For each of the 4 segmentation tools, write a handler test first, then implement:

```go
// analytics.go
func querySegmentation(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	params := map[string]string{
		"event":     argStr(args, "event"),
		"from_date": argStr(args, "from_date"),
		"to_date":   argStr(args, "to_date"),
	}
	// Optional params
	if v := argStr(args, "on"); v != "" { params["on"] = v }
	if v := argStr(args, "unit"); v != "" { params["unit"] = v }
	if v := argStr(args, "where"); v != "" { params["where"] = v }
	if v := argInt(args, "limit"); v > 0 { params["limit"] = strconv.Itoa(v) }
	if v := argStr(args, "type"); v != "" { params["type"] = v }

	data, err := m.query(ctx, "GET", "segmentation"+queryEncode(params), nil)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
```

Pattern repeats for `querySegmentationNumeric` (path: `segmentation/numeric`), `querySegmentationSum` (path: `segmentation/sum`), `querySegmentationAverage` (path: `segmentation/average`).

Update dispatch map to replace `stubHandler` with real handlers.

### Task 3B: Funnel handlers (TDD)

- `listFunnels`: `GET /funnels/list` — no params beyond project_id
- `queryFunnel`: `GET /funnels` — funnel_id, from_date, to_date, optional length/unit/on/where/limit

### Task 3C: Retention handlers (TDD)

- `queryRetention`: `GET /retention` — from_date, to_date, optional retention_type/born_event/event/where/unit/interval
- `queryFrequency`: `GET /retention/addiction` — from_date, to_date, unit, addiction_unit, optional event/where/on/limit

### Task 3D: Insights + JQL handlers (TDD)

- `queryInsight`: `GET /insights` — bookmark_id
- `queryJQL`: `POST /jql` — script (body), optional params (body)

---

**CHECKPOINT: Phase 3 complete. Run `go test ./mixpanel/... -v`. 10 analytics handlers passing.**

---

## Phase 4: Events + Profiles Handlers (10 tools)

- Unblocks: nothing downstream (leaf phase)
- Goal: Implement event breakdown, raw export, activity feed, profiles, and cohorts handlers
- Parts:
  - Part A: Event breakdown handlers (6 tools) — `aggregateEvents`, `topEventsToday`, `topEventNames`, `eventPropertyValues`, `topEventProperties`, `topPropertyValues`
  - Part B: Raw export handler (1 tool) — `exportEvents` — **JSONL→JSON array conversion**
  - Part C: Activity + profiles + cohorts handlers (3 tools) — `queryActivity`, `queryProfiles`, `listCohorts`
  - Part D: Commit
- Done when: all 10 handlers pass, JSONL conversion tested
- Files: `mixpanel/events.go`, `mixpanel/mixpanel_test.go`

**Dependencies:**
- Parts A, B, C: ALL independent — **parallelize**
- Part D: blocks on A, B, C

**Can run in parallel with Phase 3.**

### Task 4B: Export handler — special JSONL handling

This is the most complex handler. The export endpoint returns JSONL (one JSON object per line), not a JSON array. The handler must:

1. Call `m.export()` which returns raw bytes
2. Split by newlines
3. Collect non-empty lines into a JSON array
4. Return as ToolResult

```go
func exportEvents(ctx context.Context, m *mixpanel, args map[string]any) (*mcp.ToolResult, error) {
	params := map[string]string{
		"from_date": argStr(args, "from_date"),
		"to_date":   argStr(args, "to_date"),
	}
	if v := argStr(args, "event"); v != "" { params["event"] = v }
	if v := argStr(args, "where"); v != "" { params["where"] = v }
	if v := argInt(args, "limit"); v > 0 { params["limit"] = strconv.Itoa(v) }

	data, err := m.export(ctx, "export", params)
	if err != nil {
		return errResult(err)
	}

	// Convert JSONL to JSON array
	lines := bytes.Split(data, []byte("\n"))
	var events []json.RawMessage
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) > 0 {
			events = append(events, json.RawMessage(line))
		}
	}

	result, err := json.Marshal(events)
	if err != nil {
		return errResult(err)
	}
	return &mcp.ToolResult{Data: string(result)}, nil
}
```

Test must verify JSONL→array conversion:

```go
func TestExportEvents_JSONLConversion(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("{\"event\":\"signup\"}\n{\"event\":\"login\"}\n"))
	}))
	defer ts.Close()

	m := testExportClient(ts)
	result, err := m.Execute(context.Background(), "mixpanel_export_events", map[string]any{
		"from_date": "2024-01-01",
		"to_date":   "2024-01-02",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, `[{"event":"signup"},{"event":"login"}]`)
}
```

---

**CHECKPOINT: Phase 4 complete. Run `go test ./mixpanel/... -v`. 10 event/profile handlers passing.**

---

## Phase 5: Schemas + Annotations Handlers (11 tools)

- Unblocks: nothing downstream (leaf phase)
- Goal: Implement annotation and lexicon schema handlers using the App API base URL
- Parts:
  - Part A: Annotation handlers (3 tools) — `listAnnotations`, `createAnnotation`, `deleteAnnotation`
  - Part B: Lexicon schema handlers (8 tools) — `listSchemas`, `listSchemasByEntity`, `getSchema`, `createSchemas`, `createSchema`, `deleteAllSchemas`, `deleteSchemasByEntity`, `deleteSchema`
  - Part C: Commit
- Done when: all 11 handlers pass with App API URL pattern verified
- Files: `mixpanel/schemas.go`, `mixpanel/mixpanel_test.go`

**Dependencies:**
- Parts A, B: independent — **parallelize**
- Part C: blocks on A, B

**Can run in parallel with Phases 3 and 4.**

### Task 5A: Annotation handlers (TDD)

All use the App API: `m.app(ctx, method, "annotations/...", body)`

### Task 5B: Lexicon schema handlers (TDD)

All use the App API: `m.app(ctx, method, "schemas/...", body)`

---

**CHECKPOINT: Phase 5 complete. Run `go test ./mixpanel/... -v`. All 31 handlers passing. No stub handlers remain.**

---

## Phase 6: Wiring + Final Verification

- Unblocks: production readiness
- Goal: Register Mixpanel in the server, add config defaults, pass `make ci`
- Parts:
  - Part A: Register in `cmd/server/main.go` — add import + `mixpanelInt.New()` to integration list
  - Part B: Add default config in `config/config.go` → `defaultConfig()`
  - Part C: Run `make ci` — must pass (build, vet, test-race, lint, security)
  - Part D: Final commit
- Done when: `make ci` passes clean
- Files: `cmd/server/main.go:14-30,167-178`, `config/config.go:45-89`

**Dependencies:**
- Part A: no blockers
- Part B: no blockers (parallel with A)
- Part C: blocks on A, B
- Part D: blocks on C

### Task 6A: Register in main.go

Modify `cmd/server/main.go`:

```go
// Add import (line ~24, alphabetical):
"github.com/daltoniam/switchboard/mixpanel"

// Add to integration list (line ~175, after posthog):
mixpanel.New(),
```

No alias needed — `mixpanel` package name doesn't collide with anything.

### Task 6B: Add default config

Modify `config/config.go` `defaultConfig()` (after the `"posthog"` entry):

```go
"mixpanel": {
	Enabled:     false,
	Credentials: mcp.Credentials{
		"service_account_username": "",
		"service_account_secret":   "",
		"project_id":               "",
		"base_url":                 "",
	},
},
```

### Task 6C: Run make ci

```bash
cd /Users/arjansingh/code/switchboard/worktrees/mixpanel && make ci
```

Expected: All checks pass (build, vet, test-race, lint, security).

### Task 6D: Final commit

```bash
git add cmd/server/main.go config/config.go
git commit -m "feat(mixpanel): wire integration into server and config"
```

---

**CHECKPOINT: Phase 6 complete. `make ci` passes. Mixpanel integration is fully operational.**

---

## Parallelization DAG

```
Phase 1 (Core) ──► Phase 2 (Tools+Dispatch) ──┬──► Phase 3 (Analytics)  ──┐
                                                ├──► Phase 4 (Events)     ──┼──► Phase 6 (Wiring)
                                                └──► Phase 5 (Schemas)    ──┘
```

**Maximum parallelism: Phases 3, 4, 5 can ALL run simultaneously** after Phase 2 completes. Within each phase, handler parts (A/B/C/D) are also independent.

---

## Testing Summary

| Component | Type | Key Scenarios |
|-----------|------|---------------|
| Constructor + Configure | Unit | New(), Name(), valid creds, 3 missing-cred failures, custom base_url |
| HTTP helpers | Unit (httptest) | query() success/error/204, export() JSONL, app() success, Basic Auth verification |
| Arg helpers | Unit | float64→int, string→bool, empty defaults |
| Dispatch parity | Unit | AllToolsCovered, NoOrphanHandlers |
| 31 tool handlers | Unit (httptest) | Success + API error per handler, JSONL conversion for export |

---

## Success Deliverables & Criteria

### Deliverables

| # | Deliverable | Acceptance Criteria |
|---|-------------|---------------------|
| 1 | **mixpanel/ package** | 31 tools, all tests pass, dispatch parity enforced |
| 2 | **Server wiring** | `mixpanel.New()` registered, config defaults present |
| 3 | **CI green** | `make ci` passes (build, vet, test-race, lint, security) |

### Quality Criteria

- All tests pass with race detector (`go test -race`)
- golangci-lint clean
- gosec + govulncheck clean
- No stub handlers remain

### What "Done" Looks Like

```
$ ./switchboard
$ curl localhost:3847/mcp -d '{"method":"tools/call","params":{"name":"search","arguments":{"query":"mixpanel segmentation"}}}'
→ Returns mixpanel_query_segmentation ToolDefinition

$ curl localhost:3847/mcp -d '{"method":"tools/call","params":{"name":"execute","arguments":{"tool_name":"mixpanel_query_segmentation","arguments":{"event":"signup","from_date":"2024-01-01","to_date":"2024-01-31"}}}}'
→ Returns segmentation data from Mixpanel API
```

---

## Progress

- [x] Phase 1: Core Adapter Foundation (`01bd214`)
- [x] Phase 2: Tool Definitions + Dispatch Skeleton (`bd27faa`)
- [x] Phase 3: Analytics Handlers (10 tools) (`78efca0`)
- [x] Phase 4: Events + Profiles Handlers (10 tools) (`78efca0`, parallel worktree)
- [x] Phase 5: Schemas + Annotations Handlers (11 tools) (`78efca0`, parallel worktree)
- [x] Phase 6: Wiring + Final Verification (`557a2ef`)
- [x] Code Review Pass (`2a7b554`, `a974c5d`)

**Blockers:** none
**Scope changes:** none

---

## Synopsis (2026-03-01)

### Implementation Complete — All 31 Tools Shipped

The Mixpanel integration is fully implemented and passing CI. Three sessions covered design, implementation, and review.

### Commit History

| Commit | Description |
|--------|-------------|
| `f0e9579` | Design doc and implementation plan |
| `01bd214` | Core adapter: struct, Configure, HTTP helpers (query/export/app), arg helpers, tests |
| `a0461f7` | Fix: expand app() to accept method+body, add export error test |
| `bd27faa` | 31 tool definitions with dispatch map and parity tests |
| `78efca0` | Analytics handlers (segmentation, funnels, retention, insights, JQL) — events and schemas done in parallel worktrees, merged here |
| `557a2ef` | Wire into server (main.go), config defaults, web UI setup page (templ) |
| `2a7b554` | Review fixes: Healthy() missing `/`, listCohorts POST→GET, Configure validate-then-assign, argPath() parse-at-boundary helper |
| `a974c5d` | Guard funnel_id/bookmark_id zero-values, fix stale comment |

### Review Findings & Resolutions

Three review agents (drjkl-code-reviewer, go-code-reviewer, feature-dev:code-reviewer) plus manual parse-don't-validate and category theory passes. Key findings:

| Finding | Resolution |
|---------|------------|
| `Healthy()` path missing leading `/` — would 404 in production | Fixed. Test also updated to not mask with trailing slash |
| `listCohorts` used POST instead of GET | Fixed to GET (matches API docs and peer list operations) |
| `Configure()` mutated struct before validation | Refactored to validate-then-assign with local variables |
| 8 path segments not `url.PathEscape`d in schemas.go | Added `argPath()` helper (parse-don't-validate at boundary), replaced all 8 sites |
| `funnel_id`/`bookmark_id` sent `"0"` when absent | Guarded with `if v > 0` pattern (CT coproduct collapse fix) |
| `SetIntegration` error discarded in web.go | Skipped — matches codebase convention across all setup handlers |

### Current State

- **CI**: `make ci` passes clean (build, vet, test-race, lint, gosec, govulncheck)
- **Coverage**: 86.4% in mixpanel package
- **Files**: `mixpanel.go` (234L), `tools.go` (234L), `analytics.go` (198L), `events.go` (216L), `schemas.go` (127L), `mixpanel_test.go` (~1357L)
- **Web UI**: `mixpanel_setup.templ` (86L) + generated `_templ.go`
- **Branch**: `feat/mixpanel`, ready for PR

### What's Left

- **Create PR** against `main`
- No known open issues or blockers
