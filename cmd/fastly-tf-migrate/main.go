package main

import (
	"context"
	"fmt"
	"os"

	migrationcli "github.com/fastly/terraform-provider-fastly-beta/internal/migration/cli"
	"github.com/fastly/terraform-provider-fastly-beta/internal/version"
)

func main() {
	command := migrationcli.NewCommand(migrationcli.Config{
		Version:   version.Version,
		Writer:    os.Stdout,
		ErrWriter: os.Stderr,
	})

	if err := command.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
