package loggingscalyr

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"

	"github.com/fastly/terraform-provider-fastly-beta/internal/constants"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"
)

func FlattenToNestedModel(l *fastly.Scalyr) NestedModel {
	m := NestedModel{}

	if l == nil {
		return m
	}

	m.Name = types.StringValue(fastly.ToValue(l.Name))
	m.Authentication = NewAuthenticationObject(
		service.StringPointerOrDefault(l.Token, ""),
	)
	m.ProjectID = service.StringPointerOrDefault(l.ProjectID, DefaultProjectID)
	m.Region = service.StringPointerOrDefault(l.Region, DefaultRegion)
	m.ProcessingRegion = service.StringPointerOrDefault(l.ProcessingRegion, DefaultProcessingRegion)
	m.Format = service.StringPointerOrDefault(l.Format, constants.LoggingScalyrDefaultFormat)
	m.FormatVersion = service.Int64PointerOrDefault(l.FormatVersion, DefaultFormatVersion)
	m.Placement = service.StringPointerOrNull(l.Placement)
	m.ResponseCondition = service.StringPointerOrDefault(l.ResponseCondition, DefaultResponseCondition)

	return m
}

// ResetVCLOnlyToDefaults restores the VCL-only fields to their schema defaults
// after a flatten. On a Compute service they are never sent, so the API's own
// values are discarded rather than reported as a diff against the plan.
func ResetVCLOnlyToDefaults(m *NestedModel) {
	m.Format = types.StringValue(constants.LoggingScalyrDefaultFormat)
	m.FormatVersion = types.Int64Value(DefaultFormatVersion)
	m.Placement = types.StringNull()
	m.ResponseCondition = types.StringValue(DefaultResponseCondition)
}

// FlattenToComputeNestedModel is FlattenToNestedModel for Compute services: it
// carries over only the attributes ComputeNestedModel exposes.
func FlattenToComputeNestedModel(l *fastly.Scalyr) ComputeNestedModel {
	return ComputeNestedModel{commonModel: FlattenToNestedModel(l).commonModel}
}

func flatten(ctx context.Context, l *fastly.Scalyr, m *Model) {
	if l == nil {
		tflog.Warn(ctx, "flatten called with nil Scalyr logging endpoint")
		return
	}

	id := fastly.ToValue(l.ServiceID) + "/" + strconv.Itoa(fastly.ToValue(l.ServiceVersion)) + "/" + fastly.ToValue(l.Name)
	m.ID = types.StringValue(id)
	m.Service = types.StringValue(fastly.ToValue(l.ServiceID))
	m.Version = types.Int64Value(int64(fastly.ToValue(l.ServiceVersion)))

	m.NestedModel = FlattenToNestedModel(l)

	tflog.Debug(ctx, "Flattened Scalyr logging endpoint state", map[string]any{
		"id":      id,
		"service": m.Service.ValueString(),
		"version": m.Version.ValueInt64(),
		"name":    m.Name.ValueString(),
	})
}
