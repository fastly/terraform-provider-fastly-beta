package rewrite

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func fixtureEdits() []Edit {
	return []Edit{
		RenameResource{File: "main.tf", FromType: "fastly_service_vcl", ToType: "fastly_service_cdn_auto", Name: "demo"},
		RenameBlock{File: "main.tf", ResourceType: "fastly_service_cdn_auto", ResourceName: "demo", From: "backend", To: "origin"},
		RenameAttribute{File: "main.tf", ResourceType: "fastly_service_cdn_auto", ResourceName: "demo", Parents: []string{"origin"}, From: "address", To: "hostname"},
		NestAttribute{File: "main.tf", ResourceType: "fastly_service_cdn_auto", ResourceName: "demo", Attribute: "version_note", TargetBlock: "settings", TargetAttribute: "note"},
		AddMoved{File: "migration.tf", From: "fastly_service_vcl.demo", To: "fastly_service_cdn_auto.demo"},
		AddImport{File: "migration.tf", To: "fastly_service_product_domain_inspector.demo", IDExpression: "fastly_service_cdn_auto.demo.id"},
	}
}

func readFixture(t *testing.T, filename string) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", "basic", filename))
	if err != nil {
		t.Fatal(err)
	}
	return content
}

func writeMainFixtureWithGroupRead(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, "main.tf")
	if err := os.WriteFile(path, readFixture(t, "before/main.tf"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Test preservation of existing group-readable permissions without
	// creating the file with overly broad initial permissions.
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
}

func TestBuildGoldenPreservesUnrelatedHCL(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeMainFixtureWithGroupRead(t, dir)
	plan, err := Build(context.Background(), dir, fixtureEdits()...)
	if err != nil {
		t.Fatal(err)
	}
	changes := plan.Changes()
	if got, want := len(changes), 2; got != want {
		t.Fatalf("changes=%d want %d", got, want)
	}
	if changes[0].Path != "main.tf" || changes[1].Path != "migration.tf" {
		t.Fatalf("unsorted changes: %#v", changes)
	}
	if changes[0].Mode != 0o640 {
		t.Fatalf("mode=%v want 0640", changes[0].Mode)
	}
	if changes[1].Exists {
		t.Fatal("generated migration file already exists")
	}
	if got, want := changes[0].After, readFixture(t, "after/main.tf"); !bytes.Equal(got, want) {
		t.Fatalf("unexpected main.tf rewrite:\nGOT:\n%s\nWANT:\n%s", got, want)
	}
	if got, want := changes[1].After, readFixture(t, "after/migration.tf"); !bytes.Equal(got, want) {
		t.Fatalf("unexpected migration.tf rewrite:\nGOT:\n%s\nWANT:\n%s", got, want)
	}
	current, err := os.ReadFile(filepath.Join(dir, "main.tf"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(current, readFixture(t, "before/main.tf")) {
		t.Fatal("Build unexpectedly wrote a source file")
	}
	// Plans do not expose mutable internal data through Changes().
	changes[0].After[0] = 'X'
	if bytes.Equal(plan.Changes()[0].After, changes[0].After) {
		t.Fatal("plan contents are aliased")
	}
}

func TestApplyAndSecondBuildAreIdempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeMainFixtureWithGroupRead(t, dir)
	plan, err := Build(context.Background(), dir, fixtureEdits()...)
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"main.tf", "migration.tf"} {
		got, err := os.ReadFile(filepath.Join(dir, path))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, readFixture(t, "after/"+path)) {
			t.Fatalf("wrong %s:\n%s", path, got)
		}
	}
	info, err := os.Stat(filepath.Join(dir, "main.tf"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("existing mode changed to %v", info.Mode())
	}
	repeat, err := Build(context.Background(), dir, fixtureEdits()...)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(repeat.Changes()); got != 0 {
		t.Fatalf("second run produced %d changes", got)
	}
	if err := repeat.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// TestApplyReplacesExistingFile exercises replacing an existing Terraform file,
// including on Windows where os.Rename uses MoveFileEx replacement semantics.
func TestApplyReplacesExistingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "main.tf")
	before := []byte(`resource "fastly_service_vcl" "demo" {}` + "\n")
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := Build(context.Background(), dir, RenameResource{
		File: "main.tf", FromType: "fastly_service_vcl", ToType: "fastly_service_cdn_auto", Name: "demo",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(context.Background()); err != nil {
		t.Fatalf("Apply could not replace existing file: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte(`resource "fastly_service_cdn_auto" "demo"`)) ||
		bytes.Contains(got, []byte(`resource "fastly_service_vcl" "demo"`)) {
		t.Fatalf("existing file was not replaced: %s", got)
	}
}

func TestBuildMultiFileDeterministicOrder(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "modules", "service"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := []byte("resource \"fastly_service_vcl\" \"demo\" {\n  name = var.name\n}\n")
	for _, path := range []string{"z.tf", "modules/service/main.tf"} {
		if err := os.WriteFile(filepath.Join(dir, path), config, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// Opposite operation order must not affect deterministic change ordering.
	edits := []Edit{
		RenameResource{File: "z.tf", FromType: "fastly_service_vcl", ToType: "fastly_service_cdn_auto", Name: "demo"},
		RenameResource{File: "modules/service/main.tf", FromType: "fastly_service_vcl", ToType: "fastly_service_cdn_auto", Name: "demo"},
	}
	plan, err := Build(context.Background(), dir, edits...)
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{}
	for _, change := range plan.Changes() {
		paths = append(paths, change.Path)
	}
	if !slices.Equal(paths, []string{"modules/service/main.tf", "z.tf"}) {
		t.Fatalf("paths=%#v", paths)
	}
}

func TestConflictAndMalformedHCLLeaveFilesUntouched(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		source string
		edits  []Edit
	}{
		{
			name:   "rename collision",
			source: "resource \"fastly_service_vcl\" \"demo\" {}\nresource \"fastly_service_cdn_auto\" \"demo\" {}\n",
			edits:  []Edit{RenameResource{File: "main.tf", FromType: "fastly_service_vcl", ToType: "fastly_service_cdn_auto", Name: "demo"}},
		},
		{
			name:   "late edit fails",
			source: "resource \"fastly_service_vcl\" \"demo\" {}\n",
			edits: []Edit{
				RenameResource{File: "main.tf", FromType: "fastly_service_vcl", ToType: "fastly_service_cdn_auto", Name: "demo"},
				RenameAttribute{File: "main.tf", ResourceType: "fastly_service_cdn_auto", ResourceName: "demo", From: "does_not_exist", To: "changed"},
			},
		},
		{
			name:   "malformed input",
			source: "resource \"fastly_service_vcl\" \"demo\" {\n",
			edits:  []Edit{RenameResource{File: "main.tf", FromType: "fastly_service_vcl", ToType: "fastly_service_cdn_auto", Name: "demo"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			original := []byte(tc.source)
			if err := os.WriteFile(filepath.Join(dir, "main.tf"), original, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Build(context.Background(), dir, tc.edits...); err == nil {
				t.Fatal("Build succeeded unexpectedly")
			}
			after, err := os.ReadFile(filepath.Join(dir, "main.tf"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(original, after) {
				t.Fatal("failed plan modified source file")
			}
		})
	}
}

func TestApplyDetectsModifiedSource(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "main.tf")
	original := []byte("resource \"fastly_service_vcl\" \"demo\" {}\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := Build(context.Background(), dir,
		RenameResource{File: "main.tf", FromType: "fastly_service_vcl", ToType: "fastly_service_cdn_auto", Name: "demo"},
	)
	if err != nil {
		t.Fatal(err)
	}
	changed := []byte("# concurrent edit\n" + string(original))
	if err := os.WriteFile(path, changed, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(context.Background()); err == nil || !strings.Contains(err.Error(), "changed since planning") {
		t.Fatalf("expected changed-input error, got %v", err)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(current, changed) {
		t.Fatal("Apply overwrote concurrent edit")
	}
}

func TestPathTraversalAndSymlinkDenied(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "outside.tf"), []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(context.Background(), dir, AddMoved{File: "../outside.tf", From: "x.a", To: "x.b"}); err == nil {
		t.Fatal("parent traversal permitted")
	}
	if err := os.Symlink(outside, filepath.Join(dir, "linked")); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(context.Background(), dir, AddMoved{File: "linked/outside.tf", From: "x.a", To: "x.b"}); err == nil {
		t.Fatal("symlink escape permitted")
	}
}

func TestInvalidAddressOrImportExpressionFailsWithoutGeneratedFiles(t *testing.T) {
	t.Parallel()
	tests := []Edit{
		AddMoved{File: "migration.tf", From: "aws_instance.x; destroy", To: "fastly_service_cdn_auto.demo"},
		AddImport{File: "migration.tf", To: "fastly_service_product_domain_inspector.demo", IDExpression: "foo }\nresource \"bad\" \"x\" {}"},
		AddImport{File: "migration.tf", To: "fastly_service_product_domain_inspector.demo", IDExpression: ""},
	}
	for _, edit := range tests {
		dir := t.TempDir()
		_, err := Build(context.Background(), dir, edit)
		if err == nil {
			t.Fatalf("expected failure for %#v", edit)
		}
		if _, statErr := os.Stat(filepath.Join(dir, "migration.tf")); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("unexpected generated file: %v", statErr)
		}
	}
}

func TestExistingConflictingMoveAndImportRejected(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	source := []byte(`moved {
  from = fastly_service_vcl.demo
  to = fastly_service_cdn_auto.demo
}

import {
  to = fastly_service_product_domain_inspector.demo
  id = "existing-id"
}
`)
	if err := os.WriteFile(filepath.Join(dir, "migration.tf"), source, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, edit := range []Edit{
		AddMoved{File: "migration.tf", From: "fastly_service_vcl.demo", To: "fastly_service_compute_auto.demo"},
		AddImport{File: "migration.tf", To: "fastly_service_product_domain_inspector.demo", IDExpression: "\"different-id\""},
	} {
		if _, err := Build(context.Background(), dir, edit); err == nil {
			t.Fatalf("conflict accepted: %#v", edit)
		}
	}
}

func TestCanceledContextDoesNotWrite(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Build(ctx, t.TempDir(), AddMoved{File: "migration.tf", From: "x.a", To: "x.b"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Build canceled error=%v", err)
	}
}

func TestNestAttributeRejectsHeredoc(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	source := []byte("resource \"fastly_service_vcl\" \"demo\" {\n  note = <<EOT\nhello\nEOT\n}\n")
	if err := os.WriteFile(filepath.Join(dir, "main.tf"), source, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Build(context.Background(), dir, NestAttribute{
		File: "main.tf", ResourceType: "fastly_service_vcl", ResourceName: "demo",
		Attribute: "note", TargetBlock: "settings",
	})
	if err == nil {
		t.Fatal("expected heredoc preservation guard")
	}
}

func TestNestAttributeIntoExistingBlock(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	source := []byte(`resource "fastly_service_vcl" "demo" {
  note = local.note

  settings {
    enabled = var.enabled
  }
}
`)
	expected := []byte(`resource "fastly_service_vcl" "demo" {

  settings {
    enabled = var.enabled
    note = local.note
  }
}
`)
	if err := os.WriteFile(filepath.Join(dir, "main.tf"), source, 0o600); err != nil {
		t.Fatal(err)
	}
	edit := NestAttribute{
		File: "main.tf", ResourceType: "fastly_service_vcl", ResourceName: "demo",
		Attribute: "note", TargetBlock: "settings",
	}
	plan, err := Build(context.Background(), dir, edit)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(plan.Changes()[0].After, expected) {
		t.Fatalf("got:\n%s\nwant:\n%s", plan.Changes()[0].After, expected)
	}
}

func TestNestAttributeUnderNestedParent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	before := []byte(`resource "fastly_service_vcl" "demo" {
  backend {
    # Retain the comment with the moved expression.
    version_note = local.note
    address = var.origin
  }
}
`)
	want := []byte(`resource "fastly_service_vcl" "demo" {
  backend {
    address = var.origin
    settings {
      # Retain the comment with the moved expression.
      note = local.note
    }
  }
}
`)
	if err := os.WriteFile(filepath.Join(dir, "main.tf"), before, 0o600); err != nil {
		t.Fatal(err)
	}
	edit := NestAttribute{
		File: "main.tf", ResourceType: "fastly_service_vcl", ResourceName: "demo",
		Parents: []string{"backend"}, Attribute: "version_note",
		TargetBlock: "settings", TargetAttribute: "note",
	}
	plan, err := Build(context.Background(), dir, edit)
	if err != nil {
		t.Fatal(err)
	}
	if got := plan.Changes(); len(got) != 1 || !bytes.Equal(got[0].After, want) {
		t.Fatalf("nested attribute migration was not source-preserving: %#v", got)
	}
	if err := plan.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	repeat, err := Build(context.Background(), dir, edit)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(repeat.Changes()); got != 0 {
		t.Fatalf("second nested attribute migration produced %d changes", got)
	}
}

func TestNestAttributeRejectsInlineClosingBrace(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	before := []byte(`resource "fastly_service_vcl" "demo" {
  note = local.note
  settings {}
}
`)
	path := filepath.Join(dir, "main.tf")
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Build(context.Background(), dir, NestAttribute{
		File: "main.tf", ResourceType: "fastly_service_vcl", ResourceName: "demo",
		Attribute: "note", TargetBlock: "settings",
	})
	if err == nil || !strings.Contains(err.Error(), "inline closing braces") {
		t.Fatalf("expected inline-brace safety guard, got %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("failed rewrite changed the source file")
	}
}

func TestRenameRepeatedNestedBlocks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	source := []byte(`resource "fastly_service_vcl" "demo" {
  backend {
    address = var.first
  }
  backend {
    address = local.second
  }
}
`)
	if err := os.WriteFile(filepath.Join(dir, "main.tf"), source, 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := Build(context.Background(), dir,
		RenameAttribute{File: "main.tf", ResourceType: "fastly_service_vcl", ResourceName: "demo", Parents: []string{"backend"}, From: "address", To: "hostname"},
	)
	if err != nil {
		t.Fatal(err)
	}
	after := string(plan.Changes()[0].After)
	if strings.Count(after, "hostname = ") != 2 ||
		!strings.Contains(after, "var.first") ||
		!strings.Contains(after, "local.second") {
		t.Fatalf("repeated blocks were not renamed correctly:\n%s", after)
	}
}

func TestApplyDetectsChangedPermissions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "main.tf")
	if err := os.WriteFile(path, []byte("resource \"old\" \"demo\" {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := Build(context.Background(), dir, RenameResource{
		File: "main.tf", FromType: "old", ToType: "new", Name: "demo",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(context.Background()); err == nil {
		t.Fatal("Apply replaced a file whose permissions changed after planning")
	}
}
