package loggingkafka

import (
	fastly "github.com/fastly/go-fastly/v17/fastly"

	"github.com/fastly/terraform-provider-fastly-beta/internal/service"
)

// buildCommonCreateInput sets the Create fields shared by VCL and Compute
// services. BuildCreateInput and BuildComputeCreateInput layer their
// service-type-specific fields on top of this.
func buildCommonCreateInput(serviceID string, version int, m commonModel) *fastly.CreateKafkaInput {
	input := &fastly.CreateKafkaInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
		Name:           new(service.StringValue(m.Name)),
		Brokers:        new(service.StringValue(m.Brokers)),
		Topic:          new(service.StringValue(m.Topic)),
	}

	input.CompressionCodec = fastly.NullString(service.StringValue(m.CompressionCodec))
	input.RequiredACKs = fastly.NullString(service.StringValue(m.RequiredACKs))
	// request_max_bytes defaults to 0 ("no limit"), the same value omitting the
	// field would leave the API to choose - fine to omit on Create.
	input.RequestMaxBytes = fastly.NullInt(int(service.Int64Value(m.RequestMaxBytes)))
	input.ParseLogKeyvals = new(fastly.Compatibool(service.BoolValue(m.ParseLogKeyvals)))
	input.AuthMethod = fastly.NullString(service.StringValue(m.AuthMethod))
	input.User = fastly.NullString(service.StringValue(m.User()))
	input.Password = fastly.NullString(service.StringValue(m.Password()))
	input.TLSCACert = fastly.NullString(service.StringValue(m.TLSCACert()))
	input.TLSClientCert = fastly.NullString(service.StringValue(m.TLSClientCert()))
	input.TLSClientKey = fastly.NullString(service.StringValue(m.TLSClientKey()))
	input.TLSHostname = fastly.NullString(service.StringValue(m.TLSHostname()))
	input.UseTLS = new(fastly.Compatibool(service.BoolValue(m.UseTLS)))
	input.ProcessingRegion = fastly.NullString(service.StringValue(m.ProcessingRegion))

	return input
}

func BuildCreateInput(serviceID string, version int, m NestedModel) *fastly.CreateKafkaInput {
	input := buildCommonCreateInput(serviceID, version, m.commonModel)
	input.Format = fastly.NullString(service.StringValue(m.Format))
	input.FormatVersion = fastly.NullInt(int(service.Int64Value(m.FormatVersion)))
	input.Placement = fastly.NullString(service.StringValue(m.Placement))
	input.ResponseCondition = fastly.NullString(service.StringValue(m.ResponseCondition))
	return input
}

// BuildComputeCreateInput is BuildCreateInput for Compute services: it never
// sets format, format_version, placement, or response_condition, since those
// only affect generated VCL and Compute services don't have any.
func BuildComputeCreateInput(serviceID string, version int, m ComputeNestedModel) *fastly.CreateKafkaInput {
	return buildCommonCreateInput(serviceID, version, m.commonModel)
}

// ClearVCLOnlyCreateFields nils out format, format_version, placement, and
// response_condition on a CreateKafkaInput. The standalone
// fastly_service_logging_kafka resource shares one schema across both service
// types, so this is called instead of BuildComputeCreateInput to strip the
// VCL-only fields once the service is confirmed to be Compute.
func ClearVCLOnlyCreateFields(input *fastly.CreateKafkaInput) {
	input.Format = nil
	input.FormatVersion = nil
	input.Placement = nil
	input.ResponseCondition = nil
}

// ClearVCLOnlyUpdateFields is ClearVCLOnlyCreateFields for UpdateKafkaInput.
func ClearVCLOnlyUpdateFields(input *fastly.UpdateKafkaInput) {
	input.Format = nil
	input.FormatVersion = nil
	input.Placement = nil
	input.ResponseCondition = nil
}

// buildCommonUpdateInput sets the Update fields shared by VCL and Compute
// services. BuildUpdateInput and BuildComputeUpdateInput layer their
// service-type-specific fields on top of this.
func buildCommonUpdateInput(serviceID string, version int, m commonModel) *fastly.UpdateKafkaInput {
	input := &fastly.UpdateKafkaInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
		Name:           service.StringValue(m.Name),
		NewName:        new(service.StringValue(m.Name)),
		Brokers:        new(service.StringValue(m.Brokers)),
		Topic:          new(service.StringValue(m.Topic)),
	}

	// compression_codec and auth_method default to "" and can be cleared. Always
	// send a concrete value via new() rather than fastly.NullString, which maps
	// "" to nil, omits the field, and leaves the previously-set value in place.
	input.CompressionCodec = new(service.StringValue(m.CompressionCodec))
	input.AuthMethod = new(service.StringValue(m.AuthMethod))
	// required_acks and processing_region default to a non-empty value ("1" and
	// "none"), so they are never empty and fastly.NullString always sends them.
	input.RequiredACKs = fastly.NullString(service.StringValue(m.RequiredACKs))
	input.ProcessingRegion = fastly.NullString(service.StringValue(m.ProcessingRegion))
	// request_max_bytes defaults to 0 ("no limit"), a legitimate explicit value a
	// practitioner may set back to after raising it, so - like
	// compression_codec/auth_method above - it must always be sent rather than
	// omitted when zero (fastly.NullInt maps 0 to nil).
	input.RequestMaxBytes = new(int(service.Int64Value(m.RequestMaxBytes)))
	input.ParseLogKeyvals = new(fastly.Compatibool(service.BoolValue(m.ParseLogKeyvals)))
	// authentication.user/password and tls.* default to "" and are clearable,
	// same reasoning as compression_codec/auth_method above.
	input.User = new(service.StringValue(m.User()))
	input.Password = new(service.StringValue(m.Password()))
	input.TLSCACert = new(service.StringValue(m.TLSCACert()))
	input.TLSClientCert = new(service.StringValue(m.TLSClientCert()))
	input.TLSClientKey = new(service.StringValue(m.TLSClientKey()))
	input.TLSHostname = new(service.StringValue(m.TLSHostname()))
	input.UseTLS = new(fastly.Compatibool(service.BoolValue(m.UseTLS)))

	return input
}

func BuildUpdateInput(serviceID string, version int, m NestedModel) *fastly.UpdateKafkaInput {
	input := buildCommonUpdateInput(serviceID, version, m.commonModel)
	input.Format = fastly.NullString(service.StringValue(m.Format))
	input.FormatVersion = fastly.NullInt(int(service.Int64Value(m.FormatVersion)))
	// placement can be cleared back to unset / nil (distinct from "none" — see
	// schema.go). UpdateKafkaInput.Placement is a *Nullable[string] specifically
	// so this can be sent as a real JSON null: omitting the field leaves the
	// previous value in place, and sending a literal empty string gets stored as
	// "" rather than reverting to null/auto-placement — neither actually clears
	// it.
	if v := service.StringValue(m.Placement); v != "" {
		input.Placement = fastly.NewNullable(v)
	} else {
		input.Placement = fastly.NullValue[string]()
	}
	input.ResponseCondition = new(service.StringValue(m.ResponseCondition))
	return input
}

// BuildComputeUpdateInput is BuildUpdateInput for Compute services: it never
// sets format, format_version, placement, or response_condition, since those
// only affect generated VCL and Compute services don't have any.
func BuildComputeUpdateInput(serviceID string, version int, m ComputeNestedModel) *fastly.UpdateKafkaInput {
	return buildCommonUpdateInput(serviceID, version, m.commonModel)
}
