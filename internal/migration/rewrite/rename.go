package rewrite

import (
	"fmt"
	"strconv"
)

// RenameResource changes only a Terraform resource type label. The resource's
// name, body, expressions, meta-arguments and comments are unchanged.
// Reference rewrites and moved blocks are separate explicit operations.
type RenameResource struct {
	File     string
	FromType string
	ToType   string
	Name     string
}

func (e RenameResource) file() string    { return e.File }
func (e RenameResource) canCreate() bool { return false }

func (e RenameResource) apply(source []byte, filename string) ([]byte, error) {
	if err := validateIdentifier(e.ToType); err != nil {
		return nil, err
	}
	if e.FromType == e.ToType {
		return source, nil
	}
	body, err := parseBody(source, filename)
	if err != nil {
		return nil, err
	}
	from := findResource(body, e.FromType, e.Name)
	to := findResource(body, e.ToType, e.Name)
	if from == nil {
		if to != nil {
			return source, nil
		} // idempotent
		return nil, fmt.Errorf("resource %s.%s not found", e.FromType, e.Name)
	}
	if to != nil {
		return nil, fmt.Errorf("target resource %s.%s already exists", e.ToType, e.Name)
	}
	if len(from.LabelRanges) != 2 {
		return nil, fmt.Errorf("unexpected label count for resource %s.%s", e.FromType, e.Name)
	}
	loc := from.LabelRanges[0]
	return replace(source, []replacement{{loc.Start.Byte, loc.End.Byte, []byte(strconv.Quote(e.ToType))}})
}

// RenameBlock renames all static blocks named From beneath matching Parents,
// relative to the specified resource. It never evaluates dynamic blocks.
type RenameBlock struct {
	File         string
	ResourceType string
	ResourceName string
	Parents      []string
	From         string
	To           string
}

func (e RenameBlock) file() string    { return e.File }
func (e RenameBlock) canCreate() bool { return false }

func (e RenameBlock) apply(source []byte, filename string) ([]byte, error) {
	if err := validateIdentifier(e.To); err != nil {
		return nil, err
	}
	if e.From == e.To {
		return source, nil
	}
	resource, err := requireResource(source, filename, e.ResourceType, e.ResourceName)
	if err != nil {
		return nil, err
	}
	bodies := matchingBodies(resource.Body, e.Parents)
	if len(bodies) == 0 {
		return nil, fmt.Errorf("block parent path %q not found", e.Parents)
	}
	var edits []replacement
	existingTarget := false
	for _, body := range bodies {
		for _, block := range body.Blocks {
			if block.Type == e.From {
				rng := block.TypeRange
				edits = append(edits, replacement{rng.Start.Byte, rng.End.Byte, []byte(e.To)})
			}
			if block.Type == e.To {
				existingTarget = true
			}
		}
	}
	if len(edits) == 0 {
		if existingTarget {
			return source, nil
		}
		return nil, fmt.Errorf("block %q not found", e.From)
	}
	return replace(source, edits)
}

// RenameAttribute renames an attribute key in every matching parent block.
// Its value expression, including traversals and interpolations, is preserved.
type RenameAttribute struct {
	File         string
	ResourceType string
	ResourceName string
	Parents      []string
	From         string
	To           string
}

func (e RenameAttribute) file() string    { return e.File }
func (e RenameAttribute) canCreate() bool { return false }

func (e RenameAttribute) apply(source []byte, filename string) ([]byte, error) {
	if err := validateIdentifier(e.To); err != nil {
		return nil, err
	}
	if e.From == e.To {
		return source, nil
	}
	resource, err := requireResource(source, filename, e.ResourceType, e.ResourceName)
	if err != nil {
		return nil, err
	}
	bodies := matchingBodies(resource.Body, e.Parents)
	if len(bodies) == 0 {
		return nil, fmt.Errorf("attribute parent path %q not found", e.Parents)
	}
	var edits []replacement
	foundTarget := false
	for _, body := range bodies {
		from := body.Attributes[e.From]
		to := body.Attributes[e.To]
		if from == nil {
			if to != nil {
				foundTarget = true
				continue
			}
			return nil, fmt.Errorf("neither attribute %q nor %q exists in matched block", e.From, e.To)
		}
		if to != nil {
			return nil, fmt.Errorf("attribute %q already exists", e.To)
		}
		loc := from.NameRange
		edits = append(edits, replacement{loc.Start.Byte, loc.End.Byte, []byte(e.To)})
	}
	if len(edits) == 0 && foundTarget {
		return source, nil
	}
	return replace(source, edits)
}
