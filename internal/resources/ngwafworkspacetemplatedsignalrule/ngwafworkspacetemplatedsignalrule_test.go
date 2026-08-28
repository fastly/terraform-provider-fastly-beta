package ngwafworkspacetemplatedsignalrule

import (
	"context"
	"testing"

	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafrule"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
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

// TestEverySettableFieldRequiresReplace enumerates the schema so that adding
// an attribute or block without a RequiresReplace modifier fails here. Update
// only reports an error, so a field that plans an in-place change would leave
// the rule unmodified.
func TestEverySettableFieldRequiresReplace(t *testing.T) {
	ctx := context.Background()

	var resp resource.SchemaResponse
	(&Resource{}).Schema(ctx, resource.SchemaRequest{}, &resp)

	for name, attr := range resp.Schema.Attributes {
		if attr.IsComputed() && !attr.IsOptional() && !attr.IsRequired() {
			continue
		}
		if !attributeRequiresReplace(ctx, attr) {
			t.Errorf("attribute %q does not force replacement", name)
		}
	}

	for name, block := range resp.Schema.Blocks {
		if !blockRequiresReplace(ctx, block) {
			t.Errorf("block %q does not force replacement", name)
		}
	}
}

// The modifiers below are compared by description against the framework's own
// RequiresReplace, so the check does not depend on a hardcoded string.
func attributeRequiresReplace(ctx context.Context, attr schema.Attribute) bool {
	switch a := attr.(type) {
	case schema.StringAttribute:
		want := stringplanmodifier.RequiresReplace().Description(ctx)
		for _, m := range a.PlanModifiers {
			if m.Description(ctx) == want {
				return true
			}
		}
	case schema.BoolAttribute:
		want := boolplanmodifier.RequiresReplace().Description(ctx)
		for _, m := range a.PlanModifiers {
			if m.Description(ctx) == want {
				return true
			}
		}
	}
	return false
}

func blockRequiresReplace(ctx context.Context, block schema.Block) bool {
	listBlock, ok := block.(schema.ListNestedBlock)
	if !ok {
		return false
	}
	want := listplanmodifier.RequiresReplace().Description(ctx)
	for _, m := range listBlock.PlanModifiers {
		if m.Description(ctx) == want {
			return true
		}
	}
	return false
}

// The API only accepts an empty description for this rule type, so the
// schema has no description attribute at all.
func TestSchemaHasNoDescription(t *testing.T) {
	var resp resource.SchemaResponse
	(&Resource{}).Schema(context.Background(), resource.SchemaRequest{}, &resp)

	if _, ok := resp.Schema.Attributes["description"]; ok {
		t.Error("schema declares a description attribute, want none")
	}
}

func testModel() Model {
	return Model{
		CommonModel: ngwafrule.CommonModel{
			WorkspaceID: types.StringValue("ws123"),
			Enabled:     types.BoolValue(true),
			Condition: []ngwafrule.ConditionModel{{
				Field:    types.StringValue("path"),
				Operator: types.StringValue("equals"),
				Value:    types.StringValue("/login"),
			}},
		},
		Action: []ngwafrule.SignalActionModel{{Signal: types.StringValue("LOGINATTEMPT")}},
	}
}

// Action has no type field, so BuildCreateInput must be the one supplying it.
// Description must stay unset.
func TestBuildCreateInput(t *testing.T) {
	input := BuildCreateInput(testModel())

	if got := *input.Type; got != RuleType {
		t.Errorf("Type = %q, want %q", got, RuleType)
	}
	if input.Description != nil {
		t.Errorf("Description = %q, want unset", *input.Description)
	}
	if got := len(input.Actions); got != 1 {
		t.Fatalf("len(Actions) = %d, want 1", got)
	}
	if got := *input.Actions[0].Type; got != ActionType {
		t.Errorf("Actions[0].Type = %q, want %q", got, ActionType)
	}
	if got := *input.Actions[0].Signal; got != "LOGINATTEMPT" {
		t.Errorf("Actions[0].Signal = %q, want %q", got, "LOGINATTEMPT")
	}
}

func TestFlattenToModel(t *testing.T) {
	rule := &rules.Rule{
		RuleID:  "rule123",
		Type:    RuleType,
		Enabled: true,
		Scope:   rules.Scope{Type: "workspace", AppliesTo: []string{"ws123"}},
		Actions: []rules.Action{{Type: ActionType, Signal: "LOGINATTEMPT"}},
	}

	got := FlattenToModel(rule)

	if got.ID.ValueString() != "rule123" {
		t.Errorf("ID = %q, want %q", got.ID.ValueString(), "rule123")
	}
	if got.WorkspaceID.ValueString() != "ws123" {
		t.Errorf("WorkspaceID = %q, want %q", got.WorkspaceID.ValueString(), "ws123")
	}
	if len(got.Action) != 1 || got.Action[0].Signal.ValueString() != "LOGINATTEMPT" {
		t.Errorf("Action = %+v, want one action with signal LOGINATTEMPT", got.Action)
	}
}
