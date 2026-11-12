package migrate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/compatibility"
)

const simpleVCLConfiguration = "resource \"fastly_service_vcl\" \"demo\" {\n  name = \"demo\"\n}\n"

// privateBackupDir makes backup root permissions explicit. Test temporary
// directory permissions can vary across Go versions and host umasks; real
// migrations must still reject an existing group/world-accessible directory.
func privateBackupDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatalf("make backup directory private: %v", err)
	}
	return dir
}

func TestApplyGoldenBackupsAndIdempotency(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	backups := privateBackupDir(t)
	before := fixture(t, "before/main.tf")
	writeConfig(t, dir, "main.tf", string(before))

	result := newPlan(t, dir)
	if result.Blocked() || len(result.Changes) != 2 {
		t.Fatalf("unexpected plan: %#v", result)
	}
	applied, err := result.Apply(context.Background(), backups)
	if err != nil {
		t.Fatal(err)
	}
	if applied.FilesWritten != 2 || applied.BackupDir == "" {
		t.Fatalf("applied = %#v", applied)
	}
	if isWithinPath(dir, applied.BackupDir) {
		t.Fatal("backup saved inside Terraform configuration")
	}
	for _, name := range []string{"main.tf", "migration.tf"} {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if want := fixture(t, "after/"+name); !bytes.Equal(got, want) {
			t.Fatalf("%s after apply:\n%s\nwant:\n%s", name, got, want)
		}
	}
	original, err := os.ReadFile(filepath.Join(applied.BackupDir, "originals", "main.tf"))
	if err != nil || !bytes.Equal(original, before) {
		t.Fatalf("recoverable original not saved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(applied.BackupDir, "originals", "migration.tf")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("new file should be recorded, not backed up: %v", err)
	}
	if runtime.GOOS != "windows" {
		for _, name := range []string{applied.BackupDir, filepath.Join(applied.BackupDir, "originals")} {
			info, err := os.Stat(name)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm()&0o077 != 0 {
				t.Fatalf("%s is accessible to other users: %v", name, info.Mode())
			}
		}
		info, err := os.Stat(filepath.Join(applied.BackupDir, "originals", "main.tf"))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("recovery copy mode = %v, want 0600", info.Mode())
		}
	}
	manifest := readManifest(t, applied.BackupDir)
	if manifest.Version != 1 || manifest.Status != backupApplied || manifest.Root != result.rewritePlan.Root() {
		t.Fatalf("unexpected recovery manifest: %#v", manifest)
	}
	if len(manifest.Files) != 2 || manifest.Files[0].Path != "main.tf" ||
		manifest.Files[1].Path != "migration.tf" ||
		!manifest.Files[0].Existed || manifest.Files[1].Existed ||
		manifest.Files[0].BeforeHash != sha256String(before) ||
		manifest.Files[0].AfterHash != sha256String(fixture(t, "after/main.tf")) {
		t.Fatalf("incorrect recovery file records: %#v", manifest.Files)
	}

	repeat := newPlan(t, dir)
	if repeat.Blocked() || len(repeat.Changes) != 0 {
		t.Fatalf("second planning run not idempotent: %#v", repeat)
	}
	noOp, err := repeat.Apply(context.Background(), backups)
	if err != nil {
		t.Fatal(err)
	}
	if noOp.FilesWritten != 0 || noOp.BackupDir != "" {
		t.Fatalf("second run wrote files: %#v", noOp)
	}
	entries, err := os.ReadDir(filepath.Dir(applied.BackupDir))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("second run created extra backup: %v", entries)
	}
}

func readManifest(t *testing.T, backupDir string) recoveryManifest {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(backupDir, backupManifestName))
	if err != nil {
		t.Fatal(err)
	}
	var manifest recoveryManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func TestApplyRequiresValidatedPreflightSnapshots(t *testing.T) {
	t.Parallel()
	dir, backups := t.TempDir(), privateBackupDir(t)
	writeConfig(t, dir, "main.tf", simpleVCLConfiguration)
	for _, tc := range []struct {
		name   string
		mutate func(*Result)
	}{
		{"nil snapshots", func(r *Result) { r.snapshots = nil }},
		{"missing analyzed file", func(r *Result) { delete(r.snapshots, "main.tf") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newPlan(t, dir)
			if r.Blocked() || r.rewritePlan == nil {
				t.Fatalf("expected supported plan: %#v", r)
			}
			tc.mutate(&r)
			if _, err := r.Apply(context.Background(), backups); err == nil ||
				!strings.Contains(err.Error(), "validated preflight snapshot") {
				t.Fatalf("missing snapshot accepted or incorrectly reported: %v", err)
			}
			if _, err := os.Stat(filepath.Join(backups, "backups")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("invalid preflight created backup data: %v", err)
			}
			got, err := os.ReadFile(filepath.Join(dir, "main.tf"))
			if err != nil || string(got) != simpleVCLConfiguration {
				t.Fatalf("invalid preflight changed source: %v", err)
			}
		})
	}
}

