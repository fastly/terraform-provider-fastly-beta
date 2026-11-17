package loggingfos

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v19/fastly"

	"github.com/fastly/terraform-provider-fastly-beta/internal/constants"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"
)

func FlattenToNestedModel(s *fastly.FOS) NestedModel {
	m := NestedModel{}

	if s == nil {
		return m
	}

	m.Name = types.StringValue(fastly.ToValue(s.Name))
	m.BucketName = types.StringValue(fastly.ToValue(s.BucketName))
	m.Region = types.StringValue(fastly.ToValue(s.Region))
	m.Path = service.StringPointerOrDefault(s.Path, DefaultPath)
	m.Period = service.Int64PointerOrDefault(s.Period, DefaultPeriod)
	m.GzipLevel = service.Int64PointerOrDefault(s.GzipLevel, DefaultGzipLevel)
	m.CompressionCodec = service.StringPointerOrDefault(s.CompressionCodec, DefaultCompressionCodec)
	m.Format = service.StringPointerOrDefault(s.Format, constants.LoggingFOSDefaultFormat)
	m.FormatVersion = service.Int64PointerOrDefault(s.FormatVersion, DefaultFormatVersion)
	m.MessageType = service.StringPointerOrDefault(s.MessageType, DefaultMessageType)
	m.TimestampFormat = service.StringPointerOrDefault(s.TimestampFormat, DefaultTimestampFormat)
	m.Placement = service.StringPointerOrNull(s.Placement)
	m.ResponseCondition = service.StringPointerOrDefault(s.ResponseCondition, DefaultResponseCondition)
	m.PublicKey = service.StringPointerOrDefault(s.PublicKey, DefaultPublicKey)
	m.FileMaxBytes = service.Int64PointerOrDefault(s.FileMaxBytes, 0)

	return m
}

// ResetVCLOnlyToDefaults discards the API's VCL-only values for Compute services, which never send them.
func ResetVCLOnlyToDefaults(m *NestedModel) {
	m.Format = types.StringValue(constants.LoggingFOSDefaultFormat)
	m.FormatVersion = types.Int64Value(DefaultFormatVersion)
	m.Placement = types.StringNull()
	m.ResponseCondition = types.StringValue(DefaultResponseCondition)
}

func FlattenToComputeNestedModel(s *fastly.FOS) ComputeNestedModel {
	return ComputeNestedModel{commonModel: FlattenToNestedModel(s).commonModel}
}

// preserveGzipSentinelCommon keeps the unset gzip_level sentinel instead of the API's auto-managed value.
func preserveGzipSentinelCommon(m *commonModel, desired commonModel) {
	if desired.GzipLevel.IsNull() {
		inferGzipSentinelOnImport(m)
		return
	}
	if service.Int64Value(desired.GzipLevel) == DefaultGzipLevel {
		m.GzipLevel = types.Int64Value(DefaultGzipLevel)
	}
}

func preserveGzipSentinel(m *NestedModel, desired NestedModel) {
	preserveGzipSentinelCommon(&m.commonModel, desired.commonModel)
}

func preserveGzipSentinelCompute(m *ComputeNestedModel, desired ComputeNestedModel) {
	preserveGzipSentinelCommon(&m.commonModel, desired.commonModel)
}

// inferGzipSentinelOnImport treats an API gzip_level of 0 with no codec as unset when there is no
// prior model (import, discovered endpoint); a genuine explicit 0 self-corrects on the next apply.
func inferGzipSentinelOnImport(m *commonModel) {
	if service.StringValue(m.CompressionCodec) == "" && service.Int64Value(m.GzipLevel) == 0 {
		m.GzipLevel = types.Int64Value(DefaultGzipLevel)
	}
}

func preserveGzipSentinelList(read, desired []NestedModel) {
	desiredByName := make(map[string]NestedModel, len(desired))
	for _, d := range desired {
		desiredByName[service.StringValue(d.Name)] = d
	}
	for i := range read {
		if d, ok := desiredByName[service.StringValue(read[i].Name)]; ok {
			preserveGzipSentinel(&read[i], d)
		} else {
			inferGzipSentinelOnImport(&read[i].commonModel)
		}
	}
}

func preserveGzipSentinelListCompute(read, desired []ComputeNestedModel) {
	desiredByName := make(map[string]ComputeNestedModel, len(desired))
	for _, d := range desired {
		desiredByName[service.StringValue(d.Name)] = d
	}
	for i := range read {
		if d, ok := desiredByName[service.StringValue(read[i].Name)]; ok {
			preserveGzipSentinelCompute(&read[i], d)
		} else {
			inferGzipSentinelOnImport(&read[i].commonModel)
		}
	}
}

func flatten(ctx context.Context, s *fastly.FOS, m *Model) {
	if s == nil {
		tflog.Warn(ctx, "flatten called with nil Fastly Object Storage logging endpoint")
		return
	}

	id := fastly.ToValue(s.ServiceID) + "-" + strconv.Itoa(fastly.ToValue(s.ServiceVersion)) + "-" + fastly.ToValue(s.Name)
	m.ID = types.StringValue(id)
	m.Service = types.StringValue(fastly.ToValue(s.ServiceID))
	m.Version = types.Int64Value(int64(fastly.ToValue(s.ServiceVersion)))

	m.NestedModel = FlattenToNestedModel(s)

	tflog.Debug(ctx, "Flattened Fastly Object Storage logging endpoint state", map[string]any{
		"id":      id,
		"service": m.Service.ValueString(),
		"version": m.Version.ValueInt64(),
		"name":    m.Name.ValueString(),
	})
}
