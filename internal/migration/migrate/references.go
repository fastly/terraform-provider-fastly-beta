package migrate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"

	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/compatibility"
	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/rewrite"
)

// compileReferences derives the reference map exclusively from resource moves
// the compiler has accepted. The registry is not permission to migrate a
// blocked resource, even when its reference mapping is known.
func compileReferences(registry *compatibility.Registry, index moduleIndex, moves []Move) ([]rewrite.References, []Issue) {
	if len(moves) == 0 {
		return nil, nil
	}
	routes := make(map[string]map[string]rewrite.ReferenceTarget)
	all := make(map[string]bool)
	for _, move := range moves {
		file := index.resources[move.Module][move.From]
		body, diags := hclsyntax.ParseConfig(index.snapshots[file], file, hcl.Pos{Line: 1, Column: 1})
		if diags.HasErrors() || body == nil {
			return nil, []Issue{{File: file, Message: "cannot inspect reference source resource", Remediation: "Rerun analysis on valid HCL."}}
		}
		typ, name, _ := strings.Cut(move.From, ".")
		found := false
		for _, block := range body.Body.(*hclsyntax.Body).Blocks {
			if block.Type == "resource" && len(block.Labels) == 2 && block.Labels[0] == typ && block.Labels[1] == name {
				if routes[move.Module] == nil {
					routes[move.Module] = make(map[string]rewrite.ReferenceTarget)
				}
				routes[move.Module][move.From] = rewrite.ResourceReferenceTarget(move.To, block)
				all[move.From], all[move.To] = true, true
				found = true
				break
			}
		}
		if !found {
			return nil, []Issue{{File: file, Message: fmt.Sprintf("reference source %s is missing", move.From), Remediation: "Rerun analysis on unchanged HCL."}}
		}
	}

	files := make([]string, 0, len(index.snapshots))
	for file := range index.snapshots {
		files = append(files, file)
	}
	sort.Strings(files)
	var edits []rewrite.References
	var issues []Issue
	for _, file := range files {
		parsed, diags := hclsyntax.ParseConfig(index.snapshots[file], file, hcl.Pos{Line: 1, Column: 1})
		if diags.HasErrors() || parsed == nil {
			issues = append(issues, Issue{File: file, Message: "cannot inspect reference file", Remediation: "Fix invalid HCL before migration."})
			continue
		}
		edit := rewrite.References{File: file, Routes: routes[moduleForFile(file)], AllRoutes: all, Registry: registry}
		inspection := edit.Inspect(parsed.Body.(*hclsyntax.Body))
		for _, problem := range inspection.Problems {
			issues = append(issues, Issue{
				File: file, Message: problem.Message, Remediation: "Resolve the reference or state address before requesting a migration.",
			})
		}
		// Scan all files, including those without local migrations, to
		// detect module-qualified state constructs. A no-route edit is a
		// no-op and also provides a post-render validation descriptor.
		edits = append(edits, edit)
	}
	return edits, issues
}

// verifyRenderedReferences re-inspects the final rendered HCL (including new
// migration files) against the original resource routes. This protects against
// future compiler operations reintroducing obsolete references after the first
// rewrite pass. Historic moved.from addresses are intentionally exempt.
func verifyRenderedReferences(index moduleIndex, edits []rewrite.References, changes []rewrite.Change) []Issue {
	if len(edits) == 0 {
		return nil
	}
	byModule := make(map[string]rewrite.References)
	files := make(map[string][]byte)
	for _, edit := range edits {
		byModule[moduleForFile(edit.File)] = edit
		files[edit.File] = index.snapshots[edit.File]
	}
	for _, change := range changes {
		files[change.Path] = change.After
	}
	var issues []Issue
	for file, source := range files {
		edit, found := byModule[moduleForFile(file)]
		if !found {
			continue
		}
		body, diags := hclsyntax.ParseConfig(source, file, hcl.Pos{Line: 1, Column: 1})
		if diags.HasErrors() || body == nil {
			issues = append(issues, Issue{File: file, Message: "cannot verify rendered references", Remediation: "Review the generated HCL."})
			continue
		}
		inspection := edit.Inspect(body.Body.(*hclsyntax.Body))
		if len(inspection.References) != 0 || len(inspection.Problems) != 0 {
			issues = append(issues, Issue{File: file, Message: "rendered HCL contains an obsolete or unsafe migration reference", Remediation: "Review the incomplete reference transformation; no files were modified."})
		}
	}
	sortIssues(issues)
	return issues
}
