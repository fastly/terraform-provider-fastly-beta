package tsigkey

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly/dns/v1/tsigkeys"
)

// FlattenToModel maps the API response onto a Model. secret is passed
// through from the caller, since the API never returns it.
func FlattenToModel(key *tsigkeys.TSIGKey, secret types.Object) Model {
	return Model{
		ID:          service.StringPointerOrNull(key.ID),
		Algorithm:   service.StringPointerOrNull(key.Algorithm),
		Description: service.StringPointerOrNull(key.Description),
		Name:        service.StringPointerOrNull(key.Name),
		Secret:      secret,
	}
}

// ReconcileDescription restores "" when the API echoes null for a description
// that was explicitly set to "" — avoids a perpetual diff.
func ReconcileDescription(returned, known types.String) types.String {
	if returned.IsNull() && known.Equal(types.StringValue("")) {
		return types.StringValue("")
	}
	return returned
}
