package ngwafworkspace

import (
	"context"
	"testing"

	ws "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
)

func TestFlattenToModel_zeroThresholdsDefaulted(t *testing.T) {
	workspace := &ws.Workspace{
		WorkspaceID:                 "abc123",
		Name:                        "example",
		Description:                 "desc",
		Mode:                        "block",
		DefaultBlockingResponseCode: 406,
		AttackSignalThresholds:      ws.AttackSignalThresholds{},
	}

	m, diags := FlattenToModel(context.Background(), workspace)
	assert.False(t, diags.HasError())
	assert.Equal(t, types.StringValue("abc123"), m.ID)
	assert.Equal(t, types.StringNull(), m.IPAnonymization)
	assert.Equal(t, types.ListNull(types.StringType), m.ClientIPHeaders)

	assert.Len(t, m.AttackSignalThresholds, 1)
	th := m.AttackSignalThresholds[0]
	assert.Equal(t, types.Int64Value(DefaultAttackSignalOneMinute), th.OneMinute)
	assert.Equal(t, types.Int64Value(DefaultAttackSignalTenMinutes), th.TenMinutes)
	assert.Equal(t, types.Int64Value(DefaultAttackSignalOneHour), th.OneHour)
	assert.Equal(t, types.BoolValue(false), th.Immediate)
}

func TestFlattenToModel_explicitValuesPreserved(t *testing.T) {
	workspace := &ws.Workspace{
		WorkspaceID:                 "abc123",
		Name:                        "example",
		Description:                 "desc",
		Mode:                        "log",
		IPAnonymization:             "hashed",
		ClientIPHeaders:             []string{"X-Forwarded-For"},
		DefaultBlockingResponseCode: 429,
		DefaultRedirectURL:          "https://example.com",
		AttackSignalThresholds: ws.AttackSignalThresholds{
			OneMinute:  5,
			TenMinutes: 50,
			OneHour:    500,
			Immediate:  true,
		},
	}

	m, diags := FlattenToModel(context.Background(), workspace)
	assert.False(t, diags.HasError())
	assert.Equal(t, types.StringValue("hashed"), m.IPAnonymization)
	assert.Equal(t, types.Int64Value(429), m.DefaultBlockingResponseCode)
	assert.Equal(t, types.StringValue("https://example.com"), m.DefaultRedirectURL)

	var headers []string
	diags = m.ClientIPHeaders.ElementsAs(context.Background(), &headers, false)
	assert.False(t, diags.HasError())
	assert.Equal(t, []string{"X-Forwarded-For"}, headers)

	th := m.AttackSignalThresholds[0]
	assert.Equal(t, types.Int64Value(5), th.OneMinute)
	assert.Equal(t, types.Int64Value(50), th.TenMinutes)
	assert.Equal(t, types.Int64Value(500), th.OneHour)
	assert.Equal(t, types.BoolValue(true), th.Immediate)
}

func TestBuildCreateInput(t *testing.T) {
	plan := Model{
		Name:                        types.StringValue("example"),
		Description:                 types.StringValue("desc"),
		Mode:                        types.StringValue("block"),
		IPAnonymization:             types.StringValue("hashed"),
		ClientIPHeaders:             mustList(t, "X-Forwarded-For"),
		DefaultBlockingResponseCode: types.Int64Value(406),
		DefaultRedirectURL:          types.StringValue(""),
		AttackSignalThresholds: []AttackSignalThresholdsModel{{
			Immediate:  types.BoolValue(true),
			OneHour:    types.Int64Value(100),
			OneMinute:  types.Int64Value(1),
			TenMinutes: types.Int64Value(60),
		}},
	}

	input, diags := BuildCreateInput(context.Background(), plan)
	assert.False(t, diags.HasError())
	assert.Equal(t, "example", *input.Name)
	assert.Equal(t, "block", *input.Mode)
	assert.Equal(t, "hashed", *input.IPAnonymization)
	assert.Equal(t, []string{"X-Forwarded-For"}, input.ClientIPHeaders)
	assert.Equal(t, 406, *input.DefaultBlockingResponseCode)
	assert.NotNil(t, input.AttackSignalThresholds)
	assert.True(t, *input.AttackSignalThresholds.Immediate)
}

func TestBuildUpdateInput(t *testing.T) {
	plan := Model{
		Name:                        types.StringValue("example"),
		Description:                 types.StringValue("desc"),
		Mode:                        types.StringValue("log"),
		IPAnonymization:             types.StringNull(),
		DefaultBlockingResponseCode: types.Int64Value(406),
		DefaultRedirectURL:          types.StringValue(""),
		AttackSignalThresholds: []AttackSignalThresholdsModel{{
			Immediate:  types.BoolValue(false),
			OneHour:    types.Int64Value(100),
			OneMinute:  types.Int64Value(1),
			TenMinutes: types.Int64Value(60),
		}},
	}

	input, diags := BuildUpdateInput(context.Background(), "workspace-id", plan)
	assert.False(t, diags.HasError())
	assert.Equal(t, "workspace-id", *input.WorkspaceID)
	assert.Equal(t, "", *input.IPAnonymization)
}

func mustList(t *testing.T, values ...string) types.List {
	t.Helper()
	l, diags := types.ListValueFrom(context.Background(), types.StringType, values)
	assert.False(t, diags.HasError())
	return l
}

func TestBlockingResponseCodeValidator(t *testing.T) {
	v := blockingResponseCodeValidator{}

	cases := []struct {
		value int64
		valid bool
	}{
		{301, true},
		{302, true},
		{400, true},
		{599, true},
		{406, true},
		{300, false},
		{303, false},
		{399, false},
		{600, false},
	}

	for _, c := range cases {
		req := validator.Int64Request{ConfigValue: types.Int64Value(c.value)}
		resp := &validator.Int64Response{}
		v.ValidateInt64(context.Background(), req, resp)
		assert.Equal(t, c.valid, !resp.Diagnostics.HasError(), "value %d", c.value)
	}
}
