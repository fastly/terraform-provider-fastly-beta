package migrate

import (
	"context"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func changedFiles(result Result) map[string]string {
	files := make(map[string]string, len(result.Changes))
	for _, change := range result.Changes {
		files[change.Path] = string(change.After)
	}
	return files
}

func TestPlanStandaloneDomainRewritesDomainIDReferences(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", `resource "fastly_service_vcl" "demo" {
  name = "demo"
}

resource "fastly_domain" "example" {
  fqdn        = "www.example.com"
  description = "demo"
  service_id  = fastly_service_vcl.demo.id
}

locals {
  domain_ids = [fastly_domain.example.domain_id]
}

output "domain_id" {
  value = fastly_domain.example.domain_id
}

output "fqdn" {
  value = fastly_domain.example.fqdn
}
`)
	writeConfig(t, dir, "link.tf", `resource "fastly_domain_service_link" "link" {
  domain_id  = fastly_domain.example.domain_id
  service_id = fastly_service_vcl.demo.id
}

module "consumer" {
  source    = "./consumer"
  domain_id = fastly_domain.example.domain_id
}

resource "fastly_dns_zone" "unrelated" {
  name = "unrelated-${var.env}"
}
`)
	if err := os.Mkdir(filepath.Join(dir, "consumer"), 0o700); err != nil {
		t.Fatal(err)
	}
	writeConfig(t, filepath.Join(dir, "consumer"), "main.tf", "variable \"domain_id\" {}\n")

	result := newPlan(t, dir)
	if result.Blocked() {
		t.Fatalf("blocked: %#v %#v", result.Issues, result.Analysis.Findings)
	}
	files := changedFiles(result)
	main, link := files["main.tf"], files["link.tf"]

	for want, in := range map[string]string{
		"fastly_domain.example.id":                      main,
		"value = fastly_domain.example.fqdn":            main,
		"domain_ids = [fastly_domain.example.id]":       main,
		"domain_id = fastly_domain.example.id":          link,
		"service_id = fastly_service_cdn_auto.demo.id":  link,
		"name = \"unrelated-${var.env}\"":               link,
		"resource \"fastly_domain\" \"example\" {":      main,
		"service_id  = fastly_service_cdn_auto.demo.id": main,
		"resource \"fastly_dns_zone\" \"unrelated\" {":  link,
	} {
		if !strings.Contains(in, want) {
			t.Errorf("missing %q in:\n%s", want, in)
		}
	}
	if strings.Contains(main+link, ".domain_id") {
		t.Errorf("legacy domain_id reference survived:\n%s\n%s", main, link)
	}
	// The domain keeps its address, so only the service is moved.
	if len(result.Moves) != 1 || result.Moves[0].From != "fastly_service_vcl.demo" {
		t.Fatalf("moves = %#v", result.Moves)
	}
	if strings.Contains(files["migration.tf"], "fastly_domain.example") {
		t.Fatalf("unchanged domain must not get a moved block:\n%s", files["migration.tf"])
	}
}

func TestPlanDomainOnlyConfigurationDoesNotRequireServiceMove(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", `resource "fastly_domain" "example" {
  fqdn = "www.example.com"
}

output "id" {
  value = fastly_domain.example.domain_id
}
`)
	result := newPlan(t, dir)
	if result.Blocked() {
		t.Fatalf("blocked: %#v %#v", result.Issues, result.Analysis.Findings)
	}
	if len(result.Moves) != 0 {
		t.Fatalf("moves = %#v", result.Moves)
	}
	if got := changedFiles(result)["main.tf"]; !strings.Contains(got, "value = fastly_domain.example.id") {
		t.Fatalf("reference not rewritten:\n%s", got)
	}
}

func TestPlanDomainWithoutLegacyReferencesIsANoOp(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", `resource "fastly_domain" "example" {
  fqdn = "www.example.com"
}

output "id" {
  value = fastly_domain.example.id
}
`)
	result := newPlan(t, dir)
	if result.Blocked() || len(result.Changes) != 0 || len(result.Moves) != 0 {
		t.Fatalf("expected a no-op preview: %#v", result)
	}
}

func TestPlanRenamesDomainV1ResourcesAcrossFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "domain.tf", `resource "fastly_domain_v1" "example" {
  fqdn = "www.example.com"
}
`)
	writeConfig(t, dir, "link.tf", `resource "fastly_domain_v1_service_link" "link" {
  domain_id  = fastly_domain_v1.example.domain_id
  service_id = "SU1Z0isxPaozGVKXdv0eY"
}

output "link_domain" {
  value = fastly_domain_v1_service_link.link.domain_id
}

output "fqdn" {
  value = fastly_domain_v1.example.fqdn
}
`)
	result := newPlan(t, dir)
	if result.Blocked() {
		t.Fatalf("blocked: %#v %#v", result.Issues, result.Analysis.Findings)
	}
	files := changedFiles(result)
	if !strings.Contains(files["domain.tf"], `resource "fastly_domain" "example"`) {
		t.Fatalf("domain not renamed:\n%s", files["domain.tf"])
	}
	link := files["link.tf"]
	for _, want := range []string{
		`resource "fastly_domain_service_link" "link"`,
		"domain_id  = fastly_domain.example.id",
		"value = fastly_domain_service_link.link.domain_id",
		"value = fastly_domain.example.fqdn",
	} {
		if !strings.Contains(link, want) {
			t.Errorf("missing %q in:\n%s", want, link)
		}
	}
	if strings.Contains(link, "_v1") {
		t.Errorf("v1 type survived:\n%s", link)
	}
	moved := files["migration.tf"]
	for _, want := range []string{
		"from = fastly_domain_v1.example", "to   = fastly_domain.example",
		"from = fastly_domain_v1_service_link.link", "to   = fastly_domain_service_link.link",
	} {
		if !strings.Contains(moved, want) {
			t.Errorf("missing %q in:\n%s", want, moved)
		}
	}
	if len(result.Moves) != 2 {
		t.Fatalf("moves = %#v", result.Moves)
	}
}

func TestPlanDomainIndexedResourceReferences(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", `resource "fastly_domain" "example" {
  for_each = toset(["a.example.com", "b.example.com"])
  fqdn     = each.value
}

output "ids" {
  value = { for k, d in fastly_domain.example : k => d.domain_id }
}

output "first" {
  value = fastly_domain.example["a.example.com"].domain_id
}
`)
	result := newPlan(t, dir)
	if !result.Blocked() {
		// A projection that reads a renamed attribute is not a verified scalar
		// projection, so it must fail closed rather than be guessed at.
		t.Fatalf("expected the renamed-attribute projection to block: %#v", result.Changes)
	}
}

func TestPlanDomainUnsupportedConfigurationsWriteNothing(t *testing.T) {
	t.Parallel()
	for name, extra := range map[string]string{
		"control: baseline is supported": "",
		"whole resource":                 "output \"all\" { value = fastly_domain.example }\n",
		"unknown attribute":              "output \"x\" { value = fastly_domain.example.managed }\n",
		"nested path":                    "output \"x\" { value = fastly_domain.example.fqdn[0] }\n",
		"conflicting import":             "import {\n  to = fastly_domain.example\n  id = \"id\"\n}\n",
		"v1 target collision":            "resource \"fastly_domain\" \"legacy\" {\n  fqdn = \"b.example.com\"\n}\n",
		"link attribute unknown":         "output \"x\" { value = fastly_domain_v1_service_link.link.managed }\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			config := `resource "fastly_domain" "example" {
  fqdn = "www.example.com"
}
resource "fastly_domain_v1" "legacy" {
  fqdn = "a.example.com"
}
resource "fastly_domain_v1_service_link" "link" {
  domain_id  = fastly_domain.example.domain_id
  service_id = "SU1Z0isxPaozGVKXdv0eY"
}
` + extra
			writeConfig(t, dir, "main.tf", config)
			result := newPlan(t, dir)
			if extra == "" {
				if result.Blocked() {
					t.Fatalf("baseline configuration must migrate: %#v", result.Issues)
				}
				return
			}
			if !result.Blocked() || len(result.Changes) != 0 || len(result.Moves) != 0 {
				t.Fatalf("unsupported configuration was not blocked: %#v", result)
			}
			got, err := os.ReadFile(filepath.Join(dir, "main.tf"))
			if err != nil || string(got) != config {
				t.Fatal("blocked plan modified the configuration")
			}
			if _, err := os.Stat(filepath.Join(dir, "migration.tf")); !os.IsNotExist(err) {
				t.Fatalf("blocked plan generated migration.tf: %v", err)
			}
		})
	}
}

// TestPlanDomainMigrationGuideConfigurationBlocksOnDeprecatedDataSource mirrors
// the configuration in the SDKv2 migrating_from_domains_v1_to_domains guide. The
// deprecated data source has no automatic rewrite, so nothing may be written.
func TestPlanDomainMigrationGuideConfigurationBlocksOnDeprecatedDataSource(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", `data "fastly_domains_v1" "all" {}

locals {
  domain_ids = {
    for domain in data.fastly_domains_v1.all.domains :
    domain.fqdn => domain.id
  }
}

resource "fastly_domain_v1" "example" {
  fqdn = "www.example.com"
}

resource "fastly_domain_v1_service_link" "example" {
  domain_id  = fastly_domain_v1.example.domain_id
  service_id = "SU1Z0isxPaozGVKXdv0eY"
}

output "domain_id" {
  value = local.domain_ids["www.example.com"]
}
`)
	result := newPlan(t, dir)
	if !result.Blocked() {
		t.Fatalf("expected the deprecated data source to block: %#v", result)
	}
	if len(result.Changes) != 0 || len(result.Moves) != 0 {
		t.Fatalf("a blocked plan must not produce changes: %#v", result)
	}
	var found bool
	for _, finding := range result.Analysis.Findings {
		if strings.Contains(finding.Message, "fastly_domains_v1") && finding.Remediation != "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing data source finding: %#v", result.Analysis.Findings)
	}
}

func TestPlanApplyPlanRenamedDomainResourcesIsStable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", `resource "fastly_domain_v1" "example" {
  fqdn = "www.example.com"
}

resource "fastly_domain_v1_service_link" "example" {
  domain_id  = fastly_domain_v1.example.domain_id
  service_id = "SU1Z0isxPaozGVKXdv0eY"
}

output "domain_id" {
  value = fastly_domain_v1.example.domain_id
}
`)
	first := newPlan(t, dir)
	if first.Blocked() || len(first.Moves) != 2 {
		t.Fatalf("first plan: %#v %#v", first.Issues, first.Analysis.Findings)
	}
	if _, err := first.Apply(context.Background(), privateBackupDir(t)); err != nil {
		t.Fatal(err)
	}
	migrated := readConfigFiles(t, dir)

	second := newPlan(t, dir)
	if second.Blocked() {
		t.Fatalf("second plan must be READY: %#v %#v", second.Issues, second.Analysis.Findings)
	}
	if len(second.Changes) != 0 || len(second.Moves) != 0 {
		t.Fatalf("second plan must not change anything: %#v", second)
	}
	if after := readConfigFiles(t, dir); !maps.Equal(migrated, after) {
		t.Fatalf("second plan modified the configuration:\n%v\n%v", migrated, after)
	}
	moved := migrated["migration.tf"]
	for _, want := range []string{
		"from = fastly_domain_v1.example",
		"to   = fastly_domain.example",
		"from = fastly_domain_v1_service_link.example",
		"to   = fastly_domain_service_link.example",
	} {
		if !strings.Contains(moved, want) {
			t.Errorf("missing moved block %q in:\n%s", want, moved)
		}
	}
}

func readConfigFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	files := make(map[string]string, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tf") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		files[entry.Name()] = string(data)
	}
	return files
}
