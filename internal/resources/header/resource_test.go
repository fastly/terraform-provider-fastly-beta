package header

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"

	fastly "github.com/fastly/go-fastly/v17/fastly"
)

func TestModelEmbedding(t *testing.T) {
	nested := fullNestedModel()
	m := Model{
		NestedModel: nested,
		ID:          types.StringValue("test-id"),
		Service:     types.StringValue("test-service"),
		Version:     types.Int64Value(1),
	}

	assert.Equal(t, nested.Name, m.Name)
	assert.Equal(t, nested.Destination, m.Destination)

	assert.Equal(t, types.StringValue("test-id"), m.ID)
	assert.Equal(t, types.StringValue("test-service"), m.Service)
	assert.Equal(t, types.Int64Value(1), m.Version)

	assert.Equal(t, nested, m.NestedModel)
}

func TestFlatten(t *testing.T) {
	tests := []struct {
		name string
		h    *fastly.Header
		want func(t *testing.T, m *Model)
	}{
		{
			name: "nil header leaves model untouched",
			h:    nil,
			want: func(t *testing.T, m *Model) {
				assert.Equal(t, types.String{}, m.ID)
				assert.Equal(t, types.String{}, m.Service)
				assert.Equal(t, types.Int64{}, m.Version)
			},
		},
		{
			name: "header with service metadata",
			h: func() *fastly.Header {
				h := fullAPIHeader()
				h.ServiceID = new("service-123")
				h.ServiceVersion = new(5)
				return h
			}(),
			want: func(t *testing.T, m *Model) {
				assert.Equal(t, types.StringValue("service-123-5-header"), m.ID)
				assert.Equal(t, types.StringValue("service-123"), m.Service)
				assert.Equal(t, types.Int64Value(5), m.Version)
				assert.Equal(t, types.StringValue("header"), m.Name)
				assert.Equal(t, types.StringValue("http.X-Custom"), m.Destination)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{}
			flatten(context.Background(), tt.h, m)
			tt.want(t, m)
		})
	}
}

func TestResourceAttributes(t *testing.T) {
	attrs := ResourceAttributes()

	for _, name := range []string{"id", "service_id", "version"} {
		assert.Contains(t, attrs, name)
	}
	for key := range CommonAttributes() {
		assert.Contains(t, attrs, key)
	}

	assert.True(t, attrs["id"].IsComputed())
	assert.True(t, attrs["service_id"].IsRequired())
	assert.True(t, attrs["version"].IsRequired())

	assert.Len(t, attrs["service_id"].(schema.StringAttribute).PlanModifiers, 1)
	assert.Len(t, attrs["name"].(schema.StringAttribute).PlanModifiers, 1)

	// The nested block's name must stay free of plan modifiers.
	assert.Empty(t, CommonAttributes()["name"].(schema.StringAttribute).PlanModifiers)
}
