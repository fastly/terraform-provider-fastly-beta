package migrate

import (
	"fmt"
	"io"
	"strings"
)

// WriteText reports blocking diagnostics or the full proposed HCL diff.
// It never prints a partial diff when any required change is unsupported.
func WriteText(writer io.Writer, result Result) error {
	status := "READY"
	if result.Blocked() {
		status = "BLOCKED"
	}
	if _, err := fmt.Fprintf(writer, "Migration dry-run: %s\n", status); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "Analyzed files: %d\n", len(result.Analysis.Files)); err != nil {
		return err
	}
	if result.Blocked() {
		if len(result.Analysis.Findings) != 0 {
			for _, finding := range result.Analysis.Findings {
				if _, err := fmt.Fprintf(writer, "  %s [%s] %s: %s\n",
					strings.ToUpper(string(finding.Severity)),
					finding.RuleID,
					displayPath(result.Analysis.Root, finding.Range.Filename),
					finding.Message); err != nil {
					return err
				}
			}
		}
		for _, issue := range result.Issues {
			rule := ""
			if issue.RuleID != "" {
				rule = " [" + issue.RuleID + "]"
			}
			if _, err := fmt.Fprintf(writer, "  BLOCKED%s %s: %s\n    %s\n",
				rule, issue.File, issue.Message, issue.Remediation); err != nil {
				return err
			}
		}
		_, err := fmt.Fprintln(writer, "No changes proposed; resolve all blockers before migration.")
		return err
	}

	if _, err := fmt.Fprintf(writer, "Resources to move: %d\nFiles to change: %d\n",
		len(result.Moves), len(result.Changes)); err != nil {
		return err
	}
	for _, move := range result.Moves {
		if _, err := fmt.Fprintf(writer, "  [%s] %s -> %s\n", move.Module, move.From, move.To); err != nil {
			return err
		}
	}
	for _, change := range result.Changes {
		kind := "modify"
		if !change.Exists {
			kind = "create"
		}
		if _, err := fmt.Fprintf(writer, "  %s (%s)\n", change.Path, kind); err != nil {
			return err
		}
	}
	for _, finding := range result.Analysis.Findings {
		if _, err := fmt.Fprintf(writer, "  WARNING %s: %s\n",
			displayPath(result.Analysis.Root, finding.Range.Filename), finding.Message); err != nil {
			return err
		}
	}
	if len(result.Changes) != 0 {
		if _, err := fmt.Fprintln(writer, "\nProposed changes:"); err != nil {
			return err
		}
		if err := WriteDiff(writer, result.Changes); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(writer,
		"\nPreview only: no files or Terraform state were changed. "+
			"Review provider version, state migration support, and terraform plan before applying.")
	return err
}
