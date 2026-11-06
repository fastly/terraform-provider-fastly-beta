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

func TestMigrateRequiresDryRunAndRejectsExtraArguments(t *testing.T) {
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
