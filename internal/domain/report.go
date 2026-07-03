package domain

import (
	"context"
	"errors"
	"fmt"
)

var (
	ErrNotFound          = errors.New("resource not found")
	ErrInvalidTemplate   = errors.New("invalid report template")
	ErrInvalidRequest    = errors.New("invalid request")
	ErrUnsupportedExport = errors.New("unsupported export format")
)

type TemplateRepository interface {
	GetByCode(ctx context.Context, code string) (ReportTemplate, error)
}

type DataRepository interface {
	Query(ctx context.Context, query string, args []any) (QueryResult, error)
	Stream(ctx context.Context, query string, args []any, onRow func(Row) error) error
}

type ReportTemplate struct {
	ID             int64
	Code           string
	Name           string
	Description    string
	BaseTable      string
	BaseAlias      string
	DefaultSort    []Sort
	DefaultGroupBy []string
	Columns        []Column
	Joins          []Join
	Filters        []FilterConfig
}

type Column struct {
	Key        string
	Expression string
	Alias      string
	DataType   string
	Visible    bool
	Sortable   bool
	Exportable bool
	Position   int
}

type Join struct {
	JoinType string
	Table    string
	Alias    string
	On       string
	Position int
}

type FilterConfig struct {
	Key         string
	Expression  string
	DataType    string
	Operators   []Operator
	Required    bool
	MultiValue  bool
	Description string
}

type Operator string

const (
	OpEquals             Operator = "equals"
	OpNotEquals          Operator = "not_equals"
	OpContains           Operator = "contains"
	OpStartsWith         Operator = "starts_with"
	OpEndsWith           Operator = "ends_with"
	OpBetween            Operator = "between"
	OpGreaterThan        Operator = "greater_than"
	OpGreaterThanOrEqual Operator = "greater_than_or_equal"
	OpLessThan           Operator = "less_than"
	OpLessThanOrEqual    Operator = "less_than_or_equal"
	OpIn                 Operator = "in"
	OpIsNull             Operator = "is_null"
	OpIsNotNull          Operator = "is_not_null"
)

type RuntimeFilter struct {
	Key      string   `json:"key"`
	Operator Operator `json:"operator"`
	Value    any      `json:"value,omitempty"`
	Values   []any    `json:"values,omitempty"`
}

type Sort struct {
	Key       string `json:"key"`
	Direction string `json:"direction"`
}

type ReportRequest struct {
	Filters  []RuntimeFilter `json:"filters"`
	Sort     []Sort          `json:"sort"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

type QueryResult struct {
	Columns []string `json:"columns"`
	Rows    []Row    `json:"rows"`
	Page    Page     `json:"page"`
}

type Row map[string]any

type Page struct {
	Number   int  `json:"number"`
	Size     int  `json:"size"`
	Returned int  `json:"returned"`
	Total    *int `json:"total,omitempty"`
	HasNext  bool `json:"has_next"`
}

func (t ReportTemplate) Validate() error {
	if t.Code == "" || t.BaseTable == "" || t.BaseAlias == "" {
		return fmt.Errorf("%w: code, base_table, and base_alias are required", ErrInvalidTemplate)
	}
	if len(t.Columns) == 0 {
		return fmt.Errorf("%w: at least one column is required", ErrInvalidTemplate)
	}
	return nil
}
