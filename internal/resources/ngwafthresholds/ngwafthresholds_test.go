package ngwafthresholds

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	th "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/thresholds"
)

// TestSchemaDefaults pins the schema-level defaults applied when duration,
// interval, or limit are omitted from configuration. These must match the
// API's own documented defaults (ThresholdDuration/ThresholdInterval/
// ThresholdLimit in the OpenAPI spec) - not an arbitrary choice, so a
// future edit to the constants should have to update this test
// deliberately.
func TestSchemaDefaults(t *testing.T) {
	attrs := ResourceAttributes()

	cases := []struct {
		name string
		want int64
	}{
		{"duration", DefaultDuration},
		{"interval", DefaultInterval},
		{"limit", DefaultLimit},
	}

	for _, c := range cases {
		attr, ok := attrs[c.name].(schema.Int64Attribute)
		assert.True(t, ok, c.name)

		var resp defaults.Int64Response
		attr.Default.DefaultInt64(context.Background(), defaults.Int64Request{}, &resp)
		assert.Equal(t, types.Int64Value(c.want), resp.PlanValue, c.name)
	}
}

func TestMetadata(t *testing.T) {
	r := NewResource()

	var resp resource.MetadataResponse
	r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "fastly"}, &resp)

	assert.Equal(t, "fastly_ngwaf_thresholds", resp.TypeName)
}

func TestBuildCreateInput(t *testing.T) {
	plan := Model{
		WorkspaceID: types.StringValue("workspace-id"),
		Action:      types.StringValue("block"),
		DontNotify:  types.BoolValue(false),
		Duration:    types.Int64Value(86400),
		Enabled:     types.BoolValue(true),
		Interval:    types.Int64Value(3600),
		Limit:       types.Int64Value(10),
		Name:        types.StringValue("example-threshold"),
		Signal:      types.StringValue("SQLI"),
	}

	input := BuildCreateInput(plan)
	assert.Equal(t, "workspace-id", *input.WorkspaceID)
	assert.Equal(t, "block", *input.Action)
	assert.False(t, *input.DontNotify)
	assert.Equal(t, 86400, *input.Duration)
	assert.True(t, *input.Enabled)
	assert.Equal(t, 3600, *input.Interval)
	assert.Equal(t, 10, *input.Limit)
	assert.Equal(t, "example-threshold", *input.Name)
	assert.Equal(t, "SQLI", *input.Signal)
}

func TestBuildUpdateInput(t *testing.T) {
	plan := Model{
		Action:     types.StringValue("log"),
		DontNotify: types.BoolValue(true),
		Duration:   types.Int64Value(43200),
		Enabled:    types.BoolValue(false),
		Interval:   types.Int64Value(600),
		Limit:      types.Int64Value(50),
		Name:       types.StringValue("example-threshold"),
		Signal:     types.StringValue("BHH"),
	}

	input := BuildUpdateInput("workspace-id", "threshold-id", plan)
	assert.Equal(t, "workspace-id", *input.WorkspaceID)
	assert.Equal(t, "threshold-id", *input.ThresholdID)
	assert.Equal(t, "log", *input.Action)
	assert.True(t, *input.DontNotify)
	assert.Equal(t, 43200, *input.Duration)
	assert.False(t, *input.Enabled)
	assert.Equal(t, 600, *input.Interval)
	assert.Equal(t, 50, *input.Limit)
	assert.Equal(t, "example-threshold", *input.Name)
	assert.Equal(t, "BHH", *input.Signal)
}

func TestFlattenToModel(t *testing.T) {
	threshold := &th.Threshold{
		ThresholdID: "threshold-id",
		Action:      "block_immediately",
		DontNotify:  true,
		Duration:    43200,
		Enabled:     false,
		Interval:    600,
		Limit:       50,
		Name:        "example-threshold",
		Signal:      "XXE",
	}

	m := FlattenToModel("workspace-id", threshold)
	assert.Equal(t, types.StringValue("threshold-id"), m.ID)
	assert.Equal(t, types.StringValue("workspace-id"), m.WorkspaceID)
	assert.Equal(t, types.StringValue("block_immediately"), m.Action)
	assert.Equal(t, types.BoolValue(true), m.DontNotify)
	assert.Equal(t, types.Int64Value(43200), m.Duration)
	assert.Equal(t, types.BoolValue(false), m.Enabled)
	assert.Equal(t, types.Int64Value(600), m.Interval)
	assert.Equal(t, types.Int64Value(50), m.Limit)
	assert.Equal(t, types.StringValue("example-threshold"), m.Name)
	assert.Equal(t, types.StringValue("XXE"), m.Signal)
}

// TestFlattenToModel_zeroValuesDefaulted pins the fallback FlattenToModel
// applies when the API returns a literal 0 for duration/interval/limit -
// the case for a threshold created out-of-band that left these unset (the
// spec explicitly allows this for a block_immediately threshold, whose
// limit's own minimum is lowered to 0). Without this, importing such a
// threshold would store a value its own schema validators reject on the
// next plan.
func TestFlattenToModel_zeroValuesDefaulted(t *testing.T) {
	threshold := &th.Threshold{
		ThresholdID: "threshold-id",
		Action:      "block_immediately",
		Enabled:     true,
		Signal:      "XXE",
	}

	m := FlattenToModel("workspace-id", threshold)
	assert.Equal(t, types.Int64Value(DefaultDuration), m.Duration)
	assert.Equal(t, types.Int64Value(DefaultInterval), m.Interval)
	assert.Equal(t, types.Int64Value(DefaultLimit), m.Limit)
}

func TestImportState(t *testing.T) {
	r := &Resource{}

	req := resource.ImportStateRequest{ID: "workspace-id/threshold-id"}
	resp := &resource.ImportStateResponse{State: importStateForTest(t)}

	r.ImportState(context.Background(), req, resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)

	var got Model
	diags := resp.State.Get(context.Background(), &got)
	require.False(t, diags.HasError(), diags)
	assert.Equal(t, "workspace-id", got.WorkspaceID.ValueString())
	assert.Equal(t, "threshold-id", got.ID.ValueString())
}

func TestImportState_invalidID(t *testing.T) {
	r := &Resource{}

	req := resource.ImportStateRequest{ID: "not-a-valid-id"}
	resp := &resource.ImportStateResponse{State: importStateForTest(t)}

	r.ImportState(context.Background(), req, resp)
	assert.True(t, resp.Diagnostics.HasError())
}

func importStateForTest(t *testing.T) tfsdk.State {
	t.Helper()

	var res Resource
	var schemaResp resource.SchemaResponse
	res.Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError(), schemaResp.Diagnostics)

	tfType := schemaResp.Schema.Type().TerraformType(context.Background())
	return tfsdk.State{
		Raw:    tftypes.NewValue(tfType, nil),
		Schema: schemaResp.Schema,
	}
}
