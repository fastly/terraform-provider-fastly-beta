package domainservicelink

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly/domainmanagement/v1/domains"
)

func FlattenToModel(d *domains.Data) Model {
	return Model{
		ID:        types.StringValue(d.DomainID),
		DomainID:  types.StringValue(d.DomainID),
		ServiceID: service.StringPointerOrNull(d.ServiceID),
	}
}
