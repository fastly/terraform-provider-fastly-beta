package rewrite

import (
	"bytes"
	"fmt"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

// AddStringDefault pins a reviewed legacy string default in each matching
// static parent that omits the attribute or explicitly sets literal null.
// Other expressions are untouched.
// Use only when the old and new defaults have different representations.
type AddStringDefault struct {
	File         string
	ResourceType string
	ResourceName string
	Parents      []string
	Attribute    string
	Value        string
}

func (e AddStringDefault) file() string    { return e.File }
func (e AddStringDefault) canCreate() bool { return false }

func (e AddStringDefault) apply(source []byte, filename string) ([]byte, error) {
	if err := validateIdentifier(e.Attribute); err != nil {
		return nil, err
	}
	resource, err := requireResource(source, filename, e.ResourceType, e.ResourceName)
	if err != nil {
		return nil, err
	}
	parents := matchingParentBlocks(resource, e.Parents)
	if len(parents) == 0 {
		return nil, fmt.Errorf("default attribute parent path not found")
	}
	// TokensForValue escapes both interpolation and template directives
	// (notably the literal VCL log-format sequence %{).
	value := hclwrite.TokensForValue(cty.StringVal(e.Value)).Bytes()
	var edits []replacement
	for _, parent := range parents {
		if attr := parent.Body.Attributes[e.Attribute]; attr != nil {
			if literal, ok := attr.Expr.(*hclsyntax.LiteralValueExpr); ok && literal.Val.IsNull() {
				rng := attr.Expr.Range()
				edits = append(edits, replacement{start: rng.Start.Byte, end: rng.End.Byte, value: value})
			}
			continue
		}
		start, indent, err := blockClosingLine(source, parent)
		if err != nil {
			return nil, err
		}
		newline := "\n"
		if start >= 2 && bytes.Equal(source[start-2:start], []byte("\r\n")) {
			newline = "\r\n"
		}
		edits = append(edits, replacement{start: start, end: start, value: []byte(indent + "  " + e.Attribute + " = " + string(value) + newline)})
	}
	return replace(source, edits)
}
