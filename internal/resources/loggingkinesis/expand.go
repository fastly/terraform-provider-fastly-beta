package loggingkinesis

import (
	fastly "github.com/fastly/go-fastly/v17/fastly"

	"github.com/fastly/terraform-provider-fastly-beta/internal/service"
)

// buildCommonCreateInput sets the Create fields shared by VCL and Compute
// services. BuildCreateInput and BuildComputeCreateInput layer their
// service-type-specific fields on top of this.
func buildCommonCreateInput(serviceID string, version int, m commonModel) *fastly.CreateKinesisInput {
	input := &fastly.CreateKinesisInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
		Name:           new(service.StringValue(m.Name)),
		StreamName:     new(service.StringValue(m.Topic)),
	}

	input.Region = fastly.NullString(service.StringValue(m.Region))
	input.AccessKey = fastly.NullString(service.StringValue(m.AccessKey()))
	input.SecretKey = fastly.NullString(service.StringValue(m.SecretKey()))
	input.IAMRole = fastly.NullString(service.StringValue(m.IAMRole()))
	input.ProcessingRegion = fastly.NullString(service.StringValue(m.ProcessingRegion))

	return input
}

func BuildCreateInput(serviceID string, version int, m NestedModel) *fastly.CreateKinesisInput {
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
func BuildComputeCreateInput(serviceID string, version int, m ComputeNestedModel) *fastly.CreateKinesisInput {
	return buildCommonCreateInput(serviceID, version, m.commonModel)
}

// ClearVCLOnlyCreateFields nils out format, format_version, placement, and
// response_condition on a CreateKinesisInput. The standalone
// fastly_service_logging_kinesis resource shares one schema across both
// service types, so this is called instead of BuildComputeCreateInput to
// strip the VCL-only fields once the service is confirmed to be Compute.
func ClearVCLOnlyCreateFields(input *fastly.CreateKinesisInput) {
	input.Format = nil
	input.FormatVersion = nil
	input.Placement = nil
	input.ResponseCondition = nil
}

// ClearVCLOnlyUpdateFields is ClearVCLOnlyCreateFields for UpdateKinesisInput.
func ClearVCLOnlyUpdateFields(input *fastly.UpdateKinesisInput) {
	input.Format = nil
	input.FormatVersion = nil
	input.Placement = nil
	input.ResponseCondition = nil
}

// buildCommonUpdateInput sets the Update fields shared by VCL and Compute
// services. BuildUpdateInput and BuildComputeUpdateInput layer their
// service-type-specific fields on top of this.
func buildCommonUpdateInput(serviceID string, version int, m commonModel) *fastly.UpdateKinesisInput {
	input := &fastly.UpdateKinesisInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
		Name:           service.StringValue(m.Name),
		NewName:        new(service.StringValue(m.Name)),
		StreamName:     new(service.StringValue(m.Topic)),
	}

	// region defaults to a non-empty value ("us-east-1"), so it is never empty
	// and fastly.NullString always sends it.
	input.Region = fastly.NullString(service.StringValue(m.Region))
	// Credentials default to "" and can be cleared — e.g. switching from
	// access_key/secret_key to iam_role auth. Always send a concrete value via
	// new() rather than fastly.NullString, which maps "" to nil, omits the field
	// (access_key,omitempty), and leaves the previously-set credential in place.
	input.AccessKey = new(service.StringValue(m.AccessKey()))
	input.SecretKey = new(service.StringValue(m.SecretKey()))
	input.IAMRole = new(service.StringValue(m.IAMRole()))
	input.ProcessingRegion = fastly.NullString(service.StringValue(m.ProcessingRegion))

	return input
}

func BuildUpdateInput(serviceID string, version int, m NestedModel) *fastly.UpdateKinesisInput {
	input := buildCommonUpdateInput(serviceID, version, m.commonModel)
	input.Format = fastly.NullString(service.StringValue(m.Format))
	input.FormatVersion = fastly.NullInt(int(service.Int64Value(m.FormatVersion)))
	// placement can be cleared back to unset / nil (distinct from "none" — see
	// schema.go). UpdateKinesisInput.Placement is a *Nullable[string]
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
func BuildComputeUpdateInput(serviceID string, version int, m ComputeNestedModel) *fastly.UpdateKinesisInput {
	return buildCommonUpdateInput(serviceID, version, m.commonModel)
}
