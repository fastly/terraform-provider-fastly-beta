package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func trafficFixture(t *testing.T, name, file string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "traffic", name, file)) // #nosec G304 -- repository-owned fixtures
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestPlanTrafficBlocksGolden(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"published_header_gzip", "published_director", "multi_block"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			source := trafficFixture(t, name, "legacy.tf")
			want := trafficFixture(t, name, "expected.tf")
			writeConfig(t, dir, "main.tf", source)
			result := newPlan(t, dir)
			if result.Blocked() || len(result.Moves) == 0 {
				t.Fatalf("traffic plan blocked: %v %v", result.Issues, result.Analysis.Findings)
			}
			if got := changedFile(t, result, "main.tf"); got != want {
				t.Fatalf("traffic golden mismatch:\n%s", got)
			}
			requireStableAfterApply(t, dir, result)
		})
	}
}

func TestPlanTrafficBlocksRejectsUnsupportedValues(t *testing.T) {
	t.Parallel()
	for i, tc := range []struct{ blocks, want string }{
		{"director {\n    name = \"d\"\n    backends = [\"a\"]\n    type = 2\n  }", `director "type"`},
		{"director {\n    name = \"d\"\n    backends = [\"a\"]\n    type = 5\n  }", `director "type"`},
		{"director {\n    name = \"d\"\n    backends = [\"a\"]\n    type = \"hash\"\n  }", `director "type"`},
		{"director {\n    name = \"d\"\n    backends = [\"a\"]\n    type = -1\n  }", `director "type"`},
		{"cache_setting {\n    name = \"c\"\n    action = \"lookup\"\n  }", `cache_setting "action"`},
		{"request_setting {\n    name = \"r\"\n    xff = \"sometimes\"\n  }", `request_setting "xff"`},
		{"request_setting {\n    name = \"r\"\n    action = \"${\"PASS\"}\"\n  }", `request_setting "action"`},
		{"condition {\n    name = var.name\n    statement = \"true\"\n    type = \"REQUEST\"\n  }\n  condition {\n    name = \"b\"\n    statement = \"true\"\n    type = \"REQUEST\"\n  }", "needs a quoted name"},
		{"header {\n    name = \"h\"\n    type = \"request\"\n    action = \"set\"\n    destination = \"http.X\"\n  }\n  header {\n    name = \"h\"\n    type = \"request\"\n    action = \"set\"\n    destination = \"http.Y\"\n  }", "duplicate names"},
	} {
		dir := t.TempDir()
		writeConfig(t, dir, "main.tf", "resource \"fastly_service_vcl\" \"demo\" {\n  name = \"demo\"\n  "+tc.blocks+"\n}\n")
		result := newPlan(t, dir)
		if !result.Blocked() || len(result.Changes) != 0 {
			t.Fatalf("case %d: unsupported value was not blocked", i)
		}
		found := false
		for _, finding := range result.Analysis.Findings {
			found = found || strings.Contains(finding.Message, tc.want)
		}
		if !found {
			t.Fatalf("case %d: missing finding %q in %v", i, tc.want, result.Analysis.Findings)
		}
	}
}

func TestPlanTrafficBlocksExpressions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	source := `resource "fastly_service_vcl" "demo" {
  name = "demo"
  director {
    name     = "d"
    backends = ["a"]
    type     = var.director_type
  }
  cache_setting {
    name   = "c"
    action = var.action
  }
}
`
	writeConfig(t, dir, "main.tf", source)
	result := newPlan(t, dir)
	if result.Blocked() {
		t.Fatalf("expression values blocked: %v %v", result.Issues, result.Analysis.Findings)
	}
	got := changedFile(t, result, "main.tf")
	for _, keep := range []string{
		"type     = var.director_type",
		"action = var.action == null ? null : lower(var.action)",
	} {
		if !strings.Contains(got, keep) {
			t.Fatalf("expression %q missing:\n%s", keep, got)
		}
	}
}
