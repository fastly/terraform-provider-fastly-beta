package analyze

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

const (
	lockfileName          = ".terraform.lock.hcl"
	legacyProviderAddress = "registry.terraform.io/fastly/fastly"
)

// The lockfile is only a selection record: it does not prove which release last
// wrote the state, and a required_providers range proves nothing.
func (a *Analyzer) checkSourceProviderVersion(state *analysisState) {
	floor, ceiling, rule := a.sourceVersionRange(state)
	if floor == nil {
		return
	}

	lockPath := filepath.Join(state.root, lockfileName)
	selected, problem := selectedLegacyVersion(lockPath)
	switch {
	case problem != "":
		state.result.Findings = append(state.result.Findings, Finding{
			Severity: SeverityError,
			RuleID:   rule,
			Range:    fileStartRange(lockPath),
			Message:  fmt.Sprintf("The legacy fastly/fastly provider version cannot be established: %s.", problem),
			Remediation: fmt.Sprintf("Select a legacy provider release within %s (run terraform init, or terraform init -upgrade "+
				"yourself if needed; the migration CLI never upgrades the provider) and keep the resulting %s before migrating. "+
				"Then run terraform apply -refresh-only so the state is persisted at the schema this migration requires.",
				versionRange(floor, ceiling), lockfileName),
		})
	case selected.LessThan(floor):
		state.result.Findings = append(state.result.Findings, Finding{
			Severity: SeverityError,
			RuleID:   rule,
			Range:    fileStartRange(lockPath),
			Message: fmt.Sprintf("The selected legacy fastly/fastly provider %s is older than %s, which is the first release "+
				"that persists the fastly_service_vcl state schema this migration supports.", selected, floor),
			Remediation: upgradeRemediation(state, floor, ceiling),
		})
	case ceiling != nil && !selected.LessThan(ceiling):
		state.result.Findings = append(state.result.Findings, Finding{
			Severity: SeverityError,
			RuleID:   rule,
			Range:    fileStartRange(lockPath),
			Message: fmt.Sprintf("The selected fastly/fastly provider %s is not a legacy SDKv2 release: %s and newer is the "+
				"dual-model provider, which is the migration target rather than a source.", selected, ceiling),
			Remediation: fmt.Sprintf("Constrain the legacy provider to %s, run terraform init -upgrade intentionally to select it, "+
				"run terraform apply -refresh-only, and keep the updated %s before migrating.", versionRange(floor, ceiling), lockfileName),
		})
	}
}

func versionRange(floor, ceiling *version.Version) string {
	if ceiling == nil {
		return fmt.Sprintf("%s or newer", floor)
	}
	return fmt.Sprintf(">= %s, < %s", floor, ceiling)
}

func (a *Analyzer) sourceVersionRange(state *analysisState) (floor, ceiling *version.Version, ruleID string) {
	for _, action := range state.result.Actions {
		rule, ok := a.registry.RuleByID(action.RuleID)
		if !ok || rule.MinSourceProviderVersion == "" {
			continue
		}
		v, err := version.NewSemver(rule.MinSourceProviderVersion)
		if err != nil {
			continue
		}
		if floor == nil || v.GreaterThan(floor) {
			floor, ruleID = v, rule.ID
		}
		if rule.MaxSourceProviderVersion == "" {
			continue
		}
		if m, err := version.NewSemver(rule.MaxSourceProviderVersion); err == nil && (ceiling == nil || m.LessThan(ceiling)) {
			ceiling = m
		}
	}
	return floor, ceiling, ruleID
}

func upgradeRemediation(state *analysisState, floor, ceiling *version.Version) string {
	text := fmt.Sprintf("Constrain the legacy fastly/fastly provider to %s, run terraform init -upgrade intentionally to select it, "+
		"run terraform apply -refresh-only, and keep the updated %s before migrating.", versionRange(floor, ceiling), lockfileName)
	for _, requirement := range state.result.ProviderRequirements {
		if requirement.Version == "" {
			continue
		}
		constraints, err := version.NewConstraint(requirement.Version)
		if err != nil || constraints.Check(floor) {
			continue
		}
		text += fmt.Sprintf(" The required_providers constraint %q excludes %s; relax it first.", requirement.Version, floor)
		break
	}
	return text
}

func selectedLegacyVersion(path string) (*version.Version, string) {
	source, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, lockfileName + " is missing"
	}
	if err != nil {
		return nil, fmt.Sprintf("%s cannot be read (%v)", lockfileName, err)
	}
	file, diags := hclsyntax.ParseConfig(source, path, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() || file == nil {
		return nil, lockfileName + " is malformed"
	}
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return nil, lockfileName + " is malformed"
	}
	for _, block := range body.Blocks {
		if block.Type != "provider" || len(block.Labels) != 1 || block.Labels[0] != legacyProviderAddress {
			continue
		}
		attr := block.Body.Attributes["version"]
		if attr == nil {
			return nil, lockfileName + " has no version for the legacy provider"
		}
		value, diags := attr.Expr.Value(nil)
		if diags.HasErrors() || !value.IsKnown() || value.IsNull() || value.Type() != cty.String {
			return nil, lockfileName + " has an unreadable version for the legacy provider"
		}
		selected, err := version.NewSemver(value.AsString())
		if err != nil {
			return nil, fmt.Sprintf("%s records an invalid version %q", lockfileName, value.AsString())
		}
		return selected, ""
	}
	return nil, lockfileName + " has no entry for " + legacyProviderAddress
}
