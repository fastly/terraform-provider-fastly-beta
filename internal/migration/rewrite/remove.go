package rewrite

import (
	"fmt"
	"strings"
)

// RemoveAttribute removes a proven-unnecessary attribute (and its immediately
// attached comments) without evaluating its value or reformatting other HCL.
// Parents select nested block types relative to the resource. The operation
// rejects ambiguous layouts using the same source-range guards as NestAttribute.
type RemoveAttribute struct {
	File         string
	ResourceType string
	ResourceName string
	Parents      []string
	Attribute    string
}

func (e RemoveAttribute) file() string    { return e.File }
func (e RemoveAttribute) canCreate() bool { return false }

func (e RemoveAttribute) apply(source []byte, filename string) ([]byte, error) {
	resource, err := requireResource(source, filename, e.ResourceType, e.ResourceName)
	if err != nil {
		return nil, err
	}
	bodies := matchingBodies(resource.Body, e.Parents)
	if len(bodies) == 0 {
		return nil, fmt.Errorf("attribute parent path %q not found", strings.Join(e.Parents, "."))
	}
	var edits []replacement
	for _, body := range bodies {
		attr := body.Attributes[e.Attribute]
		if attr == nil {
			continue // idempotent, including repeated parent blocks
		}
		start, end, _, _, err := attributeLines(source, attr)
		if err != nil {
			return nil, err
		}
		edits = append(edits, replacement{start: start, end: end})
	}
	return replace(source, edits)
}
