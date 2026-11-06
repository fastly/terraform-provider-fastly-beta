package cli

import (
	"context"
	"fmt"
	"io"

	urfavecli "github.com/urfave/cli/v3"
)

const (
	commandName  = "fastly-tf-migrate"
	commandUsage = "Migrate Fastly Terraform configuration to the dual-model provider"
)

// Config contains the process-level dependencies needed by the CLI shell.
type Config struct {
	Version       string
	Writer        io.Writer
	ErrWriter     io.Writer
	Analyze       AnalyzeFunc
	MigrateDryRun MigrateDryRunFunc
}

// NewCommand constructs the fastly-tf-migrate root command.
//
// Migration behavior intentionally lives outside this package. Subcommands
// delegate to ordinary Go packages and keep urfave/cli confined to this
// command layer.
func NewCommand(config Config) *urfavecli.Command {
	return &urfavecli.Command{
		Name:           commandName,
		Usage:          commandUsage,
		Version:        config.Version,
		Writer:         config.Writer,
		ErrWriter:      config.ErrWriter,
		ExitErrHandler: returnExitErrorToCaller,
		Commands: []*urfavecli.Command{
			newVersionCommand(config),
			newAnalyzeCommand(config),
			newMigrateCommand(config),
		},
	}
}

func newVersionCommand(config Config) *urfavecli.Command {
	return &urfavecli.Command{
		Name:      "version",
		Usage:     "Print the fastly-tf-migrate version",
		Writer:    config.Writer,
		ErrWriter: config.ErrWriter,
		Action: func(_ context.Context, command *urfavecli.Command) error {
			_, err := fmt.Fprintln(command.Writer, config.Version)
			return err
		},
	}
}

// returnExitErrorToCaller overrides urfave/cli's default exit-code handler,
// which can terminate the process when an Action returns an ExitCoder.
// The executable's main function is responsible for printing the error and
// selecting the process exit status; tests can inspect errors from Run().
func returnExitErrorToCaller(_ context.Context, _ *urfavecli.Command, _ error) {}
