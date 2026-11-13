package migrate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"

	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/compatibility"
	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/rewrite"
)

// compileSimpleProducts splits the legacy VCL aggregate into standalone
// resources adopted via Terraform import. No state or API lookups occur here.
// The service ID is referenced from the moved _auto resource so this works in
// local modules and without reading credentials or remote backend state.
func compileSimpleProducts(key resourceKey, actions []selectedAction, index moduleIndex) ([]rewrite.Edit, []Issue) {
	var issues []Issue
	issue := func(message, remediation string) {
		issues = append(issues, Issue{File: key.file, Message: message, Remediation: remediation})
	}

	source := index.snapshots[key.file]
	file, diags := hclsyntax.ParseConfig(source, key.file, hcl.Pos{Line: 1, Column: 1})
	if file == nil || diags.HasErrors() {
		issue("cannot inspect legacy product_enablement configuration", "Fix invalid HCL before migration.")
		return nil, issues
	}
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		issue("unexpected HCL body", "Use native Terraform HCL before migration.")
		return nil, issues
	}
	var resource *hclsyntax.Block
	for _, block := range body.Blocks {
		if block.Type == "resource" && len(block.Labels) == 2 &&
			block.Labels[0] == key.typ && block.Labels[1] == key.name {
			resource = block
			break
		}
	}
	if resource == nil {
		issue("legacy service not found for product migration", "Check for duplicate or changed resource declarations.")
		return nil, issues
	}
	var products []*hclsyntax.Block
	for _, block := range resource.Body.Blocks {
		// The analyzer already rejects dynamic service blocks. Keep the
		// compiler fail-closed as well: a dynamic aggregate must never be
		// left inside the renamed _auto resource.
		if block.Type == "dynamic" && len(block.Labels) == 1 && block.Labels[0] == "product_enablement" {
			issue("dynamic product_enablement cannot be migrated safely",
				"Replace it with a static product_enablement block containing literal true, false or null values.")
			return nil, issues
		}
		if block.Type == "product_enablement" {
			products = append(products, block)
		}
	}
	if len(products) == 0 {
		for _, action := range actions {
			if action.rule.Kind == compatibility.ConstructProduct {
				issue("product migration action has no product_enablement block", "Review the compatibility registry.")
			}
		}
		return nil, issues
	}
	if len(products) != 1 {
		issue("service has multiple product_enablement blocks", "Consolidate legacy product settings into one block before migration.")
		return nil, issues
	}
	product := products[0]
	if len(product.Labels) != 0 {
		issue("product_enablement must not have labels", "Remove the labels from the legacy product_enablement block.")
	}

	productRules := make(map[string]compatibility.Rule)
	for _, action := range actions {
		if action.rule.Kind != compatibility.ConstructProduct {
			continue
		}
		rule := action.rule
		prefix := "fastly_service_vcl.product_enablement."
		name := strings.TrimPrefix(rule.Legacy, prefix)
		if !strings.HasPrefix(rule.Legacy, prefix) || name == "" || strings.Contains(name, ".") ||
			rule.Support != compatibility.SupportAutomatic || rule.HCL != compatibility.HCLSplit ||
			rule.State != compatibility.StateImport || rule.ImportIdentity != compatibility.ImportServiceID ||
			rule.Target != "fastly_service_product_"+name {
			issue(fmt.Sprintf("unverified product compatibility rule %q", rule.ID), "Review the product compatibility mapping.")
			continue
		}
		productRules[name] = rule
	}

	if (len(product.Body.Attributes) != 0 || len(product.Body.Blocks) != 0) &&
		(resource.Body.Attributes["count"] != nil || resource.Body.Attributes["for_each"] != nil) {
		issue("product_enablement on an indexed VCL service cannot be split safely",
			"Migrate count/for_each service instances explicitly with matching product addresses and import IDs.")
	}

	providerRef := ""
	if attr := resource.Body.Attributes["provider"]; attr != nil {
		r := attr.Expr.Range()
		if r.Start.Byte < 0 || r.End.Byte > len(source) || r.End.Byte <= r.Start.Byte {
			issue("cannot read the legacy service provider reference", "Use a standard Fastly provider configuration.")
		} else {
			providerRef = strings.TrimSpace(string(source[r.Start.Byte:r.End.Byte]))
			parts := strings.Split(providerRef, ".")
			if (len(parts) != 1 && len(parts) != 2) || parts[0] != "fastly" ||
				(len(parts) == 2 && parts[1] == "") {
				issue(fmt.Sprintf("unsupported legacy provider reference %q", providerRef),
					"Use an ordinary fastly or fastly.<alias> provider reference before migration.")
			}
		}
	}

	// Sort product names: HCL maps have no guaranteed iteration order.
	names := make([]string, 0, len(product.Body.Attributes))
	for name := range product.Body.Attributes {
		names = append(names, name)
	}
	sort.Strings(names)

	module := moduleForFile(key.file)
	migrationFile := migrationFileForModule(module)
	var edits []rewrite.Edit
	emit := func(rule compatibility.Rule, enabled bool, attrs []rewrite.ProductAttribute) {
		target := rule.Target + "." + key.name
		if file := index.resources[module][target]; file != "" {
			issue(fmt.Sprintf("standalone product %s already exists in %s", target, file),
				"Resolve the generated product address collision before migration.")
		}
		if file := index.imports[module][target]; file != "" {
			issue(fmt.Sprintf("standalone product %s is already targeted by an import in %s", target, file),
				"Remove or reconcile the conflicting import block before migration.")
		}
		// Root import blocks can address resources in local child modules.
		// Conservatively reject a matching product type/name anywhere in the
		// analyzed tree when it is module-qualified; we cannot reliably
		// resolve arbitrary module call addresses to local filesystem paths.
		for importModule, imports := range index.imports {
			if importModule == module {
				continue
			}
			for existingTo, file := range imports {
				if strings.HasSuffix(existingTo, "."+target) {
					issue(fmt.Sprintf("standalone product %s may already be targeted by a module-qualified import in %s", target, file),
						"Reconcile the existing import's module address before generating another import.")
				}
			}
		}
		edits = append(edits, rewrite.AddSimpleProduct{
			File: migrationFile, TypeName: rule.Target, Name: key.name,
			ServiceName: key.name, ProviderRef: providerRef, Enabled: enabled, Attributes: attrs,
		}, rewrite.AddImport{
			File: migrationFile, To: target,
			IDExpression: "fastly_service_cdn_auto." + key.name + ".id",
		})
	}

	for _, name := range names {
		attr := product.Body.Attributes[name]
		value, ds := attr.Expr.Value(nil)
		if ds.HasErrors() || !value.IsKnown() {
			issue(fmt.Sprintf("product_enablement.%s cannot be determined statically", name),
				"Use literal true, false or null before migrating; unknown values may enable products unexpectedly.")
			continue
		}
		if value.IsNull() {
			continue // Absent product: do not invent a resource or an import.
		}
		if value.Type() != cty.Bool {
			issue(fmt.Sprintf("product_enablement.%s must be a boolean or null", name),
				"Use a literal boolean or null before migrating.")
			continue
		}
		rule, found := productRules[name]
		if !found {
			issue(fmt.Sprintf("product_enablement.%s has no automatic import action", name),
				"Migrate the unsupported product manually before moving the service.")
			continue
		}
		emit(rule, value.True(), nil)
	}
	seen := make(map[string]bool)
	for _, nested := range product.Body.Blocks {
		name := nested.Type
		spec, known := configurableProducts[name]
		if !known {
			issue(fmt.Sprintf("product_enablement.%s has no automatic import action", name),
				"Migrate the unsupported product manually before moving the service.")
			continue
		}
		if seen[name] {
			issue(fmt.Sprintf("product_enablement has multiple %s blocks", name), "Keep a single block per product.")
			continue
		}
		seen[name] = true
		if len(nested.Labels) != 0 || len(nested.Body.Blocks) != 0 {
			issue(fmt.Sprintf("product_enablement.%s contains unsupported nested configuration", name),
				"Use a plain block with literal attributes.")
			continue
		}
		rule, found := productRules[name]
		if !found {
			issue(fmt.Sprintf("product_enablement.%s has no automatic import action", name),
				"Migrate the unsupported product manually before moving the service.")
			continue
		}
		enabled, attrs, ok := spec.literals(nested, name, issue)
		if !ok {
			continue
		}
		emit(rule, enabled, attrs)
	}
	// Even an empty/null-only aggregate block must go: _auto has no such block.
	// The rewrite edit itself detects comments and other unsafe source shapes.
	edits = append([]rewrite.Edit{rewrite.RemoveProductEnablement{
		File: key.file, ResourceName: key.name,
	}}, edits...)
	if len(issues) != 0 {
		return nil, issues
	}
	return edits, nil
}

