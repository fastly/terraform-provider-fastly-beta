package rewrite

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// RemoveProductEnablement removes one unambiguous legacy product_enablement
// block. The planner must first account for every configured product.
// Comments inside the deleted block are rejected rather than silently lost.
type RemoveProductEnablement struct {
	File, ResourceName string
}

func (e RemoveProductEnablement) file() string    { return e.File }
func (e RemoveProductEnablement) canCreate() bool { return false }

func (e RemoveProductEnablement) apply(source []byte, filename string) ([]byte, error) {
	resource, err := requireResource(source, filename, "fastly_service_vcl", e.ResourceName)
	if err != nil {
		return nil, err
	}
	var product *hclsyntax.Block
	for _, block := range resource.Body.Blocks {
		if block.Type != "product_enablement" {
			continue
		}
		if product != nil {
			return nil, fmt.Errorf("multiple product_enablement blocks in fastly_service_vcl.%s", e.ResourceName)
		}
		product = block
	}
	if product == nil {
		return nil, fmt.Errorf("product_enablement block not found in fastly_service_vcl.%s", e.ResourceName)
	}
	start, end := product.Range().Start.Byte, product.Range().End.Byte
	if start < 0 || end > len(source) || end <= start || source[end-1] != '}' {
		return nil, fmt.Errorf(
			"cannot locate product_enablement block boundaries in %s (fastly_service_vcl.%s) at byte range [%d,%d)",
			filename, e.ResourceName, start, end,
		)
	}
	// Do not discard comments, even when they occur inside an empty or
	// null-only product block. They may describe user intent.
	contents := source[start:end]
	if bytes.Contains(contents, []byte("#")) || bytes.Contains(contents, []byte("//")) ||
		bytes.Contains(contents, []byte("/*")) {
		return nil, fmt.Errorf("product_enablement contains comments that cannot be preserved automatically")
	}
	lineStart := bytes.LastIndexByte(source[:start], '\n') + 1
	if strings.TrimSpace(string(source[lineStart:start])) != "" {
		return nil, fmt.Errorf("product_enablement must start on its own line")
	}
	lineEnd := end
	for lineEnd < len(source) && source[lineEnd] != '\n' {
		lineEnd++
	}
	if strings.TrimSpace(string(source[end:lineEnd])) != "" {
		return nil, fmt.Errorf("product_enablement has trailing content that cannot be preserved")
	}
	if lineEnd < len(source) {
		lineEnd++
	}
	return replace(source, []replacement{{start: lineStart, end: lineEnd}})
}

// AddSimpleProduct creates a separately managed, service-scoped product.
// The imported product is never created by the CLI. Terraform imports it
// later with the ID of the moved legacy service.
type AddSimpleProduct struct {
	File, TypeName, Name, ServiceName, ProviderRef string
	Enabled                                        bool
	// Attributes are additional literal settings (HCL expression source) for
	// configurable products, rendered after enabled in the given order.
	Attributes []ProductAttribute
}

// ProductAttribute is one literal attribute of a generated product resource.
type ProductAttribute struct{ Name, Value string }

func (e AddSimpleProduct) file() string    { return e.File }
func (e AddSimpleProduct) canCreate() bool { return true }

func (e AddSimpleProduct) apply(source []byte, filename string) ([]byte, error) {
	if !strings.HasPrefix(e.TypeName, "fastly_service_product_") {
		return nil, fmt.Errorf("invalid standalone product type %q", e.TypeName)
	}
	to := e.TypeName + "." + e.Name
	service := "fastly_service_cdn_auto." + e.ServiceName
	if err := checkTraversal(to); err != nil {
		return nil, err
	}
	if err := checkTraversal(service); err != nil {
		return nil, err
	}
	if e.ProviderRef != "" {
		if err := checkTraversal(e.ProviderRef); err != nil {
			return nil, err
		}
	}
	body, err := parseBody(source, filename)
	if err != nil {
		return nil, err
	}
	if findResource(body, e.TypeName, e.Name) != nil {
		return nil, fmt.Errorf("standalone product resource %s already exists in %s", to, filename)
	}
	enabled := "false"
	if e.Enabled {
		enabled = "true"
	}
	lines := []ProductAttribute{{"service_id", service + ".id"}, {"enabled", enabled}}
	if e.ProviderRef != "" {
		lines = append(lines, ProductAttribute{"provider", e.ProviderRef})
	}
	width := 0
	for _, attribute := range append(lines, e.Attributes...) {
		if !validAttributeName(attribute.Name) {
			return nil, fmt.Errorf("invalid product attribute name %q", attribute.Name)
		}
		if len(attribute.Name) > width {
			width = len(attribute.Name)
		}
	}
	block := fmt.Sprintf("resource %q %q {\n", e.TypeName, e.Name)
	for _, attribute := range append(lines, e.Attributes...) {
		block += fmt.Sprintf("  %-*s = %s\n", width, attribute.Name, attribute.Value)
	}
	return appendBlock(source, block+"}\n"), nil
}

func validAttributeName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if r != '_' && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}
