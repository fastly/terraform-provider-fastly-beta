package userserviceauthorization

import (
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly"
)

func flattenToModel(sa *fastly.ServiceAuthorization) Model {
	return Model{
		ID:         types.StringValue(sa.ID),
		ServiceID:  types.StringValue(sa.Service.ID),
		UserID:     types.StringValue(sa.User.ID),
		Permission: types.StringValue(sa.Permission),
	}
}
