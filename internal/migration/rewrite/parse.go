package rewrite

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func parseBody(source []byte, filename string) (*hclsyntax.Body, error) {
	file, diagnostics := hclsyntax.ParseConfig(source, filename, hcl.Pos{Line: 1, Column: 1})
	if diagnostics.HasErrors() {
		return nil, fmt.Errorf("parse native HCL: %s", diagnostics.Error())
	}
	if file == nil {
		return nil, fmt.Errorf("parse native HCL: no HCL file returned")
	}
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return nil, fmt.Errorf("unexpected HCL body type %T", file.Body)
	}
	return body, nil
}

func findResource(body *hclsyntax.Body, typ, name string) *hclsyntax.Block {
	for _, block := range body.Blocks {
		if block.Type == "resource" && len(block.Labels) == 2 &&
			block.Labels[0] == typ && block.Labels[1] == name {
			return block
		}
	}
	return nil
}

func matchingBodies(body *hclsyntax.Body, path []string) []*hclsyntax.Body {
	if len(path) == 0 {
		return []*hclsyntax.Body{body}
	}
	var result []*hclsyntax.Body
	for _, block := range body.Blocks {
		if block.Type == path[0] {
			result = append(result, matchingBodies(block.Body, path[1:])...)
		}
	}
	return result
}

type replacement struct {
	start int
	end   int
	value []byte
}

func replace(source []byte, changes []replacement) ([]byte, error) {
	if len(changes) == 0 {
		return source, nil
	}
	// Apply from end of file backward so every source offset stays valid.
	for i := 0; i < len(changes); i++ {
		for j := i + 1; j < len(changes); j++ {
			if changes[i].start < changes[j].start {
				changes[i], changes[j] = changes[j], changes[i]
			}
		}
	}
	previous := len(source)
	for _, change := range changes {
		if change.start < 0 || change.end < change.start ||
			change.end > previous {
			return nil, fmt.Errorf("overlapping or invalid HCL source ranges")
		}
		source = append(append(append([]byte{}, source[:change.start]...), change.value...), source[change.end:]...)
		previous = change.start
	}
	return source, nil
}

func requireResource(source []byte, filename, typ, name string) (*hclsyntax.Block, error) {
	body, err := parseBody(source, filename)
	if err != nil {
		return nil, err
	}
	resource := findResource(body, typ, name)
	if resource == nil {
		return nil, fmt.Errorf("resource %q.%q not found", typ, name)
	}
	return resource, nil
}

func validateIdentifier(name string) error {
	if name == "" {
		return fmt.Errorf("empty HCL identifier")
	}
	// HCL identifiers can contain dashes, but never punctuation or whitespace.
	for i, r := range name {
		if r == '-' && i > 0 {
			continue
		}
		if r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			continue
		}
		if i > 0 && r >= '0' && r <= '9' {
			continue
		}
		return fmt.Errorf("invalid HCL identifier %q", name)
	}
	if strings.HasSuffix(name, "-") {
		return fmt.Errorf("invalid HCL identifier %q", name)
	}
	return nil
}
