package migrate

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSimpleProductMigrationRealServiceShape(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	const input = `terraform {
  required_providers {
    fastly = {
      source  = "fastly/fastly"
      version = "~> 9.0"
    }
  }
}
provider "fastly" {}
resource "fastly_service_vcl" "demo_service" {
  name = "philipps-minimal-vcl-service"
  force_destroy = true
  domain {
    name = "www.philipps-minimal-vcl-service.com"
    comment = "Main public entrypoint"
  }
  backend {
    name = "origin-backend"
    address = "www.example.com"
    port = 443
  }
  product_enablement {
    domain_inspector = true
  }
}
`
	writeConfig(t, dir, "main.tf", input)
	plan := newPlan(t, dir)
	if plan.Blocked() {
		t.Fatalf("plan blocked: issues=%#v findings=%#v", plan.Issues, plan.Analysis.Findings)
	}
	if len(plan.Changes) != 2 || len(plan.Moves) != 1 {
		t.Fatalf("wrong change/move counts: %#v", plan)
	}
	main := changedFile(t, plan, "main.tf")
	generated := changedFile(t, plan, "migration.tf")
	for _, expected := range []string{
		`resource "fastly_service_cdn_auto" "demo_service"`,
		`version = ">= 20.0.0"`,
		`name = "www.philipps-minimal-vcl-service.com"`,
		`address = "www.example.com"`,
	} {
		if !strings.Contains(main, expected) {
			t.Errorf("lost %q from main.tf:\n%s", expected, main)
		}
	}
	if strings.Contains(main, "product_enablement") {
		t.Fatalf("legacy product block remains:\n%s", main)
	}
	for _, expected := range []string{
		`resource "fastly_service_product_domain_inspector" "demo_service"`,
		`service_id = fastly_service_cdn_auto.demo_service.id`,
		`enabled    = true`,
		`import {`,
		`to = fastly_service_product_domain_inspector.demo_service`,
		`id = fastly_service_cdn_auto.demo_service.id`,
		`moved {`,
		`from = fastly_service_vcl.demo_service`,
		`to   = fastly_service_cdn_auto.demo_service`,
	} {
		if !strings.Contains(generated, expected) {
			t.Errorf("missing %q from migration.tf:\n%s", expected, generated)
		}
	}
	original, err := os.ReadFile(filepath.Join(dir, "main.tf"))
	if err != nil || string(original) != input {
		t.Fatalf("preview wrote input: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "migration.tf")); !os.IsNotExist(err) {
		t.Fatalf("preview wrote migration.tf: %v", err)
	}
}

func TestSimpleProductMultipleValuesAndIdempotency(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", `provider "fastly" {
  alias = "edge"
}
resource "fastly_service_vcl" "demo" {
  provider = fastly.edge
  name = "demo"
  product_enablement {
    domain_inspector = true
    websockets = false
    origin_inspector = null
    brotli_compression = true
  }
}
`)
	plan := newPlan(t, dir)
	if plan.Blocked() {
		t.Fatalf("blocked: %#v findings=%#v", plan.Issues, plan.Analysis.Findings)
	}
	generated := changedFile(t, plan, "migration.tf")
	if got := strings.Count(generated, "import {"); got != 3 {
		t.Errorf("imports = %d, want 3:\n%s", got, generated)
	}
	if got := strings.Count(generated, `resource "fastly_service_product_`); got != 3 {
		t.Errorf("product resources = %d, want 3:\n%s", got, generated)
	}
	if strings.Contains(generated, "origin_inspector") {
		t.Fatalf("null product was imported:\n%s", generated)
	}
	for _, expected := range []string{
		`resource "fastly_service_product_websockets" "demo"`,
		`enabled    = false`,
		`provider   = fastly.edge`,
		`id = fastly_service_cdn_auto.demo.id`,
	} {
		if !strings.Contains(generated, expected) {
			t.Errorf("missing %q:\n%s", expected, generated)
		}
	}
	for _, change := range plan.Changes {
		writeConfig(t, dir, change.Path, string(change.After))
	}
	rerun := newPlan(t, dir)
	if rerun.Blocked() || len(rerun.Changes) != 0 || len(rerun.Moves) != 0 {
		t.Fatalf("second run not idempotent: changes=%#v issues=%#v", rerun.Changes, rerun.Issues)
	}
}

func TestSimpleProductEmptyAndNullOnlyBlocks(t *testing.T) {
	t.Parallel()
	for _, content := range []string{
		"product_enablement {}",
		"product_enablement {\n domain_inspector = null\n}",
	} {
		t.Run(strings.ReplaceAll(content, "\n", "-"), func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeConfig(t, dir, "main.tf", `resource "fastly_service_vcl" "demo" {
  name = "demo"
  `+content+`
}
`)
			plan := newPlan(t, dir)
			if plan.Blocked() {
				t.Fatalf("blocked: %#v findings=%#v", plan.Issues, plan.Analysis.Findings)
			}
			main := changedFile(t, plan, "main.tf")
			generated := changedFile(t, plan, "migration.tf")
			if strings.Contains(main, "product_enablement") {
				t.Fatalf("block remains:\n%s", main)
			}
			if strings.Contains(generated, "import {") || strings.Contains(generated, "fastly_service_product_") {
				t.Fatalf("empty/null product generated import:\n%s", generated)
			}
		})
	}
}

