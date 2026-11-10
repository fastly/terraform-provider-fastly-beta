package migrate

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"

	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/analyze"
	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/compatibility"
	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/rewrite"
)

const gaVersionConstraint = ">= 20.0.0"

// compileProviderMigration handles only the original SDKv2 provider address.
// The original state address remains fastly/fastly: neither -legacy nor -beta
// is a valid target for the GA source-preserving transition.
func compileProviderMigration(analysis analyze.Result, index moduleIndex) ([]rewrite.Edit, []Issue) {
	if len(analysis.Actions) == 0 {
		return nil, nil
	}
	type fileBlock struct {
		file string
	}
	type moduleInfo struct {
		active      bool
		required    []fileBlock
		terraform   []fileBlock
		providers   map[string]bool
		aliases     map[string]string
		requirement []analyze.ProviderRequirement
	}
	modules := make(map[string]*moduleInfo)
	infoFor := func(module string) *moduleInfo {
		if modules[module] == nil {
			modules[module] = &moduleInfo{
				providers: make(map[string]bool),
				aliases:   make(map[string]string),
			}
		}
		return modules[module]
	}
	migratingVCL := false
	for _, action := range analysis.Actions {
		if action.Kind == compatibility.ConstructResource && strings.HasPrefix(action.From, "fastly_service_vcl.") {
			migratingVCL = true
			file := displayPath(analysis.Root, action.Range.Filename)
			infoFor(filepath.ToSlash(filepath.Dir(file))).active = true
		}
	}
	if !migratingVCL {
		return nil, nil
	}
	var issues []Issue
	addIssue := func(file, message, remediation string) {
		issues = append(issues, Issue{File: file, Message: message, Remediation: remediation})
	}
	for _, requirement := range analysis.ProviderRequirements {
		file := displayPath(analysis.Root, requirement.Range.Filename)
		module := filepath.ToSlash(filepath.Dir(file))
		infoFor(module).requirement = append(infoFor(module).requirement, requirement)
	}

	files := make([]string, 0, len(index.snapshots))
	for filename := range index.snapshots {
		files = append(files, filename)
	}
	sort.Strings(files)
	for _, filename := range files {
		source := index.snapshots[filename]
		parsed, diags := hclsyntax.ParseConfig(source, filename, hcl.Pos{Line: 1, Column: 1})
		if diags.HasErrors() || parsed == nil {
			addIssue(filename, "cannot inspect provider declarations", "Fix invalid HCL before migration.")
			continue
		}
		body, ok := parsed.Body.(*hclsyntax.Body)
		if !ok {
			addIssue(filename, "unexpected provider configuration body", "Use native Terraform HCL before migration.")
			continue
		}
		module := filepath.ToSlash(filepath.Dir(filename))
		info := infoFor(module)
		for _, block := range body.Blocks {
			switch block.Type {
			case "terraform":
				info.terraform = append(info.terraform, fileBlock{file: filename})
				for _, child := range block.Body.Blocks {
					if child.Type == "required_providers" {
						info.required = append(info.required, fileBlock{file: filename})
					}
				}
			case "provider":
				if len(block.Labels) != 1 {
					continue
				}
				switch block.Labels[0] {
				case "fastly":
					info.providers[filename] = true
					alias := ""
					if configured := block.Body.Attributes["alias"]; configured != nil {
						value, diagnostics := configured.Expr.Value(nil)
						if diagnostics.HasErrors() || !value.IsKnown() || value.IsNull() || value.Type() != cty.String {
							addIssue(filename, "Fastly provider alias cannot be determined statically",
								"Use a literal alias so provider configuration can be checked.")
						} else {
							alias = value.AsString()
						}
					}
					if previous, exists := info.aliases[alias]; exists {
						addIssue(filename, fmt.Sprintf("duplicate Fastly provider alias %q (also in %s)", alias, previous),
							"Keep a single provider configuration per alias in each Terraform module.")
					} else {
						info.aliases[alias] = filename
					}
					for name := range block.Body.Attributes {
						switch name {
						case "alias", "api_key", "api_token", "base_url", "api_endpoint":
							// Supported unchanged or by a source-preserving key rename.
						case "force_http2", "no_auth":
							addIssue(filename, fmt.Sprintf("legacy provider argument %q is unsupported by the dual-model provider", name),
								"Remove or manually replace this setting after checking its behavior; no configuration will be written.")
						default:
							addIssue(filename, fmt.Sprintf("Fastly provider argument %q cannot be migrated safely", name),
								"Review this provider argument against the GA dual-model provider schema.")
						}
					}
					if len(block.Body.Blocks) != 0 {
						addIssue(filename, "Fastly provider contains unsupported nested blocks",
							"Remove or migrate these blocks manually before using --write.")
					}
					if block.Body.Attributes["api_key"] != nil && block.Body.Attributes["api_token"] != nil {
						addIssue(filename, "Fastly provider sets both api_key and api_token",
							"Keep one credential setting; the GA provider uses api_token.")
					}
					if block.Body.Attributes["base_url"] != nil && block.Body.Attributes["api_endpoint"] != nil {
						addIssue(filename, "Fastly provider sets both base_url and api_endpoint",
							"Keep one endpoint setting; the GA provider uses api_endpoint.")
					}
					if legacy := block.Body.Attributes["base_url"]; legacy != nil {
						value, diagnostics := legacy.Expr.Value(nil)
						if diagnostics.HasErrors() || !value.IsKnown() || value.IsNull() || value.Type() != cty.String ||
							!absoluteHTTPSURL(value.AsString()) {
							addIssue(filename, "base_url cannot be proven compatible with the GA api_endpoint",
								"Use a literal absolute https:// URL in base_url, or migrate this expression and validate its value manually.")
						}
					}
				case "fastly-beta":
					addIssue(filename, "fastly-beta provider configuration cannot be treated as legacy SDKv2",
						"Keep beta provider migration separate from this legacy-to-GA migration.")
				}
			}
		}
	}

	orderedModules := make([]string, 0, len(modules))
	for module := range modules {
		orderedModules = append(orderedModules, module)
	}
	sort.Strings(orderedModules)
	var edits []rewrite.Edit
	for _, module := range orderedModules {
		info := modules[module]
		if !info.active && len(info.requirement) == 0 && len(info.providers) == 0 {
			continue
		}
		if len(info.required) > 1 {
			// Report the second declaration, which introduces the conflict.
			addIssue(info.required[1].file, "module contains multiple required_providers blocks",
				"Consolidate the declarations into one required_providers block before migrating.")
			continue
		}
		if len(info.requirement) > 1 {
			addIssue(displayPath(analysis.Root, info.requirement[1].Range.Filename),
				"module contains multiple Fastly provider requirements",
				"Keep one unambiguous fastly/fastly requirement per module.")
			continue
		}
		if len(info.requirement) == 1 {
			requirement := info.requirement[0]
			file := displayPath(analysis.Root, requirement.Range.Filename)
			if requirement.LocalName != "fastly" {
				addIssue(file, fmt.Sprintf("Fastly provider local name %q is not supported for automatic rewriting", requirement.LocalName),
					"Use the standard fastly local provider name before migrating; provider aliases remain supported.")
			} else if requirement.Source != "fastly/fastly" &&
				requirement.Source != "registry.terraform.io/fastly/fastly" &&
				requirement.Source != "<implicit>" {
				addIssue(file, fmt.Sprintf("provider source %q is not the legacy fastly/fastly provider", requirement.Source),
					"Do not replace a beta, legacy-renamed, or unrelated provider source automatically.")
			} else if err := validateProviderConstraint(requirement.Version); err != nil {
				addIssue(file, fmt.Sprintf("Fastly provider constraint %q cannot be safely rewritten: %v", requirement.Version, err),
					"Review the constraint and use a supported legacy requirement before migrating to >= 20.0.0.")
			} else {
				edits = append(edits, rewrite.UpdateProviderRequirement{File: file, LocalName: "fastly"})
			}
		} else {
			// The GA provider must have an explicit source/version declaration.
			// Avoid silently relying on the default hashicorp/fastly source.
			filename := filepath.ToSlash(filepath.Join(module, "migration.tf"))
			if len(info.required) != 0 {
				filename = info.required[0].file
			} else if len(info.terraform) != 0 {
				filename = info.terraform[0].file
			}
			edits = append(edits, rewrite.EnsureProviderRequirement{File: filename})
		}
		names := make([]string, 0, len(info.providers))
		for filename := range info.providers {
			names = append(names, filename)
		}
		sort.Strings(names)
		for _, filename := range names {
			edits = append(edits, rewrite.RenameProviderArguments{File: filename, Name: "fastly"})
		}
	}
	return edits, issues
}

