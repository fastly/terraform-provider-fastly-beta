package analyze

import (
	"sort"

	"github.com/hashicorp/hcl/v2"

	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/compatibility"
)

// Severity describes whether a finding blocks automatic migration.
type Severity string

const (
	// SeverityWarning identifies a non-blocking condition worth surfacing.
	SeverityWarning Severity = "warning"
	// SeverityError identifies a condition that prevents safe automatic migration.
	SeverityError Severity = "error"
)

// Finding is an actionable diagnostic produced while analyzing Terraform configuration.
type Finding struct {
	Severity    Severity
	RuleID      string
	Range       hcl.Range
	Message     string
	Remediation string
}

// Action describes one deterministic migration operation implied by a compatibility rule.
type Action struct {
	RuleID string
	Kind   compatibility.ConstructKind
	From   string
	To     string
	HCL    compatibility.HCLStrategy
	State  compatibility.StateStrategy
	Range  hcl.Range
}

// Reference records a Terraform expression that refers to a legacy resource address.
type Reference struct {
	From  string
	To    string
	Range hcl.Range
}

// ProviderRequirement records a Fastly entry in terraform.required_providers.
type ProviderRequirement struct {
	LocalName string
	Source    string
	Version   string
	Range     hcl.Range
}

// Result is the complete, read-only preflight result for a Terraform working directory.
type Result struct {
	Root                 string
	Modules              []string
	Files                []string
	ProviderRequirements []ProviderRequirement
	Actions              []Action
	References           []Reference
	Findings             []Finding
}

// Migratable reports whether analysis found any condition that blocks automatic migration.
func (r Result) Migratable() bool {
	return r.BlockingCount() == 0
}

// BlockingCount returns the number of error-severity findings.
func (r Result) BlockingCount() int {
	count := 0
	for _, finding := range r.Findings {
		if finding.Severity == SeverityError {
			count++
		}
	}
	return count
}

// WarningCount returns the number of non-blocking warning findings.
func (r Result) WarningCount() int {
	count := 0
	for _, finding := range r.Findings {
		if finding.Severity == SeverityWarning {
			count++
		}
	}
	return count
}

func (r *Result) sort() {
	sort.Strings(r.Modules)
	sort.Strings(r.Files)
	sort.Slice(r.ProviderRequirements, func(i, j int) bool {
		left, right := r.ProviderRequirements[i], r.ProviderRequirements[j]
		if lessRange(left.Range, right.Range) {
			return true
		}
		if lessRange(right.Range, left.Range) {
			return false
		}
		return left.LocalName < right.LocalName
	})
	sort.Slice(r.Actions, func(i, j int) bool {
		left, right := r.Actions[i], r.Actions[j]
		if lessRange(left.Range, right.Range) {
			return true
		}
		if lessRange(right.Range, left.Range) {
			return false
		}
		if left.RuleID != right.RuleID {
			return left.RuleID < right.RuleID
		}
		return left.From < right.From
	})
	sort.Slice(r.References, func(i, j int) bool {
		left, right := r.References[i], r.References[j]
		if lessRange(left.Range, right.Range) {
			return true
		}
		if lessRange(right.Range, left.Range) {
			return false
		}
		if left.From != right.From {
			return left.From < right.From
		}
		return left.To < right.To
	})
	sort.Slice(r.Findings, func(i, j int) bool {
		left, right := r.Findings[i], r.Findings[j]
		if lessRange(left.Range, right.Range) {
			return true
		}
		if lessRange(right.Range, left.Range) {
			return false
		}
		if left.RuleID != right.RuleID {
			return left.RuleID < right.RuleID
		}
		if left.Severity != right.Severity {
			return left.Severity < right.Severity
		}
		return left.Message < right.Message
	})
}

func lessRange(left, right hcl.Range) bool {
	if left.Filename != right.Filename {
		return left.Filename < right.Filename
	}
	if left.Start.Line != right.Start.Line {
		return left.Start.Line < right.Start.Line
	}
	if left.Start.Column != right.Start.Column {
		return left.Start.Column < right.Start.Column
	}
	if left.End.Line != right.End.Line {
		return left.End.Line < right.End.Line
	}
	return left.End.Column < right.End.Column
}
