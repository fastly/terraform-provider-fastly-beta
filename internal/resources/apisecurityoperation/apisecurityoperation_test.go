package apisecurityoperation

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"

	"github.com/fastly/go-fastly/v17/fastly/apisecurity/operations"
)

func TestMetadata(t *testing.T) {
	r := NewResource()

	var resp resource.MetadataResponse
	r.Metadata(context.Background(), resource.MetadataRequest{
		ProviderTypeName: "fastly",
	}, &resp)

	require.Equal(t, "fastly_api_security_operation", resp.TypeName)
}

func TestSchema(t *testing.T) {
	r := NewResource()

	var resp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)

	forceNew := []string{"service_id", "method", "domain", "path"}
	for _, name := range forceNew {
		attr, ok := resp.Schema.Attributes[name].(resourceschema.StringAttribute)
		require.True(t, ok, name)
		require.True(t, attr.Required, name)
		require.NotEmpty(t, attr.PlanModifiers, "%s should require replacement", name)
	}

	description, ok := resp.Schema.Attributes["description"].(resourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, description.Optional)
	require.True(t, description.Computed)

	tagIDs, ok := resp.Schema.Attributes["tag_ids"].(resourceschema.SetAttribute)
	require.True(t, ok)
	require.True(t, tagIDs.Optional)
	require.True(t, tagIDs.Computed)
	require.Equal(t, types.StringType, tagIDs.ElementType)

	computedOnly := []string{"operation_id", "status", "rps", "created_at", "updated_at", "last_seen_at"}
	for _, name := range computedOnly {
		attr := resp.Schema.Attributes[name]
		require.NotNil(t, attr, name)
	}
}

func TestExpandTagIDs(t *testing.T) {
	ctx := context.Background()

	require.Nil(t, expandTagIDs(ctx, types.SetNull(types.StringType)))
	require.Nil(t, expandTagIDs(ctx, types.SetUnknown(types.StringType)))

	set, diags := types.SetValueFrom(ctx, types.StringType, []string{"tag-1", "tag-2"})
	require.False(t, diags.HasError())
	require.ElementsMatch(t, []string{"tag-1", "tag-2"}, expandTagIDs(ctx, set))
}

func TestBuildCreateInput(t *testing.T) {
	ctx := context.Background()

	plan := Model{
		Method:      types.StringValue("GET"),
		Domain:      types.StringValue("api.example.com"),
		Path:        types.StringValue("/v1/things"),
		Description: types.StringNull(),
		TagIDs:      types.SetNull(types.StringType),
	}

	in := buildCreateInput(ctx, "service-1", plan)
	require.Equal(t, "service-1", *in.ServiceID)
	require.Equal(t, "GET", *in.Method, "method enum validation on the schema guarantees the plan value is already uppercase")
	require.Equal(t, "api.example.com", *in.Domain)
	require.Equal(t, "/v1/things", *in.Path)
	require.Nil(t, in.Description)
	require.Nil(t, in.TagIDs)

	plan.Description = types.StringValue("desc")
	set, diags := types.SetValueFrom(ctx, types.StringType, []string{"tag-1"})
	require.False(t, diags.HasError())
	plan.TagIDs = set

	in = buildCreateInput(ctx, "service-1", plan)
	require.Equal(t, "desc", *in.Description)
	require.Equal(t, []string{"tag-1"}, in.TagIDs)
}

func TestBuildUpdateInput(t *testing.T) {
	ctx := context.Background()

	plan := Model{
		Description: types.StringValue("old"),
		TagIDs:      types.SetNull(types.StringType),
	}

	in := buildUpdateInput(ctx, "service-1", "op-1", plan)
	require.Equal(t, "service-1", *in.ServiceID)
	require.Equal(t, "op-1", *in.OperationID)
	require.NotNil(t, in.Description, "description must always be sent, even when unchanged - the API does not merge partial PATCH input")
	require.Equal(t, "old", *in.Description)
	require.Nil(t, in.TagIDs)

	// Changing only description must still resend the current (unchanged) tag_ids,
	// or the API clears them.
	set, diags := types.SetValueFrom(ctx, types.StringType, []string{"tag-1"})
	require.False(t, diags.HasError())
	plan.TagIDs = set
	in = buildUpdateInput(ctx, "service-1", "op-1", plan)
	require.Equal(t, []string{"tag-1"}, in.TagIDs)
	require.Equal(t, "old", *in.Description)

	plan.Description = types.StringValue("new")
	in = buildUpdateInput(ctx, "service-1", "op-1", plan)
	require.Equal(t, "new", *in.Description)

	plan.Description = types.StringNull()
	in = buildUpdateInput(ctx, "service-1", "op-1", plan)
	require.NotNil(t, in.Description)
	require.Equal(t, "", *in.Description, "clearing description must send an explicit empty string")
}

func TestFlatten(t *testing.T) {
	var model Model

	op := &operations.Operation{
		ID:          "op-1",
		Method:      "GET",
		Domain:      "api.example.com",
		Path:        "/v1/things",
		Description: "",
		Status:      "",
		TagIDs:      nil,
		CreatedAt:   "",
		UpdatedAt:   "",
		LastSeenAt:  "",
		RPS:         0,
	}

	flatten(&model, op, "service-1")

	require.Equal(t, types.StringValue("service-1/op-1"), model.ID)
	require.Equal(t, types.StringValue("service-1"), model.ServiceID)
	require.Equal(t, types.StringValue("op-1"), model.OperationID)
	require.Equal(t, types.StringValue("GET"), model.Method)
	require.Equal(t, types.StringValue(""), model.Description, "an explicit empty description must round-trip as itself, not null")
	require.True(t, model.Status.IsNull())
	require.True(t, model.CreatedAt.IsNull())
	require.False(t, model.TagIDs.IsNull(), "no tags must flatten to an empty set, not null")
	require.Len(t, model.TagIDs.Elements(), 0)
	require.Equal(t, types.Float64Value(0), model.RPS)

	op.Description = "desc"
	op.Status = "active"
	op.TagIDs = []string{"tag-1", "tag-2"}
	op.RPS = 12.5

	flatten(&model, op, "service-1")
	require.Equal(t, types.StringValue("desc"), model.Description)
	require.Equal(t, types.StringValue("active"), model.Status)
	require.False(t, model.TagIDs.IsNull())
	require.Equal(t, types.Float64Value(12.5), model.RPS)
}

func TestImportStateInvalidID(t *testing.T) {
	r := &Resource{}

	for _, id := range []string{"", "no-slash", "/missing-service", "missing-op/"} {
		var resp resource.ImportStateResponse
		r.ImportState(context.Background(), resource.ImportStateRequest{ID: id}, &resp)
		require.True(t, resp.Diagnostics.HasError(), id)
	}
}