func absoluteHTTPSURL(input string) bool {
	if !strings.HasPrefix(input, "https://") {
		return false
	}
	parsed, err := url.Parse(input)
	return err == nil && parsed.Scheme == "https" && parsed.Host != ""
}

// Replacing a legacy constraint is deliberate, but never silently drop an
// exclusion or an already-GA-specific pin/ceiling that the user may need.
func validateProviderConstraint(raw string) error {
	if raw == "" {
		return nil
	}
	if strings.Contains(raw, "!=") {
		return fmt.Errorf("explicit version exclusions require manual review")
	}
	constraints, err := version.NewConstraint(raw)
	if err != nil {
		return fmt.Errorf("invalid Terraform provider version constraint: %w", err)
	}
	legacyAllowed, targetAllowed := constraintCompatibleRanges(raw, constraints)
	if !legacyAllowed && !targetAllowed {
		return fmt.Errorf("version requirements appear contradictory or incompatible with the supported release ranges")
	}
	if strings.TrimSpace(raw) == gaVersionConstraint ||
		strings.TrimSpace(raw) == ">= 20" ||
		strings.TrimSpace(raw) == ">= 20.0" {
		return nil
	}
	// GA-only constraints may encode an exact pin or an upper limit: do
	// not silently broaden them to an unconstrained >= 20.0.0.
	if !legacyAllowed && targetAllowed {
		return fmt.Errorf("GA-only constraint has additional user restrictions")
	}
	return nil
}

