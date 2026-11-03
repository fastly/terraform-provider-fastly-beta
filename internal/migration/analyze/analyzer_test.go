package analyze

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/compatibility"
)

func TestAnalyzeSupportedConfiguration(t *testing.T) {
	t.Parallel()

	result := analyzeFixture(t, "supported")

	if !result.Migratable() {
		t.Fatalf("Migratable() = false, findings = %#v", result.Findings)
	}
	if got, want := result.Modules, []string{".", "modules/service"}; !slices.Equal(got, want) {
		t.Fatalf("Modules = %#v, want %#v", got, want)
	}
	if len(result.Files) != 2 {
		t.Fatalf("Files = %d, want 2", len(result.Files))
	}
	if len(result.ProviderRequirements) != 1 {
		t.Fatalf("ProviderRequirements = %d, want 1", len(result.ProviderRequirements))
	}
	requirement := result.ProviderRequirements[0]
	if requirement.Source != "fastly/fastly" {
		t.Fatalf("ProviderRequirement = %#v, want fastly/fastly", requirement)
	}

	for _, ruleID := range []string{
		"resource-service-vcl",
		"cdn-attr-activate-default",
		"cdn-attr-stage-default",
		"cdn-attr-version-comment-empty",
		"cdn-block-backend",
		"cdn-product-domain-inspector",
		"cdn-product-websockets",
	} {
		if !hasAction(result, ruleID) {
			t.Errorf("missing action for rule %q", ruleID)
		}
	}

	if len(result.References) != 1 {
		t.Fatalf("References = %#v, want one legacy service reference", result.References)
	}
	if got, want := result.References[0].From, "fastly_service_vcl.demo"; got != want {
		t.Fatalf("Reference.From = %q, want %q", got, want)
	}
	if got, want := result.References[0].To, "fastly_service_cdn_auto.demo"; got != want {
		t.Fatalf("Reference.To = %q, want %q", got, want)
	}
}

func TestAnalyzeTerraformMetaArgumentsAreNotSchemaFindings(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "main.tf", `
variable "services" {
  type = set(string)
}

resource "fastly_service_vcl" "demo" {
  for_each = var.services
  name     = each.key

  lifecycle {
    ignore_changes = [comment]
  }
}
`)

	result := analyzeDir(t, dir)

	if !result.Migratable() {
		t.Fatalf("Migratable() = false, findings = %#v", result.Findings)
	}
	if !hasAction(result, "resource-service-vcl") {
		t.Fatal("missing VCL service migration action")
	}
}

func TestAnalyzeProviderRequirementIgnoresConfigurationAliasesForStaticDetection(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "main.tf", `
terraform {
  required_providers {
    fastly = {
      source                = "fastly/fastly"
      version               = "~> 5.0"
      configuration_aliases = [fastly.edge]
    }
  }
}
`)

	result := analyzeDir(t, dir)

	if !result.Migratable() {
		t.Fatalf("Migratable() = false, findings = %#v", result.Findings)
	}
	if len(result.ProviderRequirements) != 1 {
		t.Fatalf("ProviderRequirements = %#v, want one Fastly requirement", result.ProviderRequirements)
	}
	requirement := result.ProviderRequirements[0]
	if requirement.Source != "fastly/fastly" || requirement.Version != "~> 5.0" {
		t.Fatalf("ProviderRequirement = %#v, want static source and version", requirement)
	}
}

func TestAnalyzeLifecycleBlockers(t *testing.T) {
	t.Parallel()

	result := analyzeFixture(t, "blocked-lifecycle")

	if result.Migratable() {
		t.Fatal("Migratable() = true, want false")
	}
	for _, ruleID := range []string{
		"cdn-attr-activate-false",
		"cdn-attr-stage-true",
		"cdn-attr-version-comment-set",
	} {
		finding, ok := findingByRuleID(result, ruleID)
		if !ok {
			t.Errorf("missing finding for rule %q", ruleID)
			continue
		}
		if finding.Range.Start.Line == 0 {
			t.Errorf("finding %q has no source line", ruleID)
		}
		if finding.Remediation == "" {
			t.Errorf("finding %q has no remediation", ruleID)
		}
	}
}

