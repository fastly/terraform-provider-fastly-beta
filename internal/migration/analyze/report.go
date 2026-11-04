package analyze

import (
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
)

// WriteText writes a deterministic human-readable analysis report.
func WriteText(writer io.Writer, result Result) error {
	status := "READY"
	if !result.Migratable() {
		status = "BLOCKED"
	}

	if _, err := fmt.Fprintf(writer, "fastly-tf-migrate analyze\n\nStatus: %s\n", status); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		writer,
		"Modules: %d\nTerraform files: %d\nFastly provider requirements: %d\nMigration actions: %d\nReferences: %d\nWarnings: %d\nBlocking issues: %d\n",
		len(result.Modules),
		len(result.Files),
		len(result.ProviderRequirements),
		len(result.Actions),
		len(result.References),
		result.WarningCount(),
		result.BlockingCount(),
	); err != nil {
		return err
	}

	if len(result.ProviderRequirements) > 0 {
		if _, err := fmt.Fprintln(writer, "\nProvider requirements:"); err != nil {
			return err
		}
		for _, requirement := range result.ProviderRequirements {
			version := ""
			if requirement.Version != "" {
				version = " version=" + strconv.Quote(requirement.Version)
			}
			if _, err := fmt.Fprintf(
				writer,
				"  %s %s => %s%s\n",
				formatLocation(result.Root, requirement.Range.Filename, requirement.Range.Start.Line, requirement.Range.Start.Column),
				requirement.LocalName,
				requirement.Source,
				version,
			); err != nil {
				return err
			}
		}
	}

	if len(result.Actions) > 0 {
		if _, err := fmt.Fprintln(writer, "\nMigration plan:"); err != nil {
			return err
		}
		for _, action := range result.Actions {
			if _, err := fmt.Fprintf(
				writer,
				"  %s [%s] %s -> %s (hcl=%s state=%s)\n",
				formatLocation(result.Root, action.Range.Filename, action.Range.Start.Line, action.Range.Start.Column),
				action.RuleID,
				action.From,
				action.To,
				action.HCL,
				action.State,
			); err != nil {
				return err
			}
		}
	}

	if len(result.References) > 0 {
		if _, err := fmt.Fprintln(writer, "\nReferences requiring rewrite:"); err != nil {
			return err
		}
		for _, reference := range result.References {
			if _, err := fmt.Fprintf(
				writer,
				"  %s %s -> %s\n",
				formatLocation(result.Root, reference.Range.Filename, reference.Range.Start.Line, reference.Range.Start.Column),
				reference.From,
				reference.To,
			); err != nil {
				return err
			}
		}
	}

	if len(result.Findings) > 0 {
		if _, err := fmt.Fprintln(writer, "\nDiagnostics:"); err != nil {
			return err
		}
		for _, finding := range result.Findings {
			rule := ""
			if finding.RuleID != "" {
				rule = " [" + finding.RuleID + "]"
			}
			if _, err := fmt.Fprintf(
				writer,
				"  %s %s%s %s\n    %s\n",
				strings.ToUpper(string(finding.Severity)),
				formatLocation(result.Root, finding.Range.Filename, finding.Range.Start.Line, finding.Range.Start.Column),
				rule,
				finding.Message,
				finding.Remediation,
			); err != nil {
				return err
			}
		}
	}

	if result.Migratable() {
		_, err := fmt.Fprintln(writer, "\nNo blocking migration issues found.")
		return err
	}
	_, err := fmt.Fprintf(writer, "\nMigration is blocked by %d issue(s).\n", result.BlockingCount())
	return err
}

func formatLocation(root, filename string, line, column int) string {
	display := filename
	if relative, err := filepath.Rel(root, filename); err == nil {
		display = relative
	}
	display = filepath.ToSlash(display)
	if display == "." || display == "" {
		display = filepath.Base(filename)
	}
	return fmt.Sprintf("%s:%d:%d", display, line, column)
}
