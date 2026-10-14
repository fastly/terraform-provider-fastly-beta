package director

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
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
	assert.Equal(t, nested.Backends, m.Backends)

	assert.Equal(t, types.StringValue("test-id"), m.ID)
	assert.Equal(t, types.StringValue("test-service"), m.Service)
	assert.Equal(t, types.Int64Value(1), m.Version)

	extracted := m.NestedModel
	assert.Equal(t, nested, extracted)
}

func TestFlatten(t *testing.T) {
	tests := []struct {
		name string
		d    *fastly.Director
		want func(t *testing.T, m *Model)
	}{
		{
			name: "nil director logs warning and leaves model untouched",
			d:    nil,
			want: func(t *testing.T, m *Model) {
				assert.Equal(t, types.String{}, m.ID)
				assert.Equal(t, types.String{}, m.Service)
				assert.Equal(t, types.Int64{}, m.Version)
			},
		},
		{
			name: "director with service metadata",
			d: func() *fastly.Director {
				directorType := fastly.DirectorTypeHash
				return &fastly.Director{
					ServiceID:      new("service-123"),
					ServiceVersion: new(5),
					Name:           new("director"),
					Backends:       []string{"origin1"},
					Comment:        new("a comment"),
					Quorum:         new(30),
					Retries:        new(10),
					Shield:         new("sjc-ca-us"),
					Type:           &directorType,
				}
			}(),
			want: func(t *testing.T, m *Model) {
				assert.Equal(t, types.StringValue("service-123-5-director"), m.ID)
				assert.Equal(t, types.StringValue("service-123"), m.Service)
				assert.Equal(t, types.Int64Value(5), m.Version)
				assert.Equal(t, types.StringValue("director"), m.Name)
				assert.Equal(t, types.StringValue("hash"), m.Type)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			m := &Model{}
			flatten(ctx, tt.d, m)
			tt.want(t, m)
		})
	}
}

func TestResourceAttributes(t *testing.T) {
	attrs := ResourceAttributes()

	for _, key := range []string{"id", "service_id", "version"} {
		if _, ok := attrs[key]; !ok {
			t.Errorf("ResourceAttributes missing %q", key)
		}
	}
	for key := range CommonAttributes() {
		if _, ok := attrs[key]; !ok {
			t.Errorf("ResourceAttributes missing nested attribute %q", key)
		}
	}
}

func TestTypeStickyDefaultAttr(t *testing.T) {
	t.Run("config omitted defaults to random", func(t *testing.T) {
		req := planmodifier.StringRequest{
			ConfigValue: types.StringNull(),
			StateValue:  types.StringNull(),
		}
		resp := &planmodifier.StringResponse{}

		typeStickyDefaultAttr{}.PlanModifyString(context.Background(), req, resp)

		assert.Equal(t, types.StringValue(DefaultType), resp.PlanValue)
	})

	t.Run("config omitted preserves round_robin", func(t *testing.T) {
		req := planmodifier.StringRequest{
			ConfigValue: types.StringNull(),
			StateValue:  types.StringValue("round_robin"),
		}
		resp := &planmodifier.StringResponse{}

		typeStickyDefaultAttr{}.PlanModifyString(context.Background(), req, resp)

		assert.Equal(t, types.StringValue("round_robin"), resp.PlanValue)
	})

	t.Run("config omitted resets a non-round_robin prior value to the default", func(t *testing.T) {
		req := planmodifier.StringRequest{
			ConfigValue: types.StringNull(),
			StateValue:  types.StringValue("hash"),
		}
		resp := &planmodifier.StringResponse{}

		typeStickyDefaultAttr{}.PlanModifyString(context.Background(), req, resp)

		assert.Equal(t, types.StringValue(DefaultType), resp.PlanValue)
	})

	t.Run("config set to a numeric alias is normalized to the friendly name", func(t *testing.T) {
		req := planmodifier.StringRequest{
			ConfigValue: types.StringValue("3"),
			StateValue:  types.StringNull(),
		}
		resp := &planmodifier.StringResponse{}

		typeStickyDefaultAttr{}.PlanModifyString(context.Background(), req, resp)

		assert.Equal(t, types.StringValue("hash"), resp.PlanValue)
	})

	t.Run("config set - left alone", func(t *testing.T) {
		req := planmodifier.StringRequest{
			ConfigValue: types.StringValue("hash"),
			StateValue:  types.StringValue("random"),
		}
		resp := &planmodifier.StringResponse{}

		typeStickyDefaultAttr{}.PlanModifyString(context.Background(), req, resp)

		assert.Equal(t, types.StringValue("hash"), resp.PlanValue)
	})
}
