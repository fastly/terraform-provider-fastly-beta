package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestNewCommand(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := NewCommand(Config{
		Version:   "v1.2.3",
		Writer:    &stdout,
		ErrWriter: &stderr,
	})

	if command.Name != commandName {
		t.Fatalf("Name = %q, want %q", command.Name, commandName)
	}
	if command.Version != "v1.2.3" {
		t.Fatalf("Version = %q, want v1.2.3", command.Version)
	}
	if len(command.Commands) != 1 || command.Commands[0].Name != "version" {
		t.Fatalf("Commands = %#v, want only the version command", command.Commands)
	}
}

func TestVersionCommand(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := NewCommand(Config{
		Version:   "v1.2.3",
		Writer:    &stdout,
		ErrWriter: &stderr,
	})

	if err := command.Run(context.Background(), []string{commandName, "version"}); err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}
	if got := stdout.String(); got != "v1.2.3\n" {
		t.Fatalf("stdout = %q, want %q", got, "v1.2.3\\n")
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("stderr = %q, want empty", got)
	}
}

func TestVersionFlag(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := NewCommand(Config{
		Version:   "v1.2.3",
		Writer:    &stdout,
		ErrWriter: &stderr,
	})

	if err := command.Run(context.Background(), []string{commandName, "--version"}); err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, "v1.2.3") {
		t.Fatalf("stdout = %q, want version output containing v1.2.3", got)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("stderr = %q, want empty", got)
	}
}

func TestHelp(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	command := NewCommand(Config{
		Version:   "v1.2.3",
		Writer:    &stdout,
		ErrWriter: &stderr,
	})

	if err := command.Run(context.Background(), []string{commandName, "--help"}); err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, commandUsage) {
		t.Fatalf("stdout = %q, want help output containing %q", got, commandUsage)
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("stderr = %q, want empty", got)
	}
}
