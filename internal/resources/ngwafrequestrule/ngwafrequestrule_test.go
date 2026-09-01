package ngwafrequestrule

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
			Enabled:       types.BoolValue(true),
			GroupOperator: types.StringValue("all"),
			Condition: []ngwafrule.ConditionModel{{
				Field:    types.StringValue("ip"),
				Operator: types.StringValue("equals"),
				Value:    types.StringValue("127.0.0.1"),
			}},
		},
		Description:    types.StringValue("Block a specific IP account-wide"),
		RequestLogging: types.StringValue("sampled"),
		Action: []ngwafrule.AccountActionModel{{
			Type:   types.StringValue("add_signal"),
			Signal: types.StringValue("SUSPECTED-BOT"),
		}},
	}
}

func TestBuildCreateInput(t *testing.T) {
	input, diags := BuildCreateInput(context.Background(), testModel("ws123", "ws456"))
	if diags.HasError() {
		t.Fatalf("BuildCreateInput: %s", diags)
	}

	if got := *input.Type; got != RuleType {
		t.Errorf("Type = %q, want %q", got, RuleType)
	}
	if got := input.Scope.Type; got != scope.ScopeTypeAccount {
		t.Errorf("Scope.Type = %q, want %q", got, scope.ScopeTypeAccount)
	}
	if got := input.Scope.AppliesTo; len(got) != 2 {
		t.Errorf("Scope.AppliesTo = %v, want two workspace IDs", got)
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
	if got := *input.Actions[0].Signal; got != "SUSPECTED-BOT" {
		t.Errorf("Actions[0].Signal = %q, want %q", got, "SUSPECTED-BOT")
	}
	// The account endpoint rejects custom responses ("corp rules cannot use
	// custom responses"), so the schema has no such fields to expand.
	if input.Actions[0].RedirectURL != nil || input.Actions[0].ResponseCode != nil {
		t.Errorf("custom response fields sent at account scope: redirect_url=%v response_code=%v", input.Actions[0].RedirectURL, input.Actions[0].ResponseCode)
	}
}

// TestBuildCreateInputWildcard pins the wildcard form, which is how a rule is
// applied to every workspace in the account.
func TestBuildCreateInputWildcard(t *testing.T) {
	input, diags := BuildCreateInput(context.Background(), testModel("*"))
	if diags.HasError() {
		t.Fatalf("BuildCreateInput: %s", diags)
	}

	if got := input.Scope.AppliesTo; len(got) != 1 || got[0] != "*" {
		t.Errorf("Scope.AppliesTo = %v, want [*]", got)
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
	if got := *input.Type; got != RuleType {
		t.Errorf("Type = %q, want %q", got, RuleType)
	}
	if got := input.Scope.Type; got != scope.ScopeTypeAccount {
		t.Errorf("Scope.Type = %q, want %q", got, scope.ScopeTypeAccount)
	}
	if got := len(input.Actions); got != 1 {
		t.Fatalf("len(Actions) = %d, want 1", got)
	}
	if got := *input.Actions[0].Type; got != "add_signal" {
		t.Errorf("Actions[0].Type = %q, want %q", got, "add_signal")
	}
}

func TestFlattenToModel(t *testing.T) {
	rule := &rules.Rule{
		RuleID:         "rule123",
		Type:           RuleType,
		Description:    "Block a specific IP account-wide",
		Enabled:        true,
		GroupOperator:  "all",
		RequestLogging: "sampled",
		Scope:          rules.Scope{Type: "account", AppliesTo: []string{"ws123", "ws456"}},
		Conditions: []rules.ConditionItem{{
			Type:   "single",
			Fields: rules.SingleCondition{Field: "ip", Operator: "equals", Value: "127.0.0.1"},
		}},
		Actions: []rules.Action{{Type: "add_signal", Signal: "SUSPECTED-BOT"}},
	}

	got, diags := FlattenToModel(context.Background(), rule)
	if diags.HasError() {
		t.Fatalf("FlattenToModel: %s", diags)
	}

	if got.ID.ValueString() != "rule123" {
		t.Errorf("ID = %q, want %q", got.ID.ValueString(), "rule123")
	}
	if n := len(got.AppliesTo.Elements()); n != 2 {
		t.Errorf("len(AppliesTo) = %d, want 2", n)
	}
	if got.RequestLogging.ValueString() != "sampled" {
		t.Errorf("RequestLogging = %q, want %q", got.RequestLogging.ValueString(), "sampled")
	}
	if len(got.Condition) != 1 || got.Condition[0].Value.ValueString() != "127.0.0.1" {
		t.Errorf("Condition = %+v, want one condition matching 127.0.0.1", got.Condition)
	}
	if len(got.Action) != 1 || got.Action[0].Signal.ValueString() != "SUSPECTED-BOT" {
		t.Errorf("Action = %+v, want one add_signal action for SUSPECTED-BOT", got.Action)
	}
}
