package rewrite

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// AddMoved appends a Terraform moved block for a validated absolute address.
// An identical existing block is a no-op; conflicting addresses are rejected.
type AddMoved struct {
	File string
	From string
	To   string
}

func (e AddMoved) file() string    { return e.File }
func (e AddMoved) canCreate() bool { return true }

func (e AddMoved) apply(source []byte, filename string) ([]byte, error) {
	if err := checkTraversal(e.From); err != nil {
		return nil, fmt.Errorf("moved.from: %w", err)
	}
	if err := checkTraversal(e.To); err != nil {
		return nil, fmt.Errorf("moved.to: %w", err)
	}
	body, err := parseBody(source, filename)
	if err != nil {
		return nil, err
	}
	for _, block := range body.Blocks {
		if block.Type != "moved" {
			continue
		}
		from, okFrom := expressionSource(source, block.Body, "from")
		to, okTo := expressionSource(source, block.Body, "to")
		if !okFrom || !okTo {
			return nil, fmt.Errorf("existing moved block lacks from/to")
		}
		if from == e.From && to == e.To {
			return source, nil
		}
		if from == e.From || to == e.To {
			return nil, fmt.Errorf("conflicting moved block for %q -> %q", e.From, e.To)
		}
	}
	return appendBlock(source, fmt.Sprintf("moved {\n  from = %s\n  to   = %s\n}\n", e.From, e.To)), nil
}

// AddImport appends a Terraform import block. IDExpression is an HCL
// expression, not an evaluated value: it can be a quoted string, variable,
// or service ID reference. Its tokens are preserved exactly.
type AddImport struct {
	File         string
	To           string
	IDExpression string
}

func (e AddImport) file() string    { return e.File }
func (e AddImport) canCreate() bool { return true }

func (e AddImport) apply(source []byte, filename string) ([]byte, error) {
	if err := checkTraversal(e.To); err != nil {
		return nil, fmt.Errorf("import.to: %w", err)
	}
	if strings.TrimSpace(e.IDExpression) == "" {
		return nil, fmt.Errorf("import.id is required")
	}
	_, diags := hclsyntax.ParseExpression([]byte(e.IDExpression), "import-id", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("invalid import.id expression: %s", diags.Error())
	}
	body, err := parseBody(source, filename)
	if err != nil {
		return nil, err
	}
	for _, block := range body.Blocks {
		if block.Type != "import" {
			continue
		}
		to, hasTo := expressionSource(source, block.Body, "to")
		if !hasTo {
			return nil, fmt.Errorf("existing import block lacks to")
		}
		if to != e.To {
			continue
		}
		id, hasID := expressionSource(source, block.Body, "id")
		if !hasID {
			return nil, fmt.Errorf("existing import block for %q lacks id", e.To)
		}
		if id == strings.TrimSpace(e.IDExpression) {
			return source, nil
		}
		return nil, fmt.Errorf("conflicting import block for %q", e.To)
	}
	return appendBlock(source, fmt.Sprintf("import {\n  to = %s\n  id = %s\n}\n", e.To, e.IDExpression)), nil
}

func checkTraversal(address string) error {
	if strings.TrimSpace(address) != address || address == "" {
		return fmt.Errorf("address must be a non-empty absolute traversal")
	}
	_, diags := hclsyntax.ParseTraversalAbs([]byte(address), "address", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return fmt.Errorf("invalid address %q: %s", address, diags.Error())
	}
	return nil
}

func expressionSource(source []byte, body *hclsyntax.Body, name string) (string, bool) {
	attr := body.Attributes[name]
	if attr == nil {
		return "", false
	}
	span := attr.Expr.Range()
	if span.Start.Byte < 0 || span.End.Byte > len(source) {
		return "", false
	}
	return strings.TrimSpace(string(source[span.Start.Byte:span.End.Byte])), true
}

func appendBlock(source []byte, block string) []byte {
	out := bytes.Clone(source)
	if len(out) > 0 {
		if out[len(out)-1] != '\n' {
			out = append(out, '\n')
		}
		if !bytes.HasSuffix(out, []byte("\n\n")) {
			out = append(out, '\n')
		}
	}
	return append(out, block...)
}
