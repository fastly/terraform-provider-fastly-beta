// Package migrate builds conservative, read-only migration previews.
//
// The compatibility registry describes intended mappings; this planner must
// also prove that each required HCL transformation has an implementation.
// A preview never writes configuration, invokes Terraform, or contacts Fastly.
package migrate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"

	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/analyze"
	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/compatibility"
	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/rewrite"
)

// Issue explains why a complete migration preview cannot be generated.
type Issue struct {
	RuleID      string
	File        string
	Message     string
	Remediation string
}

// Move is a Terraform-native state address movement proposed in a module.
type Move struct {
	Module string
	From   string
	To     string
}

// Result contains either a complete read-only preview or blocking diagnostics.
// When Blocked is true, Changes and Moves are always empty.
type Result struct {
	Analysis analyze.Result
	Changes  []rewrite.Change
	Moves    []Move
	Issues   []Issue

	// Keep the validated rewrite plan private: callers may inspect Changes,
	// but a write must use the exact immutable plan built during preflight.
	rewritePlan  *rewrite.Plan
	snapshots    map[string][]byte
	withholdDiff bool // recognized sensitive configuration anywhere in the analyzed module tree
}

// Blocked reports whether the preflight or planner found incomplete migration work.
func (r Result) Blocked() bool {
	return !r.Analysis.Migratable() || len(r.Issues) != 0
}

// Planner coordinates the registry, static analyzer, and source-preserving writer.
type Planner struct {
	registry *compatibility.Registry
	analyzer *analyze.Analyzer
}

// New constructs a planner using the same registry as the analyzer.
func New(registry *compatibility.Registry) *Planner {
	return &Planner{registry: registry, analyzer: analyze.New(registry)}
}

type resourceKey struct {
	file string
	typ  string
	name string
}

type selectedAction struct {
	rule compatibility.Rule
}

type moduleIndex struct {
	resources map[string]map[string]string // module -> resource address -> file
	moves     map[string]map[string]string // module -> moved.from -> moved.to
	imports   map[string]map[string]string // module -> import.to -> file
	snapshots map[string][]byte            // filename -> bytes read during preflight planning
}

