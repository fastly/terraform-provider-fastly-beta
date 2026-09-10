package serviceauthorization

import (
	"github.com/fastly/go-fastly/v17/fastly"
)

func buildCreateInput(plan Model) *fastly.CreateServiceAuthorizationInput {
	return &fastly.CreateServiceAuthorizationInput{
		Service:    &fastly.SAService{ID: plan.ServiceID.ValueString()},
		User:       &fastly.SAUser{ID: plan.UserID.ValueString()},
		Permission: plan.Permission.ValueString(),
	}
}

func buildUpdateInput(id string, plan Model) *fastly.UpdateServiceAuthorizationInput {
	return &fastly.UpdateServiceAuthorizationInput{
		ID:         id,
		Permission: plan.Permission.ValueString(),
	}
}
