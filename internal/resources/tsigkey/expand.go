package tsigkey

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/fastly/go-fastly/v17/fastly"
	"github.com/fastly/go-fastly/v17/fastly/dns/v1/tsigkeys"
)

func BuildCreateInput(plan Model) *tsigkeys.CreateInput {
	input := &tsigkeys.CreateInput{
		Name:      new(service.StringValue(plan.Name)),
		Algorithm: new(service.StringValue(plan.Algorithm)),
		Secret:    new(service.StringValue(plan.SecretValue())),
	}

	if !plan.Description.IsNull() {
		input.Description = new(service.StringValue(plan.Description))
	}

	return input
}

func BuildUpdateInput(keyID string, plan, state Model) *tsigkeys.UpdateInput {
	input := &tsigkeys.UpdateInput{
		TSIGKeyID: new(keyID),
	}

	if !plan.Name.Equal(state.Name) {
		input.Name = new(service.StringValue(plan.Name))
	}
	if !plan.Algorithm.Equal(state.Algorithm) {
		input.Algorithm = new(service.StringValue(plan.Algorithm))
	}
	if !plan.SecretValue().Equal(state.SecretValue()) {
		input.Secret = new(service.StringValue(plan.SecretValue()))
	}
	if !plan.Description.Equal(state.Description) {
		input.Description = fastly.NewNullable(service.StringValue(plan.Description))
	}

	return input
}
