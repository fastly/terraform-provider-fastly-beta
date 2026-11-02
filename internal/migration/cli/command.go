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
	Version   string
	Writer    io.Writer
	ErrWriter io.Writer
}

// NewCommand constructs the fastly-tf-migrate root command.
//
// Migration behavior intentionally lives outside this package. Follow-up
// commands such as analyze and migrate should delegate to ordinary Go packages
// and keep urfave/cli confined to this command layer.
func NewCommand(config Config) *urfavecli.Command {
	return &urfavecli.Command{
		Name:      commandName,
		Usage:     commandUsage,
		Version:   config.Version,
		Writer:    config.Writer,
		ErrWriter: config.ErrWriter,
		Commands: []*urfavecli.Command{
			newVersionCommand(config),
		},
	}
}

func newVersionCommand(config Config) *urfavecli.Command {
	return &urfavecli.Command{
		Name:  "version",
		Usage: "Print the fastly-tf-migrate version",
		Action: func(_ context.Context, _ *urfavecli.Command) error {
			_, err := fmt.Fprintln(config.Writer, config.Version)
			return err
		},
	}
}
