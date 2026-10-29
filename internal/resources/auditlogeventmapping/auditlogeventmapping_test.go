package auditlogeventmapping

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fastly/go-fastly/v18/fastly"
	"github.com/fastly/go-fastly/v18/fastly/notifications/v1/eventmappings"
)

func mustSet(t *testing.T, values ...string) types.Set {
	t.Helper()
	s, diags := types.SetValueFrom(context.Background(), types.StringType, values)
	require.False(t, diags.HasError())
	return s
}

func TestBuildCreateInput(t *testing.T) {
	plan := Model{
		Name:           types.StringValue("my mapping"),
		Description:    types.StringValue("my description"),
		ScopeType:      types.StringValue(eventmappings.ScopeTypeVCL),
		ScopeIDs:       mustSet(t, "svc1"),
		EventTypes:     mustSet(t, "backend.delete"),
		IntegrationIDs: mustSet(t, "int1"),
	}

	input, diags := BuildCreateInput(context.Background(), plan)
	require.False(t, diags.HasError())

	assert.Equal(t, "my mapping", fastly.ToValue(input.Name))
	assert.Equal(t, "my description", fastly.ToValue(input.Description))
	assert.Equal(t, eventmappings.ScopeTypeVCL, fastly.ToValue(input.ScopeType))
	assert.Equal(t, []string{"svc1"}, input.ScopeIDs)
	assert.Equal(t, []string{"backend.delete"}, input.EventTypes)
	assert.Equal(t, []string{"int1"}, input.IntegrationIDs)
}

func TestBuildCreateInput_omitsUnsetOptionals(t *testing.T) {
	plan := Model{
		Name:           types.StringValue("my mapping"),
		Description:    types.StringNull(),
		ScopeType:      types.StringValue(eventmappings.ScopeTypeAccount),
		ScopeIDs:       types.SetNull(types.StringType),
		EventTypes:     mustSet(t, "address.create"),
		IntegrationIDs: mustSet(t, "int1"),
	}

	input, diags := BuildCreateInput(context.Background(), plan)
	require.False(t, diags.HasError())

	assert.Nil(t, input.Description)
	assert.Empty(t, input.ScopeIDs)
}

func TestBuildUpdateInput_clearsDescriptionWhenNull(t *testing.T) {
	plan := Model{
		Name:           types.StringValue("my mapping"),
		Description:    types.StringNull(),
		ScopeType:      types.StringValue(eventmappings.ScopeTypeAccount),
		ScopeIDs:       types.SetNull(types.StringType),
		EventTypes:     mustSet(t, "address.create"),
		IntegrationIDs: mustSet(t, "int1"),
	}

	input, diags := BuildUpdateInput(context.Background(), "abc", plan)
	require.False(t, diags.HasError())

	assert.Equal(t, "abc", fastly.ToValue(input.MappingID))
	require.NotNil(t, input.Description)
	assert.Empty(t, *input.Description)
}

func TestFlattenToModel(t *testing.T) {
	created := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	m := &eventmappings.EventMapping{
		ID:             "abc",
		Name:           "my mapping",
		Description:    "my description",
		ScopeType:      eventmappings.ScopeTypeVCL,
		ScopeIDs:       []string{"svc1"},
		EventTypes:     []string{"backend.delete"},
		IntegrationIDs: []string{"int1"},
		MappingStatus:  eventmappings.MappingStatusActive,
		CreatedAt:      created,
		UpdatedAt:      created,
	}

	got, diags := FlattenToModel(context.Background(), m, Model{})
	require.False(t, diags.HasError())

	assert.Equal(t, "abc", got.ID.ValueString())
	assert.Equal(t, "my description", got.Description.ValueString())
	assert.Equal(t, mustSet(t, "svc1"), got.ScopeIDs)
	assert.Equal(t, mustSet(t, "backend.delete"), got.EventTypes)
	assert.Equal(t, "active", got.MappingStatus.ValueString())
	assert.Equal(t, "2026-10-29T12:00:00Z", got.CreatedAt.ValueString())
}

func TestFlattenToModel_emptyOptionalsStayNull(t *testing.T) {
	m := &eventmappings.EventMapping{
		ID:             "abc",
		ScopeType:      eventmappings.ScopeTypeAccount,
		EventTypes:     []string{"address.create"},
		IntegrationIDs: []string{"int1"},
	}

	prior := Model{Description: types.StringNull(), ScopeIDs: types.SetNull(types.StringType)}
	got, diags := FlattenToModel(context.Background(), m, prior)
	require.False(t, diags.HasError())

	assert.True(t, got.Description.IsNull())
	assert.True(t, got.ScopeIDs.IsNull())
}

func TestFlattenToModel_emptyScopeIDsKeepsPriorEmptySet(t *testing.T) {
	m := &eventmappings.EventMapping{
		ID:             "abc",
		ScopeType:      eventmappings.ScopeTypeAccount,
		EventTypes:     []string{"address.create"},
		IntegrationIDs: []string{"int1"},
	}

	prior := Model{Description: types.StringNull(), ScopeIDs: mustSet(t, []string{}...)}
	got, diags := FlattenToModel(context.Background(), m, prior)
	require.False(t, diags.HasError())

	assert.False(t, got.ScopeIDs.IsNull())
	assert.Empty(t, got.ScopeIDs.Elements())
}
