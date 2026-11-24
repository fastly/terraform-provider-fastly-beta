package rewrite

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// SortBlocksByName reorders a resource's direct child blocks of one type into
// ascending literal name order. Legacy SDKv2 stored these blocks as sets, but
// the target reads them back as lists sorted by name, so unsorted HCL would
// produce a migration-only reorder. Each block keeps its attached comments
// and source bytes, and blocks of other types keep their positions.
type SortBlocksByName struct {
	File         string
	ResourceType string
	ResourceName string
	Block        string
}

func (e SortBlocksByName) file() string    { return e.File }
func (e SortBlocksByName) canCreate() bool { return false }

func (e SortBlocksByName) apply(source []byte, filename string) ([]byte, error) {
	if err := validateIdentifier(e.Block); err != nil {
		return nil, err
	}
	resource, err := requireResource(source, filename, e.ResourceType, e.ResourceName)
	if err != nil {
		return nil, err
	}
	type segment struct {
		name       string
		start, end int
	}
	var blocks []*hclsyntax.Block
	for _, block := range resource.Body.Blocks {
		if block.Type == e.Block {
			blocks = append(blocks, block)
		}
	}
	if len(blocks) < 2 {
		return source, nil
	}
	var segments []segment
	seen := map[string]bool{}
	for _, block := range blocks {
		name, err := literalBlockName(block)
		if err != nil {
			return nil, err
		}
		if seen[name] {
			return nil, fmt.Errorf("%s blocks have duplicate names", e.Block)
		}
		seen[name] = true
		start, end, err := blockLines(source, block)
		if err != nil {
			return nil, err
		}
		segments = append(segments, segment{name: name, start: start, end: end})
	}
	sorted := append([]segment(nil), segments...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].name < sorted[j].name })
	var edits []replacement
	for i, slot := range segments {
		if sorted[i].name == slot.name {
			continue
		}
		edits = append(edits, replacement{
			start: slot.start, end: slot.end,
			value: append([]byte(nil), source[sorted[i].start:sorted[i].end]...),
		})
	}
	return replace(source, edits)
}

func literalBlockName(block *hclsyntax.Block) (string, error) {
	attr := block.Body.Attributes["name"]
	if attr == nil {
		return "", literalNameError(block)
	}
	name, isString, ok := LiteralKey(attr.Expr)
	if !ok || !isString {
		return "", literalNameError(block)
	}
	return name, nil
}

// blockLines returns a block's full lines, including adjacent comment-only
// lines immediately above it. A block must start and end on its own lines.
func blockLines(source []byte, block *hclsyntax.Block) (int, int, error) {
	pos := block.TypeRange.Start.Byte
	if pos < 0 || pos > len(source) {
		return 0, 0, fmt.Errorf("invalid block source range")
	}
	start := bytes.LastIndexByte(source[:pos], '\n') + 1
	indent := string(source[start:pos])
	if strings.TrimSpace(indent) != "" {
		return 0, 0, fmt.Errorf("%s block must start on its own line", block.Type)
	}
	closeEnd := block.CloseBraceRange.End.Byte
	if closeEnd < pos || closeEnd > len(source) {
		return 0, 0, fmt.Errorf("invalid block end range")
	}
	end := len(source)
	if next := bytes.IndexByte(source[closeEnd:], '\n'); next >= 0 {
		end = closeEnd + next + 1
	}
	trailer := strings.TrimSpace(string(source[closeEnd:end]))
	if trailer != "" && !strings.HasPrefix(trailer, "#") && !strings.HasPrefix(trailer, "//") {
		return 0, 0, fmt.Errorf("%s block has additional syntax after its closing brace", block.Type)
	}
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
	return start, end, nil
}