func TestAnalyzeNullLifecycleValuesUseOmittedSemantics(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "main.tf", `
resource "fastly_service_vcl" "demo" {
  name            = "migration-demo"
  activate        = null
  stage           = null
  version_comment = null
}
`)

	result := analyzeDir(t, dir)

	if !result.Migratable() {
		t.Fatalf("Migratable() = false, findings = %#v", result.Findings)
	}
	for _, ruleID := range []string{
		"cdn-attr-activate-default",
		"cdn-attr-stage-default",
		"cdn-attr-version-comment-empty",
	} {
		if !hasAction(result, ruleID) {
			t.Errorf("missing action for null/omitted rule %q", ruleID)
		}
	}
}

func TestAnalyzeAmbiguousSelector(t *testing.T) {
	t.Parallel()

	result := analyzeFixture(t, "ambiguous")

	if result.Migratable() {
		t.Fatal("Migratable() = true, want false")
	}
	if !containsFinding(result, "cannot be classified statically") {
		t.Fatalf("Findings = %#v, want static-value diagnostic", result.Findings)
	}
	if !containsFinding(result, "true or omitted") || !containsFinding(result, "false") {
		t.Fatalf("Findings = %#v, want supported selector values in remediation", result.Findings)
	}
}

func TestAnalyzeTargetParityBlocker(t *testing.T) {
	t.Parallel()

	result := analyzeFixture(t, "parity-blocked")

	finding, ok := findingByRuleID(result, "resource-service-compute")
	if !ok {
		t.Fatalf("Findings = %#v, want resource-service-compute blocker", result.Findings)
	}
	if !strings.Contains(finding.Remediation, "ResourceWithMoveState") {
		t.Fatalf("Remediation = %q, want provider prerequisite", finding.Remediation)
	}
}

func TestAnalyzeUnsupportedServiceBlock(t *testing.T) {
	t.Parallel()

	result := analyzeFixture(t, "unsupported")

	finding, ok := findingByRuleID(result, "cdn-block-logging-logentries")
	if !ok {
		t.Fatalf("Findings = %#v, want logging_logentries blocker", result.Findings)
	}
	if !strings.Contains(finding.Remediation, "No logging_logentries block") {
		t.Fatalf("Remediation = %q, want compatibility notes", finding.Remediation)
	}
}

func TestAnalyzeUnsupportedNestedAttribute(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "main.tf", `
resource "fastly_service_vcl" "demo" {
  name = "migration-demo"

  backend {
    name            = "origin"
    address         = "origin.example.com"
    error_threshold = 1
  }
}
`)

	result := analyzeDir(t, dir)

	finding, ok := findingByRuleID(result, "cdn-backend-error-threshold")
	if !ok {
		t.Fatalf("Findings = %#v, want backend error_threshold blocker", result.Findings)
	}
	if !strings.Contains(finding.Remediation, "cannot be preserved") {
		t.Fatalf("Remediation = %q, want compatibility guidance", finding.Remediation)
	}
}

func TestAnalyzeNestedServiceBlockAppliesRegisteredRules(t *testing.T) {
	t.Parallel()

	const legacy = "fastly_service_vcl.backend.authentication"
	dir := t.TempDir()
	writeFile(t, dir, "main.tf", `
resource "fastly_service_vcl" "demo" {
  name = "migration-demo"

  backend {
    name    = "origin"
    address = "origin.example.com"

    authentication {}
  }
}
`)

	tests := []struct {
		name      string
		support   compatibility.SupportStatus
		wantBlock bool
	}{
		{name: "automatic", support: compatibility.SupportAutomatic},
		{name: "blocked", support: compatibility.SupportBlocked, wantBlock: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rules := append(compatibility.Default().All(), compatibility.Rule{
				ID:                   "test-nested-authentication",
				Kind:                 compatibility.ConstructServiceBlock,
				Family:               compatibility.FamilyCDN,
				Legacy:               legacy,
				Target:               "fastly_service_cdn_auto.backend.authentication",
				Support:              tc.support,
				HCL:                  compatibility.HCLReshape,
				State:                compatibility.StateMove,
				ImportIdentity:       compatibility.ImportNone,
				ProviderPrerequisite: "Nested block migration requires provider support.",
			})
			registry, err := compatibility.NewRegistry(rules)
			if err != nil {
				t.Fatalf("NewRegistry(): %v", err)
			}
			result, err := New(registry).Analyze(context.Background(), dir)
			if err != nil {
				t.Fatalf("Analyze(): %v", err)
			}
			if got := !result.Migratable(); got != tc.wantBlock {
				t.Fatalf("blocked = %t, want %t; findings = %#v", got, tc.wantBlock, result.Findings)
			}
			if tc.wantBlock {
				if _, ok := findingByRuleID(result, "test-nested-authentication"); !ok {
					t.Fatalf("missing nested block finding: %#v", result.Findings)
				}
			} else if !hasAction(result, "test-nested-authentication") {
				t.Fatalf("missing nested block action: %#v", result.Actions)
			}
		})
	}
}

