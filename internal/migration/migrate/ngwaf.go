package migrate

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"

	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/compatibility"
	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/rewrite"
)

// ngwafRemoval drops a legacy attribute the target type does not have.
// When value is set, the attribute may only be dropped if it is that literal:
// any other value would carry meaning the target cannot represent.
type ngwafRemoval struct {
	parents []string
	name    string
	value   *string
}

var (
	ngwafExcludeSignal   = "exclude_signal"
	ngwafTemplatedSignal = "templated_signal"
	ngwafEmpty           = ""
)

// ngwafRemovals lists, per target type, the legacy attributes to drop. The
// split discriminator `type` is dropped from every split target; it already
// selected the target type.
var ngwafRemovals = func() map[string][]ngwafRemoval {
	discriminator := ngwafRemoval{name: "type"}
	requestLogging := ngwafRemoval{name: "request_logging"}
	removals := map[string][]ngwafRemoval{
		"fastly_ngwaf_request_rule":           {discriminator},
		"fastly_ngwaf_workspace_request_rule": {discriminator},
		"fastly_ngwaf_signal_rule": {
			discriminator, requestLogging,
			{parents: []string{"action"}, name: "type", value: &ngwafExcludeSignal},
		},
		"fastly_ngwaf_workspace_signal_rule": {
			discriminator, requestLogging,
			{parents: []string{"action"}, name: "type", value: &ngwafExcludeSignal},
		},
		"fastly_ngwaf_workspace_rate_limit_rule": {discriminator, requestLogging},
		"fastly_ngwaf_workspace_templated_signal_rule": {
			discriminator, requestLogging,
			{name: "description", value: &ngwafEmpty},
			{parents: []string{"action"}, name: "type", value: &ngwafTemplatedSignal},
		},
	}
	for _, kind := range []string{"ip", "string", "country", "wildcard", "signal"} {
		removals["fastly_ngwaf_"+kind+"_list"] = []ngwafRemoval{discriminator}
		removals["fastly_ngwaf_workspace_"+kind+"_list"] = []ngwafRemoval{discriminator}
	}
	return removals
}()

func compileNGWAFResource(key resourceKey, actions []selectedAction, index moduleIndex) ([]rewrite.Edit, *Move, []Issue) {
	var issues []Issue
	issue := func(ruleID, message string) {
		issues = append(issues, Issue{
			RuleID: ruleID, File: key.file, Message: message,
			Remediation: "Adjust the legacy configuration or migrate this resource manually.",
		})
	}

	var resourceRule *compatibility.Rule
	for i := range actions {
		rule := actions[i].rule
		if rule.Kind != compatibility.ConstructResource {
			continue
		}
		if resourceRule != nil || rule.Legacy != key.typ || rule.State != compatibility.StateMove ||
			(rule.HCL != compatibility.HCLRename && rule.HCL != compatibility.HCLSplit) {
			issue(rule.ID, fmt.Sprintf("unimplemented resource conversion %s -> %s", rule.Legacy, rule.Target))
			continue
		}
		resourceRule = &rule
	}
	if resourceRule == nil {
		issue("", fmt.Sprintf("resource %s.%s has no supported NGWAF conversion action", key.typ, key.name))
		return nil, nil, issues
	}
	target := resourceRule.Target

	module := filepath.ToSlash(filepath.Dir(key.file))
	fromAddress := key.typ + "." + key.name
	toAddress := target + "." + key.name
	issues = append(issues, moveIssues(key, module, fromAddress, toAddress, index)...)

	block, err := ngwafSourceBlock(index.snapshots[key.file], key)
	if err != nil {
		issue(resourceRule.ID, err.Error())
		return nil, nil, issues
	}

	var edits []rewrite.Edit
	for _, removal := range ngwafRemovals[target] {
		message, present := checkNGWAFRemoval(block, removal)
		if message != "" {
			issue(resourceRule.ID, message)
			continue
		}
		if !present {
			continue
		}
		edits = append(edits, rewrite.RemoveAttribute{
			File: key.file, ResourceType: key.typ, ResourceName: key.name,
			Parents: removal.parents, Attribute: removal.name,
		})
	}

	var authentication []rewrite.ObjectField
	for _, a := range actions {
		rule := a.rule
		if rule.Kind != compatibility.ConstructAttribute {
			continue
		}
		if field, ok := ngwafAuthenticationField(rule, key.typ, target); ok {
			authentication = append(authentication, field)
			continue
		}
		edit, err := compileAttribute(key, rule, target)
		if err != nil {
			issue(rule.ID, err.Error())
		} else if edit != nil {
			edits = append(edits, edit)
		}
	}
	if len(authentication) != 0 {
		edits = append(edits, rewrite.NestObject{
			File: key.file, ResourceType: key.typ, ResourceName: key.name,
			Target: "authentication", Fields: authentication,
		})
	}
	if len(issues) != 0 {
		return nil, nil, issues
	}

	// Attribute edits run against the legacy resource type; rename it last.
	edits = append(edits, rewrite.RenameResource{File: key.file, FromType: key.typ, ToType: target, Name: key.name})
	if _, exists := index.moves[module][fromAddress]; !exists {
		edits = append(edits, rewrite.AddMoved{
			File: filepath.ToSlash(filepath.Join(module, "migration.tf")), From: fromAddress, To: toAddress,
		})
	}
	return edits, &Move{Module: module, From: fromAddress, To: toAddress}, nil
}

