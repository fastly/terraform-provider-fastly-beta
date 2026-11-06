package rewrite

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoveAttributePreservesUnrelatedHCLAndIsIdempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	before := []byte(`resource "fastly_service_vcl" "demo" {
  name = var.name
  # Remove this attached comment along with the attribute.
  activate = true # trailing comment

  backend {
    name = "origin"
    address = var.address
  }
}
`)
	want := []byte(`resource "fastly_service_vcl" "demo" {
  name = var.name

  backend {
    name = "origin"
    address = var.address
  }
}
`)
	path := filepath.Join(dir, "main.tf")
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}
	edit := RemoveAttribute{
		File: "main.tf", ResourceType: "fastly_service_vcl",
		ResourceName: "demo", Attribute: "activate",
	}
	plan, err := Build(context.Background(), dir, edit)
	if err != nil {
		t.Fatal(err)
	}
	if got := plan.Changes(); len(got) != 1 || !bytes.Equal(got[0].After, want) {
		t.Fatalf("unexpected removal: %#v", got)
	}
	if err := plan.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	second, err := Build(context.Background(), dir, edit)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Changes()) != 0 {
		t.Fatalf("removal is not idempotent: %#v", second.Changes())
	}
}

func TestRemoveAttributeRejectsUnsafeLayoutWithoutWriting(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	source := []byte(`resource "fastly_service_vcl" "demo" {
  version_comment = <<EOT
hello
EOT
}
`)
	path := filepath.Join(dir, "main.tf")
	if err := os.WriteFile(path, source, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Build(context.Background(), dir, RemoveAttribute{
		File: "main.tf", ResourceType: "fastly_service_vcl",
		ResourceName: "demo", Attribute: "version_comment",
	})
	if err == nil || !strings.Contains(err.Error(), "heredoc") {
		t.Fatalf("expected safe refusal to remove heredoc, got %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, source) {
		t.Fatal("failed rewrite changed source")
	}
}

func TestRemoveAttributeMissingParentPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	source := []byte(`resource "fastly_service_vcl" "demo" {
  name = "demo"
}
`)
	path := filepath.Join(dir, "main.tf")
	if err := os.WriteFile(path, source, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Build(context.Background(), dir, RemoveAttribute{
		File:         "main.tf",
		ResourceType: "fastly_service_vcl",
		ResourceName: "demo",
		Parents:      []string{"backend", "settings"},
		Attribute:    "activate",
	})
	if err == nil || !strings.Contains(err.Error(), `attribute parent path "backend.settings" not found`) {
		t.Fatalf("expected readable missing-parent path, got %v", err)
	}
	actual, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, source) {
		t.Fatal("failed edit modified original source")
	}
}
