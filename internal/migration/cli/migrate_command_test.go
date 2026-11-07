package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	migrationmigrate "github.com/fastly/terraform-provider-fastly-beta/internal/migration/migrate"
)

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

func TestMigrateDryRunCommandEndToEndDoesNotWrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	before := []byte(`resource "fastly_service_vcl" "demo" {
  name = "demo"
}
`)
	path := filepath.Join(dir, "main.tf")
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	command := NewCommand(Config{Writer: &stdout, ErrWriter: &stderr})
	if err := command.Run(context.Background(),
		[]string{commandName, "migrate", "--dry-run", dir}); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{
		"Migration dry-run: READY", "fastly_service_vcl.demo",
		"fastly_service_cdn_auto.demo", "+++ b/migration.tf", "Files to change: 2",
	} {
		if !strings.Contains(stdout.String(), text) {
			t.Fatalf("preview missing %q:\n%s", text, stdout.String())
		}
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, before) {
		t.Fatal("dry-run modified Terraform config")
	}
	if _, err := os.Stat(filepath.Join(dir, "migration.tf")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created Terraform file: %v", err)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("unexpected stderr: %s", got)
	}
}

func TestMigrateDryRunReturnsBlockerExitCodeWithoutPartialDiff(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	input := []byte(`resource "fastly_service_vcl" "demo" {
  name = "demo"
  product_enablement {
    domain_inspector = true
  }
}
`)
	if err := os.WriteFile(filepath.Join(dir, "main.tf"), input, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	command := NewCommand(Config{Writer: &stdout, ErrWriter: io.Discard})
	err := command.Run(context.Background(), []string{commandName, "migrate", "--dry-run", dir})
	var coder interface{ ExitCode() int }
	if !errors.As(err, &coder) || coder.ExitCode() != exitCodeAnalysisBlocked {
		t.Fatalf("blocked exit = %v, want %d", err, exitCodeAnalysisBlocked)
	}
	if !strings.Contains(stdout.String(), "Migration dry-run: BLOCKED") ||
		!strings.Contains(stdout.String(), "No changes proposed") ||
		strings.Contains(stdout.String(), "--- a/main.tf") {
		t.Fatalf("blocked command leaked partial diff:\n%s", stdout.String())
	}
	got, err := os.ReadFile(filepath.Join(dir, "main.tf"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, input) {
		t.Fatal("blocked command wrote source")
	}
}

func TestMigrateRequiresExplicitModeAndRejectsExtraArguments(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{commandName, "migrate"},
		{commandName, "migrate", "somewhere"},
		{commandName, "migrate", "--dry-run", "one", "two"},
	} {
		var called bool
		command := NewCommand(Config{
			Writer: io.Discard, ErrWriter: io.Discard,
			MigrateDryRun: func(context.Context, string) (migrationmigrate.Result, error) {
				called = true
				return migrationmigrate.Result{}, nil
			},
		})
		err := command.Run(context.Background(), args)
		var coder interface{ ExitCode() int }
		if !errors.As(err, &coder) || coder.ExitCode() != exitCodeUsage {
			t.Errorf("args=%q: exit=%v, want usage error", args, err)
		}
		if called {
			t.Fatalf("invalid args %q invoked preview", args)
		}
	}
}

func TestMigrateDryRunUsesDefaultAndSelectedPath(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{commandName, "migrate", "--dry-run"}, "."},
		{[]string{commandName, "migrate", "--dry-run", "./project"}, "./project"},
	} {
		var got string
		command := NewCommand(Config{
			Writer: io.Discard, ErrWriter: io.Discard,
			MigrateDryRun: func(_ context.Context, path string) (migrationmigrate.Result, error) {
				got = path
				return migrationmigrate.Result{}, nil
			},
		})
		if err := command.Run(context.Background(), tc.args); err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Errorf("args=%q: got path %q, want %q", tc.args, got, tc.want)
		}
	}
}

