package logginggooglepubsub

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"

	"github.com/fastly/terraform-provider-fastly-beta/internal/constants"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"
)

func FlattenToNestedModel(p *fastly.Pubsub) NestedModel {
	m := NestedModel{}

	if p == nil {
		return m
	}

	m.Name = types.StringValue(fastly.ToValue(p.Name))
	m.ProjectID = types.StringValue(fastly.ToValue(p.ProjectID))
	m.Topic = types.StringValue(fastly.ToValue(p.Topic))
	m.Authentication = NewAuthenticationObject(
		service.StringPointerOrDefault(p.AccountName, ""),
		service.StringPointerOrDefault(p.User, ""),
		service.StringPointerOrDefault(p.SecretKey, ""),
	)
	m.ProcessingRegion = service.StringPointerOrDefault(p.ProcessingRegion, DefaultProcessingRegion)
	m.Format = service.StringPointerOrDefault(p.Format, constants.LoggingGooglePubSubDefaultFormat)
	m.FormatVersion = service.Int64PointerOrDefault(p.FormatVersion, DefaultFormatVersion)
	m.Placement = service.StringPointerOrNull(p.Placement)
	m.ResponseCondition = service.StringPointerOrDefault(p.ResponseCondition, DefaultResponseCondition)

	return m
}

// ResetVCLOnlyToDefaults restores the VCL-only fields to their schema defaults
// after a flatten. On a Compute service they are never sent, so the API's own
// values are discarded rather than reported as a diff against the plan.
func ResetVCLOnlyToDefaults(m *NestedModel) {
	m.Format = types.StringValue(constants.LoggingGooglePubSubDefaultFormat)
	m.FormatVersion = types.Int64Value(DefaultFormatVersion)
	m.Placement = types.StringNull()
	m.ResponseCondition = types.StringValue(DefaultResponseCondition)
}

// FlattenToComputeNestedModel is FlattenToNestedModel for Compute services: it
// carries over only the attributes ComputeNestedModel exposes.
func FlattenToComputeNestedModel(p *fastly.Pubsub) ComputeNestedModel {
	return ComputeNestedModel{commonModel: FlattenToNestedModel(p).commonModel}
}

func flatten(ctx context.Context, p *fastly.Pubsub, m *Model) {
	if p == nil {
		tflog.Warn(ctx, "flatten called with nil Pub/Sub logging endpoint")
		return
	}

	id := fastly.ToValue(p.ServiceID) + "-" + strconv.Itoa(fastly.ToValue(p.ServiceVersion)) + "-" + fastly.ToValue(p.Name)
	m.ID = types.StringValue(id)
	m.Service = types.StringValue(fastly.ToValue(p.ServiceID))
	m.Version = types.Int64Value(int64(fastly.ToValue(p.ServiceVersion)))

	m.NestedModel = FlattenToNestedModel(p)

	tflog.Debug(ctx, "Flattened Pub/Sub logging endpoint state", map[string]any{
		"id":      id,
		"service": m.Service.ValueString(),
		"version": m.Version.ValueInt64(),
		"name":    m.Name.ValueString(),
	})
}