func TestAnalyzeNestedServiceBlockSelectorsFailClosed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "main.tf", `
resource "fastly_service_vcl" "demo" {
  name = "migration-demo"
  backend {
    name    = "origin"
    address = "origin.example.com"
    authentication {}
  }
}
`)
	rules := append(compatibility.Default().All(), compatibility.Rule{
		ID:             "test-selected-nested-block",
		Kind:           compatibility.ConstructServiceBlock,
		Family:         compatibility.FamilyCDN,
		Legacy:         "fastly_service_vcl.backend.authentication",
		Target:         "fastly_service_cdn_auto.backend.authentication",
		Selector:       `mode = "required"`,
		Support:        compatibility.SupportAutomatic,
		HCL:            compatibility.HCLReshape,
		State:          compatibility.StateMove,
		ImportIdentity: compatibility.ImportNone,
	})
	registry, err := compatibility.NewRegistry(rules)
	if err != nil {
		t.Fatalf("NewRegistry(): %v", err)
	}
	result, err := New(registry).Analyze(context.Background(), dir)
	if err != nil {
		t.Fatalf("Analyze(): %v", err)
	}
	if result.Migratable() || !containsFinding(result, "cannot be classified unambiguously") {
		t.Fatalf("expected nested selector blocker, findings = %#v", result.Findings)
	}
}

func TestAnalyzeRenamedVersionlessResourceParityBlocker(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "main.tf", `resource "fastly_compute_acl" "example" {}`)

	result := analyzeDir(t, dir)

	finding, ok := findingByRuleID(result, "resource-compute-acl")
	if !ok {
		t.Fatalf("Findings = %#v, want resource-compute-acl blocker", result.Findings)
	}
	if !strings.Contains(finding.Remediation, "renamed versionless resources") {
		t.Fatalf("Remediation = %q, want state-migration prerequisite", finding.Remediation)
	}
}

func TestAnalyzeSelectorBasedVersionlessResource(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "main.tf", `
resource "fastly_ngwaf_account_list" "example" {
  type = "ip"
}
`)

	result := analyzeDir(t, dir)

	if _, ok := findingByRuleID(result, "resource-ngwaf-account-list-ip"); !ok {
		t.Fatalf("Findings = %#v, want selected ip compatibility rule", result.Findings)
	}
	if containsFinding(result, "cannot be determined statically") {
		t.Fatalf("Findings = %#v, did not expect ambiguity", result.Findings)
	}
}

func TestAnalyzeAmbiguousVersionlessSelector(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "main.tf", `
variable "list_type" {
  type = string
}

resource "fastly_ngwaf_account_list" "example" {
  type = var.list_type
}
`)

	result := analyzeDir(t, dir)

	if result.Migratable() {
		t.Fatal("Migratable() = true, want false")
	}
	if !containsFinding(result, `selector "type" cannot be determined statically`) {
		t.Fatalf("Findings = %#v, want ambiguous type selector", result.Findings)
	}
}

