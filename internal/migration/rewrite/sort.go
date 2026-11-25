package rewrite

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
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
	tokens, diags := hclsyntax.LexConfig(source, filename, hcl.InitialPos)
	if diags.HasErrors() {
		return nil, diags
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
		start, end, err := blockLines(source, tokens, block)
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
// lines immediately above it and comments after its closing brace. A block
// must start and end on its own lines.
func blockLines(source []byte, tokens hclsyntax.Tokens, block *hclsyntax.Block) (int, int, error) {
	pos := block.TypeRange.Start.Byte
	if pos < 0 || pos > len(source) {
		return 0, 0, fmt.Errorf("invalid block source range")
	}
	start := lineStart(source, pos)
	indent := string(source[start:pos])
	if strings.TrimSpace(indent) != "" {
		return 0, 0, fmt.Errorf("%s block must start on its own line", block.Type)
	}
	closeEnd := block.CloseBraceRange.End.Byte
	if closeEnd < pos || closeEnd > len(source) {
		return 0, 0, fmt.Errorf("invalid block end range")
	}
	first := sort.Search(len(tokens), func(i int) bool { return tokens[i].Range.Start.Byte >= pos })
	next := sort.Search(len(tokens), func(i int) bool { return tokens[i].Range.Start.Byte >= closeEnd })

	end := len(source)
trailer:
	for ; next < len(tokens); next++ {
		token := tokens[next]
		switch {
		case token.Type == hclsyntax.TokenEOF:
			break trailer
		case token.Type == hclsyntax.TokenNewline,
			token.Type == hclsyntax.TokenComment && bytes.HasSuffix(token.Bytes, []byte("\n")):
			end = token.Range.End.Byte
			break trailer
		case token.Type == hclsyntax.TokenComment && !bytes.Contains(token.Bytes, []byte("\n")):
		default:
			return 0, 0, fmt.Errorf("%s block has additional syntax after its closing brace", block.Type)
		}
	}

	// Line comments include their newline; block comments are followed by a
	// separate newline token.
	for i := first - 1; i >= 0; i-- {
		comment := tokens[i]
		if comment.Type == hclsyntax.TokenNewline && i > 0 &&
			tokens[i-1].Type == hclsyntax.TokenComment && !bytes.HasSuffix(tokens[i-1].Bytes, []byte("\n")) {
			i--
			comment = tokens[i]
		} else if comment.Type != hclsyntax.TokenComment || !bytes.HasSuffix(comment.Bytes, []byte("\n")) {
			break
		}
		commentStart := lineStart(source, comment.Range.Start.Byte)
		lead := string(source[commentStart:comment.Range.Start.Byte])
		if !strings.HasPrefix(lead, indent) || strings.TrimSpace(lead) != "" {
			break
		}
		start = commentStart
	}
	return start, end, nil
}

func lineStart(source []byte, pos int) int {
	return bytes.LastIndexByte(source[:pos], '\n') + 1
}
