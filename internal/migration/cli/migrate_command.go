package cli

import (
	"context"
	"fmt"
	"io"

	urfavecli "github.com/urfave/cli/v3"

	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/compatibility"
	migrationmigrate "github.com/fastly/terraform-provider-fastly-beta/internal/migration/migrate"
)

// MigrateDryRunFunc builds an in-memory migration plan without applying changes.
// The same plan is used by --dry-run and --write; applying a synthetic result
// without the original validated plan is rejected by migrate.Result.Apply.
type MigrateDryRunFunc func(context.Context, string) (migrationmigrate.Result, error)

const exitCodeChangesNeeded = 4

func newMigrateCommand(config Config) *urfavecli.Command {
	planner := config.MigrateDryRun
	if planner == nil {
		planner = migrationmigrate.New(compatibility.Default()).Plan
	}
	return &urfavecli.Command{
		Name:           "migrate",
		Usage:          "Preview or explicitly write supported Terraform migration changes",
		Writer:         config.Writer,
		ErrWriter:      config.ErrWriter,
		ExitErrHandler: returnExitErrorToCaller,
		Flags: []urfavecli.Flag{
			&urfavecli.BoolFlag{
				Name:  "dry-run",
				Usage: "Print a deterministic HCL diff; never write configuration or state",
			},
			&urfavecli.BoolFlag{
				Name:  "write",
				Usage: "Apply the exact previewed HCL plan after writing private recovery backups",
			},
			&urfavecli.BoolFlag{
				Name:  "check",
				Usage: "With --dry-run, return exit code 4 if migration changes are needed",
			},
			&urfavecli.StringFlag{
				Name:  "backup-dir",
				Usage: "Private backup storage outside the Terraform tree (default: ~/.fastly-tf-migrate); only with --write",
			},
		},
		Action: func(ctx context.Context, command *urfavecli.Command) error {
			args := command.Args().Slice()
			if len(args) > 1 {
				return exitError{message: "migrate accepts at most one PATH argument", code: exitCodeUsage}
			}
			dryRun := command.Bool("dry-run")
			write := command.Bool("write")
			check := command.Bool("check")
			backupDir := command.String("backup-dir")
			if dryRun == write {
				return exitError{
					message: "migrate requires exactly one of --dry-run or --write",
					code:    exitCodeUsage,
				}
			}
			if (check && !dryRun) || (backupDir != "" && !write) {
				return exitError{
					message: "--check requires --dry-run; --backup-dir requires --write",
					code:    exitCodeUsage,
				}
			}
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			result, err := planner(ctx, dir)
			if err != nil {
				return exitError{
					message: fmt.Sprintf("plan migration %q: %v", dir, err),
					code:    exitCodeFailure,
				}
			}
			if write {
				// Stream the complete preview before touching Terraform files.
				// A short write or output error prevents the migration from applying.
				err = migrationmigrate.WriteApplyPreview(
					strictOutputWriter{Writer: command.Writer}, result)
			} else {
				err = migrationmigrate.WriteText(command.Writer, result)
			}
			if err != nil {
				return exitError{
					message: fmt.Sprintf("write migration preview: %v", err),
					code:    exitCodeFailure,
				}
			}
			if result.Blocked() {
				return exitError{
					message: "migration blocked by unsupported or incomplete transformations",
					code:    exitCodeAnalysisBlocked,
				}
			}
			if !write {
				if check && len(result.Changes) != 0 {
					return exitError{
						message: "migration changes are needed",
						code:    exitCodeChangesNeeded,
					}
				}
				return nil
			}

			applied, err := result.Apply(ctx, backupDir)
			if err != nil {
				return exitError{
					message: fmt.Sprintf("apply migration %q: %v", dir, err),
					code:    exitCodeFailure,
				}
			}
			if applied.FilesWritten == 0 {
				_, err = fmt.Fprintln(command.Writer, "Migration complete: no changes necessary.")
			} else {
				_, err = fmt.Fprintf(command.Writer,
					"Migration complete: %d file(s) written.\nRecovery backups: %s\n"+
						"Review and run terraform plan before applying state changes.\n",
					applied.FilesWritten, applied.BackupDir)
			}
			if err != nil {
				return exitError{
					message: fmt.Sprintf("write migration completion report: %v", err),
					code:    exitCodeFailure,
				}
			}
			return nil
		},
	}
}

// strictOutputWriter turns a short successful write into an error so the
// migration cannot proceed after an incomplete preview was printed.
type strictOutputWriter struct {
	io.Writer
}

func (w strictOutputWriter) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	return n, err
}