func TestAnalyzeReferencesAreScopedPerModule(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "main.tf", `
module "ip" {
  source = "./ip"
}

module "strings" {
  source = "./strings"
}
`)
	writeFile(t, dir, "ip/main.tf", `
resource "fastly_ngwaf_account_list" "shared" {
  type = "ip"
}

output "id" {
  value = fastly_ngwaf_account_list.shared.id
}
`)
	writeFile(t, dir, "strings/main.tf", `
resource "fastly_ngwaf_account_list" "shared" {
  type = "string"
}

output "id" {
  value = fastly_ngwaf_account_list.shared.id
}
`)

	result := analyzeDir(t, dir)

	if len(result.References) != 2 {
		t.Fatalf("References = %#v, want two module-scoped references", result.References)
	}
	targets := map[string]bool{}
	for _, reference := range result.References {
		targets[reference.To] = true
	}
	for _, want := range []string{
		"fastly_ngwaf_ip_list.shared",
		"fastly_ngwaf_string_list.shared",
	} {
		if !targets[want] {
			t.Errorf("reference targets = %#v, missing %q", targets, want)
		}
	}
}

func TestAnalyzeEmptyProductEnablementDoesNotInventProductActions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "main.tf", `
resource "fastly_service_vcl" "demo" {
  name = "migration-demo"

  product_enablement {}
}
`)

	result := analyzeDir(t, dir)

	if !result.Migratable() {
		t.Fatalf("Migratable() = false, findings = %#v", result.Findings)
	}
	for _, action := range result.Actions {
		if action.Kind == compatibility.ConstructProduct {
			t.Fatalf("unexpected product action for empty product_enablement: %#v", action)
		}
	}
}

func TestAnalyzeNullProductEnablementDoesNotInventProductAction(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "main.tf", `
resource "fastly_service_vcl" "demo" {
  name = "migration-demo"

  product_enablement {
    domain_inspector = null
  }
}
`)

	result := analyzeDir(t, dir)

	if !result.Migratable() {
		t.Fatalf("Migratable() = false, findings = %#v", result.Findings)
	}
	for _, action := range result.Actions {
		if action.RuleID == "cdn-product-domain-inspector" {
			t.Fatalf("unexpected product action for null enablement: %#v", action)
		}
	}
}

func TestAnalyzeConfigurableProductParityBlocker(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "main.tf", `
resource "fastly_service_vcl" "demo" {
  name = "migration-demo"

  product_enablement {
    bot_management {
      enabled      = true
      contentguard = "on"
    }
  }
}
`)

	result := analyzeDir(t, dir)

	if _, ok := findingByRuleID(result, "cdn-product-bot-management"); !ok {
		t.Fatalf("Findings = %#v, want configurable product blocker", result.Findings)
	}
}

func TestAnalyzeDynamicLegacyServiceBlockIsAmbiguous(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "main.tf", `
variable "backends" {
  type = map(string)
}

resource "fastly_service_vcl" "demo" {
  name = "migration-demo"

  dynamic "backend" {
    for_each = var.backends
    content {
      name    = backend.key
      address = backend.value
    }
  }
}
`)

	result := analyzeDir(t, dir)

	if result.Migratable() {
		t.Fatal("Migratable() = true, want false")
	}
	if !containsFinding(result, `Dynamic legacy service block "fastly_service_vcl.backend"`) {
		t.Fatalf("Findings = %#v, want dynamic-block diagnostic", result.Findings)
	}
}

func TestAnalyzeTerraformJSONFailsClosed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "main.tf.json", `{"resource":{}}`)

	result := analyzeDir(t, dir)

	if result.Migratable() {
		t.Fatal("Migratable() = true, want false")
	}
	if !containsFinding(result, "Terraform JSON configuration is not supported") {
		t.Fatalf("Findings = %#v, want .tf.json diagnostic", result.Findings)
	}
}

func TestAnalyzeSyntaxErrorHasSourceDiagnostic(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "main.tf", `resource "fastly_service_vcl" "demo" {`)

	result := analyzeDir(t, dir)

	if result.Migratable() {
		t.Fatal("Migratable() = true, want false")
	}
	if len(result.Findings) == 0 {
		t.Fatal("Findings is empty, want parse diagnostic")
	}
	if result.Findings[0].Range.Start.Line == 0 {
		t.Fatalf("Finding range = %#v, want source position", result.Findings[0].Range)
	}
}

