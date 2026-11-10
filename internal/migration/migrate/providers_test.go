package migrate

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func changedFile(t *testing.T, result Result, path string) string {
	t.Helper()
	for _, change := range result.Changes {
		if change.Path == path {
			return string(change.After)
		}
	}
	t.Fatalf("no change to %q in %#v", path, result.Changes)
	return ""
}

func TestProviderRequirementsMigration(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, config, expectedFile string
		want, preserved            []string
	}{
		{
			name: "legacy version with comments",
			config: `terraform {
  required_providers {
    fastly = {
      source = "fastly/fastly"
      version = "~> 9.0" # keep this comment
      configuration_aliases = [fastly.edge]
    }
  }
}
provider "fastly" {
  alias   = "edge"
  api_key = var.api_key # preserve expression
}
`,
			expectedFile: "main.tf",
			want: []string{
				`version = ">= 20.0.0" # keep this comment`,
				`api_token = var.api_key # preserve expression`,
				`configuration_aliases = [fastly.edge]`,
				`alias   = "edge"`,
			},
			preserved: []string{`source = "fastly/fastly"`},
		},
		{
			name: "requirement without version",
			config: `terraform {
  required_providers {
    fastly = {
      source = "fastly/fastly"
    }
  }
}
`,
			expectedFile: "main.tf",
			want:         []string{`version = ">= 20.0.0"`, `source = "fastly/fastly"`},
		},
		{
			name: "implicit source",
			config: `terraform {
  required_providers {
    fastly = {
      version = "~> 9.0"
    }
  }
}
`,
			expectedFile: "main.tf",
			want:         []string{`version = ">= 20.0.0"`, `source = "fastly/fastly"`},
		},
		{
			name: "shorthand version",
			config: `terraform {
  required_providers {
    fastly = "~> 9.0"
  }
}
`,
			expectedFile: "main.tf",
			want:         []string{`source = "fastly/fastly"`, `version = ">= 20.0.0"`},
		},
		{
			name: "inline object with no version",
			config: `terraform {
  required_providers {
    fastly = { source = "fastly/fastly" }
  }
}
`,
			expectedFile: "main.tf",
			want:         []string{`source = "fastly/fastly"`, `version = ">= 20.0.0"`},
		},
		{
			name: "inline object with existing version",
			config: `terraform {
  required_providers {
    fastly = { source = "fastly/fastly", version = "~> 9.0" }
  }
}
`,
			expectedFile: "main.tf",
			want:         []string{`source = "fastly/fastly"`, `version = ">= 20.0.0"`},
		},
		{
			name: "HTTPS endpoint and credentials",
			config: `terraform {
  required_providers {
    fastly = {
      source  = "fastly/fastly"
      version = ">= 9.1.0, < 20.0.0"
    }
  }
}
provider "fastly" {
  alias    = "edge"
  api_key  = var.token
  base_url = "https://api.fastly.com"
}
`,
			expectedFile: "main.tf",
			want:         []string{`version = ">= 20.0.0"`, `api_token`, `api_endpoint = "https://api.fastly.com"`, `alias    = "edge"`},
		},
		{
			name: "existing GA minimum",
			config: `terraform {
  required_providers {
    fastly = {
      source = "fastly/fastly"
      version = ">= 20.0.0"
    }
  }
}
`,
			expectedFile: "main.tf",
			want:         []string{`version = ">= 20.0.0"`},
		},
		{
			name: "missing Fastly in existing required_providers",
			config: `terraform {
  required_providers {
    null = {
      source  = "hashicorp/null"
      version = "~> 3.0"
    }
  }
}
`,
			expectedFile: "main.tf",
			want:         []string{`source = "fastly/fastly"`, `version = ">= 20.0.0"`, `"hashicorp/null"`},
		},
		{
			name: "terraform block without required providers",
			config: `terraform {
  required_version = ">= 1.5.0"
}
`,
			expectedFile: "main.tf",
			want:         []string{`required_version = ">= 1.5.0"`, `required_providers {`, `source = "fastly/fastly"`},
		},
		{
			name:         "no terraform block",
			expectedFile: "migration.tf",
			want:         []string{`required_providers {`, `source  = "fastly/fastly"`, `version = ">= 20.0.0"`, `moved {`},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			source := tc.config + "\nresource \"fastly_service_vcl\" \"demo\" {\n  name = \"demo\"\n}\n"
			writeConfig(t, dir, "main.tf", source)
			result := newPlan(t, dir)
			if result.Blocked() {
				t.Fatalf("blocked: %#v findings=%#v", result.Issues, result.Analysis.Findings)
			}
			after := changedFile(t, result, tc.expectedFile)
			for _, fragment := range append(tc.want, tc.preserved...) {
				if !strings.Contains(after, fragment) {
					t.Errorf("missing %q in %s:\n%s", fragment, tc.expectedFile, after)
				}
			}
			if strings.Contains(after, `version = "~> 9.0"`) {
				t.Errorf("old provider version remains:\n%s", after)
			}
			if got, err := os.ReadFile(filepath.Join(dir, "main.tf")); err != nil || string(got) != source {
				t.Fatalf("dry-run changed configuration: %v", err)
			}
			for _, change := range result.Changes {
				writeConfig(t, dir, change.Path, string(change.After))
			}
			repeat := newPlan(t, dir)
			if repeat.Blocked() || len(repeat.Changes) != 0 || len(repeat.Moves) != 0 {
				t.Fatalf("non-idempotent after write: issues=%#v changes=%#v", repeat.Issues, repeat.Changes)
			}
		})
	}
}

