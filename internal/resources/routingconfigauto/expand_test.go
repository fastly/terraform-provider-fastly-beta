package routingconfigauto

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func testRule(actionValue string) RuleModel {
	return RuleModel{
		ActionType:  types.StringValue("service"),
		ActionValue: types.StringValue(actionValue),
	}
}

func TestSameRules(t *testing.T) {
	a, b := testRule("a"), testRule("b")

	tests := map[string]struct {
		old, desired []RuleModel
		want         bool
	}{
		"identical":       {[]RuleModel{a, b}, []RuleModel{a, b}, true},
		"reordered":       {[]RuleModel{a, b}, []RuleModel{b, a}, false},
		"length differs":  {[]RuleModel{a}, []RuleModel{a, b}, false},
		"content differs": {[]RuleModel{a}, []RuleModel{b}, false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := sameRules(tc.old, tc.desired)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("sameRules() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestValidateRules_defaultRule(t *testing.T) {
	conditional := testRule("a")
	conditional.Condition = []ConditionModel{{
		Key:      types.StringValue("X-Beta"),
		Operator: types.StringValue("equals"),
		Type:     types.StringValue("header"),
		Value:    types.StringValue("1"),
	}}

	tests := map[string]struct {
		rules   []RuleModel
		wantErr bool
	}{
		"default only":          {[]RuleModel{testRule("a")}, false},
		"conditional + default": {[]RuleModel{conditional, testRule("b")}, false},
		"no default":            {[]RuleModel{conditional}, true},
		"no rules":              {nil, true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var diags diag.Diagnostics
			validateRules(0, PathModel{Rule: tc.rules}, &diags)
			if diags.HasError() != tc.wantErr {
				t.Fatalf("HasError() = %v, want %v: %v", diags.HasError(), tc.wantErr, diags)
			}
		})
	}
}

func TestPathsDiffer(t *testing.T) {
	path := func(p string, rules ...RuleModel) PathModel {
		return PathModel{Path: types.StringValue(p), Rule: rules}
	}
	a, b := testRule("a"), testRule("b")

	tests := map[string]struct {
		live, state []PathModel
		want        bool
	}{
		"identical":           {[]PathModel{path("/", a)}, []PathModel{path("/", a)}, false},
		"path order ignored":  {[]PathModel{path("/x", a), path("/y", b)}, []PathModel{path("/y", b), path("/x", a)}, false},
		"extra live path":     {[]PathModel{path("/", a), path("/x", a)}, []PathModel{path("/", a)}, true},
		"live path missing":   {[]PathModel{path("/", a)}, []PathModel{path("/x", a)}, true},
		"live rules differ":   {[]PathModel{path("/", a, b)}, []PathModel{path("/", a)}, true},
		"live rule reordered": {[]PathModel{path("/", b, a)}, []PathModel{path("/", a, b)}, true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := pathsDiffer(tc.live, tc.state)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("pathsDiffer() = %v, want %v", got, tc.want)
			}
		})
	}
}
