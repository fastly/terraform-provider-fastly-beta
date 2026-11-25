package rewrite

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

// ReplaceLiteral rewrites a literal string or whole-number attribute value
// that matches a key in Values to the mapped string. Other literals,
// expressions, and omitted attributes are left unchanged.
type ReplaceLiteral struct {
	File         string
	ResourceType string
	ResourceName string
	Parents      []string
	Attribute    string
	Values       map[string]string
	// CaseInsensitive matches string literals regardless of case.
	CaseInsensitive bool
}

func (e ReplaceLiteral) file() string    { return e.File }
func (e ReplaceLiteral) canCreate() bool { return false }

func (e ReplaceLiteral) apply(source []byte, filename string) ([]byte, error) {
	if err := validateIdentifier(e.Attribute); err != nil {
		return nil, err
	}
	resource, err := requireResource(source, filename, e.ResourceType, e.ResourceName)
	if err != nil {
		return nil, err
	}
	var edits []replacement
	for _, body := range matchingBodies(resource.Body, e.Parents) {
		attr := body.Attributes[e.Attribute]
		if attr == nil {
			continue
		}
		current, isString, ok := LiteralKey(attr.Expr)
		if !ok {
			continue
		}
		key := current
		if isString && e.CaseInsensitive {
			key = strings.ToLower(current)
		}
		target, ok := e.Values[key]
		if !ok || (isString && target == current) {
			continue
		}
		rng := attr.Expr.Range()
		edits = append(edits, replacement{
			start: rng.Start.Byte, end: rng.End.Byte,
			value: hclwrite.TokensForValue(cty.StringVal(target)).Bytes(),
		})
	}
	return replace(source, edits)
}

// LowercaseExpression wraps a string expression that references variables or
// other values in a null-safe lower() call, so values the legacy provider
// lowercased before sending still pass the target's lowercase validation.
// Literals, constant expressions and existing lower() calls are left alone.
type LowercaseExpression struct {
	File         string
	ResourceType string
	ResourceName string
	Parents      []string
	Attribute    string
}

func (e LowercaseExpression) file() string    { return e.File }
func (e LowercaseExpression) canCreate() bool { return false }

func (e LowercaseExpression) apply(source []byte, filename string) ([]byte, error) {
	if err := validateIdentifier(e.Attribute); err != nil {
		return nil, err
	}
	resource, err := requireResource(source, filename, e.ResourceType, e.ResourceName)
	if err != nil {
		return nil, err
	}
	var edits []replacement
	for _, body := range matchingBodies(resource.Body, e.Parents) {
		attr := body.Attributes[e.Attribute]
		if attr == nil {
			continue
		}
		if _, diags := attr.Expr.Value(nil); !diags.HasErrors() {
			continue
		}
		if call, ok := attr.Expr.(*hclsyntax.FunctionCallExpr); ok && call.Name == "lower" {
			continue
		}
		rng := attr.Expr.Range()
		expr := string(source[rng.Start.Byte:rng.End.Byte])
		subject := expr
		switch attr.Expr.(type) {
		case *hclsyntax.ScopeTraversalExpr, *hclsyntax.RelativeTraversalExpr,
			*hclsyntax.IndexExpr, *hclsyntax.FunctionCallExpr, *hclsyntax.ParenthesesExpr:
		default:
			subject = "(" + expr + ")"
		}
		edits = append(edits, replacement{
			start: rng.Start.Byte, end: rng.End.Byte,
			value: []byte(subject + " == null ? null : lower(" + expr + ")"),
		})
	}
	return replace(source, edits)
}

// RemoveLiteralString removes an attribute (and its attached comments) only
// where its value is exactly the literal string Value.
type RemoveLiteralString struct {
	File         string
	ResourceType string
	ResourceName string
	Parents      []string
	Attribute    string
	Value        string
}

func (e RemoveLiteralString) file() string    { return e.File }
func (e RemoveLiteralString) canCreate() bool { return false }

func (e RemoveLiteralString) apply(source []byte, filename string) ([]byte, error) {
	resource, err := requireResource(source, filename, e.ResourceType, e.ResourceName)
	if err != nil {
		return nil, err
	}
	var edits []replacement
	for _, body := range matchingBodies(resource.Body, e.Parents) {
		attr := body.Attributes[e.Attribute]
		if attr == nil {
			continue
		}
		value, isString, ok := LiteralKey(attr.Expr)
		if !ok || !isString || value != e.Value {
			continue
		}
		start, end, _, _, err := attributeLines(source, attr)
		if err != nil {
			return nil, err
		}
		edits = append(edits, replacement{start: start, end: end})
	}
	return replace(source, edits)
}

// LiteralKey returns the value of a literal string, or the decimal form of a
// literal whole number. Templates with interpolation, variables, function
// calls and other expressions are not literals.
func LiteralKey(expr hclsyntax.Expression) (value string, isString bool, ok bool) {
	switch e := expr.(type) {
	case *hclsyntax.TemplateExpr:
		if !e.IsStringLiteral() {
			return "", false, false
		}
		v, diags := e.Value(nil)
		if diags.HasErrors() || !v.IsKnown() || v.IsNull() || v.Type() != cty.String {
			return "", false, false
		}
		return v.AsString(), true, true
	case *hclsyntax.LiteralValueExpr:
		if e.Val.IsNull() || !e.Val.IsKnown() {
			return "", false, false
		}
		switch e.Val.Type() {
		case cty.String:
			return e.Val.AsString(), true, true
		case cty.Number:
			if !e.Val.AsBigFloat().IsInt() {
				return "", false, false
			}
			return e.Val.AsBigFloat().Text('f', 0), false, true
		}
	}
	return "", false, false
}

// literalNameError explains why a nested block cannot be keyed by name.
func literalNameError(block *hclsyntax.Block) error {
	return fmt.Errorf("%s block at %s does not have a literal string name", block.Type, block.TypeRange.String())
}
