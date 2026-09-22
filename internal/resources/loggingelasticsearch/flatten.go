package loggingelasticsearch

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"

	"github.com/fastly/terraform-provider-fastly-beta/internal/constants"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"
)

func FlattenToNestedModel(es *fastly.Elasticsearch) NestedModel {
	m := NestedModel{}

	if es == nil {
		return m
	}

	m.Name = types.StringValue(fastly.ToValue(es.Name))
	m.Index = types.StringValue(fastly.ToValue(es.Index))
	m.URL = types.StringValue(fastly.ToValue(es.URL))
	m.Authentication = NewAuthenticationObject(
		service.StringPointerOrDefault(es.User, DefaultUser),
		service.StringPointerOrDefault(es.Password, DefaultPassword),
	)
	m.TLS = NewTLSObject(
		service.StringPointerOrDefault(es.TLSCACert, ""),
		service.StringPointerOrDefault(es.TLSClientCert, ""),
		service.StringPointerOrDefault(es.TLSClientKey, ""),
		service.StringPointerOrDefault(es.TLSHostname, DefaultTLSHostname),
	)
	m.Pipeline = service.StringPointerOrDefault(es.Pipeline, DefaultPipeline)
	m.ProcessingRegion = service.StringPointerOrDefault(es.ProcessingRegion, DefaultProcessingRegion)
	m.RequestMaxBytes = service.Int64PointerOrDefault(es.RequestMaxBytes, DefaultRequestMaxBytes)
	m.RequestMaxEntries = service.Int64PointerOrDefault(es.RequestMaxEntries, DefaultRequestMaxEntries)
	m.Format = service.StringPointerOrDefault(es.Format, constants.LoggingElasticsearchDefaultFormat)
	m.FormatVersion = service.Int64PointerOrDefault(es.FormatVersion, DefaultFormatVersion)
	m.Placement = service.StringPointerOrNull(es.Placement)
	m.ResponseCondition = service.StringPointerOrDefault(es.ResponseCondition, DefaultResponseCondition)

	return m
}

// ResetVCLOnlyToDefaults restores the VCL-only fields to their schema defaults
// after a flatten. On a Compute service they are never sent, so the API's own
// values are discarded rather than reported as a diff against the plan.
func ResetVCLOnlyToDefaults(m *NestedModel) {
	m.Format = types.StringValue(constants.LoggingElasticsearchDefaultFormat)
	m.FormatVersion = types.Int64Value(DefaultFormatVersion)
	m.Placement = types.StringNull()
	m.ResponseCondition = types.StringValue(DefaultResponseCondition)
}

// FlattenToComputeNestedModel is FlattenToNestedModel for Compute services: it
// carries over only the attributes ComputeNestedModel exposes.
func FlattenToComputeNestedModel(es *fastly.Elasticsearch) ComputeNestedModel {
	return ComputeNestedModel{commonModel: FlattenToNestedModel(es).commonModel}
}

func flatten(ctx context.Context, es *fastly.Elasticsearch, m *Model) {
	if es == nil {
		tflog.Warn(ctx, "flatten called with nil Elasticsearch logging endpoint")
		return
	}

	id := fastly.ToValue(es.ServiceID) + "/" + strconv.Itoa(fastly.ToValue(es.ServiceVersion)) + "/" + fastly.ToValue(es.Name)
	m.ID = types.StringValue(id)
	m.Service = types.StringValue(fastly.ToValue(es.ServiceID))
	m.Version = types.Int64Value(int64(fastly.ToValue(es.ServiceVersion)))

	m.NestedModel = FlattenToNestedModel(es)

	tflog.Debug(ctx, "Flattened Elasticsearch logging endpoint state", map[string]any{
		"id":      id,
		"service": m.Service.ValueString(),
		"version": m.Version.ValueInt64(),
		"name":    m.Name.ValueString(),
	})
}
