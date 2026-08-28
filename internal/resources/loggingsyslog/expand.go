package loggingsyslog

import (
	fastly "github.com/fastly/go-fastly/v17/fastly"

	"github.com/fastly/terraform-provider-fastly-beta/internal/service"
)

// buildCommonCreateInput sets the Create fields shared by VCL and Compute
// services. BuildCreateInput and BuildComputeCreateInput layer their
// service-type-specific fields on top of this.
func buildCommonCreateInput(serviceID string, version int, m commonModel) *fastly.CreateSyslogInput {
	input := &fastly.CreateSyslogInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
		Name:           new(service.StringValue(m.Name)),
		Address:        new(service.StringValue(m.Address)),
	}

	input.Port = fastly.NullInt(int(service.Int64Value(m.Port)))
	input.Token = fastly.NullString(service.StringValue(m.Token()))
	input.TLSCACert = fastly.NullString(service.StringValue(m.TLSCACert()))
	input.TLSClientCert = fastly.NullString(service.StringValue(m.TLSClientCert()))
	input.TLSClientKey = fastly.NullString(service.StringValue(m.TLSClientKey()))
	input.TLSHostname = fastly.NullString(service.StringValue(m.TLSHostname()))
	input.UseTLS = new(fastly.Compatibool(service.BoolValue(m.UseTLS)))
	input.MessageType = fastly.NullString(service.StringValue(m.MessageType))
	input.ProcessingRegion = fastly.NullString(service.StringValue(m.ProcessingRegion))

	return input
}

func BuildCreateInput(serviceID string, version int, m NestedModel) *fastly.CreateSyslogInput {
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
func BuildComputeCreateInput(serviceID string, version int, m ComputeNestedModel) *fastly.CreateSyslogInput {
	return buildCommonCreateInput(serviceID, version, m.commonModel)
}

// buildCommonUpdateInput sets the Update fields shared by VCL and Compute
// services. BuildUpdateInput and BuildComputeUpdateInput layer their
// service-type-specific fields on top of this.
func buildCommonUpdateInput(serviceID string, version int, m commonModel) *fastly.UpdateSyslogInput {
	input := &fastly.UpdateSyslogInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
		Name:           service.StringValue(m.Name),
		NewName:        new(service.StringValue(m.Name)),
		Address:        new(service.StringValue(m.Address)),
	}

	input.Port = fastly.NullInt(int(service.Int64Value(m.Port)))
	// token and tls.* default to "" (directly, or via their environment
	// variables) rather than being omitted-when-empty, so they must always be
	// sent as a concrete value on update — otherwise clearing one back to ""
	// would be silently dropped (fastly.NullString maps "" to nil, which omits
	// the field and leaves a previously-set value in place).
	input.Token = new(service.StringValue(m.Token()))
	input.TLSCACert = new(service.StringValue(m.TLSCACert()))
	input.TLSClientCert = new(service.StringValue(m.TLSClientCert()))
	input.TLSClientKey = new(service.StringValue(m.TLSClientKey()))
	input.TLSHostname = new(service.StringValue(m.TLSHostname()))
	input.UseTLS = new(fastly.Compatibool(service.BoolValue(m.UseTLS)))
	input.MessageType = fastly.NullString(service.StringValue(m.MessageType))
	input.ProcessingRegion = fastly.NullString(service.StringValue(m.ProcessingRegion))

	return input
}

func BuildUpdateInput(serviceID string, version int, m NestedModel) *fastly.UpdateSyslogInput {
	input := buildCommonUpdateInput(serviceID, version, m.commonModel)
	input.Format = fastly.NullString(service.StringValue(m.Format))
	input.FormatVersion = fastly.NullInt(int(service.Int64Value(m.FormatVersion)))
	// placement can be cleared back to unset / nil (distinct from "none" — see
	// schema.go). UpdateSyslogInput.Placement is a *Nullable[string]
	// specifically so this can be sent as a real JSON null: omitting the field
	// leaves the previous value in place, and sending a literal empty string
	// gets stored as "" rather than reverting to null/auto-placement — neither
	// actually clears it.
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
func BuildComputeUpdateInput(serviceID string, version int, m ComputeNestedModel) *fastly.UpdateSyslogInput {
	return buildCommonUpdateInput(serviceID, version, m.commonModel)
}

// ClearVCLOnlyCreateFields nils out format, format_version, placement, and
// response_condition on a CreateSyslogInput. The standalone
// fastly_service_logging_syslog resource shares one schema across both
// service types, so this is called instead of BuildComputeCreateInput to strip
// the VCL-only fields once the service is confirmed to be Compute.
func ClearVCLOnlyCreateFields(input *fastly.CreateSyslogInput) {
	input.Format = nil
	input.FormatVersion = nil
	input.Placement = nil
	input.ResponseCondition = nil
}

// ClearVCLOnlyUpdateFields is ClearVCLOnlyCreateFields for UpdateSyslogInput.
func ClearVCLOnlyUpdateFields(input *fastly.UpdateSyslogInput) {
	input.Format = nil
	input.FormatVersion = nil
	input.Placement = nil
	input.ResponseCondition = nil
}
