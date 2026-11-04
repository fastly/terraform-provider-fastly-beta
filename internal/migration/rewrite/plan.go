// Package rewrite provides source-preserving, rule-agnostic Terraform HCL edits.
//
// Callers decide which transformations are safe using the compatibility registry
// and analyzer. This package never evaluates expressions or changes Terraform state.
package rewrite

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Edit is a source-preserving HCL operation. Implementations are provided by
// this package; a failed edit leaves the entire in-memory plan unmodified.
type Edit interface {
	file() string
	apply([]byte, string) ([]byte, error)
	canCreate() bool
}

// Change is one proposed complete-file replacement.
// Before/After are defensively copied when returned by Plan.Changes.
type Change struct {
	Path   string // slash-separated, relative to the selected root
	Before []byte
	After  []byte
	Mode   fs.FileMode
	Exists bool
}

// Plan is an immutable snapshot of intended file replacements.
type Plan struct {
	root    string
	changes []Change
}

// Root returns the canonical directory used when building the plan.
func (p *Plan) Root() string {
	if p == nil {
		return ""
	}
	return p.root
}

// Changes returns changes in deterministic path order.
func (p *Plan) Changes() []Change {
	if p == nil {
		return nil
	}
	out := make([]Change, len(p.changes))
	for i, change := range p.changes {
		out[i] = cloneChange(change)
	}
	return out
}

func cloneChange(change Change) Change {
	change.Before = bytes.Clone(change.Before)
	change.After = bytes.Clone(change.After)
	return change
}

// Build applies all edits in memory, validating every resulting file before
// returning. No files are written if an edit fails or context is canceled.
// Edit order is significant within a file; file output order is deterministic.
func Build(ctx context.Context, root string, edits ...Edit) (*Plan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	directory, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve root: %w", err)
	}
	directory, err = filepath.EvalSymlinks(directory)
	if err != nil {
		return nil, fmt.Errorf("resolve root: %w", err)
	}
	info, err := os.Stat(directory)
	if err != nil {
		return nil, fmt.Errorf("stat root: %w", err)
	}
	if !info.IsDir() {
		return nil, errors.New("rewrite root must be a directory")
	}

	staged := make(map[string]*Change)
	for index, edit := range edits {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if edit == nil {
			return nil, fmt.Errorf("edit %d is nil", index)
		}
		name, path, err := resolveFile(directory, edit.file())
		if err != nil {
			return nil, fmt.Errorf("edit %d: %w", index, err)
		}
		change, ok := staged[name]
		if !ok {
			change = &Change{Path: name, Mode: 0o600}
			fileInfo, err := os.Lstat(path)
			switch {
			case err == nil:
				if !fileInfo.Mode().IsRegular() {
					return nil, fmt.Errorf("edit %d: %q is not a regular file", index, name)
				}
				content, err := os.ReadFile(path)
				if err != nil {
					return nil, fmt.Errorf("read %q: %w", name, err)
				}
				change.Before = content
				change.After = bytes.Clone(content)
				change.Mode = fileInfo.Mode().Perm()
				change.Exists = true
			case errors.Is(err, fs.ErrNotExist):
				if !edit.canCreate() {
					return nil, fmt.Errorf("edit %d: file %q does not exist", index, name)
				}
			default:
				return nil, fmt.Errorf("stat %q: %w", name, err)
			}
			staged[name] = change
		}

		next, err := edit.apply(change.After, name)
		if err != nil {
			return nil, fmt.Errorf("edit %d on %q: %w", index, name, err)
		}
		if _, err := parseBody(next, name); err != nil {
			return nil, fmt.Errorf("edit %d produces invalid HCL in %q: %w", index, name, err)
		}
		change.After = next
	}

	plan := &Plan{root: directory}
	for _, change := range staged {
		if !bytes.Equal(change.Before, change.After) {
			plan.changes = append(plan.changes, cloneChange(*change))
		}
	}
	sort.Slice(plan.changes, func(i, j int) bool {
		return plan.changes[i].Path < plan.changes[j].Path
	})
	return plan, nil
}

// resolveFile forbids writes outside the selected root, including via existing
// symlinked parent directories. New files are allowed only inside existing dirs.
func resolveFile(root, name string) (string, string, error) {
	if name == "" || filepath.IsAbs(name) {
		return "", "", fmt.Errorf("invalid relative Terraform file %q", name)
	}
	cleaned := filepath.Clean(filepath.FromSlash(name))
	if cleaned == "." || cleaned == ".." ||
		strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) ||
		filepath.Ext(cleaned) != ".tf" {
		return "", "", fmt.Errorf("invalid relative Terraform file %q", name)
	}
	path := filepath.Join(root, cleaned)
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return "", "", fmt.Errorf("resolve parent of %q: %w", name, err)
	}
	relative, err := filepath.Rel(root, parent)
	if err != nil || relative == ".." ||
		strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("terraform file %q is outside selected root", name)
	}
	return filepath.ToSlash(cleaned), path, nil
}
