package ngwafalertintegration

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/require"
)

func TestImportState(t *testing.T) {
	req := resource.ImportStateRequest{ID: "workspace-id/alert-id"}
	resp := &resource.ImportStateResponse{State: importStateForTest(t)}

	ImportState(context.Background(), req, resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)

	var workspaceID types.String
	diags := resp.State.GetAttribute(context.Background(), path.Root("workspace_id"), &workspaceID)
	require.False(t, diags.HasError(), diags)
	require.Equal(t, "workspace-id", workspaceID.ValueString())

	var alertID types.String
	diags = resp.State.GetAttribute(context.Background(), path.Root("id"), &alertID)
	require.False(t, diags.HasError(), diags)
	require.Equal(t, "alert-id", alertID.ValueString())
}

func TestImportStateInvalidID(t *testing.T) {
	cases := []string{
		"not-valid",
		"workspace-id/",
		"/alert-id",
		"workspace-id/alert-id/extra",
	}

	for _, id := range cases {
		req := resource.ImportStateRequest{ID: id}
		resp := &resource.ImportStateResponse{State: importStateForTest(t)}

		ImportState(context.Background(), req, resp)
		require.True(t, resp.Diagnostics.HasError(), id)
	}
}

func TestAttributes(t *testing.T) {
	attrs := Attributes(Definition{
		Type: "webhook",
		ConfigAttrs: []ConfigAttribute{
			{Name: "webhook", Sensitive: true, Description: "Webhook URL."},
		},
	})
	require.Len(t, attrs, 4)

	workspaceID, ok := attrs["workspace_id"].(schema.StringAttribute)
	require.True(t, ok)
	require.True(t, workspaceID.Required)

	authentication, ok := attrs["authentication"].(schema.SingleNestedAttribute)
	require.True(t, ok)
	require.True(t, authentication.Required)

	webhook, ok := authentication.Attributes["webhook"].(schema.StringAttribute)
	require.True(t, ok)
	require.True(t, webhook.Required)
	require.True(t, webhook.Sensitive)
	require.NotEmpty(t, webhook.Validators)

	_, ok = attrs["webhook"]
	require.False(t, ok)
}

func TestAttributesOptionalDefault(t *testing.T) {
	attrs := Attributes(Definition{
		Type: "jira",
		ConfigAttrs: []ConfigAttribute{
			{Name: "issue_type", Description: "The Jira issue type associated with the ticket.", Optional: true, Default: "Task"},
		},
	})

	issueType, ok := attrs["issue_type"].(schema.StringAttribute)
	require.True(t, ok)
	require.True(t, issueType.Optional)
	require.True(t, issueType.Computed)
	require.False(t, issueType.Required)
	require.NotNil(t, issueType.Default)
}

func TestAttributesAddressValidator(t *testing.T) {
	attrs := Attributes(Definition{
		Type: "mailinglist",
		ConfigAttrs: []ConfigAttribute{
			{Name: "address", Description: "An email address.", Sensitive: true},
		},
	})

	authentication, ok := attrs["authentication"].(schema.SingleNestedAttribute)
	require.True(t, ok)

	address, ok := authentication.Attributes["address"].(schema.StringAttribute)
	require.True(t, ok)
	require.True(t, address.Required)
	require.True(t, address.Sensitive)
	require.NotEmpty(t, address.Validators)
}

func TestAttributesRequiredStringValidators(t *testing.T) {
	cases := []struct {
		name      string
		sensitive bool
		valid     string
		tooLong   string
	}{
		{name: "webhook", sensitive: true, valid: "https://example.com/webhooks/test"},
		{name: "key", sensitive: true, valid: "secret"},
		{name: "address", sensitive: true, valid: "alerts@example.com", tooLong: strings.Repeat("a", 321)},
		{name: "host", valid: "https://example.atlassian.net"},
		{name: "username", valid: "terraform"},
		{name: "project", valid: "TF"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			attrs := Attributes(Definition{
				Type: "test",
				ConfigAttrs: []ConfigAttribute{
					{Name: c.name, Sensitive: c.sensitive, Description: "test"},
				},
			})

			var attr schema.StringAttribute
			if c.sensitive {
				authentication, ok := attrs["authentication"].(schema.SingleNestedAttribute)
				require.True(t, ok)

				attr, ok = authentication.Attributes[c.name].(schema.StringAttribute)
				require.True(t, ok)
			} else {
				var ok bool
				attr, ok = attrs[c.name].(schema.StringAttribute)
				require.True(t, ok)
			}

			require.True(t, stringValidatorsHaveError(attr.Validators, ""), "expected empty %s to fail validation", c.name)
			require.False(t, stringValidatorsHaveError(attr.Validators, c.valid), "expected valid %s to pass validation", c.name)

			if c.tooLong != "" {
				require.True(t, stringValidatorsHaveError(attr.Validators, c.tooLong), "expected overlong %s to fail validation", c.name)
			}
		})
	}
}

func stringValidatorsHaveError(validators []validator.String, value string) bool {
	req := validator.StringRequest{ConfigValue: types.StringValue(value)}
	resp := &validator.StringResponse{}

	for _, v := range validators {
		v.ValidateString(context.Background(), req, resp)
	}

	return resp.Diagnostics.HasError()
}

func importStateForTest(t *testing.T) tfsdk.State {
	t.Helper()

	res := NewWorkspaceResource(Definition{
		Type:        "webhook",
		TypeSuffix:  "webhook",
		DisplayName: "Webhook",
		Description: "test",
		ConfigAttrs: []ConfigAttribute{
			{Name: "webhook", Sensitive: true, Description: "Webhook URL."},
		},
		Operations: nil,
	})
	var schemaResp resource.SchemaResponse
	res.Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError(), schemaResp.Diagnostics)

	tfType := schemaResp.Schema.Type().TerraformType(context.Background())
	return tfsdk.State{
		Raw:    tftypes.NewValue(tfType, nil),
		Schema: schemaResp.Schema,
	}
}
