package domainmanagement

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/fastly/go-fastly/v17/fastly/domainmanagement/v1/domains"
)

func BuildCreateInput(plan Model) *domains.CreateInput {
	input := &domains.CreateInput{
		FQDN: new(service.StringValue(plan.FQDN)),
	}
	if !plan.Description.IsNull() {
		input.Description = new(service.StringValue(plan.Description))
	}
	if !plan.ServiceID.IsNull() {
		input.ServiceID = new(service.StringValue(plan.ServiceID))
	}
	return input
}

// BuildUpdateInput sends description/service_id explicitly - the API has
// no omitempty, so an unset field here would clear it server-side.
func BuildUpdateInput(domainID string, plan Model) *domains.UpdateInput {
	input := &domains.UpdateInput{
		DomainID: new(domainID),
	}
	if !plan.Description.IsNull() {
		input.Description = new(service.StringValue(plan.Description))
	}
	if !plan.ServiceID.IsNull() {
		input.ServiceID = new(service.StringValue(plan.ServiceID))
	}
	return input
}
