package apisecurityoperationtag

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

	require.Equal(t, "fastly_api_security_operation_tag", resp.TypeName)
}

func TestSchema(t *testing.T) {
	r := NewResource()

	var resp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)

	serviceID, ok := resp.Schema.Attributes["service_id"].(resourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, serviceID.Required)
	require.NotEmpty(t, serviceID.PlanModifiers, "service_id should require replacement")

	name, ok := resp.Schema.Attributes["name"].(resourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, name.Required)
	require.Empty(t, name.PlanModifiers, "name is mutable in place and must not require replacement")

	description, ok := resp.Schema.Attributes["description"].(resourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, description.Optional)
	require.True(t, description.Computed)

	computedOnly := []string{"id", "tag_id", "operation_count", "created_at", "updated_at"}
	for _, attrName := range computedOnly {
		attr := resp.Schema.Attributes[attrName]
		require.NotNil(t, attr, attrName)
	}
}

func TestBuildCreateInput(t *testing.T) {
	plan := Model{
		Name:        types.StringValue("production"),
		Description: types.StringNull(),
	}

	in := buildCreateInput("service-1", plan)
	require.Equal(t, "service-1", *in.ServiceID)
	require.Equal(t, "production", *in.Name)
	require.Nil(t, in.Description)

	plan.Description = types.StringValue("desc")
	in = buildCreateInput("service-1", plan)
	require.Equal(t, "desc", *in.Description)
}

func TestBuildUpdateInput(t *testing.T) {
	plan := Model{
		Name:        types.StringValue("production"),
		Description: types.StringValue("old"),
	}

	in := buildUpdateInput("service-1", "tag-1", plan)
	require.Equal(t, "service-1", *in.ServiceID)
	require.Equal(t, "tag-1", *in.TagID)
	require.Equal(t, "production", *in.Name)
	require.NotNil(t, in.Description, "description must always be sent, even when unchanged - the API does not merge partial PATCH input")
	require.Equal(t, "old", *in.Description)

	// Renaming (name-only change) must still resend the current description,
	// or the API clears it.
	plan.Name = types.StringValue("renamed")
	in = buildUpdateInput("service-1", "tag-1", plan)
	require.Equal(t, "renamed", *in.Name)
	require.Equal(t, "old", *in.Description)

	plan.Description = types.StringNull()
	in = buildUpdateInput("service-1", "tag-1", plan)
	require.NotNil(t, in.Description)
	require.Equal(t, "", *in.Description, "clearing description must send an explicit empty string")
}

func TestFlatten(t *testing.T) {
	var model Model

	tag := &operations.OperationTag{
		ID:          "tag-1",
		Name:        "production",
		Description: "",
		Count:       0,
		CreatedAt:   "",
		UpdatedAt:   "",
	}

	flatten(&model, tag, "service-1")

	require.Equal(t, types.StringValue("service-1/tag-1"), model.ID)
	require.Equal(t, types.StringValue("service-1"), model.ServiceID)
	require.Equal(t, types.StringValue("tag-1"), model.TagID)
	require.Equal(t, types.StringValue("production"), model.Name)
	require.Equal(t, types.StringValue(""), model.Description, "an explicit empty description must round-trip as itself, not null")
	require.True(t, model.CreatedAt.IsNull())
	require.Equal(t, types.Int64Value(0), model.OperationCount, "a real zero count must not be nulled out")

	tag.Description = "desc"
	tag.Count = 3
	tag.CreatedAt = "2026-01-01T00:00:00Z"

	flatten(&model, tag, "service-1")
	require.Equal(t, types.StringValue("desc"), model.Description)
	require.Equal(t, types.Int64Value(3), model.OperationCount)
	require.False(t, model.CreatedAt.IsNull())
}

func TestImportStateInvalidID(t *testing.T) {
	r := &Resource{}

	for _, id := range []string{"", "no-slash", "/missing-service", "missing-tag/"} {
		var resp resource.ImportStateResponse
		r.ImportState(context.Background(), resource.ImportStateRequest{ID: id}, &resp)
		require.True(t, resp.Diagnostics.HasError(), id)
	}
}