func TestApplyWritesNestedModuleInPlace(t *testing.T) {
	t.Parallel()
	dir, backups := t.TempDir(), privateBackupDir(t)
	writeConfig(t, dir, "main.tf", `module "service" {
  source = "./modules/service"
}
`)
	original := []byte(`resource "fastly_service_vcl" "nested" {
  name = "nested"
}
`)
	writeConfig(t, dir, "modules/service/main.tf", string(original))
	result := newPlan(t, dir)
	if result.Blocked() {
		t.Fatalf("blocked: %#v", result.Issues)
	}
	outcome, err := result.Apply(context.Background(), backups)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.FilesWritten != 2 {
		t.Fatalf("unexpected applied files: %#v", outcome)
	}
	content, err := os.ReadFile(filepath.Join(dir, "main.tf"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `source = "./modules/service"`) {
		t.Fatal("root module changed unexpectedly")
	}
	after, err := os.ReadFile(filepath.Join(dir, "modules/service/main.tf"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(after, []byte(`resource "fastly_service_cdn_auto" "nested"`)) {
		t.Fatalf("nested resource was not migrated: %s", after)
	}
	moved, err := os.ReadFile(filepath.Join(dir, "modules/service/migration.tf"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(moved, []byte(`from = fastly_service_vcl.nested`)) {
		t.Fatalf("child-module moved block missing: %s", moved)
	}
	backup, err := os.ReadFile(filepath.Join(outcome.BackupDir, "originals", "modules/service/main.tf"))
	if err != nil || !bytes.Equal(backup, original) {
		t.Fatalf("child module backup missing: %v", err)
	}
}

func TestApplyRejectsBlockedAndForgedResults(t *testing.T) {
	t.Parallel()
	dir, backups := t.TempDir(), privateBackupDir(t)
	source := `resource "fastly_service_vcl" "demo" {
  name = "demo"
  product_enablement {
    domain_inspector = var.enabled
  }
}
`
	writeConfig(t, dir, "main.tf", source)
	result := newPlan(t, dir)
	if !result.Blocked() {
		t.Fatal("expected unknown product enablement to block")
	}
	if _, err := result.Apply(context.Background(), backups); err == nil {
		t.Fatal("applied blocked migration")
	}
	if _, err := (Result{}).Apply(context.Background(), backups); err == nil {
		t.Fatal("accepted an unverified synthetic result")
	}
	if got, err := os.ReadFile(filepath.Join(dir, "main.tf")); err != nil || string(got) != source {
		t.Fatalf("blocked write changed Terraform: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "migration.tf")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("blocked migration generated a file: %v", err)
	}
}

func TestApplyRejectsChangedAndUnrelatedFiles(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		changeFile string
	}{
		{name: "target file modified", changeFile: "main.tf"},
		{name: "unrelated analyzed file modified", changeFile: "variables.tf"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir, backups := t.TempDir(), privateBackupDir(t)
			writeConfig(t, dir, "main.tf", simpleVCLConfiguration)
			writeConfig(t, dir, "variables.tf", "variable \"name\" {\n  type = string\n}\n")
			result := newPlan(t, dir)
			if result.Blocked() {
				t.Fatalf("unexpected blockers: %#v", result.Issues)
			}
			path := filepath.Join(dir, tc.changeFile)
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			updated := append([]byte("# concurrent edit\n"), original...)
			writeConfig(t, dir, tc.changeFile, string(updated))
			if _, err := result.Apply(context.Background(), backups); err == nil ||
				!strings.Contains(err.Error(), "changed after migration planning") {
				t.Fatalf("expected snapshot failure, got %v", err)
			}
			if _, err := os.Stat(filepath.Join(dir, "migration.tf")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("unexpected generated file: %v", err)
			}
			got, err := os.ReadFile(path)
			if err != nil || !bytes.HasPrefix(got, updated) {
				t.Fatal("concurrent edit was overwritten")
			}
		})
	}
}

func TestApplyRefusesBackupInsideModule(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	source := simpleVCLConfiguration
	writeConfig(t, dir, "main.tf", source)
	result := newPlan(t, dir)
	if _, err := result.Apply(context.Background(), filepath.Join(dir, ".backup")); err == nil ||
		!strings.Contains(err.Error(), "outside the Terraform root") {
		t.Fatalf("in-tree backup accepted: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "main.tf")); err != nil || string(got) != source {
		t.Fatal("invalid backup location changed Terraform")
	}
}

// resolvedTempDir avoids platforms (macOS) where t.TempDir sits behind a symlink,
// which Apply resolves before deriving the backup lock path.
func resolvedTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestApplyRefusesInterruptedBackup(t *testing.T) {
	t.Parallel()
	dir, backups := resolvedTempDir(t), privateBackupDir(t)
	writeConfig(t, dir, "main.tf", simpleVCLConfiguration)
	result := newPlan(t, dir)
	project, unlock, err := lockProject(backups, result.Analysis.Root)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareBackup(context.Background(), project, result.Analysis.Root, result.Changes)
	unlock()
	if err != nil {
		t.Fatal(err)
	}
	if readManifest(t, prepared.path).Status != backupPrepared {
		t.Fatal("expected an interrupted prepared transaction")
	}
	if _, err := result.Apply(context.Background(), backups); err == nil ||
		!strings.Contains(err.Error(), "unfinished migration backup") {
		t.Fatalf("expected fail-closed interrupted backup: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(dir, "main.tf"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(content, []byte(`fastly_service_vcl`)) {
		t.Fatal("file modified despite previous pending transaction")
	}
}

func TestApplyRefusesConcurrentWriterLock(t *testing.T) {
	t.Parallel()
	dir, backups := resolvedTempDir(t), privateBackupDir(t)
	writeConfig(t, dir, "main.tf", simpleVCLConfiguration)
	result := newPlan(t, dir)
	_, unlock, err := lockProject(backups, result.Analysis.Root)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if _, err := result.Apply(context.Background(), backups); err == nil ||
		!strings.Contains(err.Error(), "migration lock") {
		t.Fatalf("concurrent write was not blocked: %v", err)
	}
}

func TestApplyRefusesPublicBackupDirectory(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission checks do not apply on Windows")
	}
	dir, backups := t.TempDir(), privateBackupDir(t)
	if err := os.Chmod(backups, 0o755); err != nil {
		t.Fatal(err)
	}
	writeConfig(t, dir, "main.tf", simpleVCLConfiguration)
	result := newPlan(t, dir)
	if _, err := result.Apply(context.Background(), backups); err == nil ||
		!strings.Contains(err.Error(), "chmod 700") {
		t.Fatalf("public backup directory accepted: %v", err)
	}
}

func TestApplyChecksRegistrySupportedResourceNoOps(t *testing.T) {
	t.Parallel()
	registry := compatibility.Default()
	for _, rule := range registry.All() {
		if rule.Kind != compatibility.ConstructResource || rule.Support != compatibility.SupportAutomatic {
			continue
		}
		t.Run(rule.ID, func(t *testing.T) {
			t.Parallel()
			dir, backups := t.TempDir(), privateBackupDir(t)
			writeConfig(t, dir, "main.tf",
				"resource \""+rule.Legacy+"\" \"example\" {\n  name = var.name\n}\n")
			result := newPlan(t, dir)
			if result.Blocked() {
				t.Fatalf("automatic resource %s blocked: %#v", rule.Legacy, result.Issues)
			}
			outcome, err := result.Apply(context.Background(), backups)
			if err != nil {
				t.Fatal(err)
			}
			if rule.Legacy == "fastly_service_vcl" {
				if outcome.FilesWritten != 2 {
					t.Fatalf("VCL write = %#v", outcome)
				}
			} else if outcome.FilesWritten != 0 || outcome.BackupDir != "" {
				t.Fatalf("unchanged automatic resource produced writes: %#v", outcome)
			}
		})
	}
}

func TestApplyRefusesSymlinkedBackupIntoModule(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", simpleVCLConfiguration)
	private := filepath.Join(dir, "secrets")
	if err := os.Mkdir(private, 0o700); err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	link := filepath.Join(other, "backup-link")
	if err := os.Symlink(private, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	result := newPlan(t, dir)
	if _, err := result.Apply(context.Background(), filepath.Join(link, "extra")); err == nil ||
		!strings.Contains(err.Error(), "outside the Terraform root") {
		t.Fatalf("backup parent symlink into Terraform module accepted: %v", err)
	}
}

func TestResolveBackupRootRejectsBackupInsideSymlinkedTerraformRoot(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	actual := filepath.Join(parent, "real-module")
	if err := os.Mkdir(actual, 0o700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(parent, "module-alias")
	if err := os.Symlink(actual, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	for _, backupPath := range []string{
		filepath.Join(actual, "backups"),
		filepath.Join(alias, "backups"),
	} {
		if _, err := resolveBackupRoot(alias, backupPath); err == nil ||
			!strings.Contains(err.Error(), "outside the Terraform root") {
			t.Fatalf("accepted in-tree backup %q for alias %q: %v", backupPath, alias, err)
		}
	}
	outside := filepath.Join(parent, "outside-backups")
	canonicalOutside, err := resolveExistingParents(outside)
	if err != nil {
		t.Fatal(err)
	}
	if path, err := resolveBackupRoot(alias, outside); err != nil || path != canonicalOutside {
		t.Fatalf("rejected an external backup: %q, %v", path, err)
	}
}

func TestApplyCanceledContextDoesNotCreateBackup(t *testing.T) {
	t.Parallel()
	dir, backups := t.TempDir(), privateBackupDir(t)
	writeConfig(t, dir, "main.tf", simpleVCLConfiguration)
	result := newPlan(t, dir)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := result.Apply(ctx, backups); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled write = %v", err)
	}
	if _, err := os.Stat(filepath.Join(backups, "backups")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("canceled write initialized backup storage: %v", err)
	}
}

func TestApplyExecutesRegistryBackedFieldChanges(t *testing.T) {
	t.Parallel()
	dir, backups := t.TempDir(), privateBackupDir(t)
	writeConfig(t, dir, "main.tf", `resource "fastly_service_vcl" "demo" {
  name = local.name
  activate = true
  stage = false
  version_comment = ""
  default_host = var.host

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
		t.Fatalf("supported field migration blocked: %#v", result.Issues)
	}
	if _, err := result.Apply(context.Background(), backups); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "main.tf"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		`resource "fastly_service_cdn_auto" "demo"`,
		"settings {", "default_host", "var.host", "custom_vcl {",
		"address = var.origin", "content = var.vcl_source",
	} {
		if !strings.Contains(string(got), required) {
			t.Errorf("migrated HCL missing %q:\n%s", required, got)
		}
	}
	for _, rejected := range []string{"activate =", "stage =", "version_comment =", "\n  vcl {"} {
		if strings.Contains(string(got), rejected) {
			t.Errorf("unsupported HCL remains: %q\n%s", rejected, got)
		}
	}
}

func TestApplyBlocksEveryNonautomaticResourceRule(t *testing.T) {
	t.Parallel()
	registry := compatibility.Default()
	var blocked int
	for _, rule := range registry.All() {
		if rule.Kind != compatibility.ConstructResource || rule.Support == compatibility.SupportAutomatic {
			continue
		}
		blocked++
		t.Run(rule.ID, func(t *testing.T) {
			t.Parallel()
			dir, backups := t.TempDir(), privateBackupDir(t)
			input := "resource \"" + rule.Legacy + "\" \"demo\" {\n  name = var.name\n}\n"
			writeConfig(t, dir, "main.tf", input)
			result := newPlan(t, dir)
			if !result.Blocked() {
				t.Fatalf("nonautomatic resource %q was treated as writable", rule.Legacy)
			}
			if _, err := result.Apply(context.Background(), backups); err == nil {
				t.Fatal("blocked resource was written")
			}
			got, err := os.ReadFile(filepath.Join(dir, "main.tf"))
			if err != nil || string(got) != input {
				t.Fatal("blocked migration changed source")
			}
			if _, err := os.Stat(filepath.Join(backups, "backups")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("blocked migration created backup storage")
			}
		})
	}
	if blocked == 0 {
		t.Fatal("registry unexpectedly has no blocked resource rules")
	}
}

func TestApplyRefusesBackupManifestForDifferentRoot(t *testing.T) {
	t.Parallel()
	dir, backups := t.TempDir(), privateBackupDir(t)
	writeConfig(t, dir, "main.tf", simpleVCLConfiguration)
	result := newPlan(t, dir)
	project, unlock, err := lockProject(backups, result.rewritePlan.Root())
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareBackup(context.Background(), project, result.rewritePlan.Root(), result.Changes)
	if err != nil {
		unlock()
		t.Fatal(err)
	}
	prepared.manifest.Root = "/some-other-project"
	if err := prepared.updateStatus(backupApplied); err != nil {
		unlock()
		t.Fatal(err)
	}
	unlock()
	if _, err := result.Apply(context.Background(), backups); err == nil ||
		!strings.Contains(err.Error(), "belongs to") {
		t.Fatalf("foreign recovery manifest was trusted: %v", err)
	}
}