// Plan runs static preflight and builds a complete in-memory preview, or blockers.
func (p *Planner) Plan(ctx context.Context, path string) (Result, error) {
	if p == nil || p.registry == nil || p.analyzer == nil {
		return Result{}, errors.New("migration planner requires a compatibility registry")
	}
	analysis, err := p.analyzer.Analyze(ctx, path)
	if err != nil {
		return Result{}, err
	}
	result := Result{Analysis: analysis}
	// An incomplete preflight cannot be turned into a purportedly safe diff.
	if !analysis.Migratable() {
		return result, nil
	}

	for _, finding := range analysis.Findings {
		// Remote module internals were not inspected by the analyzer.
		if finding.Severity == analyze.SeverityWarning &&
			finding.Code == analyze.FindingCodeRemoteModuleSource {
			result.Issues = append(result.Issues, Issue{
				File:        displayPath(analysis.Root, finding.Range.Filename),
				Message:     "remote module contents cannot be verified by the migration planner",
				Remediation: "Analyze and migrate that module separately before previewing this module tree.",
			})
		}
	}

	index, indexIssues, err := inspectFiles(ctx, analysis)
	if err != nil {
		return Result{}, err
	}
	result.Issues = append(result.Issues, indexIssues...)
	groups := make(map[resourceKey][]selectedAction)
	for _, action := range analysis.Actions {
		rule, ok := p.registry.RuleByID(action.RuleID)
		if !ok || rule.Support != compatibility.SupportAutomatic ||
			rule.Kind != action.Kind || rule.HCL != action.HCL || rule.State != action.State {
			result.Issues = append(result.Issues, Issue{
				RuleID:      action.RuleID,
				File:        displayPath(analysis.Root, action.Range.Filename),
				Message:     "analyzer action does not match an automatic compatibility rule",
				Remediation: "Review the compatibility registry and analyzer before migrating.",
			})
			continue
		}
		key, keyErr := actionKey(analysis.Root, action, rule)
		if keyErr != nil {
			result.Issues = append(result.Issues, issueFor(action, analysis.Root, keyErr.Error()))
			continue
		}
		groups[key] = append(groups[key], selectedAction{rule: rule})
	}

	keys := make([]resourceKey, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].file != keys[j].file {
			return keys[i].file < keys[j].file
		}
		if keys[i].typ != keys[j].typ {
			return keys[i].typ < keys[j].typ
		}
		return keys[i].name < keys[j].name
	})

	// Provider requirements and credentials are part of the same immutable
	// rewrite plan as service conversion. Never offer a partial preview.
	providerEdits, providerIssues := compileProviderMigration(analysis, index)
	result.Issues = append(result.Issues, providerIssues...)
	var edits []rewrite.Edit
	edits = append(edits, providerEdits...)
	var moves []Move
	for _, key := range keys {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		groupEdits, move, issues := compileResource(key, groups[key], index)
		result.Issues = append(result.Issues, issues...)
		edits = append(edits, groupEdits...)
		if move != nil {
			moves = append(moves, *move)
		}
	}
	sort.Slice(moves, func(i, j int) bool {
		if moves[i].Module != moves[j].Module {
			return moves[i].Module < moves[j].Module
		}
		return moves[i].From < moves[j].From
	})
	// Reference edits run before resource/attribute transformations, so
	// self and lifecycle paths still belong to their original resource type.
	// Build uses only successful, compiler-authorized resource moves.
	referenceEdits, referenceIssues := compileReferences(p.registry, index, moves)
	result.Issues = append(result.Issues, referenceIssues...)
	var referenceOperations []rewrite.Edit
	for _, edit := range referenceEdits {
		if len(edit.Routes) != 0 {
			referenceOperations = append(referenceOperations, edit)
		}
	}
	edits = append(referenceOperations, edits...)
	sortIssues(result.Issues)
	if len(result.Issues) != 0 {
		return result, nil // Never produce a partial diff.
	}

	plan, err := rewrite.Build(ctx, analysis.Root, edits...)
	if err != nil {
		result.Issues = []Issue{{
			File:        ".",
			Message:     fmt.Sprintf("source-preserving rewrite cannot be completed: %v", err),
			Remediation: "Review the HCL layout or implement the missing transformation; no files were modified.",
		}}
		return result, nil
	}
	changes := plan.Changes()
	if issues := verifyRenderedReferences(index, referenceEdits, changes); len(issues) != 0 {
		result.Issues = issues
		return result, nil
	}
	if err := checkModuleFileSet(analysis.Root, analysis.Modules, index.snapshots); err != nil {
		result.Issues = []Issue{{File: ".", Message: err.Error(), Remediation: "Rerun migration after configuration changes have stopped."}}
		return result, nil //nolint:nilerr // Configuration drift is a blocking preview diagnostic, not an execution error.
	}
	if err := checkSnapshots(analysis.Root, index.snapshots, changes); err != nil {
		result.Issues = []Issue{{
			File:        ".",
			Message:     fmt.Sprintf("Terraform configuration changed during migration planning: %v", err),
			Remediation: "Rerun dry-run after concurrent filesystem modifications have stopped.",
		}}
		return result, nil
	}
	result.Changes = changes
	result.withholdDiff = containsSensitiveConfiguration(index.snapshots)
	result.Moves = moves
	result.rewritePlan = plan
	result.snapshots = index.snapshots
	return result, nil
}

