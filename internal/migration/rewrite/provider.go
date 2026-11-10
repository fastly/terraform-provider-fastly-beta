package rewrite

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

const (
	gaFastlyVersion = ">= 20.0.0"
	gaFastlySource  = "fastly/fastly"
)

// UpdateProviderRequirement changes only the selected Fastly requirement
// expression. Other providers, comments, and Terraform configuration survive.
type UpdateProviderRequirement struct {
	File      string
	LocalName string
}

func (e UpdateProviderRequirement) file() string    { return e.File }
func (e UpdateProviderRequirement) canCreate() bool { return false }

func (e UpdateProviderRequirement) apply(source []byte, filename string) ([]byte, error) {
	body, err := parseBody(source, filename)
	if err != nil {
		return nil, err
	}
	var found *hclsyntax.Attribute
	for _, tf := range body.Blocks {
		if tf.Type != "terraform" {
			continue
		}
		for _, required := range tf.Body.Blocks {
			if required.Type != "required_providers" {
				continue
			}
			if a := required.Body.Attributes[e.LocalName]; a != nil {
				if found != nil {
					return nil, fmt.Errorf("duplicate required_providers entry %q", e.LocalName)
				}
				found = a
			}
		}
	}
	if found == nil {
		return nil, fmt.Errorf("required_providers entry %q was not found", e.LocalName)
	}
	if object, ok := found.Expr.(*hclsyntax.ObjectConsExpr); ok {
		var changes []replacement
		hasSource, hasVersion := false, false
		for _, item := range object.Items {
			key, diags := item.KeyExpr.Value(nil)
			if diags.HasErrors() || !key.IsKnown() || key.IsNull() || key.Type() != cty.String {
				return nil, fmt.Errorf("nonliteral required_providers key in %q", e.LocalName)
			}
			switch key.AsString() {
			case "source":
				hasSource = true // Checked by the planner's source gate.
			case "version":
				hasVersion = true
				rng := item.ValueExpr.Range()
				changes = append(changes, replacement{rng.Start.Byte, rng.End.Byte, []byte(strconv.Quote(gaFastlyVersion))})
			}
		}
		var additions []string
		if !hasSource {
			additions = append(additions, "source = "+strconv.Quote(gaFastlySource))
		}
		if !hasVersion {
			additions = append(additions, "version = "+strconv.Quote(gaFastlyVersion))
		}
		if len(additions) != 0 {
			pos, snippet, err := addObjectFields(source, object, additions)
			if err != nil {
				return nil, err
			}
			changes = append(changes, replacement{pos, pos, []byte(snippet)})
		}
		return replace(source, changes)
	}

	// Older Terraform supported a string shorthand: fastly = "~> 9.0".
	// Replace just the value expression with a complete modern requirement.
	value, diags := found.Expr.Value(nil)
	if diags.HasErrors() || !value.IsKnown() || value.IsNull() || value.Type() != cty.String {
		return nil, fmt.Errorf("required_providers %q must be a literal object or version string", e.LocalName)
	}
	rng := found.Expr.Range()
	indent := lineIndent(source, found.NameRange.Start.Byte)
	updated := "{\n" + indent + "  source = " + strconv.Quote(gaFastlySource) +
		"\n" + indent + "  version = " + strconv.Quote(gaFastlyVersion) + "\n" + indent + "}"
	return replace(source, []replacement{{rng.Start.Byte, rng.End.Byte, []byte(updated)}})
}

// EnsureProviderRequirement adds the Fastly requirement to an existing
// required_providers block, then an existing terraform block, or creates a new
// terraform block in the module's migration.tf. It never creates a second
// required_providers block when one already exists in the module.
type EnsureProviderRequirement struct {
	File string
}

func (e EnsureProviderRequirement) file() string    { return e.File }
func (e EnsureProviderRequirement) canCreate() bool { return true }

