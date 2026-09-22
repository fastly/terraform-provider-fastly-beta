package loggingkafka

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"

	"github.com/fastly/terraform-provider-fastly-beta/internal/constants"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"
)

func FlattenToNestedModel(k *fastly.Kafka) NestedModel {
	m := NestedModel{}

	if k == nil {
		return m
	}

	m.Name = types.StringValue(fastly.ToValue(k.Name))
	m.Brokers = types.StringValue(fastly.ToValue(k.Brokers))
	m.Topic = types.StringValue(fastly.ToValue(k.Topic))
	m.CompressionCodec = service.StringPointerOrDefault(k.CompressionCodec, DefaultCompressionCodec)
	m.RequiredACKs = service.StringPointerOrDefault(k.RequiredACKs, DefaultRequiredACKs)
	m.RequestMaxBytes = service.Int64PointerOrDefault(k.RequestMaxBytes, DefaultRequestMaxBytes)
	m.ParseLogKeyvals = service.BoolPointerOrDefault(k.ParseLogKeyvals, DefaultParseLogKeyvals)
	m.AuthMethod = service.StringPointerOrDefault(k.AuthMethod, DefaultAuthMethod)
	m.Authentication = NewAuthenticationObject(
		service.StringPointerOrDefault(k.User, DefaultUser),
		service.StringPointerOrDefault(k.Password, DefaultPassword),
	)
	m.TLS = NewTLSObject(
		service.StringPointerOrDefault(k.TLSCACert, ""),
		service.StringPointerOrDefault(k.TLSClientCert, ""),
		service.StringPointerOrDefault(k.TLSClientKey, ""),
		service.StringPointerOrDefault(k.TLSHostname, DefaultTLSHostname),
	)
	m.UseTLS = service.BoolPointerOrDefault(k.UseTLS, DefaultUseTLS)
	m.ProcessingRegion = service.StringPointerOrDefault(k.ProcessingRegion, DefaultProcessingRegion)
	m.Format = service.StringPointerOrDefault(k.Format, constants.LoggingKafkaDefaultFormat)
	m.FormatVersion = service.Int64PointerOrDefault(k.FormatVersion, DefaultFormatVersion)
	m.Placement = service.StringPointerOrNull(k.Placement)
	m.ResponseCondition = service.StringPointerOrDefault(k.ResponseCondition, DefaultResponseCondition)

	return m
}

// ResetVCLOnlyToDefaults restores the VCL-only fields to their schema defaults
// after a flatten. On a Compute service they are never sent, so the API's own
// values are discarded rather than reported as a diff against the plan.
func ResetVCLOnlyToDefaults(m *NestedModel) {
	m.Format = types.StringValue(constants.LoggingKafkaDefaultFormat)
	m.FormatVersion = types.Int64Value(DefaultFormatVersion)
	m.Placement = types.StringNull()
	m.ResponseCondition = types.StringValue(DefaultResponseCondition)
}

// FlattenToComputeNestedModel is FlattenToNestedModel for Compute services: it
// carries over only the attributes ComputeNestedModel exposes.
func FlattenToComputeNestedModel(k *fastly.Kafka) ComputeNestedModel {
	return ComputeNestedModel{commonModel: FlattenToNestedModel(k).commonModel}
}

func flatten(ctx context.Context, k *fastly.Kafka, m *Model) {
	if k == nil {
		tflog.Warn(ctx, "flatten called with nil Kafka logging endpoint")
		return
	}

	id := fastly.ToValue(k.ServiceID) + "/" + strconv.Itoa(fastly.ToValue(k.ServiceVersion)) + "/" + fastly.ToValue(k.Name)
	m.ID = types.StringValue(id)
	m.Service = types.StringValue(fastly.ToValue(k.ServiceID))
	m.Version = types.Int64Value(int64(fastly.ToValue(k.ServiceVersion)))

	m.NestedModel = FlattenToNestedModel(k)

	tflog.Debug(ctx, "Flattened Kafka logging endpoint state", map[string]any{
		"id":      id,
		"service": m.Service.ValueString(),
		"version": m.Version.ValueInt64(),
		"name":    m.Name.ValueString(),
	})
}