func TestProviderMigrationBlocksUnsafeConfigurationsWithoutDiff(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, input, diagnostic string }{
		{"unsupported force_http2", `provider "fastly" {
  force_http2 = true
}`, "force_http2"},
		{"unsupported no_auth", `provider "fastly" {
  no_auth = false
}`, "no_auth"},
		{"dynamic base url", `provider "fastly" {
  base_url = var.endpoint
}`, "base_url cannot be proven"},
		{"http base url", `provider "fastly" {
  base_url = "http://example.com"
}`, "base_url cannot be proven"},
		{"collision API key", `provider "fastly" {
  api_key = var.old
  api_token = var.new
}`, "both api_key and api_token"},
		{"collision endpoint", `provider "fastly" {
  base_url = "https://example.com"
  api_endpoint = "https://example.com"
}`, "both base_url and api_endpoint"},
		{"unsupported provider attr", `provider "fastly" {
  insecure = true
}`, "insecure"},
		{"beta source", `terraform {
  required_providers {
    fastly = {
      source = "fastly/fastly-beta"
      version = ">= 0.2.8"
    }
  }
}`, "not the legacy"},
		{"explicit exclusion", `terraform {
  required_providers {
    fastly = {
      source = "fastly/fastly"
      version = "!= 20.0.0"
    }
  }
}`, "exclusions require manual review"},
		{"contradictory constraints", `terraform {
  required_providers {
    fastly = {
      source = "fastly/fastly"
      version = "~> 9.0, >= 20.0.0"
    }
  }
}`, "contradictory"},
		{"constraint below source floor", `terraform {
  required_providers {
    fastly = {
      source = "fastly/fastly"
      version = "~> 9.0.0"
    }
  }
}`, "incompatible with the supported release ranges"},
		{"GA only with special cap", `terraform {
  required_providers {
    fastly = {
      source = "fastly/fastly"
      version = "~> 20.0"
    }
  }
}`, "additional user restrictions"},
		{"local name beta", `terraform {
  required_providers {
    fastly-beta = {
      source = "fastly/fastly"
      version = "~> 9.0"
    }
  }
}`, "local name"},
		{"provider source conflicting", `terraform {
  required_providers {
    fastly = {
      source = "hashicorp/fastly"
      version = "~> 9.0"
    }
  }
}`, "not the legacy"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			source := tc.input + "\nresource \"fastly_service_vcl\" \"demo\" { name = \"demo\" }\n"
			writeConfig(t, dir, "main.tf", source)
			result := newPlan(t, dir)
			if !result.Blocked() || len(result.Changes) != 0 || len(result.Moves) != 0 {
				t.Fatalf("unsafe configuration produced preview: %#v", result)
			}
			var messages []string
			for _, issue := range result.Issues {
				messages = append(messages, issue.Message)
			}
			if !strings.Contains(strings.Join(messages, " "), tc.diagnostic) {
				t.Fatalf("missing %q in issues %#v (findings %#v)", tc.diagnostic, result.Issues, result.Analysis.Findings)
			}
			if sourceAfter, err := os.ReadFile(filepath.Join(dir, "main.tf")); err != nil || !bytes.Equal(sourceAfter, []byte(source)) {
				t.Fatalf("blocked plan modified main.tf: %v", err)
			}
			if _, err := os.Stat(filepath.Join(dir, "migration.tf")); !os.IsNotExist(err) {
				t.Fatalf("blocked preview created migration.tf: %v", err)
			}
		})
	}
}

