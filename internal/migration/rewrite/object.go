package rewrite

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// ObjectField maps a flat attribute to a key in a nested object attribute.
type ObjectField struct {
	From string
	To   string
}

// NestObject moves a group of flat attributes into a single object attribute.
// Each repeated parent is handled independently: absent fields stay absent.
// Expressions are copied verbatim, including multiline and heredoc contents.
// An existing destination alongside legacy fields is ambiguous and is rejected.
type NestObject struct {
	File         string
	ResourceType string
	ResourceName string
	Parents      []string
	Target       string
	Fields       []ObjectField
}

func (e NestObject) file() string    { return e.File }
func (e NestObject) canCreate() bool { return false }

func (e NestObject) apply(source []byte, filename string) ([]byte, error) {
	if err := validateIdentifier(e.Target); err != nil {
		return nil, err
	}
	fields := make(map[string]string, len(e.Fields))
	targets := make(map[string]bool, len(e.Fields))
	for _, field := range e.Fields {
		if err := validateIdentifier(field.From); err != nil {
			return nil, err
		}
		if err := validateIdentifier(field.To); err != nil {
			return nil, err
		}
		if field.From == e.Target || fields[field.From] != "" || targets[field.To] {
			return nil, fmt.Errorf("ambiguous object attribute mapping")
		}
		fields[field.From], targets[field.To] = field.To, true
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("empty object attribute mapping")
	}

	resource, err := requireResource(source, filename, e.ResourceType, e.ResourceName)
	if err != nil {
		return nil, err
	}
	parents := matchingParentBlocks(resource, e.Parents)
	if len(parents) == 0 {
		return nil, fmt.Errorf("object attribute parent path not found")
	}
	var edits []replacement
	for _, parent := range parents {
		parentEdits, err := nestObjectInBody(source, parent.Body, e.Target, fields)
		if err != nil {
			return nil, err
		}
		edits = append(edits, parentEdits...)
	}
	return replace(source, edits)
}

func nestObjectInBody(source []byte, body *hclsyntax.Body, target string, fields map[string]string) ([]replacement, error) {
	var attributes []*hclsyntax.Attribute
	for name := range fields {
		if original := body.Attributes[name]; original != nil {
			attributes = append(attributes, original)
		}
	}
	if len(attributes) == 0 {
		return nil, nil
	}
	if body.Attributes[target] != nil {
		return nil, fmt.Errorf("target object %q already exists alongside legacy attributes", target)
	}
	for _, block := range body.Blocks {
		if block.Type == target {
			return nil, fmt.Errorf("target object %q conflicts with an existing block", target)
		}
	}

	// Use source order, not map or registry iteration order. Insert the new
	// object at the first moved attribute and leave unrelated bytes in place.
	sort.Slice(attributes, func(i, j int) bool {
		return attributes[i].NameRange.Start.Byte < attributes[j].NameRange.Start.Byte
	})
	var edits []replacement
	var object bytes.Buffer
	var indent, newline string
	for i, attribute := range attributes {
		start, end, lineIndent, err := objectAttributeLines(source, attribute)
		if err != nil {
			return nil, err
		}
		if i == 0 {
			indent = lineIndent
			newline = "\n"
			if bytes.HasSuffix(source[start:end], []byte("\r\n")) {
				newline = "\r\n"
			}
			object.WriteString(indent + target + " = {" + newline)
		}
		// Copy everything following the identifier without reindenting it.
		// In particular, changing indentation inside <<- heredocs can change
		// their evaluated strings. Comments and the original expression bytes
		// are preserved, with only the outer key's indentation/name changed.
		nameStart, nameEnd := attribute.NameRange.Start.Byte, attribute.NameRange.End.Byte
		lineStart := bytes.LastIndexByte(source[:nameStart], '\n') + 1
		object.Write(source[start:lineStart])
		object.WriteString(indent + "  " + fields[attribute.Name])
		object.Write(source[nameEnd:end])
		if source[end-1] != '\n' {
			object.WriteString(newline)
		}
		edits = append(edits, replacement{start: start, end: end})
	}
	object.WriteString(indent + "}" + newline)
	edits[0].value = object.Bytes()
	return edits, nil
}

// objectAttributeLines includes adjacent line comments and the attribute's
// trailing line, while rejecting inline layouts that could swallow other HCL.
// Unlike the block writer, this does not change the expression's indentation.
func objectAttributeLines(source []byte, attribute *hclsyntax.Attribute) (int, int, string, error) {
	name := attribute.NameRange.Start.Byte
	end := attribute.SrcRange.End.Byte
	if name < 0 || end <= name || end > len(source) ||
		attribute.NameRange.End.Byte > end {
		return 0, 0, "", fmt.Errorf("invalid object attribute source range")
	}
	start := bytes.LastIndexByte(source[:name], '\n') + 1
	indent := string(source[start:name])
	if strings.TrimSpace(indent) != "" {
		return 0, 0, "", fmt.Errorf("object attribute must start on its own line")
	}
	// A heredoc range can include the terminating newline already. Never
	// capture the next attribute or closing brace in that case.
	if source[end-1] != '\n' {
		lineEnd := len(source)
		if next := bytes.IndexByte(source[end:], '\n'); next >= 0 {
			lineEnd = end + next + 1
		}
		trailer := strings.TrimSpace(string(source[end:lineEnd]))
		if trailer != "" && !strings.HasPrefix(trailer, "#") && !strings.HasPrefix(trailer, "//") {
			return 0, 0, "", fmt.Errorf("object attribute has additional syntax on the same line")
		}
		end = lineEnd
	}
	for start > 0 {
		previousEnd := start - 1
		previousStart := bytes.LastIndexByte(source[:previousEnd], '\n') + 1
		line := string(source[previousStart:previousEnd])
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(line, indent) ||
			(!strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "//")) {
			break
		}
		start = previousStart
	}
	return start, end, indent, nil
}