func TestSimpleProductLocalChildModule(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", `module "edge" {
  source = "./modules/edge"
}
`)
	writeConfig(t, dir, "modules/edge/main.tf", `resource "fastly_service_vcl" "edge" {
  name = "edge"
  product_enablement {
    domain_inspector = true
  }
}
`)
	plan := newPlan(t, dir)
	if plan.Blocked() {
		t.Fatalf("blocked: %#v findings=%#v", plan.Issues, plan.Analysis.Findings)
	}
	child := changedFile(t, plan, "modules/edge/migration.tf")
	for _, expected := range []string{
		`service_id = fastly_service_cdn_auto.edge.id`,
		`to = fastly_service_product_domain_inspector.edge`,
		`id = fastly_service_cdn_auto.edge.id`,
		`from = fastly_service_vcl.edge`,
	} {
		if !strings.Contains(child, expected) {
			t.Errorf("missing %q:\n%s", expected, child)
		}
	}
	if bytes.Contains([]byte(child), []byte("module.edge.")) {
		t.Fatal("module-local import must not use root module addresses")
	}
}

func TestSimpleProductFailClosedWithoutPartialDiff(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, src, extra, expected string }{
		{"unknown variable", `product_enablement { domain_inspector = var.enabled }`, "", "cannot be determined statically"},
		{"dynamic product enablement", `dynamic "product_enablement" {
  for_each = [1]
  content {
    domain_inspector = true
  }
}`, "", "Dynamic legacy service block"},
		{"literal plus dynamic product enablement", `product_enablement { domain_inspector = true }
dynamic "product_enablement" {
  for_each = [1]
  content {
    websockets = true
  }
}`, "", "Dynamic legacy service block"},
		{"invalid literal", `product_enablement { domain_inspector = "yes" }`, "", "must be a boolean or null"},
		{"count on service", `count = 1
product_enablement { domain_inspector = true }`, "", "indexed VCL service"},
		{"for_each on service", `for_each = toset(["edge"])
product_enablement { domain_inspector = true }`, "", "indexed VCL service"},
		{"comments would be discarded", `product_enablement {
 # Preserve this comment
 domain_inspector = true
}`, "", "comments"},
		{"duplicate aggregate blocks", `product_enablement { domain_inspector = true }
product_enablement { websockets = false }`, "", "multiple product_enablement blocks"},
		{
			"existing product collision", `product_enablement { domain_inspector = true }`,
			`resource "fastly_service_product_domain_inspector" "demo" {
  service_id = "existing"
}`, "already exists",
		},
		{
			"existing import collision", `product_enablement { domain_inspector = true }`,
			`import {
  to = fastly_service_product_domain_inspector.demo
  id = "another-service"
}`, "already targeted by an import",
		},
		{"configurable product non-literal enabled", `product_enablement {
 bot_management {
  enabled = var.on
 }
}`, "", "product_enablement.bot_management.enabled must be a literal boolean"},
		{"configurable product non-literal setting", `product_enablement {
 ngwaf {
  enabled      = true
  workspace_id = var.workspace
 }
}`, "", "product_enablement.ngwaf.workspace_id must be a literal string"},
		{"configurable product unknown attribute", `product_enablement {
 ddos_protection {
  enabled = true
  level   = "high"
 }
}`, "", "product_enablement.ddos_protection.level is not supported"},
		{"configurable product duplicate block", `product_enablement {
 bot_management { enabled = true }
 bot_management { enabled = false }
}`, "", "multiple bot_management blocks"},
		{"bot_management missing contentguard", `product_enablement {
 bot_management { enabled = true }
}`, "", "product_enablement.bot_management.contentguard is required"},
		{"ddos_protection missing mode", `product_enablement {
 ddos_protection { enabled = true }
}`, "", "product_enablement.ddos_protection.mode is required"},
		{"ngwaf missing workspace_id", `product_enablement {
 ngwaf {
  enabled      = true
  traffic_ramp = 25
 }
}`, "", "product_enablement.ngwaf.workspace_id is required"},
		{"count on service with configurable-only product", `count = 1
product_enablement {
 bot_management {
  enabled      = true
  contentguard = "on"
 }
}`, "", "indexed VCL service"},
		{"for_each on service with configurable-only product", `for_each = toset(["edge"])