func sortIssues(issues []Issue) {
	sort.Slice(issues, func(i, j int) bool {
		a, b := issues[i], issues[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.RuleID != b.RuleID {
			return a.RuleID < b.RuleID
		}
		if a.Message != b.Message {
			return a.Message < b.Message
		}
		return a.Remediation < b.Remediation
	})
}

func issueFor(action analyze.Action, root, message string) Issue {
	return Issue{
		RuleID:      action.RuleID,
		File:        displayPath(root, action.Range.Filename),
		Message:     message,
		Remediation: "Implement and validate this transformation before running a complete migration preview.",
	}
}

func displayPath(root, filename string) string {
	rel, err := filepath.Rel(root, filename)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filename
	}
	return filepath.ToSlash(rel)
}

func actionKey(root string, action analyze.Action, rule compatibility.Rule) (resourceKey, error) {
	file, err := filepath.Rel(root, action.Range.Filename)
	if err != nil || file == ".." || strings.HasPrefix(file, ".."+string(filepath.Separator)) ||
		!strings.HasSuffix(file, ".tf") {
		return resourceKey{}, errors.New("action file is outside the selected Terraform tree")
	}
	sourceType, suffix, _ := strings.Cut(rule.Legacy, ".")
	prefix := sourceType + "."
	if !strings.HasPrefix(action.From, prefix) {
		return resourceKey{}, fmt.Errorf("action source %q does not match %q", action.From, rule.Legacy)
	}
	name := strings.TrimPrefix(action.From, prefix)
	if suffix != "" {
		expectedSuffix := "." + suffix
		if !strings.HasSuffix(name, expectedSuffix) {
			return resourceKey{}, fmt.Errorf("action source %q has an unexpected path", action.From)
		}
		name = strings.TrimSuffix(name, expectedSuffix)
	}
	if name == "" || strings.Contains(name, ".") {
		return resourceKey{}, fmt.Errorf("resource name %q cannot be represented safely in a moved address", name)
	}
	return resourceKey{file: filepath.ToSlash(file), typ: sourceType, name: name}, nil
}

// inspectFiles checks source-level features which are not represented as
// individual analyzer actions (notably explicit-null product blocks), and
// detects cross-file resource and moved-block collisions.
func inspectFiles(ctx context.Context, analysis analyze.Result) (moduleIndex, []Issue, error) {
	index := moduleIndex{resources: make(map[string]map[string]string), moves: make(map[string]map[string]string), imports: make(map[string]map[string]string), snapshots: make(map[string][]byte)}
	var issues []Issue
	for _, file := range analysis.Files {
		if err := ctx.Err(); err != nil {
			return moduleIndex{}, nil, err
		}
		if !strings.HasSuffix(file, ".tf") {
			continue
		}
		source, err := os.ReadFile(filepath.Join(analysis.Root, filepath.FromSlash(file)))
		if err != nil {
			return moduleIndex{}, nil, fmt.Errorf("read %q: %w", file, err)
		}
		index.snapshots[file] = source
		parsed, diags := hclsyntax.ParseConfig(source, file, hcl.Pos{Line: 1, Column: 1})
		if diags.HasErrors() || parsed == nil {
			return moduleIndex{}, nil, fmt.Errorf("parse analyzed file %q: invalid HCL syntax (source text omitted)", file)
		}
		body, ok := parsed.Body.(*hclsyntax.Body)
		if !ok {
			return moduleIndex{}, nil, fmt.Errorf("unexpected HCL body in %q", file)
		}
		module := filepath.ToSlash(filepath.Dir(file))
		if index.resources[module] == nil {
			index.resources[module] = make(map[string]string)
			index.moves[module] = make(map[string]string)
			index.imports[module] = make(map[string]string)
		}
		for _, block := range body.Blocks {
			switch {
			case block.Type == "resource" && len(block.Labels) == 2:
				addr := block.Labels[0] + "." + block.Labels[1]
				if previous, found := index.resources[module][addr]; found {
					issues = append(issues, Issue{
						File: file, Message: fmt.Sprintf("resource %s is also declared in %s", addr, previous),
						Remediation: "Resolve duplicate resource declarations before migration.",
					})
				}
				index.resources[module][addr] = file
			case block.Type == "import":
				to, ok := sourceExpression(source, block.Body, "to")
				if !ok {
					issues = append(issues, Issue{
						File: file, Message: "existing import block cannot be classified",
						Remediation: "Use a literal to resource address in existing imports.",
					})
					continue
				}
				if previous, exists := index.imports[module][to]; exists {
					issues = append(issues, Issue{
						File:        file,
						Message:     fmt.Sprintf("multiple import blocks target %q (also in %s)", to, previous),
						Remediation: "Keep only one import block per resource address.",
					})
				}
				index.imports[module][to] = file
			case block.Type == "moved":
				from, okFrom := sourceExpression(source, block.Body, "from")
				to, okTo := sourceExpression(source, block.Body, "to")
				if !okFrom || !okTo {
					issues = append(issues, Issue{
						File: file, Message: "existing moved block cannot be classified",
						Remediation: "Use literal from/to resource addresses in existing moved blocks.",
					})
					continue
				}
				if previous, found := index.moves[module][from]; found {
					issues = append(issues, Issue{
						File: file, Message: fmt.Sprintf("multiple moved blocks use %q (previous target %q)", from, previous),
						Remediation: "Keep exactly one moved block for each original resource address.",
					})
				}
				index.moves[module][from] = to
			}
		}
	}
	return index, issues, nil
}

