package rewrite

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// NestAttribute moves one attribute into a named nested block beneath the
// matching resource and Parents. It preserves the raw expression and any
// single-line comments attached to that attribute. An absent target block is
// created. For ambiguous/multiline layouts that cannot be moved safely the
// operation fails without altering any files.
type NestAttribute struct {
	File            string
	ResourceType    string
	ResourceName    string
	Parents         []string
	Attribute       string
	TargetBlock     string
	TargetAttribute string // if empty, retains Attribute name
}

func (e NestAttribute) file() string    { return e.File }
func (e NestAttribute) canCreate() bool { return false }

func (e NestAttribute) apply(source []byte) ([]byte, error) {
	if err := validateIdentifier(e.TargetBlock); err != nil {
		return nil, err
	}
	targetName := e.TargetAttribute
	if targetName == "" {
		targetName = e.Attribute
	}
	if err := validateIdentifier(targetName); err != nil {
		return nil, err
	}

	resource, err := requireResource(source, e.ResourceType, e.ResourceName)
	if err != nil {
		return nil, err
	}
	parents := matchingParentBlocks(resource, e.Parents)
	if len(parents) == 0 {
		return nil, fmt.Errorf("attribute parent path %q not found", e.Parents)
	}
	var edits []replacement
	for _, parent := range parents {
		original := parent.Body.Attributes[e.Attribute]
		var target *hclsyntax.Block
		for _, block := range parent.Body.Blocks {
			if block.Type != e.TargetBlock {
				continue
			}
			if target != nil {
				return nil, fmt.Errorf("multiple %q target blocks are ambiguous", e.TargetBlock)
			}
			target = block
		}
		if original == nil {
			if target != nil && target.Body.Attributes[targetName] != nil {
				continue // already nested
			}
			return nil, fmt.Errorf("attribute %q not found and target %q absent", e.Attribute, targetName)
		}
		if target != nil && target.Body.Attributes[targetName] != nil {
			return nil, fmt.Errorf("target attribute %q already exists", targetName)
		}

		start, end, indent, segment, err := attributeLines(source, original)
		if err != nil {
			return nil, err
		}

		var insertPos int
		var newIndent string
		var addition []byte
		if target != nil {
			insertPos, newIndent, err = blockClosingLine(source, target)
			if err != nil {
				return nil, err
			}
			newIndent += "  "
			addition, err = reindentAttribute(segment, indent, newIndent, targetName, e.Attribute)
			if err != nil {
				return nil, err
			}
		} else {
			var parentIndent string
			insertPos, parentIndent, err = blockClosingLine(source, parent)
			if err != nil {
				return nil, err
			}
			blockIndent := parentIndent + "  "
			newIndent = blockIndent + "  "
			moved, err := reindentAttribute(segment, indent, newIndent, targetName, e.Attribute)
			if err != nil {
				return nil, err
			}
			addition = []byte(blockIndent + e.TargetBlock + " {\n" +
				string(moved) + blockIndent + "}\n")
		}
		// Edits may touch multiple repeated parent blocks; no offset is
		// altered until every parent has been validated.
		edits = append(edits,
			replacement{start, end, nil},
			replacement{insertPos, insertPos, addition},
		)
	}
	return replace(source, edits)
}

// matchingParentBlocks returns the exact blocks containing the attribute.
// Keep the block (rather than only its body) so the closing-brace token can
// be located through its source range when inserting into nested HCL.
func matchingParentBlocks(resource *hclsyntax.Block, parents []string) []*hclsyntax.Block {
	matches := []*hclsyntax.Block{resource}
	for _, name := range parents {
		var children []*hclsyntax.Block
		for _, parent := range matches {
			for _, child := range parent.Body.Blocks {
				if child.Type == name {
					children = append(children, child)
				}
			}
		}
		matches = children
	}
	return matches
}