// ngwafAuthenticationField authorizes only the registry's automatic alert
// credential reshapes into the target's single-nested authentication object.
func ngwafAuthenticationField(rule compatibility.Rule, legacyType, targetType string) (rewrite.ObjectField, bool) {
	if rule.Support != compatibility.SupportAutomatic || rule.HCL != compatibility.HCLReshape {
		return rewrite.ObjectField{}, false
	}
	name, ok := strings.CutPrefix(rule.Legacy, legacyType+".")
	if !ok || strings.Contains(name, ".") || rule.Target != targetType+".authentication."+name {
		return rewrite.ObjectField{}, false
	}
	return rewrite.ObjectField{From: name, To: name}, true
}

// checkNGWAFRemoval returns why an attribute cannot be dropped safely, or "",
// and whether the attribute appears at all.
func checkNGWAFRemoval(resource *hclsyntax.Block, removal ngwafRemoval) (string, bool) {
	path := strings.Join(append(append([]string(nil), removal.parents...), removal.name), ".")
	bodies := []*hclsyntax.Body{resource.Body}
	for _, parent := range removal.parents {
		var next []*hclsyntax.Body
		for _, body := range bodies {
			for _, child := range body.Blocks {
				if child.Type == "dynamic" && len(child.Labels) == 1 && child.Labels[0] == parent {
					return fmt.Sprintf("dynamic %q blocks cannot be migrated: %s must be removed from each generated block", parent, path), false
				}
				if child.Type == parent {
					next = append(next, child.Body)
				}
			}
		}
		bodies = next
	}
	present := false
	for _, body := range bodies {
		attr := body.Attributes[removal.name]
		if attr == nil {
			continue
		}
		present = true
		if removal.value == nil {
			continue
		}
		value, known := ngwafLiteralString(attr.Expr)
		if !known || value != *removal.value {
			return fmt.Sprintf("%s must be the literal %s to migrate; the target resource cannot represent another value", path, strconv.Quote(*removal.value)), false
		}
	}
	return "", present
}

func ngwafLiteralString(expr hclsyntax.Expression) (string, bool) {
	value, diags := expr.Value(nil)
	if diags.HasErrors() || !value.IsKnown() || value.IsNull() || value.Type() != cty.String {
		return "", false
	}
	return value.AsString(), true
}

func ngwafSourceBlock(source []byte, key resourceKey) (*hclsyntax.Block, error) {
	parsed, diags := hclsyntax.ParseConfig(source, key.file, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() || parsed == nil {
		return nil, fmt.Errorf("cannot inspect %s.%s", key.typ, key.name)
	}
	for _, block := range parsed.Body.(*hclsyntax.Body).Blocks {
		if block.Type == "resource" && len(block.Labels) == 2 && block.Labels[0] == key.typ && block.Labels[1] == key.name {
			return block, nil
		}
	}
	return nil, fmt.Errorf("resource %s.%s was not found at its analyzed location", key.typ, key.name)
}
