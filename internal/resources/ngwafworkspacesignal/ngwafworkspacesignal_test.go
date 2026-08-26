package ngwafworkspacesignal

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/signals"
)

func TestSchema(t *testing.T) {
	attrs := ResourceAttributes()

	require.Len(t, attrs, 5)

	workspaceID, ok := attrs["workspace_id"].(schema.StringAttribute)
	require.True(t, ok)
	assert.True(t, workspaceID.Required)

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

func TestFlattenToModel(t *testing.T) {
	signal := &signals.Signal{
		SignalID:    "signal-id",
		ReferenceID: "site.example-signal",
		Name:        "Example Signal",
		Description: "description",
		Scope: signals.Scope{
			Type:      "workspace",
			AppliesTo: []string{"workspace-id"},
		},
	}

	model, err := FlattenToModel(signal)
	require.NoError(t, err)

	assert.Equal(t, types.StringValue("signal-id"), model.ID)
	assert.Equal(t, types.StringValue("workspace-id"), model.WorkspaceID)
	assert.Equal(t, types.StringValue("Example Signal"), model.Name)
	assert.Equal(t, types.StringValue("description"), model.Description)
	assert.Equal(t, types.StringValue("site.example-signal"), model.ReferenceID)
}

func TestFlattenToModelInvalid(t *testing.T) {
	_, err := FlattenToModel(nil)
	require.Error(t, err)

	_, err = FlattenToModel(&signals.Signal{})
	require.Error(t, err)

	_, err = FlattenToModel(&signals.Signal{
		SignalID: "signal-id",
		Scope: signals.Scope{
			Type:      "account",
			AppliesTo: []string{"*"},
		},
	})
	require.Error(t, err)

	_, err = FlattenToModel(&signals.Signal{
		Scope: signals.Scope{
			Type:      "workspace",
			AppliesTo: []string{"workspace-id"},
		},
	})
	require.Error(t, err)
}

func TestBuildCreateInput(t *testing.T) {
	plan := Model{
		WorkspaceID: types.StringValue("workspace-id"),
		Name:        types.StringValue("Example Signal"),
		Description: types.StringValue("description"),
	}

	input := BuildCreateInput(plan)

	require.NotNil(t, input.Name)
	require.NotNil(t, input.Description)
	require.NotNil(t, input.Scope)
	assert.Equal(t, "Example Signal", *input.Name)
	assert.Equal(t, "description", *input.Description)
	assert.Equal(t, "workspace", string(input.Scope.Type))
	assert.Equal(t, []string{"workspace-id"}, input.Scope.AppliesTo)
}

func TestBuildUpdateInput(t *testing.T) {
	plan := Model{
		WorkspaceID: types.StringValue("workspace-id"),
		Description: types.StringValue("updated"),
	}

	input := BuildUpdateInput("signal-id", plan)

	require.NotNil(t, input.SignalID)
	require.NotNil(t, input.Description)
	require.NotNil(t, input.Scope)
	assert.Equal(t, "signal-id", *input.SignalID)
	assert.Equal(t, "updated", *input.Description)
	assert.Equal(t, "workspace", string(input.Scope.Type))
	assert.Equal(t, []string{"workspace-id"}, input.Scope.AppliesTo)
}

func TestParseImportID(t *testing.T) {
	workspaceID, signalID, err := ParseImportID("workspace-id/signal-id")
	require.NoError(t, err)
	assert.Equal(t, "workspace-id", workspaceID)
	assert.Equal(t, "signal-id", signalID)

	_, _, err = ParseImportID("workspace-id")
	require.Error(t, err)

	_, _, err = ParseImportID("/signal-id")
	require.Error(t, err)

	_, _, err = ParseImportID("workspace-id/")
	require.Error(t, err)
}
