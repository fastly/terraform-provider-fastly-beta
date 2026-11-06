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

func compileResource(
	key resourceKey,
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

	if key.typ != "fastly_service_vcl" {
		for _, a := range actions {
			issue(a, fmt.Sprintf("no dry-run writer is implemented for resource %s", key.typ))
		}
		return nil, nil, issues
	}

	module := filepath.ToSlash(filepath.Dir(key.file))
	fromAddress := key.typ + "." + key.name
	toAddress := "fastly_service_cdn_auto." + key.name
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

	// Nested HCL operations must run while parent blocks still have legacy
	// names. Resource and block renames are applied last.
	sort.Slice(actions, func(i, j int) bool {
		if actions[i].rule.Kind != actions[j].rule.Kind {
			return rankKind(actions[i].rule.Kind) < rankKind(actions[j].rule.Kind)
		}
		return actions[i].rule.Legacy < actions[j].rule.Legacy
	})

	var edits []rewrite.Edit
	foundResource := false
	for _, a := range actions {
		rule := a.rule
		switch rule.Kind {
		case compatibility.ConstructAttribute:
			edit, err := compileAttribute(key, rule)
			if err != nil {
				issue(a, err.Error())
			} else if edit != nil {
				edits = append(edits, edit)
			}
		case compatibility.ConstructServiceBlock:
			edit, err := compileServiceBlock(key, rule)
			if err != nil {
				issue(a, err.Error())
			} else if edit != nil {
				edits = append(edits, edit)
			}
		case compatibility.ConstructResource:
			if rule.ID != "resource-service-vcl" || rule.Legacy != key.typ ||
				rule.Target != "fastly_service_cdn_auto" ||
				rule.HCL != compatibility.HCLReshape || rule.State != compatibility.StateMove {
				issue(a, fmt.Sprintf("unimplemented resource conversion %s -> %s", rule.Legacy, rule.Target))
				continue
			}
			foundResource = true
			edits = append(edits, rewrite.RenameResource{
				File: key.file, FromType: key.typ, ToType: rule.Target, Name: key.name,
			})
		case compatibility.ConstructProduct:
			issue(a, fmt.Sprintf("product migration %s requires standalone resource and import generation", rule.Legacy))
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

func compileAttribute(key resourceKey, rule compatibility.Rule) (rewrite.Edit, error) {
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
		if len(target) < 2 || target[0] != "fastly_service_cdn_auto" {
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
			// Targets such as backend.ssl_client_secrets, logging.authentication,
			// and logging.tls are schema.SingleNestedAttribute objects and
			// require `key = { ... }`, not `key { ... }`. NestAttribute would
			// produce invalid target HCL for those constructs.
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

func compileServiceBlock(key resourceKey, rule compatibility.Rule) (rewrite.Edit, error) {
	source := strings.Split(rule.Legacy, ".")
	target := strings.Split(rule.Target, ".")
	if len(source) != 2 || source[0] != key.typ ||
		len(target) != 2 || target[0] != "fastly_service_cdn_auto" {
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
