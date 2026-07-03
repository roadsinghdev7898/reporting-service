package usecase

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"gradeflow/reporting-service/internal/domain"
	"gradeflow/reporting-service/internal/export"
	"gradeflow/reporting-service/internal/querybuilder"
)

type ReportService struct {
	templates       domain.TemplateRepository
	data            domain.DataRepository
	exporters       export.Registry
	maxPageSize     int
	exportFetchSize int
}

func NewReportService(templates domain.TemplateRepository, data domain.DataRepository, exporters export.Registry, maxPageSize, exportFetchSize int) *ReportService {
	return &ReportService{
		templates:       templates,
		data:            data,
		exporters:       exporters,
		maxPageSize:     maxPageSize,
		exportFetchSize: exportFetchSize,
	}
}

func (s *ReportService) GetTemplate(ctx context.Context, code string) (domain.ReportTemplate, error) {
	return s.templates.GetByCode(ctx, code)
}

func (s *ReportService) Generate(ctx context.Context, code string, req domain.ReportRequest) (domain.QueryResult, error) {
	template, err := s.templates.GetByCode(ctx, code)
	if err != nil {
		return domain.QueryResult{}, err
	}
	query, err := querybuilder.Build(querybuilder.BuildOptions{
		Template:      template,
		Request:       req,
		IncludePaging: true,
		MaxPageSize:   s.maxPageSize,
	})
	if err != nil {
		return domain.QueryResult{}, err
	}

	result, err := s.data.Query(ctx, query.SQL, query.Args)
	if err != nil {
		return domain.QueryResult{}, err
	}
	if len(result.Rows) > query.Page.Size {
		result.Rows = result.Rows[:query.Page.Size]
		query.Page.HasNext = true
	}
	query.Page.Returned = len(result.Rows)
	query.Page.Total = nil
	result.Columns = query.Columns
	result.Page = query.Page
	return result, nil
}

func (s *ReportService) Export(ctx context.Context, code, format string, req domain.ReportRequest) (string, string, io.Reader, error) {
	template, err := s.templates.GetByCode(ctx, code)
	if err != nil {
		return "", "", nil, err
	}
	exporter, ok := s.exporters[format]
	if !ok {
		return "", "", nil, fmt.Errorf("%w: %s", domain.ErrUnsupportedExport, format)
	}

	query, err := querybuilder.Build(querybuilder.BuildOptions{
		Template:        template,
		Request:         req,
		IncludePaging:   false,
		MaxPageSize:     s.maxPageSize,
		ExportFetchSize: s.exportFetchSize,
	})
	if err != nil {
		return "", "", nil, err
	}

	buffer := bytes.NewBuffer(nil)
	writer, err := exporter.NewWriter(buffer, query.Columns)
	if err != nil {
		return "", "", nil, err
	}
	if err := s.data.Stream(ctx, query.SQL, query.Args, writer.WriteRow); err != nil {
		return "", "", nil, err
	}
	if err := writer.Close(); err != nil {
		return "", "", nil, err
	}

	filename := fmt.Sprintf("%s.%s", template.Code, exporter.Extension())
	return filename, exporter.ContentType(), buffer, nil
}
