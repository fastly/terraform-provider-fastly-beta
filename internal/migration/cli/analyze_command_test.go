package cli

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	urfavecli "github.com/urfave/cli/v3"

	migrationanalyze "github.com/fastly/terraform-provider-fastly-beta/internal/migration/analyze"
)

func TestAnalyzeCommandIntegration(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	config := `
terraform {
  required_providers {
    fastly = {
      source = "fastly/fastly"
    }
  }
}

resource "fastly_service_vcl" "demo" {
  name     = "migration-demo"
  activate = true
}
`
	if err := writeConfigFile(filepath.Join(dir, "main.tf"), []byte(config)); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}

	var stdout bytes.Buffer
	command := NewCommand(Config{
		Version: "v1.2.3",
		Writer:  &stdout,
	})

	if err := command.Run(
		context.Background(),
		[]string{commandName, "analyze", dir},
	); err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := stdout.String()
	for _, want := range []string{
		"Status: READY",
		"[resource-service-vcl]",
		"fastly_service_vcl.demo -> fastly_service_cdn_auto.demo",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("stdout does not contain %q:\n%s", want, output)
		}
	}
}

func TestAnalyzeCommandDefaultsToCurrentDirectory(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var gotPath string

	command := NewCommand(Config{
		Version: "v1.2.3",
		Writer:  &stdout,
		Analyze: func(_ context.Context, path string) (migrationanalyze.Result, error) {
			gotPath = path
			return migrationanalyze.Result{Root: "."}, nil
		},
	})

	if err := command.Run(context.Background(), []string{commandName, "analyze"}); err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}
	if gotPath != "." {
		t.Fatalf("Analyze path = %q, want %q", gotPath, ".")
	}
	if !strings.Contains(stdout.String(), "Status: READY") {
		t.Fatalf("stdout = %q, want READY report", stdout.String())
	}
}

func TestAnalyzeCommandUsesSelectedDirectory(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var gotPath string

	command := NewCommand(Config{
		Version: "v1.2.3",
		Writer:  &stdout,
		Analyze: func(_ context.Context, path string) (migrationanalyze.Result, error) {
			gotPath = path
			return migrationanalyze.Result{Root: path}, nil
		},
	})

	if err := command.Run(
		context.Background(),
		[]string{commandName, "analyze", "./legacy-service"},
	); err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}
	if gotPath != "./legacy-service" {
		t.Fatalf("Analyze path = %q, want %q", gotPath, "./legacy-service")
	}
}

func TestAnalyzeCommandReturnsBlockedExitCodeAfterWritingReport(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	command := NewCommand(Config{
		Version: "v1.2.3",
		Writer:  &stdout,
		Analyze: func(_ context.Context, _ string) (migrationanalyze.Result, error) {
			return migrationanalyze.Result{
				Root: ".",
				Findings: []migrationanalyze.Finding{
					{
						Severity:    migrationanalyze.SeverityError,
						Range:       hcl.Range{Filename: "main.tf", Start: hcl.Pos{Line: 4, Column: 3}},
						Message:     "migration blocker",
						Remediation: "fix it",
					},
				},
			}, nil
		},
	})

	err := command.Run(context.Background(), []string{commandName, "analyze"})
	if err == nil {
		t.Fatal("Run() error = nil, want blocked error")
	}
	coder, ok := err.(interface{ ExitCode() int })
	if !ok {
		t.Fatalf("Run() error = %T, want exit-code error", err)
	}
	if got := coder.ExitCode(); got != exitCodeAnalysisBlocked {
		t.Fatalf("ExitCode() = %d, want %d", got, exitCodeAnalysisBlocked)
	}
	if !strings.Contains(stdout.String(), "Status: BLOCKED") {
		t.Fatalf("stdout = %q, want BLOCKED report", stdout.String())
	}
}

func TestAnalyzeCommandRejectsExtraArguments(t *testing.T) {
	t.Parallel()

	called := false
	command := NewCommand(Config{
		Version: "v1.2.3",
		Writer:  new(bytes.Buffer),
		Analyze: func(_ context.Context, _ string) (migrationanalyze.Result, error) {
			called = true
			return migrationanalyze.Result{}, nil
		},
	})

	err := command.Run(
		context.Background(),
		[]string{commandName, "analyze", "one", "two"},
	)
	if err == nil {
		t.Fatal("Run() error = nil, want usage error")
	}
	if called {
		t.Fatal("Analyze was called for invalid CLI arguments")
	}
	coder, ok := err.(interface{ ExitCode() int })
	if !ok || coder.ExitCode() != exitCodeUsage {
		t.Fatalf("Run() error = %#v, want exit code %d", err, exitCodeUsage)
	}
}

func TestAnalyzeCommandReturnsExecutionFailure(t *testing.T) {
	t.Parallel()

	command := NewCommand(Config{
		Version: "v1.2.3",
		Writer:  new(bytes.Buffer),
		Analyze: func(_ context.Context, _ string) (migrationanalyze.Result, error) {
			return migrationanalyze.Result{}, errors.New("read failed")
		},
	})

	err := command.Run(context.Background(), []string{commandName, "analyze"})
	if err == nil {
		t.Fatal("Run() error = nil, want execution error")
	}
	coder, ok := err.(interface{ ExitCode() int })
	if !ok || coder.ExitCode() != exitCodeFailure {
		t.Fatalf("Run() error = %#v, want exit code %d", err, exitCodeFailure)
	}
	if !strings.Contains(err.Error(), "read failed") {
		t.Fatalf("Run() error = %q, want underlying error", err)
	}
}

func TestAnalyzeCommandUsesRuntimeWriter(t *testing.T) {
	t.Parallel()

	var configuredStdout bytes.Buffer
	var runtimeStdout bytes.Buffer

	command := NewCommand(Config{
		Version: "v1.2.3",
		Writer:  &configuredStdout,
		Analyze: func(_ context.Context, _ string) (migrationanalyze.Result, error) {
			return migrationanalyze.Result{Root: "."}, nil
		},
	})

	analyzeCommand := findSubcommand(t, command, "analyze")
	analyzeCommand.Writer = &runtimeStdout

	if err := analyzeCommand.Run(context.Background(), []string{"analyze"}); err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}
	if !strings.Contains(runtimeStdout.String(), "Status: READY") {
		t.Fatalf("runtime stdout = %q, want READY report", runtimeStdout.String())
	}
	if configuredStdout.String() != "" {
		t.Fatalf("configured stdout = %q, want empty", configuredStdout.String())
	}
}

func findSubcommand(t *testing.T, command *urfavecli.Command, name string) *urfavecli.Command {
	t.Helper()
	for _, subcommand := range command.Commands {
		if subcommand.Name == name {
			return subcommand
		}
	}
	t.Fatalf("subcommand %q not found", name)
	return nil
}
