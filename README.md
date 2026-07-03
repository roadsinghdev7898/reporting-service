# Reporting/MIS Service

Generic, configuration-driven reporting service built with Go, PostgreSQL, and Chi. New reports are introduced by inserting or updating report template configuration in PostgreSQL rather than changing application code.

## High-Level Design

The service follows Clean Architecture:

- `cmd/server`: process bootstrap, database connection, router wiring, graceful shutdown.
- `internal/domain`: core report entities, repository interfaces, request/response models, domain errors.
- `internal/querybuilder`: converts a persisted report template plus runtime filters/sort/page inputs into parameterized PostgreSQL SQL.
- `internal/usecase`: report generation and export orchestration.
- `internal/infrastructure/postgres`: PostgreSQL template and data repositories.
- `internal/transport/http`: Chi routes, JSON decoding, error mapping, request logging.
- `internal/export`: CSV and XLSX writers.
- `migrations`: database schema and example template.

Runtime request flow:

1. Client calls `POST /api/v1/reports/{code}/generate` or `/export/{format}`.
2. Service loads the active template by `code`.
3. Query builder validates configured SQL fragments and runtime inputs.
4. Filters become parameterized predicates such as `status = $1`.
5. Data repository executes the generated query.
6. Generate returns JSON rows; export streams rows into CSV or Excel.

## Database Schema

Run [migrations/001_report_templates.sql](/home/developer/gradeflow/reporting-service/migrations/001_report_templates.sql) against the reporting database.

Tables:

- `report_templates`: report identity, base table, base alias, default sort, default grouping, active flag.
- `report_columns`: selectable output columns, aliases, order, data type, sortable/exportable flags.
- `report_joins`: ordered join definitions from the base table to related tables.
- `report_filters`: allowed runtime filters and supported operators per filter.

Supported operators:

- `equals`
- `not_equals`
- `contains`
- `starts_with`
- `ends_with`
- `between`
- `greater_than`
- `greater_than_or_equal`
- `less_than`
- `less_than_or_equal`
- `in`
- `is_null`
- `is_not_null`

## API Design

Health:

```http
GET /healthz
```

Fetch template metadata:

```http
GET /api/v1/reports/{code}/template
```

Generate paginated JSON:

```http
POST /api/v1/reports/{code}/generate
Content-Type: application/json

{
  "filters": [
    {"key": "status", "operator": "in", "values": ["submitted", "graded"]},
    {"key": "submitted_at", "operator": "between", "values": ["2026-01-01", "2026-07-01"]}
  ],
  "sort": [
    {"key": "submitted_at", "direction": "desc"}
  ],
  "page": 1,
  "page_size": 50
}
```

Export:

```http
POST /api/v1/reports/{code}/export/csv
POST /api/v1/reports/{code}/export/xlsx
```

The export body is the same as the generate request. The response is an attachment named `{report_code}.csv` or `{report_code}.xlsx`.

## Dynamic Query Builder

Templates define only whitelisted SQL fragments:

- base table and alias
- selected column expressions
- joins and `ON` clauses
- filter expressions
- sortable columns
- group-by expressions

Runtime values never get interpolated into SQL. They are converted into PostgreSQL placeholders and passed as `[]any` arguments.

Example:

```sql
SELECT s.id AS "submission_id", u.full_name AS "student_name"
FROM submissions AS "s"
LEFT JOIN users AS "u" ON u.id = s.student_id
WHERE s.status = $1 AND u.full_name ILIKE $2
ORDER BY s.submitted_at DESC
LIMIT $3 OFFSET $4
```

Query safety controls:

- runtime filters must exist in `report_filters`
- runtime operators must be configured for that filter
- sort keys must map to sortable report columns
- SQL fragments reject semicolons, comments, and unsafe characters
- all runtime values use parameterized placeholders

## Export Mechanism

CSV uses Go's standard `encoding/csv`.

Excel uses `github.com/xuri/excelize/v2`.

The use case exposes a common exporter registry so more formats can be added without touching HTTP handlers or query building.

## Error Handling And Logging

Domain errors are mapped to HTTP responses:

- `ErrNotFound`: `404`
- `ErrInvalidRequest`: `400`
- `ErrInvalidTemplate`: `400`
- `ErrUnsupportedExport`: `415`
- unexpected errors: `500`

Logging uses structured `log/slog` JSON logs. The router records method, path, and remote address. Server-side failures are logged at error level.

## Performance Notes

- Pagination fetches `page_size + 1` rows to determine `has_next` without running an expensive count query.
- Exports use row iteration through `DataRepository.Stream`.
- PostgreSQL indexes should be added on source-table columns commonly used by configured filters, joins, and sorts.
- Keep report columns narrow for large exports and use database views/materialized views when reports require expensive derived expressions.
- Connection pool sizes are configurable through environment variables.

## Configuration

Environment variables:

| Variable | Default |
| --- | --- |
| `HTTP_ADDR` | `:8080` |
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/reporting?sslmode=disable` |
| `LOG_LEVEL` | `info` |
| `DB_MAX_OPEN_CONNS` | `20` |
| `DB_MAX_IDLE_CONNS` | `10` |
| `DB_CONN_MAX_LIFETIME_MINUTES` | `30` |
| `MAX_PAGE_SIZE` | `500` |
| `EXPORT_FETCH_SIZE` | `1000` |

## Local Development

```bash
cd reporting-service
go mod tidy
go test ./...
go run ./cmd/server
```

Build container:

```bash
docker build -t reporting-service .
```

## Adding A New Report

1. Insert a row in `report_templates`.
2. Insert output columns in `report_columns`.
3. Insert any joins in `report_joins`.
4. Insert runtime filters in `report_filters`.
5. Call `POST /api/v1/reports/{new_code}/generate`.

No Go code changes are required when the new report fits the existing template model.
