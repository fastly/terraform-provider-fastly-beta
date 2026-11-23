package migrate

import (
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/compatibility"
	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/rewrite"
)

// serviceAutoTypes maps each legacy aggregate service type to its _auto target.
var serviceAutoTypes = map[string]string{
	"fastly_service_vcl":     "fastly_service_cdn_auto",
	"fastly_service_compute": "fastly_service_compute_auto",
}

// versionlessResourceTypes maps legacy versionless resources to their target
// type. fastly_domain keeps its name, so only its references are rewritten.
var versionlessResourceTypes = map[string]string{
	"fastly_domain":                 "fastly_domain",
	"fastly_domain_v1":              "fastly_domain",
	"fastly_domain_v1_service_link": "fastly_domain_service_link",
	"fastly_compute_acl":            "fastly_acl",
	"fastly_compute_acl_entries":    "fastly_acl_entries",
	"fastly_configstore_entries":    "fastly_configstore_items",
	"fastly_service_acl_entries":    "fastly_service_cdn_acl_entries",
}

func compileResource(
	key resourceKey,
	actions []selectedAction,
	index moduleIndex,
) ([]rewrite.Edit, *Move, []Issue) {
	if target, ok := versionlessResourceTypes[key.typ]; ok {
		return compileVersionlessResource(key, target, actions, index)
	}
	var issues []Issue
	issue := func(a selectedAction, message string) {
		issues = append(issues, Issue{
			RuleID:      a.rule.ID,
			File:        key.file,
			Message:     message,
			Remediation: "Implement a reviewed HCL mapping or migrate this construct manually.",
		})
	}

	if strings.HasPrefix(key.typ, "fastly_ngwaf_") {
		return compileNGWAFResource(key, actions, index)
	}
	autoType, supportedService := serviceAutoTypes[key.typ]
	if !supportedService {
		for _, a := range actions {
			issue(a, fmt.Sprintf("no dry-run writer is implemented for resource %s", key.typ))
		}
		return nil, nil, issues
	}

	module := filepath.ToSlash(filepath.Dir(key.file))
	fromAddress := key.typ + "." + key.name
	toAddress := autoType + "." + key.name
	issues = append(issues, moveIssues(key, module, fromAddress, toAddress, index)...)

	// Nested HCL operations must run while parent blocks still have legacy
	// names. Resource and block renames are applied last.
	sort.Slice(actions, func(i, j int) bool {
		if actions[i].rule.Kind != actions[j].rule.Kind {
			return rankKind(actions[i].rule.Kind) < rankKind(actions[j].rule.Kind)
		}
		return actions[i].rule.Legacy < actions[j].rule.Legacy
	})

	productEdits, productIssues := compileSimpleProducts(key, actions, index)
	issues = append(issues, productIssues...)
	var edits []rewrite.Edit
	edits = append(edits, productEdits...)
	// Group object fields before applying any individual attribute edits.
	// Each backend may configure a different subset of the selected fields.
	var fields []rewrite.ObjectField
	for _, action := range actions {
		if field, ok := backendTLSField(action.rule, key.typ, autoType); ok && !slices.Contains(fields, field) {
			fields = append(fields, field)
		}
	}
	if len(fields) != 0 {
		edits = append(edits, rewrite.NestObject{
			File: key.file, ResourceType: key.typ, ResourceName: key.name,
			Parents: []string{"backend"}, Target: "ssl_client_secrets", Fields: fields,
		})
	}
	foundResource := false
	for _, a := range actions {
		rule := a.rule
		switch rule.Kind {
		case compatibility.ConstructAttribute:
			if _, ok := backendTLSField(rule, key.typ, autoType); ok {
				continue // grouped above, not one object per field
			}
			edit, err := compileAttribute(key, rule, autoType)
			if err != nil {
				issue(a, err.Error())
			} else if edit != nil {
				edits = append(edits, edit)
			}
		case compatibility.ConstructServiceBlock:
			edit, err := compileServiceBlock(key, rule, autoType)
			if err != nil {
				issue(a, err.Error())
			} else if edit != nil {
				edits = append(edits, edit)
			}
		case compatibility.ConstructResource:
			if rule.Legacy != key.typ || rule.Target != autoType ||
				rule.HCL != compatibility.HCLReshape || rule.State != compatibility.StateMove {
				issue(a, fmt.Sprintf("unimplemented resource conversion %s -> %s", rule.Legacy, rule.Target))
				continue
			}
			foundResource = true
			edits = append(edits, rewrite.RenameResource{
				File: key.file, FromType: key.typ, ToType: rule.Target, Name: key.name,
			})
		case compatibility.ConstructProduct:
			// Compiled together by compileSimpleProducts so the aggregate is removed exactly once.
		default:
			issue(a, fmt.Sprintf("unrecognized compatibility rule kind %q", rule.Kind))
		}
	}
	if !foundResource {
		issues = append(issues, Issue{
			File: key.file, Message: fmt.Sprintf("resource %s has no supported service conversion action", fromAddress),
			Remediation: "Review the compatibility registry; the migration must include the complete service conversion.",
		})
	}
	if len(issues) != 0 {
		return nil, nil, issues
	}

	// Terraform moved addresses are module-local; never write child-module
	// moved blocks into the root module.
	migrationFile := filepath.ToSlash(filepath.Join(module, "migration.tf"))
	if _, exists := index.moves[module][fromAddress]; !exists {
		edits = append(edits, rewrite.AddMoved{
			File: migrationFile, From: fromAddress, To: toAddress,
		})
	}
	move := &Move{Module: module, From: fromAddress, To: toAddress}
	return edits, move, nil
}

