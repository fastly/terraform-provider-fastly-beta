package migrate

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"

	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/analyze"
)

func TestWriteTextReadyUsesFindingSeverity(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sourceRange := hcl.Range{Filename: filepath.Join(dir, "main.tf")}
	result := Result{
		Analysis: analyze.Result{
			Root:  dir,
			Files: []string{"main.tf"},
			Findings: []analyze.Finding{
				{Severity: analyze.SeverityWarning, Range: sourceRange, Message: "needs review"},
				{Severity: analyze.Severity("info"), Range: sourceRange, Message: "informational notice"},
			},
		},
	}
	var output bytes.Buffer
	if err := WriteText(&output, result); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"Migration dry-run: READY",
		"WARNING main.tf: needs review",
		"INFO main.tf: informational notice",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("missing %q in output:\n%s", expected, output.String())
		}
	}
	if strings.Contains(output.String(), "WARNING main.tf: informational notice") {
		t.Fatalf("information finding mislabeled as warning:\n%s", output.String())
	}
}
