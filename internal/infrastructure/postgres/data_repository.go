package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"gradeflow/reporting-service/internal/domain"
)

type DataRepository struct {
	db *sql.DB
}

func NewDataRepository(db *sql.DB) *DataRepository {
	return &DataRepository{db: db}
}

func (r *DataRepository) Query(ctx context.Context, query string, args []any) (domain.QueryResult, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return domain.QueryResult{}, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return domain.QueryResult{}, err
	}

	result := domain.QueryResult{Columns: columns, Rows: make([]domain.Row, 0)}
	for rows.Next() {
		row, err := scanRow(rows, columns)
		if err != nil {
			return domain.QueryResult{}, err
		}
		result.Rows = append(result.Rows, row)
	}
	return result, rows.Err()
}

func (r *DataRepository) Stream(ctx context.Context, query string, args []any, onRow func(domain.Row) error) error {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return err
	}
	for rows.Next() {
		row, err := scanRow(rows, columns)
		if err != nil {
			return err
		}
		if err := onRow(row); err != nil {
			return err
		}
	}
	return rows.Err()
}

func scanRow(rows *sql.Rows, columns []string) (domain.Row, error) {
	values := make([]any, len(columns))
	pointers := make([]any, len(columns))
	for i := range values {
		pointers[i] = &values[i]
	}
	if err := rows.Scan(pointers...); err != nil {
		return nil, err
	}

	row := make(domain.Row, len(columns))
	for i, column := range columns {
		row[column] = normalizeDBValue(values[i])
	}
	return row, nil
}

func normalizeDBValue(value any) any {
	switch v := value.(type) {
	case []byte:
		return string(v)
	case time.Time:
		return v.Format(time.RFC3339)
	default:
		if v == nil {
			return nil
		}
		return fmt.Sprint(v)
	}
}
