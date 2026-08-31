package ngwafworkspaceredaction

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/redactions"
)

func TestSchema(t *testing.T) {
	attrs := ResourceAttributes()

	require.Len(t, attrs, 4)

	workspaceID, ok := attrs["workspace_id"].(schema.StringAttribute)
	require.True(t, ok)
	assert.True(t, workspaceID.Required)
	assert.Len(t, workspaceID.PlanModifiers, 1)

	field, ok := attrs["field"].(schema.StringAttribute)
	require.True(t, ok)
	assert.True(t, field.Required)
	assert.Len(t, field.Validators, 1)

	fieldType, ok := attrs["type"].(schema.StringAttribute)
	require.True(t, ok)
	assert.True(t, fieldType.Required)
	assert.Len(t, fieldType.Validators, 1)

	id, ok := attrs["id"].(schema.StringAttribute)
	require.True(t, ok)
	assert.True(t, id.Computed)
}

func TestFlattenToModel(t *testing.T) {
	redaction := &redactions.Redaction{
		RedactionID: "redaction-id",
		Field:       "credit-card",
		Type:        "request_parameter",
	}

	model := FlattenToModel("workspace-id", redaction)

	assert.Equal(t, types.StringValue("redaction-id"), model.ID)
	assert.Equal(t, types.StringValue("workspace-id"), model.WorkspaceID)
	assert.Equal(t, types.StringValue("credit-card"), model.Field)
	assert.Equal(t, types.StringValue("request_parameter"), model.Type)
}

func TestBuildCreateInput(t *testing.T) {
	plan := Model{
		WorkspaceID: types.StringValue("workspace-id"),
		Field:       types.StringValue("credit-card"),
		Type:        types.StringValue("request_parameter"),
	}

	input := BuildCreateInput(plan)

	require.NotNil(t, input.Field)
	require.NotNil(t, input.Type)
	require.NotNil(t, input.WorkspaceID)
	assert.Equal(t, "credit-card", *input.Field)
	assert.Equal(t, "request_parameter", *input.Type)
	assert.Equal(t, "workspace-id", *input.WorkspaceID)
}

func TestBuildUpdateInput(t *testing.T) {
	plan := Model{
		WorkspaceID: types.StringValue("workspace-id"),
		Field:       types.StringValue("updated-field"),
		Type:        types.StringValue("request_header"),
	}

	input := BuildUpdateInput("redaction-id", plan)

	require.NotNil(t, input.Field)
	require.NotNil(t, input.RedactionID)
	require.NotNil(t, input.Type)
	require.NotNil(t, input.WorkspaceID)
	assert.Equal(t, "updated-field", *input.Field)
	assert.Equal(t, "redaction-id", *input.RedactionID)
	assert.Equal(t, "request_header", *input.Type)
	assert.Equal(t, "workspace-id", *input.WorkspaceID)
}

func TestBuildGetInput(t *testing.T) {
	input := BuildGetInput("workspace-id", "redaction-id")

	require.NotNil(t, input.WorkspaceID)
	require.NotNil(t, input.RedactionID)
	assert.Equal(t, "workspace-id", *input.WorkspaceID)
	assert.Equal(t, "redaction-id", *input.RedactionID)
}

func TestBuildDeleteInput(t *testing.T) {
	input := BuildDeleteInput("workspace-id", "redaction-id")

	require.NotNil(t, input.WorkspaceID)
	require.NotNil(t, input.RedactionID)
	assert.Equal(t, "workspace-id", *input.WorkspaceID)
	assert.Equal(t, "redaction-id", *input.RedactionID)
}

func TestParseImportID(t *testing.T) {
	workspaceID, redactionID, err := ParseImportID("workspace-id/redaction-id")
	require.NoError(t, err)
	assert.Equal(t, "workspace-id", workspaceID)
	assert.Equal(t, "redaction-id", redactionID)

	_, _, err = ParseImportID("workspace-id")
	require.EqualError(t, err, `invalid composite import ID format: expected workspace_id/redaction_id, got "workspace-id"`)

	_, _, err = ParseImportID("workspace-id/redaction-id/extra")
	require.EqualError(t, err, `invalid composite import ID format: expected workspace_id/redaction_id, got "workspace-id/redaction-id/extra"`)

	_, _, err = ParseImportID("")
	require.EqualError(t, err, `invalid composite import ID format: expected workspace_id/redaction_id, got ""`)

	_, _, err = ParseImportID("/redaction-id")
	require.EqualError(t, err, `workspace_id cannot be empty in import ID "/redaction-id"`)

	_, _, err = ParseImportID("workspace-id/")
	require.EqualError(t, err, `redaction_id cannot be empty in import ID "workspace-id/"`)
}