// blockClosingLine returns the start of the brace line and its indentation.
// Body.EndRange does not necessarily point at the closing brace; use the
// containing block's explicit CloseBraceRange instead. Inline closing braces
// are rejected to avoid disturbing adjacent syntax.
func blockClosingLine(source []byte, block *hclsyntax.Block) (int, string, error) {
	pos := block.CloseBraceRange.Start.Byte
	if pos < 0 || pos >= len(source) || source[pos] != '}' {
		return 0, "", fmt.Errorf("cannot identify closing brace for nested HCL edit")
	}
	start := bytes.LastIndexByte(source[:pos], '\n') + 1
	indent := string(source[start:pos])
	if strings.TrimSpace(indent) != "" {
		return 0, "", fmt.Errorf("inline closing braces cannot be safely rewritten")
	}
	return start, indent, nil
}

// attributeLines includes an inline comment and any immediately preceding
// comment-only lines. We do not rewrite a heredoc, because indentation can
// change its literal value (not just its formatting).
func attributeLines(source []byte, attr *hclsyntax.Attribute) (int, int, string, []byte, error) {
	pos := attr.NameRange.Start.Byte
	if pos < 0 || pos > len(source) {
		return 0, 0, "", nil, fmt.Errorf("invalid attribute source range")
	}
	start := bytes.LastIndexByte(source[:pos], '\n') + 1
	indent := string(source[start:pos])
	if strings.TrimSpace(indent) != "" {
		return 0, 0, "", nil, fmt.Errorf("attribute must start on its own line")
	}
	exprEnd := attr.SrcRange.End.Byte
	if exprEnd < pos || exprEnd > len(source) {
		return 0, 0, "", nil, fmt.Errorf("invalid attribute end range")
	}
	end := len(source)
	if next := bytes.IndexByte(source[exprEnd:], '\n'); next >= 0 {
		end = exprEnd + next + 1
	}
	trailer := strings.TrimSpace(string(source[exprEnd:end]))
	if trailer != "" && !strings.HasPrefix(trailer, "#") && !strings.HasPrefix(trailer, "//") {
		return 0, 0, "", nil, fmt.Errorf("attribute has additional syntax on the same line")
	}
	if bytes.Contains(source[start:end], []byte("<<")) {
		return 0, 0, "", nil, fmt.Errorf("heredoc attributes require a dedicated migration rule")
	}
	// Move only adjacent single-line comments; never capture unrelated
	// comments separated by a blank line.
	for start > 0 {
		previousEnd := start - 1
		previousStart := bytes.LastIndexByte(source[:previousEnd], '\n') + 1
		line := string(source[previousStart:previousEnd])
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(line, indent) &&
			(strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//")) {
			start = previousStart
			continue
		}
		break
	}
	return start, end, indent, source[start:end], nil
}

func reindentAttribute(segment []byte, oldIndent, newIndent, newName, oldName string) ([]byte, error) {
	lines := strings.SplitAfter(string(segment), "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		trimmed := strings.TrimSuffix(line, "\n")
		if strings.TrimSpace(trimmed) == "" {
			lines[i] = line
			continue
		}
		if !strings.HasPrefix(line, oldIndent) {
			return nil, fmt.Errorf("cannot preserve indentation of a multiline attribute")
		}
		lines[i] = newIndent + strings.TrimPrefix(line, oldIndent)
	}
	output := strings.Join(lines, "")
	// Rename only the attribute identifier on its own declaration line,
	// not occurrences within comments or its expression.
	offset := 0
	for _, line := range strings.SplitAfter(output, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "#") || strings.HasPrefix(t, "//") {
			offset += len(line)
			continue
		}
		prefix := newIndent + oldName
		if !strings.HasPrefix(line, prefix) {
			return nil, fmt.Errorf("attribute identifier not found")
		}
		following := strings.TrimPrefix(line, prefix)
		if strings.TrimLeft(following, " \t") == following && !strings.HasPrefix(following, "=") {
			return nil, fmt.Errorf("unexpected attribute syntax")
		}
		original := newIndent + oldName
		// Replace only the attribute declaration, not identifiers in comments
		// or expression tokens. The byte offset is tracked during this pass.
		return []byte(output[:offset] + newIndent + newName + line[len(original):] + output[offset+len(line):]), nil
	}
	return nil, fmt.Errorf("missing attribute declaration")
}
