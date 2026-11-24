package analyze

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"

	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/compatibility"
	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/rewrite"
)

// analyzeVCLBlockValues blocks VCL configuration that the migrator cannot
// put into the target's order and value spelling. Unknown expressions are
// left for Terraform to validate. Findings never include the values.
func analyzeVCLBlockValues(state *analysisState, body *hclsyntax.Body) {
	reject := func(rng hcl.Range, message, remediation string) {
		state.result.Findings = append(state.result.Findings, Finding{
			Severity: SeverityError, Range: rng, Message: message, Remediation: remediation,
		})
	}
	counts := map[string]int{}
	for _, block := range body.Blocks {
		counts[block.Type]++
	}
	names := map[string]map[string]bool{}
	for _, block := range body.Blocks {
		if counts[block.Type] < 2 || !slices.Contains(compatibility.VCLNameOrderedBlocks, block.Type) {
			continue
		}
		attr := block.Body.Attributes["name"]
		var name string
		var isString, ok bool
		if attr != nil {
			name, isString, ok = rewrite.LiteralKey(attr.Expr)
		}
		if !ok || !isString {
			reject(block.TypeRange, fmt.Sprintf("This %s block needs a quoted name so the migration can sort it with the other %s blocks.", block.Type, block.Type),
				`Set name to a quoted string such as name = "origin" instead of a variable or expression. fastly_service_cdn_auto stores these blocks in name order.`)
			continue
		}
		if names[block.Type] == nil {
			names[block.Type] = map[string]bool{}
		}
		if names[block.Type][name] {
			reject(block.TypeRange, fmt.Sprintf("Repeated %s blocks have duplicate names.", block.Type),
				"Give each block a unique name before migrating.")
		}
		names[block.Type][name] = true
	}

	for _, block := range body.Blocks {
		for _, change := range compatibility.VCLLiteralRewrites {
			if change.Block != block.Type {
				continue
			}
			attr := block.Body.Attributes[change.Attribute]
			if attr == nil {
				continue
			}
			value, diags := attr.Expr.Value(nil)
			if diags.HasErrors() || !value.IsKnown() || value.IsNull() {
				continue
			}
			literal, isString, isLiteral := rewrite.LiteralKey(attr.Expr)
			if isLiteral && isString && literal == "" && change.RemoveEmpty {
				continue
			}
			if len(change.Values) == 0 {
				continue
			}
			key, accepted := "", false
			if isLiteral {
				key = literal
				if isString && change.CaseInsensitive {
					key = strings.ToLower(literal)
				}
				_, accepted = change.Values[key]
			}
			if !accepted {
				reject(attr.NameRange, fmt.Sprintf("The %s %q value cannot be migrated. Accepted values: %s.", block.Type, change.Attribute, acceptedValues(change)),
					"Set one of the accepted values directly, not through an expression, or remove the attribute to use its default.")
			}
		}
	}
}

// acceptedValues lists the legacy values a rewrite accepts, quoting strings.
func acceptedValues(change compatibility.LiteralRewrite) string {
	values := make([]string, 0, len(change.Values))
	for value := range change.Values {
		if _, err := strconv.Atoi(value); err != nil {
			value = strconv.Quote(value)
		}
		values = append(values, value)
	}
	sort.Strings(values)
	return strings.Join(values, ", ")
}
