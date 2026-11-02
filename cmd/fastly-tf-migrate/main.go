package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	migrationcli "github.com/fastly/terraform-provider-fastly-beta/internal/migration/cli"
	"github.com/fastly/terraform-provider-fastly-beta/internal/version"
)

type exitCoder interface {
	ExitCode() int
}

func main() {
	command := migrationcli.NewCommand(migrationcli.Config{
		Version:   version.Version,
		Writer:    os.Stdout,
		ErrWriter: os.Stderr,
	})

	if err := command.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitCode(err))
	}
}

func exitCode(err error) int {
	var coder exitCoder
	if errors.As(err, &coder) {
		return coder.ExitCode()
	}
	return 1
}
