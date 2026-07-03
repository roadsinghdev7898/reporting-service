package export

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/xuri/excelize/v2"

	"gradeflow/reporting-service/internal/domain"
)

type Registry map[string]Exporter

type Exporter interface {
	NewWriter(w io.Writer, columns []string) (Writer, error)
	ContentType() string
	Extension() string
}

type Writer interface {
	WriteRow(row domain.Row) error
	Close() error
}

type CSVExporter struct{}

func NewCSVExporter() CSVExporter { return CSVExporter{} }

func (CSVExporter) NewWriter(w io.Writer, columns []string) (Writer, error) {
	cw := csv.NewWriter(w)
	if err := cw.Write(columns); err != nil {
		return nil, err
	}
	return &csvWriter{writer: cw, columns: columns}, nil
}

func (CSVExporter) ContentType() string { return "text/csv" }
func (CSVExporter) Extension() string   { return "csv" }

type csvWriter struct {
	writer  *csv.Writer
	columns []string
}

func (w *csvWriter) WriteRow(row domain.Row) error {
	record := make([]string, 0, len(w.columns))
	for _, column := range w.columns {
		record = append(record, stringify(row[column]))
	}
	return w.writer.Write(record)
}

func (w *csvWriter) Close() error {
	w.writer.Flush()
	return w.writer.Error()
}

type ExcelExporter struct{}

func NewExcelExporter() ExcelExporter { return ExcelExporter{} }

func (ExcelExporter) NewWriter(w io.Writer, columns []string) (Writer, error) {
	file := excelize.NewFile()
	sheet := "Report"
	file.SetSheetName("Sheet1", sheet)
	for i, column := range columns {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		file.SetCellValue(sheet, cell, column)
	}
	return &excelWriter{out: w, file: file, sheet: sheet, columns: columns, row: 2}, nil
}

func (ExcelExporter) ContentType() string {
	return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
}

func (ExcelExporter) Extension() string { return "xlsx" }

type excelWriter struct {
	out     io.Writer
	file    *excelize.File
	sheet   string
	columns []string
	row     int
}

func (w *excelWriter) WriteRow(row domain.Row) error {
	for i, column := range w.columns {
		cell, _ := excelize.CoordinatesToCellName(i+1, w.row)
		if err := w.file.SetCellValue(w.sheet, cell, row[column]); err != nil {
			return err
		}
	}
	w.row++
	return nil
}

func (w *excelWriter) Close() error {
	defer w.file.Close()
	return w.file.Write(w.out)
}

func stringify(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case []byte:
		return string(v)
	case time.Time:
		return v.Format(time.RFC3339)
	case fmt.Stringer:
		return v.String()
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		return fmt.Sprint(v)
	}
}
