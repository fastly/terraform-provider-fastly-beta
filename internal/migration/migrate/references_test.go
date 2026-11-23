package migrate

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanReferencesAcrossFilesAndModules(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	main := `provider "fastly" {
  alias = "edge"
}
module "first" {
  source = "./modules/vcl"
  for_each = toset(["prod", "test"])
  providers = { fastly = fastly.edge }
}
module "second" {
  source = "./modules/vcl"
  count = 1
  providers = { fastly = fastly.edge }
}
module "compute" {
  source = "./modules/compute"
  providers = { fastly = fastly.edge }
}
output "ids" {
  value = [module.first["prod"].id, module.second[0].id, module.compute.id]
}
`
	vcl := `resource "fastly_service_vcl" "demo" {
  count = 2
  name = "vcl-${count.index}"
  lifecycle {
    ignore_changes = [comment]
    postcondition {
      condition = self.id != ""
      error_message = "service ID required"
    }
  }
}
`
	compute := `resource "fastly_service_compute" "demo" {
  for_each = toset(["prod"])
  name = each.key
  package {
    filename = "pkg.tar.gz"
  }
}
`
	vclRefs := `# fastly_service_vcl.demo[0].id is a literal comment
output "id" {
  value = fastly_service_vcl.demo[0].id
}
resource "terraform_data" "consumer" {
  for_each = {for i, svc in fastly_service_vcl.demo : i => svc.id}
  input = fastly_service_vcl.demo[tonumber(each.key)].id
  depends_on = [fastly_service_vcl.demo]
}
`
	computeRefs := `output "id" {
  value = fastly_service_compute.demo["prod"].id
}
resource "aws_route53_record" "integration" {
  name = fastly_service_compute.demo["prod"].name
  type = "TXT"
  records = ["service=${fastly_service_compute.demo["prod"].id}"]
  zone_id = var.zone_id
}
`
	files := map[string]string{
		"main.tf": main, "modules/vcl/main.tf": vcl, "modules/vcl/refs.tf": vclRefs,
		"modules/compute/main.tf": compute, "modules/compute/refs.tf": computeRefs,
	}
	for file, source := range files {
		writeConfig(t, dir, file, source)
	}
	result := newPlan(t, dir)
	if result.Blocked() || len(result.Moves) != 2 {
		t.Fatalf("reference plan blocked: %#v findings=%#v", result.Issues, result.Analysis.Findings)
	}
	for file, want := range map[string]string{
		"modules/vcl/refs.tf":     strings.ReplaceAll(vclRefs, "fastly_service_vcl.demo", "fastly_service_cdn_auto.demo"),
		"modules/compute/refs.tf": strings.ReplaceAll(computeRefs, "fastly_service_compute.demo", "fastly_service_compute_auto.demo"),
	} {
		// Literal comments must retain their original content.
		want = strings.Replace(want, "# fastly_service_cdn_auto.demo", "# fastly_service_vcl.demo", 1)
		if got := changedFile(t, result, file); got != want {
			t.Fatalf("%s:\n%s\nwant:\n%s", file, got, want)
		}
	}
	for _, change := range result.Changes {
		if change.Path == "main.tf" && !strings.Contains(string(change.After), `module.first["prod"].id, module.second[0].id, module.compute.id`) {
			t.Fatal("root module addresses changed")
		}
	}
	for file, source := range files {
		got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(file))) // #nosec G304 G703 -- fixed test fixture paths
		if err != nil || string(got) != source {
			t.Fatalf("preview modified %s: %v", file, err)
		}
	}
	for _, move := range result.Moves {
		if move.Module == "." || strings.Contains(move.From, "module.") {
			t.Fatalf("move not scoped to child module: %#v", move)
		}
	}
	if _, err := result.Apply(context.Background(), privateBackupDir(t)); err != nil {
		t.Fatal(err)
	}
	again := newPlan(t, dir)
	if again.Blocked() || len(again.Changes) != 0 || len(again.Moves) != 0 {
		t.Fatalf("repeated plan not idempotent: %#v", again)
	}
}

