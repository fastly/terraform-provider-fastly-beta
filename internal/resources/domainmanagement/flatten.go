package domainmanagement

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly/domainmanagement/v1/domains"
)

func FlattenToModel(d *domains.Data) Model {
	return Model{
		ID:          types.StringValue(d.DomainID),
		Description: types.StringValue(d.Description),
		FQDN:        types.StringValue(d.FQDN),
		ServiceID:   service.StringPointerOrNull(d.ServiceID),
	}
}

// ReconcileDescription restores null when unconfigured: the API's
// description is a plain string, never null, so it reads back as "".
func ReconcileDescription(returned, known types.String) types.String {
	if returned.ValueString() == "" && known.IsNull() {
		return types.StringNull()
	}
	return returned
}