// backendTLSField authorizes only the exact automatic backend routes already
// reviewed in the registry. Other object destinations need their own schema
// and state validation before using the generic writer.
func backendTLSField(rule compatibility.Rule, legacyType, targetType string) (rewrite.ObjectField, bool) {
	if rule.Kind != compatibility.ConstructAttribute ||
		rule.Support != compatibility.SupportAutomatic || rule.HCL != compatibility.HCLReshape ||
		rule.State != compatibility.StateMove {
		return rewrite.ObjectField{}, false
	}
	for _, name := range []string{"ssl_client_cert", "ssl_client_key"} {
		if rule.Legacy == legacyType+".backend."+name &&
			rule.Target == targetType+".backend.ssl_client_secrets."+name {
			return rewrite.ObjectField{From: name, To: name}, true
		}
	}
	return rewrite.ObjectField{}, false
}

func rankKind(kind compatibility.ConstructKind) int {
	switch kind {
	case compatibility.ConstructAttribute:
		return 0
	case compatibility.ConstructServiceBlock:
		return 1
	case compatibility.ConstructResource:
		return 2
	case compatibility.ConstructProduct:
		return 3
	default:
		return 4
	}
}

func compileAttribute(key resourceKey, rule compatibility.Rule, autoType string) (rewrite.Edit, error) {
	source := strings.Split(rule.Legacy, ".")
	if len(source) < 2 || source[0] != key.typ {
		return nil, fmt.Errorf("invalid legacy attribute route %q", rule.Legacy)
	}
	parents := source[1 : len(source)-1]
	attr := source[len(source)-1]

	switch rule.HCL {
	case compatibility.HCLRemove:
		if rule.Target != "" {
			return nil, fmt.Errorf("removal rule %q unexpectedly has a target", rule.ID)
		}
		return rewrite.RemoveAttribute{
			File: key.file, ResourceType: key.typ, ResourceName: key.name,
			Parents: parents, Attribute: attr,
		}, nil
	case compatibility.HCLRename, compatibility.HCLReshape:
		target := strings.Split(rule.Target, ".")
		if len(target) < 2 || target[0] != autoType {
			return nil, fmt.Errorf("invalid target for attribute %q", rule.Legacy)
		}
		targetPath := target[1:]
		switch rule.HCL {
		case compatibility.HCLRename:
			// The current provider declares the target rate_limiter_id
			// Computed-only. Renaming a configured legacy value would make
			// the generated HCL invalid even though the registry route is
			// automatic; defer this to the dedicated rate-limiter migration.
			if rule.ID == "cdn-rate-limiter-id" {
				return nil, fmt.Errorf("rate_limiter_id is computed-only on the target provider")
			}
			if len(targetPath) != len(source)-1 ||
				!slices.Equal(targetPath[:len(targetPath)-1], parents) {
				return nil, fmt.Errorf("attribute %q requires an unsupported rename path", rule.Legacy)
			}
			return rewrite.RenameAttribute{
				File: key.file, ResourceType: key.typ, ResourceName: key.name,
				Parents: parents, From: attr, To: targetPath[len(targetPath)-1],
			}, nil
		case compatibility.HCLReshape:
			// Only the top-level CDN settings fields move into an HCL block.
			// Backend TLS is grouped by the compiler before this function.
			// Other object routes (e.g. logging) remain blocked until their
			// schema/state mappings have been reviewed.
			if len(parents) != 0 || len(targetPath) != 2 ||
				targetPath[0] != "settings" {
				return nil, fmt.Errorf("attribute %q requires an object-attribute writer, not an HCL block", rule.Legacy)
			}
			if len(targetPath) != len(source) ||
				!slices.Equal(targetPath[:len(parents)], parents) {
				return nil, fmt.Errorf("attribute %q requires an unsupported multi-level reshape", rule.Legacy)
			}
			return rewrite.NestAttribute{
				File: key.file, ResourceType: key.typ, ResourceName: key.name,
				Parents: parents, Attribute: attr,
				TargetBlock:     targetPath[len(parents)],
				TargetAttribute: targetPath[len(targetPath)-1],
			}, nil
		case compatibility.HCLUnchanged, compatibility.HCLSplit, compatibility.HCLRemove, compatibility.HCLReject:
			return nil, fmt.Errorf("attribute %q requires an unsupported nesting strategy", rule.Legacy)
		default:
			return nil, fmt.Errorf("attribute %q requires an unsupported nesting strategy", rule.Legacy)
		}
	case compatibility.HCLUnchanged:
		return nil, nil
	case compatibility.HCLSplit, compatibility.HCLReject:
		return nil, fmt.Errorf("attribute %q uses unsupported HCL strategy %s", rule.Legacy, rule.HCL)
	default:
		return nil, fmt.Errorf("attribute %q uses unsupported HCL strategy %s", rule.Legacy, rule.HCL)
	}
}

