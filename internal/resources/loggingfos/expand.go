package loggingfos

import (
	fastly "github.com/fastly/go-fastly/v19/fastly"

	"github.com/fastly/terraform-provider-fastly-beta/internal/service"
)

func buildCommonCreateInput(serviceID string, version int, m commonModel) *fastly.CreateFOSInput {
	input := &fastly.CreateFOSInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
		Name:           new(service.StringValue(m.Name)),
		BucketName:     new(service.StringValue(m.BucketName)),
		Region:         new(service.StringValue(m.Region)),
	}

	input.Path = new(service.StringValue(m.Path))
	input.Period = fastly.NullInt(int(service.Int64Value(m.Period)))
	input.CompressionCodec = fastly.NullString(service.StringValue(m.CompressionCodec))
	// Send gzip_level only when set: the API rejects it alongside compression_codec. fastly.NullInt
	// would drop an explicit 0 (no compression).
	if gl := service.Int64Value(m.GzipLevel); gl != DefaultGzipLevel {
		input.GzipLevel = new(int(gl))
	}
	input.MessageType = fastly.NullString(service.StringValue(m.MessageType))
	input.TimestampFormat = fastly.NullString(service.StringValue(m.TimestampFormat))
	input.PublicKey = fastly.NullString(service.StringValue(m.PublicKey))

	if fmb := service.Int64Value(m.FileMaxBytes); fmb != 0 {
		v := int(fmb)
		input.FileMaxBytes = &v
	}

	return input
}

func BuildCreateInput(serviceID string, version int, m NestedModel) *fastly.CreateFOSInput {
	input := buildCommonCreateInput(serviceID, version, m.commonModel)
	input.Format = fastly.NullString(service.StringValue(m.Format))
	input.FormatVersion = fastly.NullInt(int(service.Int64Value(m.FormatVersion)))
	input.Placement = fastly.NullString(service.StringValue(m.Placement))
	input.ResponseCondition = fastly.NullString(service.StringValue(m.ResponseCondition))
	return input
}

// ClearVCLOnlyCreateFields strips VCL-only fields when the standalone resource targets a Compute service.
func ClearVCLOnlyCreateFields(input *fastly.CreateFOSInput) {
	input.Format = nil
	input.FormatVersion = nil
	input.Placement = nil
	input.ResponseCondition = nil
}

func ClearVCLOnlyUpdateFields(input *fastly.UpdateFOSInput) {
	input.Format = nil
	input.FormatVersion = nil
	input.Placement = nil
	input.ResponseCondition = nil
}

func BuildComputeCreateInput(serviceID string, version int, m ComputeNestedModel) *fastly.CreateFOSInput {
	return buildCommonCreateInput(serviceID, version, m.commonModel)
}

func buildCommonUpdateInput(serviceID string, version int, m commonModel) *fastly.UpdateFOSInput {
	input := &fastly.UpdateFOSInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
		Name:           service.StringValue(m.Name),
		NewName:        new(service.StringValue(m.Name)),
		BucketName:     new(service.StringValue(m.BucketName)),
		Region:         new(service.StringValue(m.Region)),
	}

	input.Path = new(service.StringValue(m.Path))
	input.Period = fastly.NullInt(int(service.Int64Value(m.Period)))
	input.CompressionCodec = new(service.StringValue(m.CompressionCodec))
	// Send gzip_level only when set: the API rejects it alongside compression_codec. With no codec
	// and no level, send 0 explicitly so a level set out of band is reset.
	if gl := service.Int64Value(m.GzipLevel); gl != DefaultGzipLevel {
		input.GzipLevel = new(int(gl))
	} else if service.StringValue(m.CompressionCodec) == "" {
		input.GzipLevel = new(0)
	}
	input.MessageType = fastly.NullString(service.StringValue(m.MessageType))
	input.TimestampFormat = fastly.NullString(service.StringValue(m.TimestampFormat))
	input.PublicKey = new(service.StringValue(m.PublicKey))

	fmb := int(service.Int64Value(m.FileMaxBytes))
	input.FileMaxBytes = &fmb

	return input
}

func BuildComputeUpdateInput(serviceID string, version int, m ComputeNestedModel) *fastly.UpdateFOSInput {
	return buildCommonUpdateInput(serviceID, version, m.commonModel)
}

func BuildUpdateInput(serviceID string, version int, m NestedModel) *fastly.UpdateFOSInput {
	input := buildCommonUpdateInput(serviceID, version, m.commonModel)
	input.Format = fastly.NullString(service.StringValue(m.Format))
	input.FormatVersion = fastly.NullInt(int(service.Int64Value(m.FormatVersion)))
	// An explicit JSON null is the only way to clear placement; omitting it keeps the old value
	// and "" is stored as "".
	if v := service.StringValue(m.Placement); v != "" {
		input.Placement = fastly.NewNullable(v)
	} else {
		input.Placement = fastly.NullValue[string]()
	}
	input.ResponseCondition = new(service.StringValue(m.ResponseCondition))
	return input
}
