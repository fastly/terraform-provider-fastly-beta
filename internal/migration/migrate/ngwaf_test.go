package migrate

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func ngwafFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "ngwaf", filepath.FromSlash(path))) // #nosec G304 -- test fixtures
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestPlanNGWAFGolden(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, file := range []string{"main.tf", "outputs.tf", ".terraform.lock.hcl"} {
		writeConfig(t, dir, file, string(ngwafFixture(t, "before/"+file)))
	}

	result := newPlan(t, dir)
	if result.Blocked() {
		t.Fatalf("preview blocked: %#v (preflight: %#v)", result.Issues, result.Analysis.Findings)
	}
	if got := len(result.Moves); got != 8 {
		t.Fatalf("moves = %d, want 8 (the workspace and workspace signal keep their names)", got)
	}
	paths := make([]string, 0, len(result.Changes))
	for _, change := range result.Changes {
		paths = append(paths, change.Path)
		if want := ngwafFixture(t, "after/"+change.Path); !bytes.Equal(change.After, want) {
			t.Fatalf("unexpected %s after:\n%s\nwant:\n%s", change.Path, change.After, want)
		}
	}
	if strings.Join(paths, ",") != "main.tf,migration.tf,outputs.tf" {
		t.Fatalf("changed files = %v", paths)
	}
}

func TestPlanNGWAFInChildModule(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", `module "waf" {
  source = "./modules/waf"
}

output "list_id" {
  value = module.waf.list_id
}
`)
	writeConfig(t, dir, "modules/waf/main.tf", `resource "fastly_ngwaf_account_list" "countries" {
  for_each = toset(["a", "b"])
  name     = "countries-${each.key}"
  type     = "country"
  entries  = ["US"]
}

output "list_id" {
  value = { for key, list in fastly_ngwaf_account_list.countries : key => list.id }
}
`)

	result := newPlan(t, dir)
	if result.Blocked() {
		t.Fatalf("preview blocked: %#v (preflight: %#v)", result.Issues, result.Analysis.Findings)
	}
	if len(result.Moves) != 1 || result.Moves[0].Module != "modules/waf" ||
		result.Moves[0].To != "fastly_ngwaf_country_list.countries" {
		t.Fatalf("moves = %#v, want one module-local country list move", result.Moves)
	}
	changed := make(map[string]string)
	for _, change := range result.Changes {
		changed[change.Path] = string(change.After)
	}
	if _, ok := changed["main.tf"]; ok {
		t.Fatal("root module output referencing the module output was rewritten")
	}
	if !strings.Contains(changed["modules/waf/migration.tf"], "from = fastly_ngwaf_account_list.countries") {
		t.Fatalf("moved block not written in the child module: %v", changed)
	}
	if got := changed["modules/waf/main.tf"]; !strings.Contains(got, `for key, list in fastly_ngwaf_country_list.countries : key => list.id`) ||
		strings.Contains(got, `type     = "country"`) {
		t.Fatalf("child module not migrated:\n%s", got)
	}
}

func TestPlanNGWAFRejectsBeforeWriting(t *testing.T) {
	t.Parallel()

	signalRule := func(action string) string {
		return `resource "fastly_ngwaf_workspace_rule" "r" {
  workspace_id = "ws"
  type         = "signal"
  description  = "rule"
  enabled      = true

  condition {
    field    = "path"
    operator = "equals"
    value    = "/"
  }

` + action + `
}
`
	}
	tests := []struct {
		name, source, contains string
	}{
		{
			name:     "signal rule action other than exclude_signal",
			source:   signalRule("  action {\n    type   = \"block\"\n    signal = \"XSS\"\n  }"),
			contains: `action.type must be the literal "exclude_signal"`,
		},
		{
			name:     "dynamic action on a signal rule",
			source:   signalRule("  dynamic \"action\" {\n    for_each = [\"XSS\"]\n    content {\n      type   = \"exclude_signal\"\n      signal = action.value\n    }\n  }"),
			contains: `dynamic "action" blocks cannot be migrated`,
		},
		{
			name: "templated signal rule with a description",
			source: `resource "fastly_ngwaf_workspace_rule" "r" {
  workspace_id = "ws"
  type         = "templated_signal"
  description  = "not empty"
  enabled      = true

  action {
    type   = "templated_signal"
    signal = "LOGINATTEMPT"
  }
}
`,
			contains: `description must be the literal ""`,
		},
		{
			name: "non-literal split discriminator",
			source: `variable "kind" {
  type = string
}

resource "fastly_ngwaf_account_list" "l" {
  name    = "list"
  type    = var.kind
  entries = ["x"]
}
`,
			contains: `selector "type" cannot be determined statically`,
		},
		{
			name: "reference to the dropped discriminator",
			source: `resource "fastly_ngwaf_account_list" "l" {
  name    = "list"
  type    = "ip"
  entries = ["10.0.0.1"]
}

output "kind" {
  value = fastly_ngwaf_account_list.l.type
}
`,
			contains: "fastly_ngwaf_account_list.l.type has no verified equivalent",
		},
		{
			name: "indexed applies_to, now a set",
			source: `resource "fastly_ngwaf_account_signal" "s" {
  name       = "signal"
  applies_to = ["*"]
}

output "first" {
  value = fastly_ngwaf_account_signal.s.applies_to[0]
}
`,
			contains: "unsupported whole-resource value or nested attribute path",
		},
		{
			name: "target address already declared",
			source: `resource "fastly_ngwaf_redaction" "r" {
  workspace_id = "ws"
  field        = "password"
  type         = "request_parameter"
}

resource "fastly_ngwaf_workspace_redaction" "r" {
  workspace_id = "ws"
  field        = "token"
  type         = "request_parameter"
}
`,
			contains: "target resource fastly_ngwaf_workspace_redaction.r already exists",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeConfig(t, dir, "main.tf", test.source)

			result := newPlan(t, dir)
			if !result.Blocked() || len(result.Changes) != 0 || len(result.Moves) != 0 {
				t.Fatalf("migration was not blocked: %#v", result)
			}
			var messages []string
			for _, issue := range result.Issues {
				messages = append(messages, issue.Message)
			}
			for _, finding := range result.Analysis.Findings {
				messages = append(messages, finding.Message)
			}
			if !strings.Contains(strings.Join(messages, "\n"), test.contains) {
				t.Fatalf("messages = %q, want one containing %q", messages, test.contains)
			}
			if _, err := result.Apply(t.Context(), privateBackupDir(t)); err == nil {
				t.Fatal("blocked migration was applied")
			}
			got, err := os.ReadFile(filepath.Join(dir, "main.tf"))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != test.source {
				t.Fatal("blocked migration modified main.tf")
			}
			if _, err := os.Stat(filepath.Join(dir, "migration.tf")); !os.IsNotExist(err) {
				t.Fatalf("blocked migration created migration.tf: %v", err)
			}
		})
	}
}