func compileServiceBlock(key resourceKey, rule compatibility.Rule, autoType string) (rewrite.Edit, error) {
	source := strings.Split(rule.Legacy, ".")
	target := strings.Split(rule.Target, ".")
	if len(source) != 2 || source[0] != key.typ ||
		len(target) != 2 || target[0] != autoType {
		return nil, fmt.Errorf("service block %q requires an unsupported transformation", rule.Legacy)
	}
	switch rule.HCL {
	case compatibility.HCLRename:
		return rewrite.RenameBlock{
			File: key.file, ResourceType: key.typ, ResourceName: key.name,
			From: source[1], To: target[1],
		}, nil
	case compatibility.HCLReshape, compatibility.HCLUnchanged:
		// Same block name and scope; nested attribute rules above implement
		// every necessary known field-level change. The rule remains the
		// authority for the rest of the block's compatible shape.
		if source[1] == target[1] {
			return nil, nil
		}
	case compatibility.HCLSplit, compatibility.HCLRemove, compatibility.HCLReject:
		return nil, fmt.Errorf("service block %q uses unsupported HCL strategy %s", rule.Legacy, rule.HCL)
	default:
		return nil, fmt.Errorf("service block %q uses unsupported HCL strategy %s", rule.Legacy, rule.HCL)
	}
	return nil, fmt.Errorf("service block %q is not supported by the dry-run writer", rule.Legacy)
}