var versionNumberPattern = regexp.MustCompile(`\d+(?:\.\d+){0,2}`)

// A conservative satisfiability probe. Terraform's constraint grammar is
// broader than any reasonable static guess: if we cannot find a witness at
// the literal endpoints or nearby releases, we block instead of replacing a
// potentially contradictory constraint. We never infer the selected version.
func constraintCompatibleRanges(raw string, constraints version.Constraints) (legacyAllowed, targetAllowed bool) {
	candidates := []string{"9.1.0", "9.8.0", "10.1.0", "19.9.9", "20.0.0", "20.1.0", "21.0.0"}
	for _, literal := range versionNumberPattern.FindAllString(raw, -1) {
		parts := strings.Split(literal, ".")
		for len(parts) < 3 {
			parts = append(parts, "0")
		}
		major, _ := strconv.Atoi(parts[0])
		minor, _ := strconv.Atoi(parts[1])
		patch, _ := strconv.Atoi(parts[2])
		candidates = append(candidates,
			fmt.Sprintf("%d.%d.%d", major, minor, patch),
			fmt.Sprintf("%d.%d.%d", major, minor, patch+1),
			fmt.Sprintf("%d.%d.0", major, minor+1),
			fmt.Sprintf("%d.0.0", major+1),
		)
	}
	floor, _ := version.NewSemver("9.1.0")
	ceiling, _ := version.NewSemver("20.0.0")
	for _, candidate := range candidates {
		v, err := version.NewSemver(candidate)
		if err != nil || !constraints.Check(v) {
			continue
		}
		if !v.LessThan(floor) && v.LessThan(ceiling) {
			legacyAllowed = true
		}
		if !v.LessThan(ceiling) {
			targetAllowed = true
		}
	}
	return legacyAllowed, targetAllowed
}