product_enablement {
 ngwaf {
  enabled      = true
  workspace_id = "ws-123"
 }
}`, "", "indexed VCL service"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			source := `resource "fastly_service_vcl" "demo" {
  name = "demo"
` + tc.src + "\n}\n" + tc.extra
			writeConfig(t, dir, "main.tf", source)
			result := newPlan(t, dir)
			if !result.Blocked() || len(result.Changes) != 0 || len(result.Moves) != 0 {
				t.Fatalf("unsafe product split generated changes: %#v", result)
			}
			var messages []string
			for _, issue := range result.Issues {
				messages = append(messages, issue.Message)
			}
			for _, finding := range result.Analysis.Findings {
				messages = append(messages, finding.Message)
			}
			if !strings.Contains(strings.Join(messages, " "), tc.expected) {
				t.Fatalf("expected %q in issues/findings: %#v %#v", tc.expected, result.Issues, result.Analysis.Findings)
			}
			if _, err := os.Stat(filepath.Join(dir, "migration.tf")); !os.IsNotExist(err) {
				t.Fatalf("blocked migration wrote migration.tf: %v", err)
			}
			actual, err := os.ReadFile(filepath.Join(dir, "main.tf"))
			if err != nil || string(actual) != source {
				t.Fatalf("blocked migration altered configuration: %v", err)
			}
		})
	}
}

func TestSimpleProductRejectsRootImportOfChildProduct(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", `module "edge" {
  source = "./modules/edge"
}

import {
  to = module.edge.fastly_service_product_domain_inspector.demo
  id = "existing"
}
`)
	writeConfig(t, dir, "modules/edge/main.tf", `resource "fastly_service_vcl" "demo" {
  name = "demo"
  product_enablement {
    domain_inspector = true
  }
}
`)
	result := newPlan(t, dir)
	if !result.Blocked() || len(result.Changes) != 0 {
		t.Fatalf("root import collision produced a partial migration: %#v", result)
	}
	var messages []string
	for _, issue := range result.Issues {
		messages = append(messages, issue.Message)
	}
	if !strings.Contains(strings.Join(messages, " "), "module-qualified import") {
		t.Fatalf("missing nested import collision diagnostic: %#v", result.Issues)
	}
}

// Test the compiler guard directly as well as the analyzer-driven planner
// tests above. A dynamic aggregate must not escape the compiler if a caller
// ever supplies a preselected resource action.
func TestCompileSimpleProductsRejectsDynamicAggregate(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, aggregate string
	}{
		{
			name: "dynamic only",
			aggregate: `dynamic "product_enablement" {
  for_each = [1]
  content {
    domain_inspector = true
  }
}`,
		},
		{
			name: "literal and dynamic",
			aggregate: `product_enablement { domain_inspector = true }
dynamic "product_enablement" {
  for_each = [1]
  content {
    websockets = false
  }
}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := `resource "fastly_service_vcl" "demo" {
  name = "demo"
` + tc.aggregate + `
}
`
			key := resourceKey{file: "main.tf", typ: "fastly_service_vcl", name: "demo"}
			index := moduleIndex{snapshots: map[string][]byte{"main.tf": []byte(source)}}
			edits, issues := compileSimpleProducts(key, nil, index)
			if len(edits) != 0 || len(issues) != 1 ||
				!strings.Contains(issues[0].Message, "dynamic product_enablement cannot be migrated safely") {
				t.Fatalf("dynamic aggregate was not blocked: edits=%#v issues=%#v", edits, issues)
			}
		})
	}
}

func TestConfigurableProductMigrationPreservesSettings(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", `resource "fastly_service_vcl" "demo" {
  name = "demo"
  product_enablement {
    websockets = true
    bot_management {
      enabled      = true
      contentguard = "on"
    }
    ddos_protection {
      enabled = false
      mode    = "block"
    }
    ngwaf {
      enabled      = true
      workspace_id = "ws-123"
      traffic_ramp = 25
    }
  }
}
`)
	plan := newPlan(t, dir)
	if plan.Blocked() {
		t.Fatalf("blocked: %#v findings=%#v", plan.Issues, plan.Analysis.Findings)
	}
	main := changedFile(t, plan, "main.tf")
	if strings.Contains(main, "product_enablement") {
		t.Fatalf("legacy product block remains:\n%s", main)
	}
	generated := changedFile(t, plan, "migration.tf")
	if got := strings.Count(generated, "import {"); got != 4 {
		t.Errorf("imports = %d, want 4:\n%s", got, generated)
	}
	for _, expected := range []string{
		`resource "fastly_service_product_bot_management" "demo"`,
		`contentguard = "on"`,
		`resource "fastly_service_product_ddos_protection" "demo"`,
		`mode       = "block"`,
		`enabled    = false`,
		`resource "fastly_service_product_ngwaf" "demo"`,
		`workspace_id = "ws-123"`,
		`traffic_ramp = 25`,
		`to = fastly_service_product_ngwaf.demo`,
	} {
		if !strings.Contains(generated, expected) {
			t.Errorf("missing %q from migration.tf:\n%s", expected, generated)
		}
	}
}
