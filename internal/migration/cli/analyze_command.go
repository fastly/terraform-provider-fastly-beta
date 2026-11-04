package cli

import (
	"context"
	"fmt"

	urfavecli "github.com/urfave/cli/v3"

	migrationanalyze "github.com/fastly/terraform-provider-fastly-beta/internal/migration/analyze"
	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/compatibility"
)

const (
	exitCodeFailure         = 1
	exitCodeUsage           = 2
	exitCodeAnalysisBlocked = 3
)

// AnalyzeFunc is the read-only analysis operation used by the CLI command.
type AnalyzeFunc func(context.Context, string) (migrationanalyze.Result, error)

type exitError struct {
	message string
	code    int
}

func (e exitError) Error() string {
	return e.message
}

func (e exitError) ExitCode() int {
	return e.code
}

func newAnalyzeCommand(config Config) *urfavecli.Command {
	analyze := config.Analyze
	if analyze == nil {
		analyzer := migrationanalyze.New(compatibility.Default())
		analyze = analyzer.Analyze
	}

	return &urfavecli.Command{
		Name:           "analyze",
		Usage:          "Analyze Terraform configuration for legacy-to-_auto migration",
		Writer:         config.Writer,
		ErrWriter:      config.ErrWriter,
		ExitErrHandler: returnExitErrorToCaller,
		Action: func(ctx context.Context, command *urfavecli.Command) error {
			args := command.Args().Slice()
			if len(args) > 1 {
				return exitError{
					message: "analyze accepts at most one PATH argument",
					code:    exitCodeUsage,
				}
			}

			workingDir := "."
			if len(args) == 1 {
				workingDir = args[0]
			}

			result, err := analyze(ctx, workingDir)
			if err != nil {
				return exitError{
					message: fmt.Sprintf("analyze %q: %v", workingDir, err),
					code:    exitCodeFailure,
				}
			}
			if err := migrationanalyze.WriteText(command.Writer, result); err != nil {
				return exitError{
					message: fmt.Sprintf("write analysis report: %v", err),
					code:    exitCodeFailure,
				}
			}
			if !result.Migratable() {
				return exitError{
					message: fmt.Sprintf(
						"analysis blocked by %d migration issue(s)",
						result.BlockingCount(),
					),
					code: exitCodeAnalysisBlocked,
				}
			}
			return nil
		},
	}
}
