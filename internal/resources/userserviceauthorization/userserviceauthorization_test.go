package userserviceauthorization

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"

	"github.com/fastly/go-fastly/v17/fastly"
)

func TestMetadata(t *testing.T) {
	r := NewResource()

	var resp resource.MetadataResponse
	r.Metadata(context.Background(), resource.MetadataRequest{
		ProviderTypeName: "fastly",
	}, &resp)

	require.Equal(t, "fastly_user_service_authorization", resp.TypeName)
}

func TestSchema(t *testing.T) {
	r := NewResource()

	var resp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
	require.Len(t, resp.Schema.Attributes, 4)

	serviceID, ok := resp.Schema.Attributes["service_id"].(resourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, serviceID.Required)
	require.NotEmpty(t, serviceID.PlanModifiers, "service_id must require replacement")

	userID, ok := resp.Schema.Attributes["user_id"].(resourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, userID.Required)
	require.NotEmpty(t, userID.PlanModifiers, "user_id must require replacement")

	permission, ok := resp.Schema.Attributes["permission"].(resourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, permission.Optional)
	require.True(t, permission.Computed)
	require.NotEmpty(t, permission.Validators)
	require.Empty(t, permission.PlanModifiers, "permission is updatable in place")

	require.NotNil(t, permission.Default)
	var defaultResp defaults.StringResponse
	permission.Default.DefaultString(context.Background(), defaults.StringRequest{}, &defaultResp)
	require.Equal(t, DefaultPermission, defaultResp.PlanValue.ValueString())
}

func TestPermissionValidator(t *testing.T) {
	permissionAttr, ok := ResourceAttributes()["permission"].(resourceschema.StringAttribute)
	require.True(t, ok)

	cases := []struct {
		value string
		valid bool
	}{
		{"full", true},
		{"read_only", true},
		{"purge_select", true},
		{"purge_all", true},
		{"not-a-real-permission", false},
		{"", false},
	}

	for _, c := range cases {
		req := validator.StringRequest{ConfigValue: types.StringValue(c.value)}
		resp := &validator.StringResponse{}
		for _, v := range permissionAttr.Validators {
			v.ValidateString(context.Background(), req, resp)
		}
		require.Equal(t, c.valid, !resp.Diagnostics.HasError(), "value %q", c.value)
	}
}

func TestBuildCreateInput(t *testing.T) {
	plan := Model{
		ServiceID:  types.StringValue("svc123"),
		UserID:     types.StringValue("user456"),
		Permission: types.StringValue("purge_all"),
	}

	input := buildCreateInput(plan)
	require.Equal(t, "svc123", input.Service.ID)
	require.Equal(t, "user456", input.User.ID)
	require.Equal(t, "purge_all", input.Permission)
}

func TestBuildUpdateInput(t *testing.T) {
	plan := Model{Permission: types.StringValue("read_only")}

	input := buildUpdateInput("sa-id", plan)
	require.Equal(t, "sa-id", input.ID)
	require.Equal(t, "read_only", input.Permission)
}

func TestFlattenToModel(t *testing.T) {
	sa := &fastly.ServiceAuthorization{
		ID:         "sa-id",
		Service:    &fastly.SAService{ID: "svc123"},
		User:       &fastly.SAUser{ID: "user456"},
		Permission: "full",
	}

	m := flattenToModel(sa)
	require.Equal(t, "sa-id", m.ID.ValueString())
	require.Equal(t, "svc123", m.ServiceID.ValueString())
	require.Equal(t, "user456", m.UserID.ValueString())
	require.Equal(t, "full", m.Permission.ValueString())
}
