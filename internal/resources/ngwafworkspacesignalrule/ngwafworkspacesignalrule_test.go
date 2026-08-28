package ngwafworkspacesignalrule

import (
	"context"
	"testing"

	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafrule"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
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

	if _, diags := types.ObjectValueFrom(ctx, objType.AttributeTypes(), Model{}); diags.HasError() {
		t.Fatalf("Model does not match schema: %s", diags)
	}
}

func testModel() Model {
	return Model{
		CommonModel: ngwafrule.CommonModel{
			WorkspaceID: types.StringValue("ws123"),
			Enabled:     types.BoolValue(true),
			Condition: []ngwafrule.ConditionModel{{
				Field:    types.StringValue("path"),
				Operator: types.StringValue("like"),
				Value:    types.StringValue("/admin*"),
			}},
		},
		Description: types.StringValue("Exclude XSS on admin paths"),
		Action:      []ngwafrule.SignalActionModel{{Signal: types.StringValue("XSS")}},
	}
}

// Action has no type field, so BuildCreateInput must be the one supplying it.
func TestBuildCreateInput_suppliesActionType(t *testing.T) {
	input := BuildCreateInput(testModel())

	if got := *input.Type; got != RuleType {
		t.Errorf("Type = %q, want %q", got, RuleType)
	}
	if got := len(input.Actions); got != 1 {
		t.Fatalf("len(Actions) = %d, want 1", got)
	}
	if got := *input.Actions[0].Type; got != ActionType {
		t.Errorf("Actions[0].Type = %q, want %q", got, ActionType)
	}
	if got := *input.Actions[0].Signal; got != "XSS" {
		t.Errorf("Actions[0].Signal = %q, want %q", got, "XSS")
	}
}

func TestBuildUpdateInput_suppliesActionType(t *testing.T) {
	input := BuildUpdateInput("rule123", testModel())

	if got := *input.RuleID; got != "rule123" {
		t.Errorf("RuleID = %q, want %q", got, "rule123")
	}
	if got := len(input.Actions); got != 1 {
		t.Fatalf("len(Actions) = %d, want 1", got)
	}
	if got := *input.Actions[0].Type; got != ActionType {
		t.Errorf("Actions[0].Type = %q, want %q", got, ActionType)
	}
}

func TestFlattenToModel(t *testing.T) {
	rule := &rules.Rule{
		RuleID:      "rule123",
		Type:        RuleType,
		Description: "Exclude XSS on admin paths",
		Enabled:     true,
		Scope:       rules.Scope{Type: "workspace", AppliesTo: []string{"ws123"}},
		Conditions: []rules.ConditionItem{{
			Type:   "single",
			Fields: rules.SingleCondition{Field: "path", Operator: "like", Value: "/admin*"},
		}},
		Actions: []rules.Action{{Type: ActionType, Signal: "XSS"}},
	}

	got := FlattenToModel(rule)

	if got.ID.ValueString() != "rule123" {
		t.Errorf("ID = %q, want %q", got.ID.ValueString(), "rule123")
	}
	if len(got.Action) != 1 || got.Action[0].Signal.ValueString() != "XSS" {
		t.Errorf("Action = %+v, want one action with signal XSS", got.Action)
	}
}
