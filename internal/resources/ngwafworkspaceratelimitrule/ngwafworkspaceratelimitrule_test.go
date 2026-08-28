package ngwafworkspaceratelimitrule

import (
	"context"
	"testing"

	"github.com/fastly/terraform-provider-fastly/internal/resources/ngwafrule"

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
				Operator: types.StringValue("equals"),
				Value:    types.StringValue("/login"),
			}},
		},
		Description: types.StringValue("Rate limit logins"),
		Action:      []ngwafrule.ActionModel{{Type: types.StringValue("log_request"), Signal: types.StringValue("sig123")}},
		RateLimit: []RateLimitModel{{
			ClientIdentifiers: []ClientIdentifierModel{{Type: types.StringValue("ip")}},
			Duration:          types.Int64Value(300),
			Interval:          types.Int64Value(60),
			Signal:            types.StringValue("sig123"),
			Threshold:         types.Int64Value(100),
		}},
	}
}

func TestBuildCreateInput(t *testing.T) {
	input := BuildCreateInput(testModel())

	if got := *input.Type; got != RuleType {
		t.Errorf("Type = %q, want %q", got, RuleType)
	}
	if input.RateLimit == nil {
		t.Fatal("RateLimit = nil, want a rate limit")
	}
	if got := *input.RateLimit.Threshold; got != 100 {
		t.Errorf("RateLimit.Threshold = %d, want 100", got)
	}
	if got := len(input.RateLimit.ClientIdentifiers); got != 1 {
		t.Fatalf("len(RateLimit.ClientIdentifiers) = %d, want 1", got)
	}
	if got := *input.RateLimit.ClientIdentifiers[0].Type; got != "ip" {
		t.Errorf("RateLimit.ClientIdentifiers[0].Type = %q, want %q", got, "ip")
	}
}

func TestBuildUpdateInput(t *testing.T) {
	input := BuildUpdateInput("rule123", testModel())

	if got := *input.RuleID; got != "rule123" {
		t.Errorf("RuleID = %q, want %q", got, "rule123")
	}
	if input.RateLimit == nil {
		t.Fatal("RateLimit = nil, want a rate limit")
	}
	if got := *input.RateLimit.Interval; got != 60 {
		t.Errorf("RateLimit.Interval = %d, want 60", got)
	}
}

func TestFlattenToModel(t *testing.T) {
	rule := &rules.Rule{
		RuleID:      "rule123",
		Type:        RuleType,
		Description: "Rate limit logins",
		Enabled:     true,
		Scope:       rules.Scope{Type: "workspace", AppliesTo: []string{"ws123"}},
		Actions:     []rules.Action{{Type: "log_request", Signal: "sig123"}},
		RateLimit: &rules.RateLimit{
			ClientIdentifiers: []rules.ClientIdentifier{{Type: "request_header", Name: "X-Forwarded-For", Key: "ip"}},
			Duration:          300,
			Interval:          60,
			Signal:            "sig123",
			Threshold:         100,
		},
	}

	got := FlattenToModel(rule)

	if len(got.RateLimit) != 1 {
		t.Fatalf("len(RateLimit) = %d, want 1", len(got.RateLimit))
	}
	if got.RateLimit[0].Threshold.ValueInt64() != 100 {
		t.Errorf("RateLimit[0].Threshold = %d, want 100", got.RateLimit[0].Threshold.ValueInt64())
	}
	if len(got.RateLimit[0].ClientIdentifiers) != 1 {
		t.Fatalf("len(ClientIdentifiers) = %d, want 1", len(got.RateLimit[0].ClientIdentifiers))
	}
	if got.RateLimit[0].ClientIdentifiers[0].Key.ValueString() != "ip" {
		t.Errorf("ClientIdentifiers[0].Key = %q, want %q", got.RateLimit[0].ClientIdentifiers[0].Key.ValueString(), "ip")
	}
}

func TestInvalidClientIdentifiers(t *testing.T) {
	tests := []struct {
		name        string
		identifiers []ClientIdentifierModel
		wantIssues  int
	}{
		{
			name:        "ip needs nothing",
			identifiers: []ClientIdentifierModel{{Type: types.StringValue("ip")}},
		},
		{
			name:        "ip must not set name",
			identifiers: []ClientIdentifierModel{{Type: types.StringValue("ip"), Name: types.StringValue("nope")}},
			wantIssues:  1,
		},
		{
			name:        "request_header requires name",
			identifiers: []ClientIdentifierModel{{Type: types.StringValue("request_header")}},
			wantIssues:  1,
		},
		{
			name:        "signal_payload must not set name and requires signal",
			identifiers: []ClientIdentifierModel{{Type: types.StringValue("signal_payload"), Name: types.StringValue("nope")}},
			wantIssues:  2,
		},
		{
			name: "ip may pair with a second identifier",
			identifiers: []ClientIdentifierModel{
				{Type: types.StringValue("ip")},
				{Type: types.StringValue("request_cookie"), Name: types.StringValue("session_id")},
			},
		},
		{
			name: "two non-ip identifiers is invalid",
			identifiers: []ClientIdentifierModel{
				{Type: types.StringValue("request_header"), Name: types.StringValue("X-Forwarded-For")},
				{Type: types.StringValue("request_cookie"), Name: types.StringValue("session_id")},
			},
			wantIssues: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InvalidClientIdentifiers(tt.identifiers); len(got) != tt.wantIssues {
				t.Errorf("InvalidClientIdentifiers() = %v, want %d issue(s)", got, tt.wantIssues)
			}
		})
	}
}
