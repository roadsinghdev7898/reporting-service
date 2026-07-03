package export

import (
	"bytes"
	"strings"
	"testing"

	"gradeflow/reporting-service/internal/domain"
)

func TestCSVExporterWritesHeaderAndRows(t *testing.T) {
	buffer := bytes.NewBuffer(nil)
	writer, err := NewCSVExporter().NewWriter(buffer, []string{"name", "score"})
	if err != nil {
		t.Fatalf("NewWriter returned error: %v", err)
	}
	if err := writer.WriteRow(domain.Row{"name": "Ana", "score": 95}); err != nil {
		t.Fatalf("WriteRow returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	got := buffer.String()
	for _, want := range []string{"name,score", "Ana,95"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in CSV, got %q", want, got)
		}
	}
}
