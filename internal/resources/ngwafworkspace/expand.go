package ngwafworkspace

import (
	"context"

	"github.com/fastly/terraform-provider-fastly/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/diag"

	ws "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces"
)

func clientIPHeaders(ctx context.Context, m Model, diags *diag.Diagnostics) []string {
	if m.ClientIPHeaders.IsNull() || m.ClientIPHeaders.IsUnknown() {
		return nil
	}

	var headers []string
	diags.Append(m.ClientIPHeaders.ElementsAs(ctx, &headers, false)...)
	return headers
}

func BuildCreateInput(ctx context.Context, plan Model) (*ws.CreateInput, diag.Diagnostics) {
	var diags diag.Diagnostics

	input := &ws.CreateInput{
		Name:                        new(service.StringValue(plan.Name)),
		Description:                 new(service.StringValue(plan.Description)),
		Mode:                        new(service.StringValue(plan.Mode)),
		IPAnonymization:             new(service.StringValue(plan.IPAnonymization)),
		DefaultBlockingResponseCode: new(int(service.Int64Value(plan.DefaultBlockingResponseCode))),
		DefaultRedirectURL:          new(service.StringValue(plan.DefaultRedirectURL)),
		ClientIPHeaders:             clientIPHeaders(ctx, plan, &diags),
	}

	if len(plan.AttackSignalThresholds) == 1 {
		t := plan.AttackSignalThresholds[0]
		input.AttackSignalThresholds = &ws.AttackSignalThresholdsCreateInput{
			OneMinute:  new(int(service.Int64Value(t.OneMinute))),
			TenMinutes: new(int(service.Int64Value(t.TenMinutes))),
			OneHour:    new(int(service.Int64Value(t.OneHour))),
			Immediate:  new(service.BoolValue(t.Immediate)),
		}
	}

	return input, diags
}

// BuildUpdateInput never sends client_ip_headers as an explicit clear: the
// underlying API has no representation - explicit JSON null and an explicit
// empty array were both tried and either ignored or unconfirmed against the
// live API - that reliably clears a previously configured list, so the
// field is only ever sent when the plan has a populated value. See
// client_ip_headers's schema description for the resulting limitation.
func BuildUpdateInput(ctx context.Context, workspaceID string, plan Model) (*ws.UpdateInput, diag.Diagnostics) {
	var diags diag.Diagnostics

	input := &ws.UpdateInput{
		WorkspaceID:                 &workspaceID,
		Name:                        new(service.StringValue(plan.Name)),
		Description:                 new(service.StringValue(plan.Description)),
		Mode:                        new(service.StringValue(plan.Mode)),
		IPAnonymization:             new(service.StringValue(plan.IPAnonymization)),
		DefaultBlockingResponseCode: new(int(service.Int64Value(plan.DefaultBlockingResponseCode))),
		DefaultRedirectURL:          new(service.StringValue(plan.DefaultRedirectURL)),
		ClientIPHeaders:             clientIPHeaders(ctx, plan, &diags),
	}

	if len(plan.AttackSignalThresholds) == 1 {
		t := plan.AttackSignalThresholds[0]
		input.AttackSignalThresholds = &ws.AttackSignalThresholdsUpdateInput{
			OneMinute:  new(int(service.Int64Value(t.OneMinute))),
			TenMinutes: new(int(service.Int64Value(t.TenMinutes))),
			OneHour:    new(int(service.Int64Value(t.OneHour))),
			Immediate:  new(service.BoolValue(t.Immediate)),
		}
	}

	return input, diags
}
