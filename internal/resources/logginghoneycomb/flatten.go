package logginghoneycomb

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"

	"github.com/fastly/terraform-provider-fastly-beta/internal/constants"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"
)

func FlattenToNestedModel(h *fastly.Honeycomb) NestedModel {
	m := NestedModel{}

	if h == nil {
		return m
	}

	m.Name = types.StringValue(fastly.ToValue(h.Name))
	m.Authentication = NewAuthenticationObject(
		service.StringPointerOrDefault(h.Token, ""),
	)
	m.Dataset = types.StringValue(fastly.ToValue(h.Dataset))
	m.ProcessingRegion = service.StringPointerOrDefault(h.ProcessingRegion, DefaultProcessingRegion)
	m.Format = service.StringPointerOrDefault(h.Format, constants.LoggingHoneycombDefaultFormat)
	m.FormatVersion = service.Int64PointerOrDefault(h.FormatVersion, DefaultFormatVersion)
	m.Placement = service.StringPointerOrNull(h.Placement)
	m.ResponseCondition = service.StringPointerOrDefault(h.ResponseCondition, DefaultResponseCondition)

	return m
}

// ResetVCLOnlyToDefaults restores the VCL-only fields to their schema defaults
// after a flatten. On a Compute service they are never sent, so the API's own
// values are discarded rather than reported as a diff against the plan.
func ResetVCLOnlyToDefaults(m *NestedModel) {
	m.Format = types.StringValue(constants.LoggingHoneycombDefaultFormat)
	m.FormatVersion = types.Int64Value(DefaultFormatVersion)
	m.Placement = types.StringNull()
	m.ResponseCondition = types.StringValue(DefaultResponseCondition)
}

// FlattenToComputeNestedModel is FlattenToNestedModel for Compute services: it
// carries over only the attributes ComputeNestedModel exposes.
func FlattenToComputeNestedModel(h *fastly.Honeycomb) ComputeNestedModel {
	return ComputeNestedModel{commonModel: FlattenToNestedModel(h).commonModel}
}

func flatten(ctx context.Context, h *fastly.Honeycomb, m *Model) {
	if h == nil {
		tflog.Warn(ctx, "flatten called with nil Honeycomb logging endpoint")
		return
	}

	id := fastly.ToValue(h.ServiceID) + "-" + strconv.Itoa(fastly.ToValue(h.ServiceVersion)) + "-" + fastly.ToValue(h.Name)
	m.ID = types.StringValue(id)
	m.Service = types.StringValue(fastly.ToValue(h.ServiceID))
	m.Version = types.Int64Value(int64(fastly.ToValue(h.ServiceVersion)))

	m.NestedModel = FlattenToNestedModel(h)

	tflog.Debug(ctx, "Flattened Honeycomb logging endpoint state", map[string]any{
		"id":      id,
		"service": m.Service.ValueString(),
		"version": m.Version.ValueInt64(),
		"name":    m.Name.ValueString(),
	})
}