func TestMigrateDryRunReturnsExecutionAndOutputErrors(t *testing.T) {
	t.Parallel()
	command := NewCommand(Config{
		Writer: io.Discard, ErrWriter: io.Discard,
		MigrateDryRun: func(context.Context, string) (migrationmigrate.Result, error) {
			return migrationmigrate.Result{}, errors.New("planning failed")
		},
	})
	err := command.Run(context.Background(), []string{commandName, "migrate", "--dry-run"})
	var coder interface{ ExitCode() int }
	if !errors.As(err, &coder) || coder.ExitCode() != exitCodeFailure {
		t.Fatalf("planning failure: got %v", err)
	}

	command = NewCommand(Config{
		Writer: failingWriter{}, ErrWriter: io.Discard,
		MigrateDryRun: func(context.Context, string) (migrationmigrate.Result, error) {
			return migrationmigrate.Result{}, nil
		},
	})
	err = command.Run(context.Background(), []string{commandName, "migrate", "--dry-run"})
	if !errors.As(err, &coder) || coder.ExitCode() != exitCodeFailure {
		t.Fatalf("output failure: got %v", err)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("output unavailable")
}

func TestMigrateDryRunUsesRuntimeWriter(t *testing.T) {
	t.Parallel()
	var configured, runtime bytes.Buffer
	command := NewCommand(Config{
		Writer: &configured, ErrWriter: io.Discard,
		MigrateDryRun: func(context.Context, string) (migrationmigrate.Result, error) {
			return migrationmigrate.Result{}, nil
		},
	})
	migrate := findSubcommand(t, command, "migrate")
	migrate.Writer = &runtime
	if err := migrate.Run(context.Background(), []string{"migrate", "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(runtime.String(), "Migration dry-run: READY") || configured.Len() != 0 {
		t.Fatalf("runtime writer not used, runtime=%q configured=%q", runtime.String(), configured.String())
	}
}

func TestMigrateWriteEndToEndAndNoOpRerun(t *testing.T) {
	t.Parallel()
	dir, backups := t.TempDir(), privateBackupDir(t)
	before := []byte(`resource "fastly_service_vcl" "demo" {
  name = "demo"
}
`)
	path := filepath.Join(dir, "main.tf")
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	cmd := NewCommand(Config{Writer: &stdout, ErrWriter: &stderr})
	args := []string{commandName, "migrate", "--write", "--backup-dir", backups, dir}
	if err := cmd.Run(context.Background(), args); err != nil {
		t.Fatal(err)
	}
	for _, wanted := range []string{
		"Migration write: READY", "Resources to move: 1", "Files to change: 2",
		"Migration complete: 2 file(s) written.", "Recovery backups:",
	} {
		if !strings.Contains(stdout.String(), wanted) {
			t.Fatalf("write result missing %q:\n%s", wanted, stdout.String())
		}
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(after, []byte("fastly_service_cdn_auto")) ||
		bytes.Contains(after, []byte(`resource "fastly_service_vcl"`)) {
		t.Fatalf("expected migrated HCL: %s", after)
	}
	if _, err := os.Stat(filepath.Join(dir, "migration.tf")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "Migration write: READY") || stderr.Len() != 0 {
		t.Fatalf("unexpected output stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	stdout.Reset()
	// A second CLI invocation uses a fresh command, as in the real binary.
	cmd = NewCommand(Config{Writer: &stdout, ErrWriter: &stderr})
	if err := cmd.Run(context.Background(), args); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "Migration complete: no changes necessary.") ||
		strings.Contains(stdout.String(), "Recovery backups:") {
		t.Fatalf("second run not a no-op: %s", stdout.String())
	}
}

func TestMigrateWriteBlockedNeverCreatesBackupOrWrites(t *testing.T) {
	t.Parallel()
	dir, backups := t.TempDir(), privateBackupDir(t)
	input := []byte(`resource "fastly_service_vcl" "demo" {
  name = "demo"
  product_enablement {
    domain_inspector = true
  }
}
`)
	path := filepath.Join(dir, "main.tf")
	if err := os.WriteFile(path, input, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	cmd := NewCommand(Config{Writer: &stdout, ErrWriter: io.Discard})
	err := cmd.Run(context.Background(), []string{commandName, "migrate", "--write", "--backup-dir", backups, dir})
	var coder interface{ ExitCode() int }
	if !errors.As(err, &coder) || coder.ExitCode() != exitCodeAnalysisBlocked {
		t.Fatalf("blocked migration exit=%v", err)
	}
	if strings.Contains(stdout.String(), "--- a/main.tf") ||
		!strings.Contains(stdout.String(), "Migration write: BLOCKED") {
		t.Fatalf("blocked migration leaked partial diff: %s", stdout.String())
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, input) {
		t.Fatal("blocked migration wrote Terraform files")
	}
	if _, err := os.Stat(filepath.Join(backups, "backups")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("blocked migration initialized backup storage")
	}
}

func TestMigrateWritePreviewOutputFailureDoesNotWrite(t *testing.T) {
	t.Parallel()
	dir, backups := t.TempDir(), privateBackupDir(t)
	input := []byte("resource \"fastly_service_vcl\" \"demo\" {\n  name = \"demo\"\n}\n")
	path := filepath.Join(dir, "main.tf")
	if err := os.WriteFile(path, input, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := NewCommand(Config{Writer: failingWriter{}, ErrWriter: io.Discard})
	err := cmd.Run(context.Background(), []string{commandName, "migrate", "--write", "--backup-dir", backups, dir})
	var coder interface{ ExitCode() int }
	if !errors.As(err, &coder) || coder.ExitCode() != exitCodeFailure {
		t.Fatalf("stdout failure = %v", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil || !bytes.Equal(got, input) {
		t.Fatal("failed output mutated source")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "migration.tf")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("failed output wrote new migration file")
	}
	if _, statErr := os.Stat(filepath.Join(backups, "backups")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("failed output created backup storage")
	}
}

func TestMigrateCheckExitCodesAndFlagConflicts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "main.tf")
	if err := os.WriteFile(path, []byte("resource \"fastly_service_vcl\" \"demo\" {\n  name = \"demo\"\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		args []string
		code int
	}{
		{"changes needed", []string{commandName, "migrate", "--dry-run", "--check", dir}, exitCodeChangesNeeded},
		{"dry-run preserves zero", []string{commandName, "migrate", "--dry-run", dir}, 0},
		{"conflicting modes", []string{commandName, "migrate", "--dry-run", "--write", dir}, exitCodeUsage},
		{"check without dry-run", []string{commandName, "migrate", "--write", "--check", dir}, exitCodeUsage},
		{"backup with dry-run", []string{commandName, "migrate", "--dry-run", "--backup-dir", t.TempDir(), dir}, exitCodeUsage},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := NewCommand(Config{Writer: io.Discard, ErrWriter: io.Discard})
			err := cmd.Run(context.Background(), tc.args)
			if tc.code == 0 {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var coder interface{ ExitCode() int }
			if !errors.As(err, &coder) || coder.ExitCode() != tc.code {
				t.Fatalf("exit=%v want=%d", err, tc.code)
			}
		})
	}
}

type shortOutputWriter struct{}

func (shortOutputWriter) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	return len(data) / 2, nil
}

func TestMigrateWriteRejectsShortPreviewOutputWithoutWriting(t *testing.T) {
	t.Parallel()
	dir, backups := t.TempDir(), privateBackupDir(t)
	path := filepath.Join(dir, "main.tf")
	before := []byte("resource \"fastly_service_vcl\" \"demo\" {\n  name = \"demo\"\n}\n")
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := NewCommand(Config{Writer: shortOutputWriter{}, ErrWriter: io.Discard})
	err := cmd.Run(context.Background(),
		[]string{commandName, "migrate", "--write", "--backup-dir", backups, dir})
	var coder interface{ ExitCode() int }
	if !errors.As(err, &coder) || coder.ExitCode() != exitCodeFailure {
		t.Fatalf("short preview output returned %v, want exit code %d", err, exitCodeFailure)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil || !bytes.Equal(got, before) {
		t.Fatal("short preview output modified Terraform")
	}
	if _, statErr := os.Stat(filepath.Join(backups, "backups")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("short preview output initialized backups")
	}
}