func (e EnsureProviderRequirement) apply(source []byte, filename string) ([]byte, error) {
	body, err := parseBody(source, filename)
	if err != nil {
		return nil, err
	}
	var terraform, required *hclsyntax.Block
	for _, block := range body.Blocks {
		if block.Type != "terraform" {
			continue
		}
		if terraform == nil {
			terraform = block
		}
		for _, child := range block.Body.Blocks {
			if child.Type != "required_providers" {
				continue
			}
			if required != nil {
				return nil, fmt.Errorf("multiple required_providers blocks in %s", filename)
			}
			required = child
		}
	}
	if required != nil {
		if required.Body.Attributes["fastly"] != nil {
			return nil, fmt.Errorf("required_providers.fastly already exists")
		}
		pos, snippet, err := insertBlockLines(source, required.Range().End.Byte-1,
			[]string{"fastly = {", "  source = \"fastly/fastly\"", "  version = \">= 20.0.0\"", "}"})
		if err != nil {
			return nil, err
		}
		return replace(source, []replacement{{pos, pos, []byte(snippet)}})
	}
	if terraform != nil {
		pos, snippet, err := insertBlockLines(source, terraform.Range().End.Byte-1,
			[]string{"required_providers {", "  fastly = {", "    source = \"fastly/fastly\"", "    version = \">= 20.0.0\"", "  }", "}"})
		if err != nil {
			return nil, err
		}
		return replace(source, []replacement{{pos, pos, []byte(snippet)}})
	}
	return appendBlock(source, "terraform {\n  required_providers {\n    fastly = {\n"+
		"      source  = \"fastly/fastly\"\n      version = \">= 20.0.0\"\n"+
		"    }\n  }\n}\n"), nil
}

// RenameProviderArguments preserves the value expressions and aliases. It
// intentionally refuses collisions rather than silently overriding arguments.
type RenameProviderArguments struct {
	File string
	Name string
}

func (e RenameProviderArguments) file() string    { return e.File }
func (e RenameProviderArguments) canCreate() bool { return false }

func (e RenameProviderArguments) apply(source []byte, filename string) ([]byte, error) {
	body, err := parseBody(source, filename)
	if err != nil {
		return nil, err
	}
	var changes []replacement
	for _, block := range body.Blocks {
		if block.Type != "provider" || len(block.Labels) != 1 || block.Labels[0] != e.Name {
			continue
		}
		for _, pair := range [][2]string{{"api_key", "api_token"}, {"base_url", "api_endpoint"}} {
			oldAttr, newAttr := block.Body.Attributes[pair[0]], block.Body.Attributes[pair[1]]
			if oldAttr == nil {
				continue
			}
			if newAttr != nil {
				return nil, fmt.Errorf("provider %q sets both %s and %s", e.Name, pair[0], pair[1])
			}
			rng := oldAttr.NameRange
			changes = append(changes, replacement{rng.Start.Byte, rng.End.Byte, []byte(pair[1])})
		}
	}
	return replace(source, changes)
}

// addObjectFields inserts attributes before the object's closing brace. The
// insertion occurs after any preceding source lines/comments and preserves all
// existing object keys and their expressions unchanged.
func addObjectFields(source []byte, object *hclsyntax.ObjectConsExpr, fields []string) (int, string, error) {
	return insertBlockLines(source, object.Range().End.Byte-1, fields)
}

func insertBlockLines(source []byte, closing int, fields []string) (int, string, error) {
	if closing < 0 || closing >= len(source) || source[closing] != '}' {
		return 0, "", fmt.Errorf("cannot locate HCL closing brace for attribute insertion")
	}
	lineStart := closing
	for lineStart > 0 && source[lineStart-1] != '\n' {
		lineStart--
	}
	before := string(source[lineStart:closing])
	// On a standalone closing-brace line, insert before its indentation.
	if strings.TrimSpace(before) == "" {
		baseIndent := before
		fieldIndent := baseIndent + "  "
		lines := make([]string, len(fields))
		for i, field := range fields {
			lines[i] = fieldIndent + field
		}
		return lineStart, strings.Join(lines, "\n") + "\n", nil
	}

	// A one-line object/block needs a newline between its existing last
	// argument and the new argument, not a comma inside an HCL block.
	baseIndent := lineIndent(source, lineStart)
	lines := make([]string, len(fields))
	for i, field := range fields {
		lines[i] = baseIndent + "  " + field
	}
	return closing, "\n" + strings.Join(lines, "\n") + "\n" + baseIndent, nil
}

func lineIndent(source []byte, pos int) string {
	start := pos
	for start > 0 && source[start-1] != '\n' {
		start--
	}
	end := start
	for end < len(source) && (source[end] == ' ' || source[end] == '\t') {
		end++
	}
	return string(source[start:end])
}
