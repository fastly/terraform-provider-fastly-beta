package responseobject

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
	assert.Equal(t, nested.Content, m.Content)
	assert.Equal(t, nested, m.NestedModel)
	assert.Equal(t, types.StringValue("test-id"), m.ID)
	assert.Equal(t, types.StringValue("test-service"), m.Service)
	assert.Equal(t, types.Int64Value(1), m.Version)
}

func TestFlatten(t *testing.T) {
	tests := []struct {
		name string
		ro   *fastly.ResponseObject
		want func(t *testing.T, m *Model)
	}{
		{
			name: "nil response object leaves model untouched",
			ro:   nil,
			want: func(t *testing.T, m *Model) {
				assert.Equal(t, types.String{}, m.ID)
				assert.Equal(t, types.String{}, m.Service)
				assert.Equal(t, types.Int64{}, m.Version)
			},
		},
		{
			name: "response object with service metadata",
			ro: &fastly.ResponseObject{
				Name:             new("response-object"),
				CacheCondition:   new("cache-condition"),
				Content:          new("test content"),
				ContentType:      new("text/html"),
				RequestCondition: new("request-condition"),
				Response:         new("Not Found"),
				ServiceID:        new("service-123"),
				ServiceVersion:   new(5),
				Status:           new(404),
			},
			want: func(t *testing.T, m *Model) {
				assert.Equal(t, types.StringValue("service-123-5-response-object"), m.ID)
				assert.Equal(t, types.StringValue("service-123"), m.Service)
				assert.Equal(t, types.Int64Value(5), m.Version)
				assert.Equal(t, types.StringValue("response-object"), m.Name)
				assert.Equal(t, types.StringValue("cache-condition"), m.CacheCondition)
				assert.Equal(t, types.StringValue("test content"), m.Content)
				assert.Equal(t, types.StringValue("text/html"), m.ContentType)
				assert.Equal(t, types.StringValue("request-condition"), m.RequestCondition)
				assert.Equal(t, types.StringValue("Not Found"), m.Response)
				assert.Equal(t, types.Int64Value(404), m.Status)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{}
			flatten(context.Background(), tt.ro, m)
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
	assert.Empty(t, attrs["version"].(schema.Int64Attribute).PlanModifiers)

	// The nested block's name must stay free of resource-level replacement semantics.
	assert.Empty(t, CommonAttributes()["name"].(schema.StringAttribute).PlanModifiers)
}