func TestAnalyzeMissingLocalModuleReturnsError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "main.tf", `
module "missing" {
  source = "./does-not-exist"
}
`)

	analyzer := New(compatibility.Default())
	_, err := analyzer.Analyze(context.Background(), dir)
	if err == nil {
		t.Fatal("Analyze() error = nil, want missing-module error")
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Fatalf("Analyze() error = %q, want missing module path", err)
	}
}

func TestAnalyzeRemoteModuleDoesNotInspectModuleCache(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "main.tf", `
module "remote" {
  source = "example.com/example/service/fastly"
}
`)

	result := analyzeDir(t, dir)

	if !result.Migratable() {
		t.Fatalf("Migratable() = false, findings = %#v", result.Findings)
	}
	if got, want := result.Modules, []string{"."}; !slices.Equal(got, want) {
		t.Fatalf("Modules = %#v, want %#v", got, want)
	}
	if !containsFinding(result, "Remote module source") {
		t.Fatalf("Findings = %#v, want remote-module warning", result.Findings)
	}
}

func TestAnalyzeDoesNotTouchTerraformState(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "main.tf", `
resource "fastly_service_vcl" "demo" {
  name = "migration-demo"
}
`)
	statePath := filepath.Join(dir, "terraform.tfstate")
	state := []byte(`{"version":4,"serial":17,"lineage":"sentinel"}`)
	if err := os.WriteFile(statePath, state, 0o600); err != nil {
		t.Fatalf("WriteFile(state): %v", err)
	}

	_ = analyzeDir(t, dir)

	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("ReadFile(state): %v", err)
	}
	if string(after) != string(state) {
		t.Fatalf("terraform.tfstate changed: got %q, want %q", after, state)
	}
}

func TestAnalyzeIsIdempotentAndReadOnly(t *testing.T) {
	t.Parallel()

	dir := fixturePath(t, "supported")
	before := hashTree(t, dir)

	first := analyzeDir(t, dir)
	second := analyzeDir(t, dir)
	after := hashTree(t, dir)

	if before != after {
		t.Fatalf("configuration changed during analyze: before=%s after=%s", before, after)
	}
	if fmt.Sprintf("%#v", first) != fmt.Sprintf("%#v", second) {
		t.Fatal("Analyze() returned different results for identical input")
	}
}

func TestAnalyzeRequiresRegistry(t *testing.T) {
	t.Parallel()

	_, err := New(nil).Analyze(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("Analyze() error = nil, want registry error")
	}
}

func analyzeFixture(t *testing.T, name string) Result {
	t.Helper()
	return analyzeDir(t, fixturePath(t, name))
}

func fixturePath(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("filepath.Abs(): %v", err)
	}
	return path
}

func analyzeDir(t *testing.T, dir string) Result {
	t.Helper()
	result, err := New(compatibility.Default()).Analyze(context.Background(), dir)
	if err != nil {
		t.Fatalf("Analyze() returned error: %v", err)
	}
	return result
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(): %v", err)
	}
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}
}

func hasAction(result Result, ruleID string) bool {
	for _, action := range result.Actions {
		if action.RuleID == ruleID {
			return true
		}
	}
	return false
}

func findingByRuleID(result Result, ruleID string) (Finding, bool) {
	for _, finding := range result.Findings {
		if finding.RuleID == ruleID {
			return finding, true
		}
	}
	return Finding{}, false
}

func containsFinding(result Result, substring string) bool {
	for _, finding := range result.Findings {
		if strings.Contains(finding.Message, substring) || strings.Contains(finding.Remediation, substring) {
			return true
		}
	}
	return false
}

func hashTree(t *testing.T, root string) string {
	t.Helper()
	rootDir, err := os.OpenRoot(root)
	if err != nil {
		t.Fatalf("OpenRoot(): %v", err)
	}
	defer rootDir.Close()

	hash := sha256.New()
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		file, err := rootDir.Open(relative)
		if err != nil {
			return err
		}
		_, _ = hash.Write([]byte(filepath.ToSlash(relative)))
		_, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if err != nil {
		t.Fatalf("WalkDir(): %v", err)
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}
