package rewrite

import (
	"strings"
	"testing"
)

func TestAddStringDefaultMissingParentDiagnostic(t *testing.T) {
	t.Parallel()

	source := []byte(`resource "fastly_service_vcl" "demo" {
  name = "demo"
}
`)
	_, err := (AddStringDefault{
		ResourceType: "fastly_service_vcl",
		ResourceName: "demo",
		Parents:      []string{"logging_splunk"},
		Attribute:    "format",
		Value:        "DO_NOT_REPORT_PRIVATE_FORMAT",
	}).apply(source, "testdata/missing-logging.tf")
	if err == nil {
		t.Fatal("missing parent path unexpectedly accepted")
	}
	for _, want := range []string{
		"testdata/missing-logging.tf",
		"fastly_service_vcl.demo",
		"logging_splunk",
		"format",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("missing %q in error: %v", want, err)
		}
	}
	if strings.Contains(err.Error(), "DO_NOT_REPORT_PRIVATE_FORMAT") {
		t.Fatal("default value leaked into diagnostic")
	}
}
