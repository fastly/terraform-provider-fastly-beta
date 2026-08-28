package ngwafworkspacerequestrule

import (
	"context"
	"testing"

	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafrule"

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

	if _, diags := types.ObjectValueFrom(ctx, objType.AttributeTypes(), Model{}); diags.HasError() {
		t.Fatalf("Model does not match schema: %s", diags)
	}
}

func testModel() Model {
	return Model{
		CommonModel: ngwafrule.CommonModel{
			WorkspaceID:   types.StringValue("ws123"),
			Enabled:       types.BoolValue(true),
			GroupOperator: types.StringValue("all"),
			Condition: []ngwafrule.ConditionModel{{
				Field:    types.StringValue("ip"),
				Operator: types.StringValue("equals"),
				Value:    types.StringValue("127.0.0.1"),
			}},
		},
		Description:    types.StringValue("Block a specific IP"),
		RequestLogging: types.StringValue("sampled"),
		Action: []ngwafrule.ActionModel{{
			Type:         types.StringValue("block"),
			RedirectURL:  types.StringValue("https://example.com/blocked"),
			ResponseCode: types.Int64Value(302),
		}},
	}
}

func TestBuildCreateInput(t *testing.T) {
	input := BuildCreateInput(testModel())

	if got := *input.Type; got != RuleType {
		t.Errorf("Type = %q, want %q", got, RuleType)
	}
	if got := input.Scope.Type; got != scope.ScopeTypeWorkspace {
		t.Errorf("Scope.Type = %q, want %q", got, scope.ScopeTypeWorkspace)
	}
	if got := input.Scope.AppliesTo; len(got) != 1 || got[0] != "ws123" {
		t.Errorf("Scope.AppliesTo = %v, want [ws123]", got)
	}
	if got := *input.RequestLogging; got != "sampled" {
		t.Errorf("RequestLogging = %q, want %q", got, "sampled")
	}
	if got := len(input.Conditions); got != 1 {
		t.Fatalf("len(Conditions) = %d, want 1", got)
	}
	if got := *input.Conditions[0].Value; got != "127.0.0.1" {
		t.Errorf("Conditions[0].Value = %q, want %q", got, "127.0.0.1")
	}
	if got := len(input.Actions); got != 1 {
		t.Fatalf("len(Actions) = %d, want 1", got)
	}
	if got := *input.Actions[0].ResponseCode; got != 302 {
		t.Errorf("Actions[0].ResponseCode = %d, want 302", got)
	}
}

func TestBuildUpdateInput(t *testing.T) {
	input := BuildUpdateInput("rule123", testModel())

	if got := *input.RuleID; got != "rule123" {
		t.Errorf("RuleID = %q, want %q", got, "rule123")
	}
	if got := *input.Type; got != RuleType {
		t.Errorf("Type = %q, want %q", got, RuleType)
	}
	if got := len(input.Actions); got != 1 {
		t.Fatalf("len(Actions) = %d, want 1", got)
	}
	if got := *input.Actions[0].Type; got != "block" {
		t.Errorf("Actions[0].Type = %q, want %q", got, "block")
	}
}

func TestFlattenToModel(t *testing.T) {
	rule := &rules.Rule{
		RuleID:         "rule123",
		Type:           RuleType,
		Description:    "Block a specific IP",
		Enabled:        true,
		GroupOperator:  "all",
		RequestLogging: "sampled",
		Scope:          rules.Scope{Type: "workspace", AppliesTo: []string{"ws123"}},
		Conditions: []rules.ConditionItem{{
			Type:   "single",
			Fields: rules.SingleCondition{Field: "ip", Operator: "equals", Value: "127.0.0.1"},
		}},
		Actions: []rules.Action{{Type: "block", ResponseCode: 302, RedirectURL: "https://example.com/blocked"}},
	}

	got := FlattenToModel(rule)

	if got.ID.ValueString() != "rule123" {
		t.Errorf("ID = %q, want %q", got.ID.ValueString(), "rule123")
	}
	if got.WorkspaceID.ValueString() != "ws123" {
		t.Errorf("WorkspaceID = %q, want %q", got.WorkspaceID.ValueString(), "ws123")
	}
	if got.RequestLogging.ValueString() != "sampled" {
		t.Errorf("RequestLogging = %q, want %q", got.RequestLogging.ValueString(), "sampled")
	}
	if len(got.Condition) != 1 || got.Condition[0].Value.ValueString() != "127.0.0.1" {
		t.Errorf("Condition = %+v, want one condition matching 127.0.0.1", got.Condition)
	}
	if len(got.Action) != 1 || got.Action[0].ResponseCode.ValueInt64() != 302 {
		t.Errorf("Action = %+v, want one block action with response_code 302", got.Action)
	}
}
