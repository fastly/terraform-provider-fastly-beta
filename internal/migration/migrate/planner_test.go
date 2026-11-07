package migrate

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/analyze"
	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/compatibility"
	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/rewrite"
)

func fixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "vcl", path))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeConfig(t *testing.T, dir, file, source string) {
	t.Helper()
	if err := writeFixtureConfig(dir, file, source); err != nil {
		t.Fatal(err)
	}
}

// writeFixtureConfig confines both directory creation and file writes to the
// fixture root. The module requires Go 1.26.10, which provides os.OpenRoot
// and os.Root.MkdirAll; do not weaken this to path validation plus os.OpenFile.
func writeFixtureConfig(dir, file, source string) error {
	relative := filepath.FromSlash(file)
	if !filepath.IsLocal(relative) {
		return fmt.Errorf("fixture path must be local: %q", file)
	}
	// Some fixtures use a new module directory under t.TempDir(). Create
	// that trusted root before opening it; all paths supplied by the
	// fixture remain confined to the root by os.Root.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create fixture root: %w", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.MkdirAll(filepath.Dir(relative), 0o755); err != nil {
		return err
	}
	f, err := root.OpenFile(relative, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(source); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func TestWriteFixtureConfigCreatesMissingRoot(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "root")
	if err := writeFixtureConfig(dir, "main.tf", "name = \"demo\"\n"); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(dir, "main.tf"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(contents), "name = \"demo\"\n"; got != want {
		t.Fatalf("fixture contents = %q, want %q", got, want)
	}
}

// A symlinked parent must not let even a test fixture write escape its root.
func TestWriteFixtureConfigRejectsEscapes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	outside := t.TempDir()
	if err := writeFixtureConfig(dir, "../escape.tf", "unsafe"); err == nil {
		t.Fatal("accepted a traversal path")
	}
	if err := os.Symlink(outside, filepath.Join(dir, "linked")); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}
	if err := writeFixtureConfig(dir, "linked/escape.tf", "unsafe"); err == nil {
		t.Fatal("followed an escaping symlink")
	}
	if _, err := os.Stat(filepath.Join(outside, "escape.tf")); !os.IsNotExist(err) {
		t.Fatalf("fixture escaped into symlink target: %v", err)
	}
}

