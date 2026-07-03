package querybuilder

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"gradeflow/reporting-service/internal/domain"
)

var safeSQLFragment = regexp.MustCompile(`^[a-zA-Z0-9_."()\s,+\-*/:<>=]+$`)

type BuildOptions struct {
	Template      domain.ReportTemplate
	Request       domain.ReportRequest
	IncludePaging bool
	ExportMode    bool
	MaxPageSize   int
}

type Query struct {
	SQL     string
	Args    []any
	Columns []string
	Page    domain.Page
}

func Build(opts BuildOptions) (Query, error) {
	t := opts.Template
	if err := t.Validate(); err != nil {
		return Query{}, err
	}
	if err := validateTemplateFragments(t); err != nil {
		return Query{}, err
	}

	page, pageSize := normalizePage(opts.Request.Page, opts.Request.PageSize, opts.MaxPageSize)
	selectedColumns := selectedColumns(t.Columns, opts.ExportMode)
	if len(selectedColumns) == 0 {
		return Query{}, fmt.Errorf("%w: no report columns are available", domain.ErrInvalidTemplate)
	}
	columnAliases := make([]string, 0, len(selectedColumns))
	selectParts := make([]string, 0, len(selectedColumns))
	for _, c := range selectedColumns {
		columnAliases = append(columnAliases, c.Alias)
		selectParts = append(selectParts, fmt.Sprintf("%s AS %s", c.Expression, quoteIdent(c.Alias)))
	}

	args := make([]any, 0)
	where, err := buildWhere(t.Filters, opts.Request.Filters, &args)
	if err != nil {
		return Query{}, err
	}

	from := fmt.Sprintf("%s AS %s", t.BaseTable, quoteIdent(t.BaseAlias))
	joins := make([]string, 0, len(t.Joins))
	for _, join := range sortedJoins(t.Joins) {
		joins = append(joins, fmt.Sprintf("%s JOIN %s AS %s ON %s", join.JoinType, join.Table, quoteIdent(join.Alias), join.On))
	}

	groupBy := ""
	if len(t.DefaultGroupBy) > 0 {
		groupBy = " GROUP BY " + strings.Join(t.DefaultGroupBy, ", ")
	}

	sortSQL, err := buildSort(t.Columns, opts.Request.Sort, t.DefaultSort)
	if err != nil {
		return Query{}, err
	}

	sql := fmt.Sprintf("SELECT %s FROM %s", strings.Join(selectParts, ", "), from)
	if len(joins) > 0 {
		sql += " " + strings.Join(joins, " ")
	}
	if where != "" {
		sql += " WHERE " + where
	}
	sql += groupBy
	if sortSQL != "" {
		sql += " ORDER BY " + sortSQL
	}
	if opts.IncludePaging {
		sql += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
		args = append(args, pageSize+1, (page-1)*pageSize)
	}

	return Query{
		SQL:     sql,
		Args:    args,
		Columns: columnAliases,
		Page: domain.Page{
			Number: page,
			Size:   pageSize,
		},
	}, nil
}

func buildWhere(configs []domain.FilterConfig, filters []domain.RuntimeFilter, args *[]any) (string, error) {
	byKey := make(map[string]domain.FilterConfig, len(configs))
	applied := make(map[string]bool, len(filters))
	for _, cfg := range configs {
		byKey[cfg.Key] = cfg
	}

	parts := make([]string, 0, len(filters))
	for _, filter := range filters {
		cfg, ok := byKey[filter.Key]
		if !ok {
			return "", fmt.Errorf("%w: unknown filter %q", domain.ErrInvalidRequest, filter.Key)
		}
		if !slices.Contains(cfg.Operators, filter.Operator) {
			return "", fmt.Errorf("%w: operator %q not allowed for filter %q", domain.ErrInvalidRequest, filter.Operator, filter.Key)
		}
		expr, err := filterExpression(cfg.Expression, filter, args)
		if err != nil {
			return "", err
		}
		parts = append(parts, expr)
		applied[filter.Key] = true
	}

	for _, cfg := range configs {
		if cfg.Required && !applied[cfg.Key] {
			return "", fmt.Errorf("%w: missing required filter %q", domain.ErrInvalidRequest, cfg.Key)
		}
	}
	return strings.Join(parts, " AND "), nil
}

