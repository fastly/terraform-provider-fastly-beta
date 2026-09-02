package ngwafsignal

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/scope"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/signals"
)

func TestMetadata(t *testing.T) {
	r := NewResource()

	var resp resource.MetadataResponse
	r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "fastly"}, &resp)

	assert.Equal(t, "fastly_ngwaf_signal", resp.TypeName)
}

func TestSchemaMatchesModel(t *testing.T) {
	ctx := context.Background()

	var resp resource.SchemaResponse
	(&Resource{}).Schema(ctx, resource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)

	if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatalf("invalid schema: %s", diags)
	}

	objType, ok := resp.Schema.Type().(types.ObjectType)
	if !ok {
		t.Fatalf("schema type is %T, want types.ObjectType", resp.Schema.Type())
	}

	// applies_to needs a typed null: bare-ID import sets only id before Read.
	empty := Model{AppliesTo: types.SetNull(types.StringType)}
	if _, diags := types.ObjectValueFrom(ctx, objType.AttributeTypes(), empty); diags.HasError() {
		t.Fatalf("Model does not match schema: %s", diags)
	}
}

func TestSchema(t *testing.T) {
	attrs := resourceAttributes()
	require.Len(t, attrs, 5)

	appliesTo, ok := attrs["applies_to"].(schema.SetAttribute)
	require.True(t, ok)
	assert.True(t, appliesTo.Required)
	assert.Len(t, appliesTo.Validators, 3)

	name, ok := attrs["name"].(schema.StringAttribute)
	require.True(t, ok)
	assert.True(t, name.Required)
	assert.Len(t, name.Validators, 2)
	assert.Len(t, name.PlanModifiers, 1)

	description, ok := attrs["description"].(schema.StringAttribute)
	require.True(t, ok)
	assert.True(t, description.Optional)
	assert.True(t, description.Computed)
	assert.Len(t, description.Validators, 1)

	referenceID, ok := attrs["reference_id"].(schema.StringAttribute)
	require.True(t, ok)
	assert.True(t, referenceID.Computed)
}

func TestBuildCreateInput(t *testing.T) {
	appliesTo := types.SetValueMust(types.StringType, []attr.Value{
		types.StringValue("workspace-one"),
		types.StringValue("workspace-two"),
	})

	plan := Model{
		AppliesTo:   appliesTo,
		Name:        types.StringValue("Example Signal"),
		Description: types.StringValue("description"),
	}

	input, diags := BuildCreateInput(context.Background(), plan)
	require.False(t, diags.HasError(), diags)

	require.NotNil(t, input.Name)
	require.NotNil(t, input.Description)
	require.NotNil(t, input.Scope)
	assert.Equal(t, "Example Signal", *input.Name)
	assert.Equal(t, "description", *input.Description)
	assert.Equal(t, scope.ScopeTypeAccount, input.Scope.Type)
	assert.ElementsMatch(t, []string{"workspace-one", "workspace-two"}, input.Scope.AppliesTo)
}

func TestBuildUpdateInput(t *testing.T) {
	appliesTo := types.SetValueMust(types.StringType, []attr.Value{
		types.StringValue("*"),
	})

	plan := Model{
		AppliesTo:   appliesTo,
		Description: types.StringValue("updated"),
	}

	input, diags := BuildUpdateInput(context.Background(), "signal-id", plan)
	require.False(t, diags.HasError(), diags)

	require.NotNil(t, input.SignalID)
	require.NotNil(t, input.Description)
	require.NotNil(t, input.Scope)
	assert.Equal(t, "signal-id", *input.SignalID)
	assert.Equal(t, "updated", *input.Description)
	assert.Equal(t, scope.ScopeTypeAccount, input.Scope.Type)
	assert.Equal(t, []string{"*"}, input.Scope.AppliesTo)
}

func TestBuildGetAndDeleteInputsUseBareAccountScope(t *testing.T) {
	getInput := BuildGetInput("signal-id")
	require.NotNil(t, getInput.SignalID)
	require.NotNil(t, getInput.Scope)
	assert.Equal(t, "signal-id", *getInput.SignalID)
	assert.Equal(t, scope.ScopeTypeAccount, getInput.Scope.Type)
	assert.Empty(t, getInput.Scope.AppliesTo)

	deleteInput := BuildDeleteInput("signal-id")
	require.NotNil(t, deleteInput.SignalID)
	require.NotNil(t, deleteInput.Scope)
	assert.Equal(t, "signal-id", *deleteInput.SignalID)
	assert.Equal(t, scope.ScopeTypeAccount, deleteInput.Scope.Type)
	assert.Empty(t, deleteInput.Scope.AppliesTo)
}

func TestFlattenToModel(t *testing.T) {
	signal := &signals.Signal{
		SignalID:    "signal-id",
		ReferenceID: "site.example-signal",
		Name:        "Example Signal",
		Description: "description",
		Scope: signals.Scope{
			Type:      "account",
			AppliesTo: []string{"*"},
		},
	}

	model, err := FlattenToModel(context.Background(), signal)
	require.NoError(t, err)

	assert.Equal(t, types.StringValue("signal-id"), model.ID)
	assert.Equal(t, types.StringValue("Example Signal"), model.Name)
	assert.Equal(t, types.StringValue("description"), model.Description)
	assert.Equal(t, types.StringValue("site.example-signal"), model.ReferenceID)
	require.Len(t, model.AppliesTo.Elements(), 1)
	assert.Equal(t, types.StringValue("*"), model.AppliesTo.Elements()[0])
}

func TestFlattenToModelRejectsInvalidOrWorkspaceScope(t *testing.T) {
	tests := []struct {
		name   string
		signal *signals.Signal
	}{
		{name: "nil", signal: nil},
		{name: "empty", signal: &signals.Signal{}},
		{
			name: "workspace scoped",
			signal: &signals.Signal{
				SignalID: "signal-id",
				Scope: signals.Scope{
					Type:      "workspace",
					AppliesTo: []string{"workspace-id"},
				},
			},
		},
		{
			name: "missing applies_to",
			signal: &signals.Signal{
				SignalID: "signal-id",
				Scope: signals.Scope{
					Type: "account",
				},
			},
		},
		{
			name: "missing id",
			signal: &signals.Signal{
				Scope: signals.Scope{
					Type:      "account",
					AppliesTo: []string{"*"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := FlattenToModel(context.Background(), tt.signal)
			require.Error(t, err)
		})
	}
}

func TestImportState(t *testing.T) {
	r := &Resource{}
	req := resource.ImportStateRequest{ID: "signal-id"}
	resp := &resource.ImportStateResponse{State: importStateForTest(t)}

	r.ImportState(context.Background(), req, resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)

	var gotID types.String
	diags := resp.State.GetAttribute(context.Background(), path.Root("id"), &gotID)
	require.False(t, diags.HasError(), diags)
	assert.Equal(t, "signal-id", gotID.ValueString())
}

func importStateForTest(t *testing.T) tfsdk.State {
	t.Helper()

	var r Resource
	var schemaResp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError(), schemaResp.Diagnostics)

	tfType := schemaResp.Schema.Type().TerraformType(context.Background())
	return tfsdk.State{
		Raw:    tftypes.NewValue(tfType, nil),
		Schema: schemaResp.Schema,
	}
}
