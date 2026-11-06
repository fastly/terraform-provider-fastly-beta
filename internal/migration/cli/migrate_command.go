package cli

import (
	"context"
	"fmt"

	urfavecli "github.com/urfave/cli/v3"

	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/compatibility"
	migrationmigrate "github.com/fastly/terraform-provider-fastly-beta/internal/migration/migrate"
)

// MigrateDryRunFunc builds a read-only preview without applying changes.
type MigrateDryRunFunc func(context.Context, string) (migrationmigrate.Result, error)

func newMigrateCommand(config Config) *urfavecli.Command {
	dryRun := config.MigrateDryRun
	if dryRun == nil {
		planner := migrationmigrate.New(compatibility.Default())
		dryRun = planner.Plan
	}
	return &urfavecli.Command{
		Name:           "migrate",
		Usage:          "Preview Terraform migration changes (writing is not yet supported)",
		Writer:         config.Writer,
		ErrWriter:      config.ErrWriter,
		ExitErrHandler: returnExitErrorToCaller,
		Flags: []urfavecli.Flag{
			&urfavecli.BoolFlag{
				Name:  "dry-run",
				Usage: "Print the proposed HCL diff without writing files or state",
			},
		},
		Action: func(ctx context.Context, command *urfavecli.Command) error {
			args := command.Args().Slice()
			if len(args) > 1 {
				return exitError{message: "migrate accepts at most one PATH argument", code: exitCodeUsage}
			}
			if !command.Bool("dry-run") {
				return exitError{
					message: "migrate requires --dry-run; applying changes is not implemented yet",
					code:    exitCodeUsage,
				}
			}
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			result, err := dryRun(ctx, dir)
			if err != nil {
				return exitError{
					message: fmt.Sprintf("migrate --dry-run %q: %v", dir, err),
					code:    exitCodeFailure,
				}
			}
			if err := migrationmigrate.WriteText(command.Writer, result); err != nil {
				return exitError{
					message: fmt.Sprintf("write migration preview: %v", err),
					code:    exitCodeFailure,
				}
			}
			if result.Blocked() {
				return exitError{
					message: "migration preview blocked by unsupported or incomplete transformations",
					code:    exitCodeAnalysisBlocked,
				}
			}
			return nil
		},
	}
}