func sourceExpression(source []byte, body *hclsyntax.Body, name string) (string, bool) {
	attr := body.Attributes[name]
	if attr == nil {
		return "", false
	}
	// Canonicalize static address traversals so whitespace and quoting do
	// not conceal a duplicate or an existing exact moved block.
	if traversal, diags := hcl.AbsTraversalForExpr(attr.Expr); !diags.HasErrors() {
		return string(hclwrite.TokensForTraversal(traversal).Bytes()), true
	}
	if name == "from" {
		return "", false
	}
	rng := attr.Expr.Range()
	if rng.Start.Byte < 0 || rng.End.Byte > len(source) || rng.End.Byte < rng.Start.Byte {
		return "", false
	}
	return strings.TrimSpace(string(source[rng.Start.Byte:rng.End.Byte])), true
}

// checkSnapshots ensures the rewrite plan was derived from exactly the HCL
// inspected for collisions and that no analyzed file changed meanwhile.
func checkSnapshots(root string, snapshots map[string][]byte, changes []rewrite.Change) error {
	for _, change := range changes {
		previous, exists := snapshots[change.Path]
		if change.Exists != exists || (exists && !bytes.Equal(change.Before, previous)) {
			return fmt.Errorf("%s differs from the analyzed source", change.Path)
		}
	}
	files := make([]string, 0, len(snapshots))
	for filename := range snapshots {
		files = append(files, filename)
	}
	sort.Strings(files)
	for _, filename := range files {
		previous := snapshots[filename]
		current, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(filename)))
		if err != nil {
			return fmt.Errorf("recheck %s: %w", filename, err)
		}
		if !bytes.Equal(previous, current) {
			return fmt.Errorf("%s changed since the initial analysis", filename)
		}
	}
	return nil
}

// checkModuleFileSet detects a newly added Terraform source, including a file
// with references the analyzer never saw. Analysis rejects existing .tf.json
// files before inspectFiles snapshots native HCL. Keep checking .tf.json here
// to reject JSON configuration added after that successful analysis.
// The existing snapshot check detects changes/deletions to the original files.
// Neither check replaces coordination with other writers while Apply installs
// the plan.
func checkModuleFileSet(root string, modules []string, snapshots map[string][]byte) error {
	for _, module := range modules {
		entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(module)))
		if err != nil {
			return fmt.Errorf("recheck module %s: %w", module, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".tf") && !strings.HasSuffix(entry.Name(), ".tf.json")) {
				continue
			}
			name := filepath.ToSlash(filepath.Join(module, entry.Name()))
			if _, ok := snapshots[name]; !ok {
				return fmt.Errorf("terraform file %s appeared after analysis", name)
			}
		}
	}
	return nil
}