func compileVersionlessResource(
	key resourceKey,
	target string,
	actions []selectedAction,
	index moduleIndex,
) ([]rewrite.Edit, *Move, []Issue) {
	var issues []Issue
	issue := func(a selectedAction, message string) {
		issues = append(issues, Issue{
			RuleID:      a.rule.ID,
			File:        key.file,
			Message:     message,
			Remediation: "Implement a reviewed HCL mapping or migrate this construct manually.",
		})
	}

	module := filepath.ToSlash(filepath.Dir(key.file))
	fromAddress := key.typ + "." + key.name
	toAddress := target + "." + key.name
	renamed := key.typ != target
	if index.resources[module][fromAddress] != key.file {
		issues = append(issues, Issue{
			File: key.file, Message: fmt.Sprintf("resource %s was not found at its analyzed location", fromAddress),
			Remediation: "Check for conflicting or modified Terraform resource declarations.",
		})
	}

	foundResource := false
	var attributeEdits []rewrite.Edit
	for _, a := range actions {
		rule := a.rule
		if rule.Kind == compatibility.ConstructAttribute && renamed {
			edit, err := compileAttribute(key, rule, target)
			if err != nil {
				issue(a, err.Error())
			} else if edit != nil {
				attributeEdits = append(attributeEdits, edit)
			}
			continue
		}
		wantHCL, wantState := compatibility.HCLUnchanged, compatibility.StateUpgrade
		if renamed {
			wantHCL, wantState = compatibility.HCLRename, compatibility.StateMove
		}
		if rule.Kind != compatibility.ConstructResource || rule.Legacy != key.typ || rule.Target != target ||
			rule.HCL != wantHCL || rule.State != wantState {
			issue(a, fmt.Sprintf("unimplemented resource conversion %s -> %s", rule.Legacy, rule.Target))
			continue
		}
		foundResource = true
	}
	if !foundResource {
		issues = append(issues, Issue{
			File: key.file, Message: fmt.Sprintf("resource %s has no supported conversion action", fromAddress),
			Remediation: "Review the compatibility registry; the migration must include the resource conversion.",
		})
	}

	if renamed {
		if file := index.resources[module][toAddress]; file != "" {
			issues = append(issues, Issue{
				File: key.file, Message: fmt.Sprintf("target resource %s already exists in %s", toAddress, file),
				Remediation: "Resolve the target address collision before migration.",
			})
		}
		if existingTo, found := index.moves[module][fromAddress]; found && existingTo != toAddress {
			issues = append(issues, Issue{
				File: key.file, Message: fmt.Sprintf("existing moved block for %s targets %s, not %s", fromAddress, existingTo, toAddress),
				Remediation: "Resolve conflicting moved blocks before migration.",
			})
		}
		for previousFrom, existingTo := range index.moves[module] {
			if existingTo == toAddress && previousFrom != fromAddress {
				issues = append(issues, Issue{
					File: key.file, Message: fmt.Sprintf("target %s is already used by a moved block from %s", toAddress, previousFrom),
					Remediation: "Resolve conflicting moved blocks before migration.",
				})
			}
		}
	}
	if len(issues) != 0 {
		return nil, nil, issues
	}

	if !renamed {
		return nil, &Move{Module: module, From: fromAddress, To: toAddress, ReferenceOnly: true}, nil
	}

	// Attribute edits run against the legacy resource type; rename it last.
	edits := append(attributeEdits, rewrite.RenameResource{
		File: key.file, FromType: key.typ, ToType: target, Name: key.name,
	})
	if _, exists := index.moves[module][fromAddress]; !exists {
		edits = append(edits, rewrite.AddMoved{
			File: filepath.ToSlash(filepath.Join(module, "migration.tf")), From: fromAddress, To: toAddress,
		})
	}
	return edits, &Move{Module: module, From: fromAddress, To: toAddress}, nil
}

// moveIssues reports resource and moved-block collisions for one planned move.
func moveIssues(key resourceKey, module, fromAddress, toAddress string, index moduleIndex) []Issue {
	var issues []Issue
	if index.resources[module][fromAddress] != key.file {
		issues = append(issues, Issue{
			File: key.file, Message: fmt.Sprintf("resource %s was not found at its analyzed location", fromAddress),
			Remediation: "Check for conflicting or modified Terraform resource declarations.",
		})
	}
	if file := index.resources[module][toAddress]; file != "" {
		issues = append(issues, Issue{
			File: key.file, Message: fmt.Sprintf("target resource %s already exists in %s", toAddress, file),
			Remediation: "Resolve the target address collision before migration.",
		})
	}
	if existingTo, found := index.moves[module][fromAddress]; found && existingTo != toAddress {
		issues = append(issues, Issue{
			File: key.file, Message: fmt.Sprintf("existing moved block for %s targets %s, not %s", fromAddress, existingTo, toAddress),
			Remediation: "Resolve conflicting moved blocks before migration.",
		})
	}
	for previousFrom, existingTo := range index.moves[module] {
		if existingTo == toAddress && previousFrom != fromAddress {
			issues = append(issues, Issue{
				File: key.file, Message: fmt.Sprintf("target %s is already used by a moved block from %s", toAddress, previousFrom),
				Remediation: "Resolve conflicting moved blocks before migration.",
			})
		}
	}
	return issues
}