func TestProviderMigrationInRootAndLocalChildModule(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", `terraform {
  required_providers {
    fastly = {
      source = "fastly/fastly"
      version = "~> 9.0"
      configuration_aliases = [fastly.edge]
    }
  }
}
provider "fastly" {
  alias = "edge"
  api_key = var.token
}
module "edge" {
  source = "./modules/edge"
  providers = {
    fastly = fastly.edge
  }
}
`)
	writeConfig(t, dir, "modules/edge/main.tf", `terraform {
  required_providers {
    fastly = {
      source = "fastly/fastly"
      version = "~> 9.0"
    }
  }
}
resource "fastly_service_vcl" "demo" {
  name = "demo"
}
`)
	result := newPlan(t, dir)
	if result.Blocked() {
		t.Fatalf("blocked: issues=%#v findings=%#v", result.Issues, result.Analysis.Findings)
	}
	root := changedFile(t, result, "main.tf")
	child := changedFile(t, result, "modules/edge/main.tf")
	for _, want := range []string{`version = ">= 20.0.0"`, `api_token = var.token`, `fastly = fastly.edge`, `configuration_aliases = [fastly.edge]`} {
		if !strings.Contains(root, want) {
			t.Errorf("missing %q in root:\n%s", want, root)
		}
	}
	for _, want := range []string{`version = ">= 20.0.0"`, `fastly_service_cdn_auto`} {
		if !strings.Contains(child, want) {
			t.Errorf("missing %q in child:\n%s", want, child)
		}
	}
	moved := changedFile(t, result, "modules/edge/migration.tf")
	if !strings.Contains(moved, "moved {") {
		t.Fatalf("child moved block missing: %s", moved)
	}
	for _, change := range result.Changes {
		writeConfig(t, dir, change.Path, string(change.After))
	}
	repeat := newPlan(t, dir)
	if repeat.Blocked() || len(repeat.Changes) != 0 {
		t.Fatalf("rerun not idempotent: issues=%#v changes=%#v", repeat.Issues, repeat.Changes)
	}
}

func TestProviderMigrationDetectsAmbiguousRequirements(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, main, extra, want string }{
		{
			name: "duplicate Fastly provider requirement",
			main: `terraform {
  required_providers {
    fastly = {
      source = "fastly/fastly"
      version = "~> 9.0"
    }
  }
}
`,
			extra: `terraform {
  required_providers {
    fastly = {
      source = "fastly/fastly"
      version = ">= 9.1.0"
    }
  }
}
`,
			want: "multiple required_providers blocks",
		},
		{
			name: "two required_providers blocks without Fastly",
			main: `terraform {
  required_providers {
    null = { source = "hashicorp/null" }
  }
}
`,
			extra: `terraform {
  required_providers {
    random = { source = "hashicorp/random" }
  }
}
`,
			want: "multiple required_providers blocks",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeConfig(t, dir, "main.tf", tc.main+`resource "fastly_service_vcl" "demo" {
  name = "demo"
}
`)
			writeConfig(t, dir, "other.tf", tc.extra)
			result := newPlan(t, dir)
			if !result.Blocked() || len(result.Changes) != 0 {
				t.Fatalf("ambiguous requirements produced changes: %#v", result)
			}
			var matching *Issue
			for i := range result.Issues {
				if strings.Contains(result.Issues[i].Message, tc.want) {
					matching = &result.Issues[i]
					break
				}
			}
			if matching == nil {
				t.Fatalf("missing diagnostic %q: %#v", tc.want, result.Issues)
			}
			if matching.File != "other.tf" {
				t.Fatalf("issue file = %q, want second conflicting declaration other.tf", matching.File)
			}
		})
	}
}

func TestProviderMigrationRejectsDuplicateAliases(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", `provider "fastly" {
  alias   = "edge"
  api_key = var.token
}
resource "fastly_service_vcl" "demo" {
  provider = fastly.edge
  name = "demo"
}
`)
	writeConfig(t, dir, "other.tf", `provider "fastly" {
  alias = "edge"
}
`)
	result := newPlan(t, dir)
	if !result.Blocked() || len(result.Changes) != 0 {
		t.Fatalf("duplicate aliases produced changes: %#v", result)
	}
	var combined string
	for _, issue := range result.Issues {
		combined += issue.Message
	}
	if !strings.Contains(combined, `duplicate Fastly provider alias "edge"`) {
		t.Fatalf("missing duplicate alias diagnostic: %#v", result.Issues)
	}
}

func TestProviderMigrationAddsMissingRequirementAndRenamesConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", `provider "fastly" {
  alias = "east"
  api_key = var.old_token
}
resource "fastly_service_vcl" "demo" {
  provider = fastly.east
  name = "demo"
}
`)
	result := newPlan(t, dir)
	if result.Blocked() {
		t.Fatalf("blocked: %#v", result.Issues)
	}
	main := changedFile(t, result, "main.tf")
	generated := changedFile(t, result, "migration.tf")
	if !strings.Contains(main, `api_token = var.old_token`) ||
		!strings.Contains(main, `provider = fastly.east`) ||
		!strings.Contains(generated, `version = ">= 20.0.0"`) ||
		!strings.Contains(generated, `source  = "fastly/fastly"`) {
		t.Fatalf("incorrect provider migration:\n%s\n%s", main, generated)
	}
}

func TestProviderMigrationNeverTouchesAlreadyMigratedProjects(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", `terraform {
  required_providers {
    fastly = {
      source = "fastly/fastly"
      version = ">= 20.0.0"
    }
  }
}
provider "fastly" {
  api_token = var.token
}
resource "fastly_service_cdn_auto" "demo" {
  name = "demo"
}
`)
	result := newPlan(t, dir)
	if result.Blocked() || len(result.Changes) != 0 {
		t.Fatalf("already migrated config changed: %#v", result)
	}
}