func newPlan(t *testing.T, dir string) Result {
	t.Helper()
	result, err := New(compatibility.Default()).Plan(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestPlanGoldenAndReadOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	before := fixture(t, "before/main.tf")
	writeConfig(t, dir, "main.tf", string(before))

	result := newPlan(t, dir)
	if result.Blocked() {
		t.Fatalf("preview blocked: %#v (preflight: %#v)", result.Issues, result.Analysis.Findings)
	}
	if got := len(result.Moves); got != 1 {
		t.Fatalf("moves=%d, want 1", got)
	}
	if len(result.Changes) != 2 ||
		result.Changes[0].Path != "main.tf" ||
		result.Changes[1].Path != "migration.tf" {
		t.Fatalf("changes out of order: %#v", result.Changes)
	}
	for _, change := range result.Changes {
		want := fixture(t, "after/"+change.Path)
		if !bytes.Equal(change.After, want) {
			t.Fatalf("unexpected %s after:\n%s\nwant:\n%s", change.Path, change.After, want)
		}
	}
	got, err := os.ReadFile(filepath.Join(dir, "main.tf"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, before) {
		t.Fatal("dry-run modified main.tf")
	}
	if _, err := os.Stat(filepath.Join(dir, "migration.tf")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created migration.tf: %v", err)
	}

	var first, second bytes.Buffer
	if err := WriteText(&first, result); err != nil {
		t.Fatal(err)
	}
	if err := WriteText(&second, newPlan(t, dir)); err != nil {
		t.Fatal(err)
	}
	if first.String() != second.String() {
		t.Fatal("repeated dry-runs produced different output")
	}
	for _, want := range []string{
		"Migration dry-run: READY",
		"Resources to move: 1",
		"Files to change: 2",
		"-resource \"fastly_service_vcl\" \"demo\"",
		"+resource \"fastly_service_cdn_auto\" \"demo\"",
		"+++ b/migration.tf",
		"moved {",
	} {
		if !strings.Contains(first.String(), want) {
			t.Fatalf("missing %q in preview:\n%s", want, first.String())
		}
	}
}

func TestPlanRepeatedAfterApplyingSameEditsIsIdempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", string(fixture(t, "before/main.tf")))
	// Apply is intentionally tested only here, never called by the dry-run path.
	result := newPlan(t, dir)
	if result.Blocked() {
		t.Fatalf("blocked: %#v", result.Issues)
	}
	for _, change := range result.Changes {
		writeConfig(t, dir, change.Path, string(change.After))
	}
	repeat := newPlan(t, dir)
	if repeat.Blocked() || len(repeat.Changes) != 0 || len(repeat.Moves) != 0 {
		t.Fatalf("repeat plan: %#v", repeat)
	}
}

func TestPlanCdnTransformsFromRegistry(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", `resource "fastly_service_vcl" "demo" {
  name            = local.name
  activate        = true
  stage           = false
  version_comment = ""
  default_host    = var.host

  backend {
    name = "origin"
    address = var.origin
  }

  vcl {
    name = "main"
    content = var.vcl_source
  }
}
`)
	result := newPlan(t, dir)
	if result.Blocked() {
		t.Fatalf("blocked: issues=%#v findings=%#v", result.Issues, result.Analysis.Findings)
	}
	if len(result.Changes) != 2 {
		t.Fatalf("changes=%#v", result.Changes)
	}
	got := string(result.Changes[0].After)
	for _, want := range []string{
		`resource "fastly_service_cdn_auto" "demo"`,
		"settings {",
		"custom_vcl {",
		"content = var.vcl_source",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in output:\n%s", want, got)
		}
	}
	// HCL source-preserving edits retain the original attribute alignment.
	// Match the assignment rather than requiring an exact number of spaces.
	if !regexp.MustCompile(`(?m)^[ \t]*default_host[ \t]*=[ \t]*var\.host[ \t]*$`).MatchString(got) {
		t.Errorf("missing preserved default_host expression in output:\n%s", got)
	}
	for _, removed := range []string{
		"activate =", "stage =", "version_comment =",
	} {
		if strings.Contains(got, removed) {
			t.Errorf("unremoved attribute %q in output:\n%s", removed, got)
		}
	}
}

func TestAllAutomaticResourceRulesAreCovered(t *testing.T) {
	t.Parallel()
	registry := compatibility.Default()
	var automatic int
	for _, rule := range registry.All() {
		if rule.Kind != compatibility.ConstructResource ||
			rule.Support != compatibility.SupportAutomatic {
			continue
		}
		automatic++
		t.Run(rule.ID, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			source := fmt.Sprintf("resource %q \"example\" {\n  name = var.name\n}\n", rule.Legacy)
			writeConfig(t, dir, "main.tf", source)
			result := newPlan(t, dir)
			if result.Blocked() {
				t.Fatalf("automatic resource rule %s blocked: issues=%#v findings=%#v",
					rule.ID, result.Issues, result.Analysis.Findings)
			}
			if rule.Legacy == "fastly_service_vcl" {
				if len(result.Changes) != 2 || len(result.Moves) != 1 {
					t.Fatalf("expected VCL preview, got %#v", result)
				}
			} else if len(result.Changes) != 0 || len(result.Moves) != 0 {
				t.Fatalf("unchanged resource %s was transformed: %#v", rule.Legacy, result)
			}
		})
	}
	if automatic < 2 {
		t.Fatal("automatic migration registry unexpectedly empty")
	}
}

func TestAllNonautomaticResourceRulesBlock(t *testing.T) {
	t.Parallel()
	var blocked int
	for _, rule := range compatibility.Default().All() {
		if rule.Kind != compatibility.ConstructResource ||
			rule.Support == compatibility.SupportAutomatic {
			continue
		}
		blocked++
		t.Run(rule.ID, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeConfig(t, dir, "main.tf",
				fmt.Sprintf("resource %q \"example\" {\n  name = var.name\n}\n", rule.Legacy))
			result := newPlan(t, dir)
			if !result.Blocked() || len(result.Changes) != 0 {
				t.Fatalf("nonautomatic resource rule %s produced preview: %#v", rule.ID, result)
			}
		})
	}
	if blocked == 0 {
		t.Fatal("nonautomatic resource coverage unexpectedly empty")
	}
}

func TestAllAutomaticCdnProductsBlockUntilWriterExists(t *testing.T) {
	t.Parallel()
	var products int
	for _, rule := range compatibility.Default().All() {
		if rule.Kind != compatibility.ConstructProduct ||
			rule.Family != compatibility.FamilyCDN ||
			rule.Support != compatibility.SupportAutomatic {
			continue
		}
		products++
		t.Run(rule.ID, func(t *testing.T) {
			t.Parallel()
			field := strings.TrimPrefix(rule.Legacy, "fastly_service_vcl.product_enablement.")
			dir := t.TempDir()
			writeConfig(t, dir, "main.tf", fmt.Sprintf(`resource "fastly_service_vcl" "demo" {
  name = "demo"
  product_enablement {
    %s = false
  }
}
`, field))
			result := newPlan(t, dir)
			if !result.Blocked() || len(result.Changes) != 0 ||
				len(result.Moves) != 0 {
				t.Fatalf("product %q produced incomplete diff: %#v", field, result)
			}
			if _, err := os.Stat(filepath.Join(dir, "migration.tf")); !os.IsNotExist(err) {
				t.Fatalf("dry-run wrote migration.tf: %v", err)
			}
		})
	}
	if products == 0 {
		t.Fatal("automatic CDN product coverage unexpectedly empty")
	}
}

func TestPlanFailClosedCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "direct reference",
			source: `resource "fastly_service_vcl" "demo" {
  name = "demo"
}
output "id" {
  value = fastly_service_vcl.demo.id
}
`,
			want: "reference",
		},
		{
			name: "count-indexed reference",
			source: `resource "fastly_service_vcl" "demo" {
  count = var.instances
  name = "demo"
}
output "id" {
  value = fastly_service_vcl.demo[0].id
}
`,
			want: "reference",
		},
		{
			name: "provider constraint",
			source: `terraform {
  required_providers {
    fastly = {
      source = "fastly/fastly"
      version = "~> 5.0"
    }
  }
}
resource "fastly_service_vcl" "demo" {
  name = "demo"
}
`,
			want: "provider version constraint",
		},
		{
			name: "null product block",
			source: `resource "fastly_service_vcl" "demo" {
  name = "demo"
  product_enablement {
    domain_inspector = null
  }
}
`,
			want: "product_enablement",
		},
		{
			name: "unknown product expression",
			source: `resource "fastly_service_vcl" "demo" {
  name = "demo"
  product_enablement {
    domain_inspector = var.enabled
  }
}
`,
			want: "product_enablement",
		},
		{
			name: "unsupported activate false",
			source: `resource "fastly_service_vcl" "demo" {
  name = "demo"
  activate = false
}
`,
			want: "activate",
		},
		{
			name: "unsupported Compute",
			source: `resource "fastly_service_compute" "demo" {
  name = "demo"
}
`,
			want: "fastly_service_compute",
		},
		{
			name:   "unknown Fastly resource",
			source: `resource "fastly_not_in_registry" "demo" {}`,
			want:   "compatibility rule",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeConfig(t, dir, "main.tf", tc.source)
			result := newPlan(t, dir)
			if !result.Blocked() || len(result.Changes) != 0 || len(result.Moves) != 0 {
				t.Fatalf("fail-closed case created partial preview: %#v", result)
			}
			var report bytes.Buffer
			if err := WriteText(&report, result); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(report.String(), tc.want) ||
				!strings.Contains(report.String(), "No changes proposed") {
				t.Errorf("unexpected blocker report:\n%s", report.String())
			}
			got, err := os.ReadFile(filepath.Join(dir, "main.tf"))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.source {
				t.Fatal("blocked preview wrote the input")
			}
		})
	}
}

func TestPlanChildModuleAndMovedPlacement(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", `module "service" {
  source = "./modules/service"
}
`)
	writeConfig(t, dir, "modules/service/main.tf", `resource "fastly_service_vcl" "nested" {
  name = "nested"
}
`)
	result := newPlan(t, dir)
	if result.Blocked() {
		t.Fatalf("blocked: %#v", result.Issues)
	}
	gotPaths := make([]string, 0, len(result.Changes))
	for _, change := range result.Changes {
		gotPaths = append(gotPaths, change.Path)
	}
	if !slices.Equal(gotPaths, []string{"modules/service/main.tf", "modules/service/migration.tf"}) {
		t.Fatalf("wrong module files: %#v", gotPaths)
	}
	if len(result.Moves) != 1 || result.Moves[0].Module != "modules/service" {
		t.Fatalf("wrong module movement: %#v", result.Moves)
	}
}

func TestPlanRejectsResourceCollisionAcrossFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "legacy.tf", `resource "fastly_service_vcl" "demo" {
  name = "demo"
}
`)
	writeConfig(t, dir, "other.tf", `resource "fastly_service_cdn_auto" "demo" {
  name = "different"
}
`)
	result := newPlan(t, dir)
	if !result.Blocked() || len(result.Changes) != 0 {
		t.Fatalf("cross-file collision accepted: %#v", result)
	}
	if len(result.Issues) == 0 || !strings.Contains(result.Issues[0].Message, "already exists") {
		t.Fatalf("missing collision diagnostic: %#v", result.Issues)
	}
}

func TestPlanMultipleVclResourcesHaveDeterministicMoves(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "z.tf", `resource "fastly_service_vcl" "zeta" {
  name = "zeta"
}
`)
	writeConfig(t, dir, "a.tf", `resource "fastly_service_vcl" "alpha" {
  name = "alpha"
}
`)
	result := newPlan(t, dir)
	if result.Blocked() {
		t.Fatalf("blocked: %#v", result.Issues)
	}
	if len(result.Changes) != 3 || len(result.Moves) != 2 {
		t.Fatalf("unexpected changes/moves: %#v", result)
	}
	if result.Moves[0].From != "fastly_service_vcl.alpha" ||
		result.Moves[1].From != "fastly_service_vcl.zeta" {
		t.Fatalf("unsorted moves: %#v", result.Moves)
	}
	for _, change := range result.Changes {
		if change.Path == "migration.tf" {
			text := string(change.After)
			if strings.Index(text, "vcl.alpha") > strings.Index(text, "vcl.zeta") {
				t.Fatalf("unsorted generated moves: %s", text)
			}
		}
	}
}

func TestCheckSnapshotsRejectsConcurrentChangeAndUnexpectedCreation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", "initial\n")
	snapshots := map[string][]byte{"main.tf": []byte("initial\n")}
	writeConfig(t, dir, "main.tf", "updated\n")
	if err := checkSnapshots(dir, snapshots, nil); err == nil {
		t.Fatal("changed input passed snapshot verification")
	}
	changes := []rewrite.Change{{Path: "migration.tf", Exists: true}}
	if err := checkSnapshots(dir, snapshots, changes); err == nil {
		t.Fatal("unexpected generated file passed snapshot verification")
	}
}

func TestPlanRejectsUnimplementedObjectAttributesAndComputedTargets(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		nested string
		want   string
	}{
		{
			name: "backend TLS client certificate is a nested object attribute",
			nested: `backend {
    name = "origin"
    address = var.origin
    ssl_client_cert = var.certificate
  }`,
			want: "object-attribute writer",
		},
		{
			name: "logging S3 authentication is a nested object attribute",
			nested: `logging_s3 {
    name = "bucket"
    bucket_name = var.bucket
    s3_access_key = var.key
  }`,
			want: "object-attribute writer",
		},
		{
			name: "rate limiter target id is computed-only",
			nested: `rate_limiter {
    name = "throttle"
    ratelimiter_id = var.limiter
  }`,
			want: "computed-only",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			source := fmt.Sprintf("resource \"fastly_service_vcl\" \"demo\" {\n  name = \"demo\"\n  %s\n}\n", tc.nested)
			writeConfig(t, dir, "main.tf", source)
			result := newPlan(t, dir)
			if !result.Blocked() || len(result.Changes) != 0 || len(result.Moves) != 0 {
				t.Fatalf("invalid target schema preview accepted: %#v", result)
			}
			var output bytes.Buffer
			if err := WriteText(&output, result); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output.String(), tc.want) {
				t.Errorf("missing %q in:\n%s", tc.want, output.String())
			}
		})
	}
}

func TestPlanDoesNotClaimRemoteModuleCoverage(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", `module "external" {
  source = "git::https://example.invalid/terraform-service.git"
}
`)
	result := newPlan(t, dir)
	if !result.Blocked() || len(result.Changes) != 0 {
		t.Fatalf("remote module internals were treated as migrated: %#v", result)
	}
	foundCode := false
	for _, finding := range result.Analysis.Findings {
		if finding.Code == analyze.FindingCodeRemoteModuleSource {
			foundCode = true
		}
	}
	if !foundCode {
		t.Fatalf("remote module diagnostic has no stable code: %#v", result.Analysis.Findings)
	}
	var output bytes.Buffer
	if err := WriteText(&output, result); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "remote module contents cannot be verified") {
		t.Fatalf("missing remote module safety diagnostic:\n%s", output.String())
	}
}

func TestPlanRejectsSymlinkedTerraformSource(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	outside := t.TempDir()
	writeConfig(t, outside, "external.tf", `resource "fastly_service_vcl" "demo" {
  name = "demo"
}
`)
	if err := os.Symlink(filepath.Join(outside, "external.tf"), filepath.Join(dir, "main.tf")); err != nil {
		t.Skipf("symlink setup unsupported: %v", err)
	}
	result := newPlan(t, dir)
	if !result.Blocked() || len(result.Changes) != 0 {
		t.Fatalf("symlinked input was accepted: %#v", result)
	}
	got, err := os.ReadFile(filepath.Join(outside, "external.tf"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `fastly_service_vcl`) {
		t.Fatal("dry-run modified a symlink target")
	}
}

func TestPlanRejectsChildModulesOutsideSelectedRoot(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	rootDir := filepath.Join(parent, "root")
	writeConfig(t, rootDir, "main.tf", `module "external" {
  source = "../outside"
}
`)
	writeConfig(t, parent, "outside/main.tf", `resource "fastly_service_vcl" "demo" {
  name = "demo"
}
`)
	result := newPlan(t, rootDir)
	if !result.Blocked() || len(result.Changes) != 0 {
		t.Fatalf("migration outside selected root was accepted: %#v", result)
	}
	if _, err := os.Stat(filepath.Join(parent, "outside/migration.tf")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created files outside root: %v", err)
	}
}
