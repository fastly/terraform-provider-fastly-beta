package ngwafworkspace

import (
	"context"

	ws "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// FlattenToModel populates state from the API response. The API returns a
// zero value for an attack signal threshold that was never explicitly set,
// so zero values are overridden with the schema defaults here to avoid
// perpetual drift and to match the legacy provider's behavior on import.
func FlattenToModel(ctx context.Context, workspace *ws.Workspace) (Model, diag.Diagnostics) {
	var diags diag.Diagnostics

	m := Model{
		ID:                          types.StringValue(workspace.WorkspaceID),
		Name:                        types.StringValue(workspace.Name),
		Description:                 types.StringValue(workspace.Description),
		Mode:                        types.StringValue(workspace.Mode),
		DefaultBlockingResponseCode: types.Int64Value(int64(workspace.DefaultBlockingResponseCode)),
		DefaultRedirectURL:          types.StringValue(workspace.DefaultRedirectURL),
	}

	if workspace.IPAnonymization == "" {
		m.IPAnonymization = types.StringNull()
	} else {
		m.IPAnonymization = types.StringValue(workspace.IPAnonymization)
	}

	headers, d := types.ListValueFrom(ctx, types.StringType, workspace.ClientIPHeaders)
	diags.Append(d...)
	if len(workspace.ClientIPHeaders) == 0 {
		m.ClientIPHeaders = types.ListNull(types.StringType)
	} else {
		m.ClientIPHeaders = headers
	}

	m.AttackSignalThresholds = []AttackSignalThresholdsModel{flattenAttackSignalThresholds(workspace.AttackSignalThresholds)}

	return m, diags
}

func flattenAttackSignalThresholds(t ws.AttackSignalThresholds) AttackSignalThresholdsModel {
	oneMinute := t.OneMinute
	if oneMinute == 0 {
		oneMinute = DefaultAttackSignalOneMinute
	}

	tenMinutes := t.TenMinutes
	if tenMinutes == 0 {
		tenMinutes = DefaultAttackSignalTenMinutes
	}

	oneHour := t.OneHour
	if oneHour == 0 {
		oneHour = DefaultAttackSignalOneHour
	}

	return AttackSignalThresholdsModel{
		Immediate:  types.BoolValue(t.Immediate),
		OneHour:    types.Int64Value(int64(oneHour)),
		OneMinute:  types.Int64Value(int64(oneMinute)),
		TenMinutes: types.Int64Value(int64(tenMinutes)),
	}
}
