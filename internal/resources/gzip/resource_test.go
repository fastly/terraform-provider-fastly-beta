package gzip

import (
	"context"
	"testing"

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
	assert.Equal(t, nested.CacheCondition, m.CacheCondition)
	assert.Equal(t, nested.ContentTypes, m.ContentTypes)
	assert.Equal(t, nested.Extensions, m.Extensions)

	assert.Equal(t, types.StringValue("test-id"), m.ID)
	assert.Equal(t, types.StringValue("test-service"), m.Service)
	assert.Equal(t, types.Int64Value(1), m.Version)

	extracted := m.NestedModel
	assert.Equal(t, nested, extracted)
}

func TestFlatten(t *testing.T) {
	tests := []struct {
		name  string
		gzip  *fastly.Gzip
		prior *NestedModel
		want  func(t *testing.T, m *Model)
	}{
		{
			name: "nil gzip logs warning and leaves model untouched",
			gzip: nil,
			want: func(t *testing.T, m *Model) {
				assert.Equal(t, types.String{}, m.ID)
				assert.Equal(t, types.String{}, m.Service)
				assert.Equal(t, types.Int64{}, m.Version)
			},
		},
		{
			name: "gzip with service metadata, no prior to normalize against",
			gzip: &fastly.Gzip{
				ServiceID:      new("service-123"),
				ServiceVersion: new(5),
				Name:           new("gzip-config"),
				CacheCondition: new("cache-condition"),
				ContentTypes:   new("text/html text/css"),
				Extensions:     new("css js"),
			},
			prior: nil,
			want: func(t *testing.T, m *Model) {
				assert.Equal(t, types.StringValue("service-123-5-gzip-config"), m.ID)
				assert.Equal(t, types.StringValue("service-123"), m.Service)
				assert.Equal(t, types.Int64Value(5), m.Version)
				assert.Equal(t, types.StringValue("gzip-config"), m.Name)
				assert.True(t, stringList("text/html", "text/css").Equal(m.ContentTypes))
				assert.True(t, stringList("css", "js").Equal(m.Extensions))
			},
		},
		{
			name: "remote's substituted default is nulled when prior never set content_types/extensions",
			gzip: &fastly.Gzip{
				ServiceID:      new("service-123"),
				ServiceVersion: new(5),
				Name:           new("gzip-config"),
				ContentTypes:   new("text/html text/css text/javascript application/json"),
				Extensions:     new("css js"),
			},
			prior: func() *NestedModel {
				m := minimalNestedModel()
				return &m
			}(),
			want: func(t *testing.T, m *Model) {
				assert.True(t, m.ContentTypes.IsNull())
				assert.True(t, m.Extensions.IsNull())
			},
		},
		{
			name: "remote value is kept when prior had it explicitly set",
			gzip: &fastly.Gzip{
				ServiceID:      new("service-123"),
				ServiceVersion: new(5),
				Name:           new("gzip-config"),
				ContentTypes:   new("text/html"),
				Extensions:     new("css js"),
			},
			prior: func() *NestedModel {
				m := fullNestedModel()
				return &m
			}(),
			want: func(t *testing.T, m *Model) {
				assert.True(t, stringList("text/html").Equal(m.ContentTypes))
				assert.True(t, stringList("css", "js").Equal(m.Extensions))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			m := &Model{}
			flatten(ctx, tt.gzip, m, tt.prior)
			tt.want(t, m)
		})
	}
}
