package tlsmutualauthentication

import (
	"sort"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly"
)

func flattenToModel(mtls *fastly.TLSMutualAuthentication) Model {
	m := Model{
		ID:             types.StringValue(mtls.ID),
		Enforced:       types.BoolValue(mtls.Enforced),
		Name:           types.StringValue(mtls.Name),
		CreatedAt:      types.StringNull(),
		UpdatedAt:      types.StringNull(),
		TLSActivations: activationsToList(mtls.Activations),
	}
	if mtls.CreatedAt != nil {
		m.CreatedAt = types.StringValue(mtls.CreatedAt.Format(time.RFC3339))
	}
	if mtls.UpdatedAt != nil {
		m.UpdatedAt = types.StringValue(mtls.UpdatedAt.Format(time.RFC3339))
	}
	return m
}

// activationsToList sorts IDs for a stable diff against API-returned order.
func activationsToList(activations []*fastly.TLSActivation) types.List {
	ids := make([]string, 0, len(activations))
	for _, a := range activations {
		if a != nil {
			ids = append(ids, a.ID)
		}
	}
	if len(ids) == 0 {
		return types.ListNull(types.StringType)
	}
	sort.Strings(ids)

	elems := make([]attr.Value, 0, len(ids))
	for _, id := range ids {
		elems = append(elems, types.StringValue(id))
	}
	return types.ListValueMust(types.StringType, elems)
}
