package objectstorageaccesskey

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"

	"github.com/fastly/go-fastly/v17/fastly/objectstorage/accesskeys"
)

func TestMetadata(t *testing.T) {
	r := NewResource()

	var resp resource.MetadataResponse
	r.Metadata(context.Background(), resource.MetadataRequest{
		ProviderTypeName: "fastly",
	}, &resp)

	require.Equal(t, "fastly_object_storage_access_keys", resp.TypeName)
}

func TestSchema(t *testing.T) {
	r := NewResource()

	var resp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
	require.Len(t, resp.Schema.Attributes, 6)

	description, ok := resp.Schema.Attributes["description"].(resourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, description.Required)
	require.NotEmpty(t, description.PlanModifiers, "description has no API endpoint to update it, so it must require replacement")

	permission, ok := resp.Schema.Attributes["permission"].(resourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, permission.Required)
	require.NotEmpty(t, permission.Validators)
	require.NotEmpty(t, permission.PlanModifiers)

	buckets, ok := resp.Schema.Attributes["buckets"].(resourceschema.ListAttribute)
	require.True(t, ok)
	require.True(t, buckets.Optional)
	require.NotEmpty(t, buckets.PlanModifiers)

	authentication, ok := resp.Schema.Attributes["authentication"].(resourceschema.SingleNestedAttribute)
	require.True(t, ok)
	require.True(t, authentication.Computed)

	secretKey, ok := authentication.Attributes["secret_key"].(resourceschema.StringAttribute)
	require.True(t, ok)
	require.True(t, secretKey.Computed)
	require.True(t, secretKey.Sensitive)
}

func TestPermissionValidator(t *testing.T) {
	permissionAttr, ok := ResourceAttributes()["permission"].(resourceschema.StringAttribute)
	require.True(t, ok)

	cases := []struct {
		value string
		valid bool
	}{
		{accesskeys.ReadWriteAdmin, true},
		{accesskeys.ReadOnlyAdmin, true},
		{accesskeys.ReadWriteObject, true},
		{accesskeys.ReadOnlyObjects, true},
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

func TestModel_SecretKey_nullObject(t *testing.T) {
	m := Model{Authentication: types.ObjectNull(authenticationAttributeTypes)}
	require.True(t, m.SecretKey().IsNull())
}

func TestBuildCreateInput(t *testing.T) {
	plan := Model{
		Description: types.StringValue("a test key"),
		Permission:  types.StringValue(accesskeys.ReadWriteObject),
		Buckets:     types.ListValueMust(types.StringType, []attr.Value{types.StringValue("bucket1"), types.StringValue("bucket2")}),
	}

	input, diags := buildCreateInput(context.Background(), plan)
	require.False(t, diags.HasError(), diags)
	require.Equal(t, "a test key", *input.Description)
	require.Equal(t, accesskeys.ReadWriteObject, *input.Permission)
	require.Equal(t, []string{"bucket1", "bucket2"}, *input.Buckets)
}

func TestBuildCreateInput_noBuckets(t *testing.T) {
	plan := Model{
		Description: types.StringValue("a test key"),
		Permission:  types.StringValue(accesskeys.ReadOnlyObjects),
		Buckets:     types.ListNull(types.StringType),
	}

	input, diags := buildCreateInput(context.Background(), plan)
	require.False(t, diags.HasError(), diags)
	require.Nil(t, input.Buckets)
}

func TestFlattenToModel(t *testing.T) {
	key := &accesskeys.AccessKey{
		AccessKeyID: "AKID",
		SecretKey:   "shh",
		Description: "a test key",
		Permission:  accesskeys.ReadWriteObject,
		Buckets:     []string{"bucket1"},
	}

	m, diags := flattenToModel(context.Background(), key, types.StringNull())
	require.False(t, diags.HasError(), diags)
	require.Equal(t, "AKID", m.ID.ValueString())
	require.Equal(t, "AKID", m.AccessKeyID.ValueString())
	require.Equal(t, "shh", m.SecretKey().ValueString())
	require.Equal(t, "a test key", m.Description.ValueString())
	require.Equal(t, accesskeys.ReadWriteObject, m.Permission.ValueString())

	var buckets []string
	require.False(t, m.Buckets.ElementsAs(context.Background(), &buckets, false).HasError())
	require.Equal(t, []string{"bucket1"}, buckets)
}

// The API only returns secret_key on creation, so a later read (which never
// includes it) must not clobber the value already in state.
func TestFlattenToModel_preservesSecretKeyOnRead(t *testing.T) {
	key := &accesskeys.AccessKey{
		AccessKeyID: "AKID",
		Description: "a test key",
		Permission:  accesskeys.ReadWriteObject,
	}

	m, diags := flattenToModel(context.Background(), key, types.StringValue("shh"))
	require.False(t, diags.HasError(), diags)
	require.Equal(t, "shh", m.SecretKey().ValueString())
}

// No buckets returned: buckets must flatten to null, not an empty list, so it
// matches an Optional attribute left unset in config.
func TestFlattenToModel_noBuckets(t *testing.T) {
	key := &accesskeys.AccessKey{
		AccessKeyID: "AKID",
		Description: "a test key",
		Permission:  accesskeys.ReadWriteObject,
	}

	m, diags := flattenToModel(context.Background(), key, types.StringNull())
	require.False(t, diags.HasError(), diags)
	require.True(t, m.Buckets.IsNull())
}
