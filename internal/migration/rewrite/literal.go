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

// NormalizeStringExpression wraps a string expression that references
// variables or other values so that "" resolves to null, which the target
// reads back for an empty value. With Lowercase, it also applies a null-safe
// lower(), so values the legacy provider lowercased before sending still pass
// the target's lowercase validation. Literals and constant expressions are
// left alone.
type NormalizeStringExpression struct {
	File         string
	ResourceType string
	ResourceName string
	Parents      []string
	Attribute    string
	Lowercase    bool
}

func (e NormalizeStringExpression) file() string    { return e.File }
func (e NormalizeStringExpression) canCreate() bool { return false }

func (e NormalizeStringExpression) apply(source []byte, filename string) ([]byte, error) {
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
		rng := attr.Expr.Range()
		expr, subject := operandSource(source, attr.Expr)
		value := subject + ` == "" ? null : ` + subject
		call, isLower := attr.Expr.(*hclsyntax.FunctionCallExpr)
		if e.Lowercase && (!isLower || call.Name != "lower") {
			value = subject + ` == null || ` + subject + ` == "" ? null : lower(` + expr + ")"
		}
		edits = append(edits, replacement{start: rng.Start.Byte, end: rng.End.Byte, value: []byte(value)})
	}
	return replace(source, edits)
}

// NormalizeEmptyList removes a constant empty list and wraps a list
// expression that references variables or other values so it resolves to
// null when empty. The target stores an unset list as null, so an empty list
// in configuration would otherwise plan a migration-only update.
type NormalizeEmptyList struct {
	File         string
	ResourceType string
	ResourceName string
	Parents      []string
	Attribute    string
}

func (e NormalizeEmptyList) file() string    { return e.File }
func (e NormalizeEmptyList) canCreate() bool { return false }

func (e NormalizeEmptyList) apply(source []byte, filename string) ([]byte, error) {
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
		if value, diags := attr.Expr.Value(nil); !diags.HasErrors() {
			if !emptyCollection(value) {
				continue
			}
			start, end, _, _, err := attributeLines(source, attr)
			if err != nil {
				return nil, err
			}
			edits = append(edits, replacement{start: start, end: end})
			continue
		}
		rng := attr.Expr.Range()
		expr, subject := operandSource(source, attr.Expr)
		edits = append(edits, replacement{
			start: rng.Start.Byte, end: rng.End.Byte,
			value: []byte(subject + " == null ? null : (length(" + expr + ") == 0 ? null : " + subject + ")"),
		})
	}
	return replace(source, edits)
}

func emptyCollection(value cty.Value) bool {
	if !value.IsWhollyKnown() || value.IsNull() {
		return false
	}
	ty := value.Type()
	return (ty.IsTupleType() || ty.IsListType() || ty.IsSetType()) && value.LengthInt() == 0
}

// operandSource returns an expression's source text, and the same text
// parenthesized where needed to use it as an operand.
func operandSource(source []byte, expr hclsyntax.Expression) (string, string) {
	rng := expr.Range()
	text := string(source[rng.Start.Byte:rng.End.Byte])
	switch expr.(type) {
	case *hclsyntax.ScopeTraversalExpr, *hclsyntax.RelativeTraversalExpr,
		*hclsyntax.IndexExpr, *hclsyntax.FunctionCallExpr, *hclsyntax.ParenthesesExpr:
		return text, text
	}
	return text, "(" + text + ")"
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