func filterExpression(column string, filter domain.RuntimeFilter, args *[]any) (string, error) {
	switch filter.Operator {
	case domain.OpEquals:
		return addUnary(column, "=", filter.Value, args)
	case domain.OpNotEquals:
		return addUnary(column, "<>", filter.Value, args)
	case domain.OpContains:
		return addUnary(column, "ILIKE", "%"+fmt.Sprint(filter.Value)+"%", args)
	case domain.OpStartsWith:
		return addUnary(column, "ILIKE", fmt.Sprint(filter.Value)+"%", args)
	case domain.OpEndsWith:
		return addUnary(column, "ILIKE", "%"+fmt.Sprint(filter.Value), args)
	case domain.OpGreaterThan:
		return addUnary(column, ">", filter.Value, args)
	case domain.OpGreaterThanOrEqual:
		return addUnary(column, ">=", filter.Value, args)
	case domain.OpLessThan:
		return addUnary(column, "<", filter.Value, args)
	case domain.OpLessThanOrEqual:
		return addUnary(column, "<=", filter.Value, args)
	case domain.OpBetween:
		if len(filter.Values) != 2 {
			return "", fmt.Errorf("%w: between requires exactly two values", domain.ErrInvalidRequest)
		}
		*args = append(*args, filter.Values[0], filter.Values[1])
		return fmt.Sprintf("%s BETWEEN $%d AND $%d", column, len(*args)-1, len(*args)), nil
	case domain.OpIn:
		if len(filter.Values) == 0 {
			return "", fmt.Errorf("%w: in requires at least one value", domain.ErrInvalidRequest)
		}
		placeholders := make([]string, 0, len(filter.Values))
		for _, value := range filter.Values {
			*args = append(*args, value)
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(*args)))
		}
		return fmt.Sprintf("%s IN (%s)", column, strings.Join(placeholders, ", ")), nil
	case domain.OpIsNull:
		return column + " IS NULL", nil
	case domain.OpIsNotNull:
		return column + " IS NOT NULL", nil
	default:
		return "", fmt.Errorf("%w: unsupported operator %q", domain.ErrInvalidRequest, filter.Operator)
	}
}

func addUnary(column, op string, value any, args *[]any) (string, error) {
	if value == nil {
		return "", fmt.Errorf("%w: value is required", domain.ErrInvalidRequest)
	}
	*args = append(*args, value)
	return fmt.Sprintf("%s %s $%d", column, op, len(*args)), nil
}

func buildSort(columns []domain.Column, requested []domain.Sort, defaults []domain.Sort) (string, error) {
	sort := requested
	if len(sort) == 0 {
		sort = defaults
	}
	if len(sort) == 0 {
		return "", nil
	}

	byKey := make(map[string]domain.Column, len(columns))
	for _, c := range columns {
		byKey[c.Key] = c
	}

	parts := make([]string, 0, len(sort))
	for _, s := range sort {
		col, ok := byKey[s.Key]
		if !ok || !col.Sortable {
			return "", fmt.Errorf("%w: column %q is not sortable", domain.ErrInvalidRequest, s.Key)
		}
		direction := strings.ToUpper(s.Direction)
		if direction == "" {
			direction = "ASC"
		}
		if direction != "ASC" && direction != "DESC" {
			return "", fmt.Errorf("%w: invalid sort direction %q", domain.ErrInvalidRequest, s.Direction)
		}
		parts = append(parts, col.Expression+" "+direction)
	}
	return strings.Join(parts, ", "), nil
}

func selectedColumns(columns []domain.Column, exportMode bool) []domain.Column {
	out := make([]domain.Column, 0, len(columns))
	for _, column := range columns {
		if !column.Visible {
			continue
		}
		if exportMode && !column.Exportable {
			continue
		}
		out = append(out, column)
	}
	slices.SortFunc(out, func(a, b domain.Column) int { return a.Position - b.Position })
	return out
}

func sortedJoins(joins []domain.Join) []domain.Join {
	out := append([]domain.Join(nil), joins...)
	slices.SortFunc(out, func(a, b domain.Join) int { return a.Position - b.Position })
	return out
}

func validateTemplateFragments(t domain.ReportTemplate) error {
	fragments := []string{t.BaseTable, t.BaseAlias}
	fragments = append(fragments, t.DefaultGroupBy...)
	for _, c := range t.Columns {
		fragments = append(fragments, c.Expression, c.Alias)
	}
	for _, j := range t.Joins {
		fragments = append(fragments, j.JoinType, j.Table, j.Alias, j.On)
	}
	for _, f := range t.Filters {
		fragments = append(fragments, f.Expression)
	}
	for _, fragment := range fragments {
		if fragment == "" || !safeSQLFragment.MatchString(fragment) || strings.Contains(fragment, ";") || strings.Contains(fragment, "--") {
			return fmt.Errorf("%w: unsafe SQL fragment %q", domain.ErrInvalidTemplate, fragment)
		}
	}
	return nil
}

func quoteIdent(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func normalizePage(page, pageSize, maxPageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if maxPageSize <= 0 {
		maxPageSize = 500
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}
