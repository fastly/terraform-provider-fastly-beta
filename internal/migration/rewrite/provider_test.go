package rewrite

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateProviderRequirementPreservesSourceAndComments(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	before := `terraform {
  required_providers {
    fastly = {
      source                = "fastly/fastly"
      version               = "~> 9.0" # keep the comment
      configuration_aliases = [fastly.edge]
    }
    null = {
      source = "hashicorp/null"
      version = "~> 3.0"
    }
  }
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.tf"), []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := Build(context.Background(), dir, UpdateProviderRequirement{
		File: "main.tf", LocalName: "fastly",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Changes()) != 1 {
		t.Fatalf("want one change: %#v", plan.Changes())
	}
	after := string(plan.Changes()[0].After)
	for _, expected := range []string{
		`version               = ">= 20.0.0" # keep the comment`,
		`source                = "fastly/fastly"`,
		`configuration_aliases = [fastly.edge]`,
		`version = "~> 3.0"`,
	} {
		if !strings.Contains(after, expected) {
			t.Fatalf("lost %q:\n%s", expected, after)
		}
	}
}

func TestEnsureProviderRequirementInGeneratedModule(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	plan, err := Build(context.Background(), dir,
		EnsureProviderRequirement{File: "migration.tf"},
		AddMoved{File: "migration.tf", From: "fastly_service_vcl.demo", To: "fastly_service_cdn_auto.demo"},
	)
	if err != nil {
		t.Fatal(err)
	}
	changes := plan.Changes()
	if len(changes) != 1 || changes[0].Exists {
		t.Fatalf("expected new migration.tf with two declarations: %#v", changes)
	}
	result := string(changes[0].After)
	for _, expected := range []string{`source  = "fastly/fastly"`, `version = ">= 20.0.0"`, `moved {`} {
		if !strings.Contains(result, expected) {
			t.Errorf("missing %q:\n%s", expected, result)
		}
	}
}

func TestRenameProviderArgumentsPreservesAliasesAndExpressions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	before := `provider "fastly" {
  alias = "edge"
  api_key = var.legacy_api_key # keep
  base_url = "https://api.fastly.com"
}

provider "fastly" {
  api_key = var.another
}

provider "aws" {
  region = "us-east-1"
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.tf"), []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := Build(context.Background(), dir, RenameProviderArguments{File: "main.tf", Name: "fastly"})
	if err != nil {
		t.Fatal(err)
	}
	after := string(plan.Changes()[0].After)
	for _, expected := range []string{
		`api_token = var.legacy_api_key # keep`,
		`api_endpoint = "https://api.fastly.com"`,
		`api_token = var.another`,
		`alias = "edge"`,
		`provider "aws"`,
	} {
		if !strings.Contains(after, expected) {
			t.Errorf("missing %q:\n%s", expected, after)
		}
	}
	if strings.Contains(after, "api_key =") || strings.Contains(after, "base_url =") {
		t.Fatalf("legacy provider attribute names remain:\n%s", after)
	}
}

func TestEnsureProviderRequirementReportsExistingDeclarationFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	const existing = `terraform {
  required_providers {
    fastly = {
      source  = "fastly/fastly"
      version = "~> 9.0"
    }
  }
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.tf"), []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Build(context.Background(), dir, EnsureProviderRequirement{File: "main.tf"})
	if err == nil || !strings.Contains(err.Error(), "main.tf: required_providers.fastly already exists") ||
		!strings.Contains(err.Error(), "review the existing declaration") {
		t.Fatalf("expected contextual existing-requirement error, got %v", err)
	}
}
