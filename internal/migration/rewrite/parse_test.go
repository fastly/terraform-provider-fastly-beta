package rewrite

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
)

func TestParseBodySyntaxErrorOmitsSensitiveSource(t *testing.T) {
	t.Parallel()
	// A duplicate attribute diagnostic may contain the attribute name.
	source := []byte("locals {\n  SYNTHETIC_SECRET = \"first\"\n  SYNTHETIC_SECRET = \"second\"\n}\n")
	body, err := parseBody(source, "main.tf")
	want := "parse native HCL in main.tf: invalid syntax at line 3, column 3 (source text omitted)"
	if body != nil || err == nil || err.Error() != want {
		t.Fatal("parser did not return only the expected safe source location")
	}
}

func TestNativeSyntaxErrorUsesFirstPositionedError(t *testing.T) {
	t.Parallel()
	warningRange := &hcl.Range{Start: hcl.Pos{Line: 1, Column: 2}}
	errorRange := &hcl.Range{Start: hcl.Pos{Line: 4, Column: 5}}
	laterRange := &hcl.Range{Start: hcl.Pos{Line: 6, Column: 7}}
	warning := &hcl.Diagnostic{
		Severity: hcl.DiagWarning, Subject: warningRange, Summary: "SYNTHETIC_WARNING",
	}
	unpositioned := &hcl.Diagnostic{
		Severity: hcl.DiagError, Summary: "SYNTHETIC_SUMMARY", Detail: "SYNTHETIC_DETAIL",
	}
	positioned := &hcl.Diagnostic{
		Severity: hcl.DiagError, Subject: errorRange, Summary: "SYNTHETIC_SECRET",
	}
	later := &hcl.Diagnostic{Severity: hcl.DiagError, Subject: laterRange}
	for _, tc := range []struct {
		name        string
		diagnostics hcl.Diagnostics
		want        string
	}{
		{"first positioned error", hcl.Diagnostics{warning, unpositioned, positioned, later},
			"parse native HCL in fixture.tf: invalid syntax at line 4, column 5 (source text omitted)"},
		{"no positioned error", hcl.Diagnostics{warning, unpositioned},
			"parse native HCL in fixture.tf: invalid syntax (source text omitted)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := nativeSyntaxError("fixture.tf", tc.diagnostics); err.Error() != tc.want {
				t.Fatal("syntax diagnostic exposed source content or reported the wrong location")
			}
		})
	}
}
