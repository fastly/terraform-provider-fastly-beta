package apisecurityoperationtag

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly/apisecurity/operations"
)

func flatten(m *Model, tag *operations.OperationTag, serviceID string) {
	m.ID = types.StringValue(fmt.Sprintf("%s/%s", serviceID, tag.ID))
	m.ServiceID = types.StringValue(serviceID)
	m.TagID = types.StringValue(tag.ID)
	m.Name = types.StringValue(tag.Name)
	m.Description = stringOrNull(tag.Description)
	m.OperationCount = types.Int64Value(int64(tag.Count))
	m.CreatedAt = stringOrNull(tag.CreatedAt)
	m.UpdatedAt = stringOrNull(tag.UpdatedAt)
}

func stringOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}
