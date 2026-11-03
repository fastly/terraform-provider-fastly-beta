package analyze

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteTextSupportedReportIsDeterministic(t *testing.T) {
	t.Parallel()

	result := analyzeFixture(t, "supported")
	var first bytes.Buffer
	var second bytes.Buffer

	if err := WriteText(&first, result); err != nil {
		t.Fatalf("WriteText() returned error: %v", err)
	}
	if err := WriteText(&second, result); err != nil {
		t.Fatalf("WriteText() returned error: %v", err)
	}
	if first.String() != second.String() {
		t.Fatal("WriteText() output is not deterministic")
	}

	output := first.String()
	for _, want := range []string{
		"Status: READY",
		"Migration plan:",
		"[resource-service-vcl]",
		"fastly_service_vcl.demo -> fastly_service_cdn_auto.demo",
		"References requiring rewrite:",
		"No blocking migration issues found.",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("report does not contain %q:\n%s", want, output)
		}
	}
}

func TestWriteTextBlockedReportIncludesActionableDiagnostics(t *testing.T) {
	t.Parallel()

	result := analyzeFixture(t, "blocked-lifecycle")
	var output bytes.Buffer

	if err := WriteText(&output, result); err != nil {
		t.Fatalf("WriteText() returned error: %v", err)
	}

	report := output.String()
	for _, want := range []string{
		"Status: BLOCKED",
		"Blocking issues: 3",
		"[cdn-attr-activate-false]",
		"[cdn-attr-stage-true]",
		"[cdn-attr-version-comment-set]",
		"Migration is blocked by 3 issue(s).",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("report does not contain %q:\n%s", want, report)
		}
	}
}
