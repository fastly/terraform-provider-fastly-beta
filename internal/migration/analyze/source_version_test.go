package analyze

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const vclService = `
resource "fastly_service_vcl" "demo" {
  name = "demo"
}
`

func lockfileFor(version string) string {
	return `provider "registry.terraform.io/fastly/fastly" {
  version = "` + version + `"
}
`
}

func sourceVersionFindings(result Result) []Finding {
	var found []Finding
	for _, finding := range result.Findings {
		if finding.RuleID == "resource-service-vcl" && strings.Contains(finding.Message, "fastly/fastly provider") {
			found = append(found, finding)
		}
	}
	return found
}

func TestSourceProviderVersionBoundary(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		version string
		blocked bool
	}{
		{"8.8.0", true},
		{"9.0.0", true},
		{"9.1.0", false},
		{"9.8.0", false},
		{"10.1.0", false},
		{"19.9.9", false},
		{"20.0.0", true},
		{"20.1.0", true},
	} {
		t.Run(tt.version, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeFile(t, dir, "main.tf", vclService)
			writeFile(t, dir, ".terraform.lock.hcl", lockfileFor(tt.version))

			result := analyzeDir(t, dir)
			if got := len(sourceVersionFindings(result)) != 0; got != tt.blocked {
				t.Fatalf("blocked = %t, want %t: %#v", got, tt.blocked, result.Findings)
			}
			if result.Migratable() == tt.blocked {
				t.Fatalf("Migratable() = %t for %s", result.Migratable(), tt.version)
			}
		})
	}
}

func TestSourceProviderVersionLockfileProblems(t *testing.T) {
	t.Parallel()

	for name, lock := range map[string]*string{
		"missing":       nil,
		"malformed":     ptr(`provider "registry.terraform.io/fastly/fastly" {`),
		"other-only":    ptr(`provider "registry.terraform.io/hashicorp/null" { version = "3.2.0" }`),
		"no-version":    ptr(`provider "registry.terraform.io/fastly/fastly" {}`),
		"bad-version":   ptr(lockfileFor("not-a-version")),
		"beta-only":     ptr(`provider "registry.terraform.io/fastly/fastly-beta" { version = "0.1.0" }`),
		"unknown-value": ptr(`provider "registry.terraform.io/fastly/fastly" { version = var.v }`),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeFile(t, dir, "main.tf", vclService)
			os.Remove(filepath.Join(dir, lockfileName))
			if lock != nil {
				writeFile(t, dir, lockfileName, *lock)
			}

			findings := sourceVersionFindings(analyzeDir(t, dir))
			if len(findings) != 1 || findings[0].Severity != SeverityError {
				t.Fatalf("findings = %#v, want one blocking prerequisite", findings)
			}
			if !strings.Contains(findings[0].Remediation, "terraform init") ||
				!strings.Contains(findings[0].Remediation, "-upgrade") {
				t.Fatalf("remediation = %q", findings[0].Remediation)
			}
		})
	}
}

func TestSourceProviderVersionConstraintIsNotEvidence(t *testing.T) {
	t.Parallel()

	const requirement = `
terraform {
  required_providers {
    fastly = {
      source  = "fastly/fastly"
      version = "~> 9.0"
    }
  }
}
`
	dir := t.TempDir()
	writeFile(t, dir, "main.tf", requirement+vclService)
	os.Remove(filepath.Join(dir, lockfileName))
	if got := len(sourceVersionFindings(analyzeDir(t, dir))); got != 1 {
		t.Fatalf("~> 9.0 without a lockfile produced %d prerequisite findings, want 1", got)
	}

	writeFile(t, dir, lockfileName, lockfileFor("9.0.0"))
	if got := len(sourceVersionFindings(analyzeDir(t, dir))); got != 1 {
		t.Fatalf("~> 9.0 with a 9.0.0 lockfile produced %d findings, want 1", got)
	}

	writeFile(t, dir, lockfileName, lockfileFor("9.3.1"))
	if got := len(sourceVersionFindings(analyzeDir(t, dir))); got != 0 {
		t.Fatalf("~> 9.0 with a 9.3.1 lockfile produced %d findings, want 0", got)
	}
}

func TestSourceProviderVersionConstraintConflictRemediation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "main.tf", `
terraform {
  required_providers {
    fastly = {
      source  = "fastly/fastly"
      version = "~> 9.0.0"
    }
  }
}
`+vclService)
	writeFile(t, dir, lockfileName, lockfileFor("9.0.0"))

	findings := sourceVersionFindings(analyzeDir(t, dir))
	if len(findings) != 1 || !strings.Contains(findings[0].Remediation, `constraint "~> 9.0.0" excludes 9.1.0`) {
		t.Fatalf("findings = %#v, want constraint conflict remediation", findings)
	}
}

func TestSourceProviderVersionSkipsProjectsWithoutLegacyServices(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "main.tf", `
resource "fastly_service_cdn_auto" "demo" {
  name = "demo"
}
`)
	os.Remove(filepath.Join(dir, lockfileName))

	result := analyzeDir(t, dir)
	if got := sourceVersionFindings(result); len(got) != 0 {
		t.Fatalf("already-migrated project produced %#v", got)
	}
}

func TestSourceProviderVersionDoesNotHideConfigurationFindings(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, dir, "main.tf", vclService+`
resource "fastly_unknown_thing" "x" {}
`)
	os.Remove(filepath.Join(dir, lockfileName))

	result := analyzeDir(t, dir)
	if len(sourceVersionFindings(result)) != 1 ||
		!containsFinding(result, `No compatibility rule exists for Fastly resource type "fastly_unknown_thing"`) {
		t.Fatalf("findings = %#v, want both prerequisite and configuration findings", result.Findings)
	}
}

func ptr(s string) *string { return &s }