func moduleForFile(file string) string {
	slash := strings.LastIndex(file, "/")
	if slash < 0 {
		return "."
	}
	return file[:slash]
}

func migrationFileForModule(module string) string {
	if module == "." {
		return "migration.tf"
	}
	return module + "/migration.tf"
}

// configurableProduct describes the settings, besides enabled, copied from a
// legacy block to the standalone resource, in output order.
type configurableProduct struct {
	attrs    []string
	required []string
	kinds    map[string]cty.Type
}

var configurableProducts = map[string]configurableProduct{
	"bot_management":  {attrs: []string{"contentguard"}, required: []string{"contentguard"}, kinds: map[string]cty.Type{"contentguard": cty.String}},
	"ddos_protection": {attrs: []string{"mode"}, required: []string{"mode"}, kinds: map[string]cty.Type{"mode": cty.String}},
	"ngwaf": {
		attrs:    []string{"workspace_id", "traffic_ramp"},
		required: []string{"workspace_id"},
		kinds:    map[string]cty.Type{"workspace_id": cty.String, "traffic_ramp": cty.Number},
	},
}

// literals returns the static settings of one configurable product block.
// Anything not statically known is refused: guessing could enable, relink or
// reconfigure a live product.
func (c configurableProduct) literals(block *hclsyntax.Block, name string, issue func(message, remediation string)) (enabled bool, attrs []rewrite.ProductAttribute, ok bool) {
	ok = true
	for attrName := range block.Body.Attributes {
		if _, known := c.kinds[attrName]; !known && attrName != "enabled" {
			issue(fmt.Sprintf("product_enablement.%s.%s is not supported", name, attrName),
				"Remove the attribute or migrate this product manually.")
			ok = false
		}
	}
	enabledAttr := block.Body.Attributes["enabled"]
	if enabledAttr == nil {
		issue(fmt.Sprintf("product_enablement.%s.enabled is required", name), "Set enabled to a literal boolean.")
		return false, nil, false
	}
	value, ds := enabledAttr.Expr.Value(nil)
	if ds.HasErrors() || !value.IsKnown() || value.IsNull() || value.Type() != cty.Bool {
		issue(fmt.Sprintf("product_enablement.%s.enabled must be a literal boolean", name),
			"Use literal true or false before migrating; unknown values may enable products unexpectedly.")
		return false, nil, false
	}
	enabled = value.True()
	for _, attrName := range c.required {
		if block.Body.Attributes[attrName] == nil {
			issue(fmt.Sprintf("product_enablement.%s.%s is required", name, attrName),
				"Set it to a literal value; the standalone resource cannot be created without it.")
			ok = false
		}
	}
	for _, attrName := range c.attrs {
		attr := block.Body.Attributes[attrName]
		if attr == nil {
			continue
		}
		value, ds := attr.Expr.Value(nil)
		if ds.HasErrors() || !value.IsKnown() || value.IsNull() || value.Type() != c.kinds[attrName] {
			issue(fmt.Sprintf("product_enablement.%s.%s must be a literal %s", name, attrName, c.kinds[attrName].FriendlyName()),
				"Use a literal value before migrating so the setting is preserved exactly.")
			ok = false
			continue
		}
		attrs = append(attrs, rewrite.ProductAttribute{
			Name: attrName, Value: strings.TrimSpace(string(hclwrite.TokensForValue(value).Bytes())),
		})
	}
	return enabled, attrs, ok
}
