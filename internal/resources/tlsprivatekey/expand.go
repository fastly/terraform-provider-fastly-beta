package tlsprivatekey

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/fastly/go-fastly/v17/fastly"
)

func buildCreateInput(plan Model) *fastly.CreatePrivateKeyInput {
	return &fastly.CreatePrivateKeyInput{
		Key:  service.StringValue(plan.PEM()),
		Name: service.StringValue(plan.Name),
	}
}
