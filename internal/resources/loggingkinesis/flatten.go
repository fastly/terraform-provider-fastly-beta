package loggingkinesis

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"

	"github.com/fastly/terraform-provider-fastly-beta/internal/constants"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"
)

func FlattenToNestedModel(k *fastly.Kinesis) NestedModel {
	m := NestedModel{}

	if k == nil {
		return m
	}

	m.Name = types.StringValue(fastly.ToValue(k.Name))
	m.Topic = types.StringValue(fastly.ToValue(k.StreamName))
	m.Region = service.StringPointerOrDefault(k.Region, DefaultRegion)
	m.Authentication = NewAuthenticationObject(
		service.StringPointerOrDefault(k.AccessKey, DefaultAccessKey),
		service.StringPointerOrDefault(k.SecretKey, DefaultSecretKey),
		service.StringPointerOrDefault(k.IAMRole, DefaultIAMRole),
	)
	m.ProcessingRegion = service.StringPointerOrDefault(k.ProcessingRegion, DefaultProcessingRegion)
	m.Format = service.StringPointerOrDefault(k.Format, constants.LoggingKinesisDefaultFormat)
	m.FormatVersion = service.Int64PointerOrDefault(k.FormatVersion, DefaultFormatVersion)
	m.Placement = service.StringPointerOrNull(k.Placement)
	m.ResponseCondition = service.StringPointerOrDefault(k.ResponseCondition, DefaultResponseCondition)

	return m
}

// ResetVCLOnlyToDefaults restores the VCL-only fields to their schema defaults
// after a flatten. On a Compute service they are never sent, so the API's own
// values are discarded rather than reported as a diff against the plan.
func ResetVCLOnlyToDefaults(m *NestedModel) {
	m.Format = types.StringValue(constants.LoggingKinesisDefaultFormat)
	m.FormatVersion = types.Int64Value(DefaultFormatVersion)
	m.Placement = types.StringNull()
	m.ResponseCondition = types.StringValue(DefaultResponseCondition)
}

// FlattenToComputeNestedModel is FlattenToNestedModel for Compute services: it
// carries over only the attributes ComputeNestedModel exposes.
func FlattenToComputeNestedModel(k *fastly.Kinesis) ComputeNestedModel {
	return ComputeNestedModel{commonModel: FlattenToNestedModel(k).commonModel}
}

func flatten(ctx context.Context, k *fastly.Kinesis, m *Model) {
	if k == nil {
		tflog.Warn(ctx, "flatten called with nil Kinesis logging endpoint")
		return
	}

	id := fastly.ToValue(k.ServiceID) + "-" + strconv.Itoa(fastly.ToValue(k.ServiceVersion)) + "-" + fastly.ToValue(k.Name)
	m.ID = types.StringValue(id)
	m.Service = types.StringValue(fastly.ToValue(k.ServiceID))
	m.Version = types.Int64Value(int64(fastly.ToValue(k.ServiceVersion)))

	m.NestedModel = FlattenToNestedModel(k)

	tflog.Debug(ctx, "Flattened Kinesis logging endpoint state", map[string]any{
		"id":      id,
		"service": m.Service.ValueString(),
		"version": m.Version.ValueInt64(),
		"name":    m.Name.ValueString(),
	})
}
