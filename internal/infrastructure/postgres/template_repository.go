package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"gradeflow/reporting-service/internal/domain"
)

type TemplateRepository struct {
	db *sql.DB
}

func NewTemplateRepository(db *sql.DB) *TemplateRepository {
	return &TemplateRepository{db: db}
}

func (r *TemplateRepository) GetByCode(ctx context.Context, code string) (domain.ReportTemplate, error) {
	var template domain.ReportTemplate
	var defaultSortJSON, defaultGroupJSON []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT id, code, name, description, base_table, base_alias, default_sort, default_group_by
		FROM report_templates
		WHERE code = $1 AND is_active = true
	`, code).Scan(&template.ID, &template.Code, &template.Name, &template.Description, &template.BaseTable, &template.BaseAlias, &defaultSortJSON, &defaultGroupJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ReportTemplate{}, fmt.Errorf("%w: report template %q", domain.ErrNotFound, code)
	}
	if err != nil {
		return domain.ReportTemplate{}, err
	}
	if len(defaultSortJSON) > 0 {
		_ = json.Unmarshal(defaultSortJSON, &template.DefaultSort)
	}
	if len(defaultGroupJSON) > 0 {
		_ = json.Unmarshal(defaultGroupJSON, &template.DefaultGroupBy)
	}

	columns, err := r.columns(ctx, template.ID)
	if err != nil {
		return domain.ReportTemplate{}, err
	}
	joins, err := r.joins(ctx, template.ID)
	if err != nil {
		return domain.ReportTemplate{}, err
	}
	filters, err := r.filters(ctx, template.ID)
	if err != nil {
		return domain.ReportTemplate{}, err
	}
	template.Columns = columns
	template.Joins = joins
	template.Filters = filters
	return template, nil
}

func (r *TemplateRepository) columns(ctx context.Context, templateID int64) ([]domain.Column, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT column_key, expression, alias, data_type, is_visible, is_sortable, is_exportable, position
		FROM report_columns
		WHERE template_id = $1
		ORDER BY position
	`, templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []domain.Column
	for rows.Next() {
		var column domain.Column
		if err := rows.Scan(&column.Key, &column.Expression, &column.Alias, &column.DataType, &column.Visible, &column.Sortable, &column.Exportable, &column.Position); err != nil {
			return nil, err
		}
		columns = append(columns, column)
	}
	return columns, rows.Err()
}

func (r *TemplateRepository) joins(ctx context.Context, templateID int64) ([]domain.Join, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT join_type, table_name, table_alias, on_expression, position
		FROM report_joins
		WHERE template_id = $1
		ORDER BY position
	`, templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var joins []domain.Join
	for rows.Next() {
		var join domain.Join
		if err := rows.Scan(&join.JoinType, &join.Table, &join.Alias, &join.On, &join.Position); err != nil {
			return nil, err
		}
		joins = append(joins, join)
	}
	return joins, rows.Err()
}

func (r *TemplateRepository) filters(ctx context.Context, templateID int64) ([]domain.FilterConfig, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT filter_key, expression, data_type, operators, is_required, is_multi_value, description
		FROM report_filters
		WHERE template_id = $1
		ORDER BY filter_key
	`, templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var filters []domain.FilterConfig
	for rows.Next() {
		var filter domain.FilterConfig
		var operatorsJSON []byte
		if err := rows.Scan(&filter.Key, &filter.Expression, &filter.DataType, &operatorsJSON, &filter.Required, &filter.MultiValue, &filter.Description); err != nil {
			return nil, err
		}
		if len(operatorsJSON) > 0 {
			if err := json.Unmarshal(operatorsJSON, &filter.Operators); err != nil {
				return nil, err
			}
		}
		filters = append(filters, filter)
	}
	return filters, rows.Err()
}
