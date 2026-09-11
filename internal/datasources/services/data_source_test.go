package services

import (
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"

	"github.com/fastly/go-fastly/v17/fastly"
)

func TestFlattenServices(t *testing.T) {
	createdAt := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)

	setValue, ids, diags := flattenServices([]*fastly.Service{
		{
			ServiceID:     fastly.ToPointer("service-1"),
			Name:          fastly.ToPointer("example"),
			Type:          fastly.ToPointer("vcl"),
			Comment:       fastly.ToPointer("a comment"),
			CustomerID:    fastly.ToPointer("customer-1"),
			CreatedAt:     &createdAt,
			ActiveVersion: fastly.ToPointer(3),
		},
	})
	assert.False(t, diags.HasError(), diags)

	assert.Equal(t, []string{"service-1"}, ids)
	assert.Len(t, setValue.Elements(), 1)

	obj, ok := setValue.Elements()[0].(types.Object)
	assert.True(t, ok)
	attributes := obj.Attributes()

	assert.Equal(t, "service-1", attributes["id"].(types.String).ValueString())
	assert.Equal(t, "example", attributes["name"].(types.String).ValueString())
	assert.Equal(t, "vcl", attributes["type"].(types.String).ValueString())
	assert.Equal(t, "a comment", attributes["comment"].(types.String).ValueString())
	assert.Equal(t, "customer-1", attributes["customer_id"].(types.String).ValueString())
	assert.Equal(t, createdAt.Format(time.RFC3339), attributes["created_at"].(types.String).ValueString())
	assert.True(t, attributes["updated_at"].(types.String).IsNull())
	assert.Equal(t, int64(3), attributes["version"].(types.Int64).ValueInt64())
}

func TestFlattenServicesEmpty(t *testing.T) {
	setValue, ids, diags := flattenServices(nil)
	assert.False(t, diags.HasError(), diags)
	assert.Empty(t, ids)
	assert.Empty(t, setValue.Elements())
}