func TestPlanReferencesPreserveExistingExactMove(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", `resource "fastly_service_vcl" "demo" {
  name = "demo"
}
output "service_id" {
  value = fastly_service_vcl.demo.id
}
`)
	existing := "moved {\n  from = fastly_service_vcl.demo\n  to = fastly_service_cdn_auto.demo\n}\n"
	writeConfig(t, dir, "historic.tf", existing)
	result := newPlan(t, dir)
	if result.Blocked() || len(result.Moves) != 1 {
		t.Fatalf("exact moved block rejected: %#v %#v", result.Issues, result.Analysis.Findings)
	}
	for _, change := range result.Changes {
		if change.Path == "historic.tf" || strings.Contains(string(change.After), "moved {") {
			t.Fatalf("history changed or duplicate move generated: %s\n%s", change.Path, change.After)
		}
	}
}

func TestPlanReferenceBlockersPreventEveryWrite(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, source string }{
		{"removed output", `output "bad" { value = fastly_service_vcl.demo.cloned_version }`},
		{"whole resource", `output "bad" { value = fastly_service_vcl.demo }`},
		{"absent settings", `output "bad" { value = fastly_service_vcl.demo.default_ttl }`},
		{"unsafe list shape", `output "bad" { value = fastly_service_vcl.demo.backend }`},
		{"invalid moved history", "moved {\n  from = fastly_service_vcl.demo[0]\n  to = fastly_service_cdn_auto.demo[0]\n}\n"},
		{"standalone import conflict", "import {\n  to = fastly_service_vcl.demo\n  id = \"id\"\n}\n"},
		{"standalone domain whole resource", "resource \"fastly_domain\" \"domain\" {\n  service_id = fastly_service_vcl.demo.id\n}\noutput \"domain\" { value = fastly_domain.domain }\n"},
		{"legacy routing config still blocked", "resource \"fastly_routing_config\" \"routing\" {\n  service_id = fastly_service_vcl.demo.id\n}\n"},
		{"legacy dictionary items still blocked", "resource \"fastly_service_dictionary_items\" \"items\" {\n  service_id = fastly_service_vcl.demo.id\n}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			main := "resource \"fastly_service_vcl\" \"demo\" {\n  name = \"demo\"\n}\n"
			writeConfig(t, dir, "main.tf", main)
			writeConfig(t, dir, "references.tf", "output \"good\" { value = fastly_service_vcl.demo.id }\n"+tc.source+"\n")
			before, err := os.ReadFile(filepath.Join(dir, "references.tf"))
			if err != nil {
				t.Fatal(err)
			}
			result := newPlan(t, dir)
			if !result.Blocked() || len(result.Changes) != 0 || len(result.Moves) != 0 {
				t.Fatalf("unsafe plan: %#v", result)
			}
			if _, err := result.Apply(context.Background(), privateBackupDir(t)); err == nil {
				t.Fatal("blocked plan applied")
			}
			after, err := os.ReadFile(filepath.Join(dir, "references.tf"))
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("blocked plan modified references")
			}
			got, err := os.ReadFile(filepath.Join(dir, "main.tf"))
			if err != nil || string(got) != main {
				t.Fatal("blocked plan modified service")
			}
			if _, err := os.Stat(filepath.Join(dir, "migration.tf")); !os.IsNotExist(err) {
				t.Fatalf("blocked plan generated migration.tf: %v", err)
			}
		})
	}
}

func TestPlanReferencesRejectsExistingTerraformJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", simpleVCLConfiguration)
	const jsonConfig = `{"output":{"legacy":{"value":"${fastly_service_vcl.demo.id}"}}}`
	writeConfig(t, dir, "references.tf.json", jsonConfig)

	result := newPlan(t, dir)
	if !result.Blocked() || len(result.Changes) != 0 || len(result.Moves) != 0 {
		t.Fatalf("unsafe JSON plan: %#v", result)
	}
	if len(result.Issues) != 0 {
		t.Fatalf("JSON should block during analysis, not planning: %#v", result.Issues)
	}
	found := false
	for _, finding := range result.Analysis.Findings {
		if strings.Contains(finding.Message, "Terraform JSON configuration is not supported") {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing unsupported JSON diagnostic: %#v", result.Analysis.Findings)
	}
	if _, err := result.Apply(context.Background(), privateBackupDir(t)); err == nil {
		t.Fatal("blocked JSON plan applied")
	}
	got, err := os.ReadFile(filepath.Join(dir, "main.tf"))
	if err != nil || string(got) != simpleVCLConfiguration {
		t.Fatal("blocked plan modified source")
	}
	got, err = os.ReadFile(filepath.Join(dir, "references.tf.json"))
	if err != nil || string(got) != jsonConfig {
		t.Fatal("blocked plan modified JSON")
	}
	if _, err := os.Stat(filepath.Join(dir, "migration.tf")); !os.IsNotExist(err) {
		t.Fatalf("blocked plan generated migration.tf: %v", err)
	}
}

func TestPlanReferencesRejectsNewTerraformJSONAfterPreview(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", simpleVCLConfiguration)
	result := newPlan(t, dir)
	if result.Blocked() {
		t.Fatalf("preview blocked: %#v %#v", result.Issues, result.Analysis.Findings)
	}
	const jsonConfig = `{"output":{"late":{"value":"${fastly_service_vcl.demo.id}"}}}`
	writeConfig(t, dir, "new-reference.tf.json", jsonConfig)
	if _, err := result.Apply(context.Background(), privateBackupDir(t)); err == nil ||
		!strings.Contains(err.Error(), "terraform file new-reference.tf.json appeared after analysis") {
		t.Fatalf("new JSON file not rejected as drift: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "main.tf"))
	if err != nil || string(got) != simpleVCLConfiguration {
		t.Fatal("stale preview modified source")
	}
	got, err = os.ReadFile(filepath.Join(dir, "new-reference.tf.json"))
	if err != nil || string(got) != jsonConfig {
		t.Fatal("stale preview modified JSON")
	}
	if _, err := os.Stat(filepath.Join(dir, "migration.tf")); !os.IsNotExist(err) {
		t.Fatalf("stale preview generated migration.tf: %v", err)
	}
}

func TestPlanReferencesRejectsNewFileAfterPreview(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", simpleVCLConfiguration)
	result := newPlan(t, dir)
	if result.Blocked() {
		t.Fatal(result.Issues)
	}
	writeConfig(t, dir, "new-reference.tf", `output "late" { value = fastly_service_vcl.demo.id }`)
	if _, err := result.Apply(context.Background(), privateBackupDir(t)); err == nil || !strings.Contains(err.Error(), "appeared after analysis") {
		t.Fatalf("new reference file not rejected: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "main.tf"))
	if err != nil || string(got) != simpleVCLConfiguration {
		t.Fatal("stale preview modified source")
	}
}

func TestPlanReferencesRenameSnippetAndIdentityMap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "service.tf", `resource "fastly_service_vcl" "demo" {
  name = "demo"
  dynamicsnippet {
    name = "snippet"
    type = "recv"
  }
}
`)
	writeConfig(t, dir, "content.tf", `resource "fastly_service_dynamic_snippet_content" "demo" {
  service_id = fastly_service_vcl.demo.id
  snippet_id = lookup({for s in fastly_service_vcl.demo.dynamicsnippet : s.name => s.snippet_id}, "snippet")
  content = "set req.http.X-Test = \"test\";"
}
`)
	result := newPlan(t, dir)
	if result.Blocked() {
		t.Fatalf("reference plan blocked: %#v %#v", result.Issues, result.Analysis.Findings)
	}
	got := changedFile(t, result, "content.tf")
	for _, want := range []string{
		"service_id = fastly_service_cdn_auto.demo.id",
		"fastly_service_cdn_auto.demo.dynamic_snippet : s.name => s.snippet_id",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
	}
}

func TestPlanConfiguredSettingsReferenceRequiresProviderProof(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", `resource "fastly_service_vcl" "demo" {
  name = "demo"
  default_ttl = 3600
}
output "ttl" {
  value = fastly_service_vcl.demo.default_ttl
}
`)
	result := newPlan(t, dir)
	if !result.Blocked() || len(result.Changes) != 0 {
		t.Fatal("configured singleton settings reference must remain blocked until provider compatibility is proven")
	}
}

func TestPlanModuleQualifiedMoveConflict(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", `module "child" {
  source = "./child"
}
moved {
  from = module.child.fastly_service_vcl.demo
  to = module.child.fastly_service_cdn_auto.demo
}
`)
	writeConfig(t, dir, "child/main.tf", simpleVCLConfiguration)
	result := newPlan(t, dir)
	if !result.Blocked() || len(result.Changes) != 0 {
		t.Fatal("module-qualified existing move must require manual review")
	}
}
