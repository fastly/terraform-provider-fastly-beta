package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/analyze"
)

const legacyContainersConfig = `resource "fastly_compute_acl" "blocklist" {
  name = "blocklist"
}

resource "fastly_compute_acl_entries" "blocklist" {
  compute_acl_id = fastly_compute_acl.blocklist.id
  entries = {
    "192.0.2.0/24" = "BLOCK"
  }
  manage_entries = true
}

resource "fastly_configstore_entries" "settings" {
  store_id = "7QYy4nSAvbCyc8OyCYHz5S"
  entries = {
    mode = "strict"
  }
}

resource "fastly_service_acl_entries" "office" {
  service_id     = "SU1Z0isxPaozGVKXdv0eY"
  acl_id         = "4Gp9l2sRnzbfn6Ym0ZgK3v"
  manage_entries = false
  entry {
    ip     = "192.0.2.0"
    subnet = "24"
  }
}

output "acl_id" {
  value = fastly_compute_acl_entries.blocklist.compute_acl_id
}

output "acl_name" {
  value = fastly_compute_acl.blocklist.name
}

output "store_id" {
  value = fastly_configstore_entries.settings.store_id
}

output "office_acl" {
  value = fastly_service_acl_entries.office.acl_id
}
`

func TestPlanRenamesContainerResources(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", legacyContainersConfig)

	result := newPlan(t, dir)
	if result.Blocked() {
		t.Fatalf("blocked: %#v %#v", result.Issues, result.Analysis.Findings)
	}
	files := changedFiles(result)
	main, moved := files["main.tf"], files["migration.tf"]

	for _, want := range []string{
		`resource "fastly_acl" "blocklist" {`,
		`resource "fastly_acl_entries" "blocklist" {`,
		`acl_id = fastly_acl.blocklist.id`,
		`resource "fastly_configstore_items" "settings" {`,
		"items = {\n    mode = \"strict\"\n  }",
		`resource "fastly_service_cdn_acl_entries" "office" {`,
		`subnet = "24"`,
		`value = fastly_acl_entries.blocklist.acl_id`,
		`value = fastly_acl.blocklist.name`,
		`value = fastly_configstore_items.settings.store_id`,
		`value = fastly_service_cdn_acl_entries.office.acl_id`,
	} {
		if !strings.Contains(main, want) {
			t.Errorf("missing %q in:\n%s", want, main)
		}
	}
	for _, legacy := range []string{"manage_entries", "compute_acl_id", "fastly_compute_acl", "fastly_configstore_entries", "fastly_service_acl_entries"} {
		if strings.Contains(main, legacy) {
			t.Errorf("legacy %q survived:\n%s", legacy, main)
		}
	}
	for from, to := range map[string]string{
		"fastly_compute_acl.blocklist":         "fastly_acl.blocklist",
		"fastly_compute_acl_entries.blocklist": "fastly_acl_entries.blocklist",
		"fastly_configstore_entries.settings":  "fastly_configstore_items.settings",
		"fastly_service_acl_entries.office":    "fastly_service_cdn_acl_entries.office",
	} {
		if !strings.Contains(moved, "from = "+from+"\n  to   = "+to+"\n") {
			t.Errorf("missing moved block %s -> %s in:\n%s", from, to, moved)
		}
	}
	if len(result.Moves) != 4 {
		t.Fatalf("moves = %#v", result.Moves)
	}
	requireStableAfterApply(t, dir, result)
}

func TestPlanWarnsWhenContainerEntriesWereSeedOnly(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		manageEntries string // "" omits the attribute
		warn          bool
	}{
		"omitted":  {"", true},
		"false":    {"false", true},
		"variable": {"var.manage", true},
		"true":     {"true", false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			for _, resource := range []struct{ typ, body string }{
				{"fastly_compute_acl_entries", "compute_acl_id = \"acl\"\n  entries = {}\n"},
				{"fastly_configstore_entries", "store_id = \"store\"\n  entries = {}\n"},
				{"fastly_service_acl_entries", "service_id = \"svc\"\n  acl_id = \"acl\"\n"},
			} {
				dir := t.TempDir()
				body := resource.body
				if tc.manageEntries != "" {
					body += "  manage_entries = " + tc.manageEntries + "\n"
				}
				writeConfig(t, dir, "main.tf", "variable \"manage\" {}\n\nresource \""+resource.typ+"\" \"demo\" {\n  "+body+"}\n")

				result := newPlan(t, dir)
				if result.Blocked() {
					t.Fatalf("%s: a seed-only container must not block: %#v %#v", resource.typ, result.Issues, result.Analysis.Findings)
				}
				warned := false
				for _, finding := range result.Analysis.Findings {
					if finding.Severity == analyze.SeverityWarning && strings.Contains(finding.Message, "only seeded its entries") {
						warned = true
					}
				}
				if warned != tc.warn {
					t.Errorf("%s: warned = %t, want %t: %#v", resource.typ, warned, tc.warn, result.Analysis.Findings)
				}
				if strings.Contains(changedFiles(result)["main.tf"], "manage_entries") {
					t.Errorf("%s: manage_entries must be removed", resource.typ)
				}
			}
		})
	}
}

// References to outputs whose value or shape changes must block the whole
// migration rather than be rewritten to a look-alike.
func TestPlanContainerReferencesWithChangedValuesBlock(t *testing.T) {
	t.Parallel()
	for name, reference := range map[string]string{
		"config store id":     "fastly_configstore_entries.settings.id",
		"config store flag":   "fastly_configstore_entries.settings.manage_entries",
		"compute acl flag":    "fastly_compute_acl_entries.blocklist.manage_entries",
		"service acl entries": "fastly_service_acl_entries.office.entry",
		"service acl flag":    "fastly_service_acl_entries.office.manage_entries",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeConfig(t, dir, "main.tf", legacyContainersConfig+"\noutput \"blocked\" {\n  value = "+reference+"\n}\n")
			before, err := os.ReadFile(filepath.Join(dir, "main.tf"))
			if err != nil {
				t.Fatal(err)
			}

			result := newPlan(t, dir)
			if !result.Blocked() || len(result.Changes) != 0 {
				t.Fatalf("reference %s must block every write: changes=%d issues=%#v", reference, len(result.Changes), result.Issues)
			}
			after, err := os.ReadFile(filepath.Join(dir, "main.tf"))
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(before) {
				t.Fatal("a blocked plan modified the configuration")
			}
		})
	}
}
