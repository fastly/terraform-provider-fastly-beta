package ngwafsignalrule

import (
	"context"
	"testing"

	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafrule"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/scope"
)

// TestSchemaMatchesModel guards the one thing the compiler cannot: Model has
// to declare exactly the schema's attributes and blocks, and it reaches most
// of them through an embedded struct.
func TestSchemaMatchesModel(t *testing.T) {
	ctx := context.Background()

	var resp resource.SchemaResponse
	(&Resource{}).Schema(ctx, resource.SchemaRequest{}, &resp)

	if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("invalid schema: %s", diags)
	}

	objType, ok := resp.Schema.Type().(types.ObjectType)
	if !ok {
		t.Fatalf("schema type is %T, want types.ObjectType", resp.Schema.Type())
	}

	// applies_to needs a typed null: a zero-value types.Set carries no
	// element type, which no schema can match. Every other field's zero
	// value is already a valid null.
	empty := Model{AppliesTo: types.SetNull(types.StringType)}
	if _, diags := types.ObjectValueFrom(ctx, objType.AttributeTypes(), empty); diags.HasError() {
		t.Fatalf("Model does not match schema: %s", diags)
	}
}

func testModel(appliesTo ...string) Model {
	elements := make([]attr.Value, 0, len(appliesTo))
	for _, id := range appliesTo {
		elements = append(elements, types.StringValue(id))
	}

	return Model{
		AppliesTo: types.SetValueMust(types.StringType, elements),
		CommonModel: ngwafrule.CommonModel{
			Enabled: types.BoolValue(true),
			Condition: []ngwafrule.ConditionModel{{
				Field:    types.StringValue("path"),
				Operator: types.StringValue("equals"),
				Value:    types.StringValue("/health"),
			}},
		},
		Description: types.StringValue("Exclude a signal account-wide"),
		Action: []ngwafrule.SignalActionModel{{
			Signal: types.StringValue("SQLI"),
		}},
	}
}

// TestBuildCreateInputHardcodesActionType pins the action type the schema does
// not expose: a signal rule's only valid action is exclude_signal, so expand
// supplies it.
func TestBuildCreateInputHardcodesActionType(t *testing.T) {
	input, diags := BuildCreateInput(context.Background(), testModel("*"))
	if diags.HasError() {
		t.Fatalf("BuildCreateInput: %s", diags)
	}

	if got := *input.Type; got != RuleType {
		t.Errorf("Type = %q, want %q", got, RuleType)
	}
	if got := input.Scope.Type; got != scope.ScopeTypeAccount {
		t.Errorf("Scope.Type = %q, want %q", got, scope.ScopeTypeAccount)
	}
	if got := input.Scope.AppliesTo; len(got) != 1 || got[0] != "*" {
		t.Errorf("Scope.AppliesTo = %v, want [*]", got)
	}
	if got := len(input.Actions); got != 1 {
		t.Fatalf("len(Actions) = %d, want 1", got)
	}
	if got := *input.Actions[0].Type; got != ActionType {
		t.Errorf("Actions[0].Type = %q, want %q", got, ActionType)
	}
	if got := *input.Actions[0].Signal; got != "SQLI" {
		t.Errorf("Actions[0].Signal = %q, want %q", got, "SQLI")
	}
}

func TestBuildUpdateInput(t *testing.T) {
	input, diags := BuildUpdateInput(context.Background(), "rule123", testModel("ws123"))
	if diags.HasError() {
		t.Fatalf("BuildUpdateInput: %s", diags)
	}

	if got := *input.RuleID; got != "rule123" {
		t.Errorf("RuleID = %q, want %q", got, "rule123")
	}
	if got := *input.Actions[0].Type; got != ActionType {
		t.Errorf("Actions[0].Type = %q, want %q", got, ActionType)
	}
}

func TestFlattenToModel(t *testing.T) {
	rule := &rules.Rule{
		RuleID:      "rule123",
		Type:        RuleType,
		Description: "Exclude a signal account-wide",
		Enabled:     true,
		Scope:       rules.Scope{Type: "account", AppliesTo: []string{"*"}},
		Conditions: []rules.ConditionItem{{
			Type:   "single",
			Fields: rules.SingleCondition{Field: "path", Operator: "equals", Value: "/health"},
		}},
		Actions: []rules.Action{{Type: ActionType, Signal: "SQLI"}},
	}

	got, diags := FlattenToModel(context.Background(), rule)
	if diags.HasError() {
		t.Fatalf("FlattenToModel: %s", diags)
	}

	if got.ID.ValueString() != "rule123" {
		t.Errorf("ID = %q, want %q", got.ID.ValueString(), "rule123")
	}
	if n := len(got.AppliesTo.Elements()); n != 1 {
		t.Errorf("len(AppliesTo) = %d, want 1", n)
	}
	if len(got.Action) != 1 || got.Action[0].Signal.ValueString() != "SQLI" {
		t.Errorf("Action = %+v, want one action excluding SQLI", got.Action)
	}
}
