package querybuilder

import (
	"strings"
	"testing"

	"gradeflow/reporting-service/internal/domain"
)

func TestBuildParameterizedQuery(t *testing.T) {
	template := sampleTemplate()
	query, err := Build(BuildOptions{
		Template: template,
		Request: domain.ReportRequest{
			Filters: []domain.RuntimeFilter{
				{Key: "status", Operator: domain.OpEquals, Value: "submitted"},
				{Key: "student_name", Operator: domain.OpContains, Value: "ana"},
				{Key: "score", Operator: domain.OpBetween, Values: []any{80, 100}},
			},
			Sort:     []domain.Sort{{Key: "score", Direction: "desc"}},
			Page:     2,
			PageSize: 25,
		},
		IncludePaging: true,
		MaxPageSize:   100,
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	wantParts := []string{
		`SELECT s.id AS "submission_id", u.full_name AS "student_name", s.status AS "status", e.score AS "score"`,
		`FROM submissions AS "s"`,
		`LEFT JOIN users AS "u" ON u.id = s.student_id`,
		`LEFT JOIN evaluations AS "e" ON e.submission_id = s.id`,
		`WHERE s.status = $1 AND u.full_name ILIKE $2 AND e.score BETWEEN $3 AND $4`,
		`ORDER BY e.score DESC LIMIT $5 OFFSET $6`,
	}
	for _, part := range wantParts {
		if !strings.Contains(query.SQL, part) {
			t.Fatalf("query missing %q\n%s", part, query.SQL)
		}
	}
	if len(query.Args) != 6 {
		t.Fatalf("expected 6 args, got %d: %#v", len(query.Args), query.Args)
	}
	if query.Args[1] != "%ana%" {
		t.Fatalf("contains filter was not wrapped, got %#v", query.Args[1])
	}
	if query.Page.Number != 2 || query.Page.Size != 25 {
		t.Fatalf("unexpected page: %#v", query.Page)
	}
}

func TestBuildRejectsUnknownFilter(t *testing.T) {
	_, err := Build(BuildOptions{
		Template: sampleTemplate(),
		Request: domain.ReportRequest{
			Filters: []domain.RuntimeFilter{{Key: "unknown", Operator: domain.OpEquals, Value: "x"}},
		},
		IncludePaging: true,
		MaxPageSize:   100,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unknown filter") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildRejectsUnsafeTemplateFragment(t *testing.T) {
	template := sampleTemplate()
	template.Columns[0].Expression = "s.id; DROP TABLE users"
	_, err := Build(BuildOptions{Template: template, IncludePaging: true, MaxPageSize: 100})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildCapsPageSize(t *testing.T) {
	query, err := Build(BuildOptions{
		Template:      sampleTemplate(),
		Request:       domain.ReportRequest{Page: 0, PageSize: 1000},
		IncludePaging: true,
		MaxPageSize:   200,
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if query.Page.Number != 1 || query.Page.Size != 200 {
		t.Fatalf("unexpected page: %#v", query.Page)
	}
}

func TestBuildExportModeUsesOnlyExportableColumns(t *testing.T) {
	template := sampleTemplate()
	template.Columns[1].Exportable = false

	query, err := Build(BuildOptions{
		Template:    template,
		ExportMode:  true,
		MaxPageSize: 100,
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if strings.Contains(query.SQL, `student_name`) {
		t.Fatalf("export query should exclude non-exportable column: %s", query.SQL)
	}
}

func sampleTemplate() domain.ReportTemplate {
	return domain.ReportTemplate{
		Code:      "submission_summary",
		Name:      "Submission Summary",
		BaseTable: "submissions",
		BaseAlias: "s",
		Columns: []domain.Column{
			{Key: "submission_id", Expression: "s.id", Alias: "submission_id", Visible: true, Sortable: true, Exportable: true, Position: 1},
			{Key: "student_name", Expression: "u.full_name", Alias: "student_name", Visible: true, Sortable: true, Exportable: true, Position: 2},
			{Key: "status", Expression: "s.status", Alias: "status", Visible: true, Sortable: true, Exportable: true, Position: 3},
			{Key: "score", Expression: "e.score", Alias: "score", Visible: true, Sortable: true, Exportable: true, Position: 4},
		},
		Joins: []domain.Join{
			{JoinType: "LEFT", Table: "users", Alias: "u", On: "u.id = s.student_id", Position: 1},
			{JoinType: "LEFT", Table: "evaluations", Alias: "e", On: "e.submission_id = s.id", Position: 2},
		},
		Filters: []domain.FilterConfig{
			{Key: "status", Expression: "s.status", Operators: []domain.Operator{domain.OpEquals, domain.OpIn}},
			{Key: "student_name", Expression: "u.full_name", Operators: []domain.Operator{domain.OpContains}},
			{Key: "score", Expression: "e.score", Operators: []domain.Operator{domain.OpBetween, domain.OpGreaterThanOrEqual}},
		},
	}
}
