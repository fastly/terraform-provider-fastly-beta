package logginggrafanacloudlogs

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"

	"github.com/fastly/terraform-provider-fastly-beta/internal/constants"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"
)

func FlattenToNestedModel(g *fastly.GrafanaCloudLogs) NestedModel {
	m := NestedModel{}

	if g == nil {
		return m
	}

	m.Name = types.StringValue(fastly.ToValue(g.Name))
	m.URL = types.StringValue(fastly.ToValue(g.URL))
	m.User = types.StringValue(fastly.ToValue(g.User))
	m.Index = types.StringValue(fastly.ToValue(g.Index))
	m.Authentication = NewAuthenticationObject(
		service.StringPointerOrDefault(g.Token, ""),
	)
	m.ProcessingRegion = service.StringPointerOrDefault(g.ProcessingRegion, DefaultProcessingRegion)
	m.Format = service.StringPointerOrDefault(g.Format, constants.LoggingGrafanaCloudLogsDefaultFormat)
	m.FormatVersion = service.Int64PointerOrDefault(g.FormatVersion, DefaultFormatVersion)
	m.Placement = service.StringPointerOrNull(g.Placement)
	m.ResponseCondition = service.StringPointerOrDefault(g.ResponseCondition, DefaultResponseCondition)

	return m
}

// ResetVCLOnlyToDefaults restores the VCL-only fields to their schema defaults
// after a flatten. On a Compute service they are never sent, so the API's own
// values are discarded rather than reported as a diff against the plan.
func ResetVCLOnlyToDefaults(m *NestedModel) {
	m.Format = types.StringValue(constants.LoggingGrafanaCloudLogsDefaultFormat)
	m.FormatVersion = types.Int64Value(DefaultFormatVersion)
	m.Placement = types.StringNull()
	m.ResponseCondition = types.StringValue(DefaultResponseCondition)
}

// FlattenToComputeNestedModel is FlattenToNestedModel for Compute services: it
// carries over only the attributes ComputeNestedModel exposes.
func FlattenToComputeNestedModel(g *fastly.GrafanaCloudLogs) ComputeNestedModel {
	return ComputeNestedModel{commonModel: FlattenToNestedModel(g).commonModel}
}

func flatten(ctx context.Context, g *fastly.GrafanaCloudLogs, m *Model) {
	if g == nil {
		tflog.Warn(ctx, "flatten called with nil GrafanaCloudLogs logging endpoint")
		return
	}

	id := fastly.ToValue(g.ServiceID) + "-" + strconv.Itoa(fastly.ToValue(g.ServiceVersion)) + "-" + fastly.ToValue(g.Name)
	m.ID = types.StringValue(id)
	m.Service = types.StringValue(fastly.ToValue(g.ServiceID))
	m.Version = types.Int64Value(int64(fastly.ToValue(g.ServiceVersion)))

	m.NestedModel = FlattenToNestedModel(g)

	tflog.Debug(ctx, "Flattened GrafanaCloudLogs logging endpoint state", map[string]any{
		"id":      id,
		"service": m.Service.ValueString(),
		"version": m.Version.ValueInt64(),
		"name":    m.Name.ValueString(),
	})
}
