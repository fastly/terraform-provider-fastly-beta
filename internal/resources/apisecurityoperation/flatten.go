package apisecurityoperation

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly/apisecurity/operations"
)

func flatten(m *Model, op *operations.Operation, serviceID string) {
	m.ID = types.StringValue(fmt.Sprintf("%s/%s", serviceID, op.ID))
	m.ServiceID = types.StringValue(serviceID)
	m.OperationID = types.StringValue(op.ID)
	m.Method = types.StringValue(op.Method)
	m.Domain = types.StringValue(op.Domain)
	m.Path = types.StringValue(op.Path)
	m.Description = stringOrNull(op.Description)
	m.TagIDs = tagIDsFromSlice(op.TagIDs)
	m.Status = stringOrNull(op.Status)
	m.RPS = types.Float64Value(op.RPS)
	m.CreatedAt = stringOrNull(op.CreatedAt)
	m.UpdatedAt = stringOrNull(op.UpdatedAt)
	m.LastSeenAt = stringOrNull(op.LastSeenAt)
}

func stringOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

func tagIDsFromSlice(ids []string) types.Set {
	if len(ids) == 0 {
		return types.SetNull(types.StringType)
	}

	elems := make([]attr.Value, len(ids))
	for i, id := range ids {
		elems[i] = types.StringValue(id)
	}
	return types.SetValueMust(types.StringType, elems)
}
