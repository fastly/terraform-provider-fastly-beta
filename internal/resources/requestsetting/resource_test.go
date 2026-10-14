package requestsetting

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
	assert.Equal(t, nested.Action, m.Action)
	assert.Equal(t, nested, m.NestedModel)
	assert.Equal(t, types.StringValue("test-id"), m.ID)
	assert.Equal(t, types.StringValue("test-service"), m.Service)
	assert.Equal(t, types.Int64Value(1), m.Version)
}

func TestFlatten(t *testing.T) {
	action := fastly.RequestSettingActionLookup
	xff := fastly.RequestSettingXFFAppend

	tests := []struct {
		name string
		rs   *fastly.RequestSetting
		want func(t *testing.T, m *Model)
	}{
		{
			name: "nil request setting leaves model untouched",
			rs:   nil,
			want: func(t *testing.T, m *Model) {
				assert.Equal(t, types.String{}, m.ID)
				assert.Equal(t, types.String{}, m.Service)
				assert.Equal(t, types.Int64{}, m.Version)
			},
		},
		{
			name: "request setting with service metadata",
			rs: &fastly.RequestSetting{
				Name:             new("request-setting"),
				Action:           &action,
				BypassBusyWait:   new(true),
				DefaultHost:      new("host.example.com"),
				ForceMiss:        new(true),
				ForceSSL:         new(true),
				HashKeys:         new("field1,field2"),
				MaxStaleAge:      new(120),
				RequestCondition: new("request-condition"),
				ServiceID:        new("service-123"),
				ServiceVersion:   new(5),
				TimerSupport:     new(true),
				XForwardedFor:    &xff,
			},
			want: func(t *testing.T, m *Model) {
				assert.Equal(t, types.StringValue("service-123-5-request-setting"), m.ID)
				assert.Equal(t, types.StringValue("service-123"), m.Service)
				assert.Equal(t, types.Int64Value(5), m.Version)
				assert.Equal(t, types.StringValue("request-setting"), m.Name)
				assert.Equal(t, types.StringValue("lookup"), m.Action)
				assert.Equal(t, types.StringValue("append"), m.XFF)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{}
			flatten(context.Background(), tt.rs, m)
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
