package cachesetting

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
	assert.Equal(t, nested.Action, m.Action)
	assert.Equal(t, nested.CacheCondition, m.CacheCondition)
	assert.Equal(t, nested.TTL, m.TTL)
	assert.Equal(t, nested.StaleTTL, m.StaleTTL)

	assert.Equal(t, types.StringValue("test-id"), m.ID)
	assert.Equal(t, types.StringValue("test-service"), m.Service)
	assert.Equal(t, types.Int64Value(1), m.Version)

	extracted := m.NestedModel
	assert.Equal(t, nested, extracted)
}

func TestFlatten(t *testing.T) {
	tests := []struct {
		name    string
		setting *fastly.CacheSetting
		want    func(t *testing.T, m *Model)
	}{
		{
			name:    "nil cache setting logs warning and leaves model untouched",
			setting: nil,
			want: func(t *testing.T, m *Model) {
				assert.Equal(t, types.String{}, m.ID)
				assert.Equal(t, types.String{}, m.Service)
				assert.Equal(t, types.Int64{}, m.Version)
			},
		},
		{
			name: "cache setting with service metadata",
			setting: func() *fastly.CacheSetting {
				action := fastly.CacheSettingActionCache
				return &fastly.CacheSetting{
					ServiceID:      new("service-123"),
					ServiceVersion: new(5),
					Name:           new("cache-setting"),
					Action:         &action,
					CacheCondition: new("cache-condition"),
					StaleTTL:       new(120),
					TTL:            new(3600),
				}
			}(),
			want: func(t *testing.T, m *Model) {
				assert.Equal(t, types.StringValue("service-123-5-cache-setting"), m.ID)
				assert.Equal(t, types.StringValue("service-123"), m.Service)
				assert.Equal(t, types.Int64Value(5), m.Version)
				assert.Equal(t, types.StringValue("cache-setting"), m.Name)
				assert.Equal(t, types.StringValue("cache"), m.Action)
				assert.Equal(t, types.Int64Value(3600), m.TTL)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			m := &Model{}
			flatten(ctx, tt.setting, m)
			tt.want(t, m)
		})
	}
}

func TestBuildCreateInput(t *testing.T) {
	input := BuildCreateInput("service-123", 5, fullNestedModel())

	assert.Equal(t, "service-123", input.ServiceID)
	assert.Equal(t, 5, input.ServiceVersion)
	assert.Equal(t, "cache-setting", *input.Name)
	assert.Equal(t, fastly.CacheSettingActionCache, *input.Action)
	assert.Equal(t, "cache-condition", *input.CacheCondition)
	assert.Equal(t, 3600, *input.TTL)
	assert.Equal(t, 120, *input.StaleTTL)
}

func TestBuildCreateInput_omitsUnsetAction(t *testing.T) {
	input := BuildCreateInput("service-123", 5, minimalNestedModel())

	assert.Nil(t, input.Action)
}

func TestBuildUpdateInput(t *testing.T) {
	input := BuildUpdateInput("service-123", 5, fullNestedModel())

	assert.Equal(t, "service-123", input.ServiceID)
	assert.Equal(t, 5, input.ServiceVersion)
	assert.Equal(t, "cache-setting", input.Name)
	assert.Equal(t, fastly.CacheSettingActionCache, *input.Action)
	assert.Equal(t, "cache-condition", *input.CacheCondition)
	assert.Equal(t, 3600, *input.TTL)
	assert.Equal(t, 120, *input.StaleTTL)
}

func TestBuildUpdateInput_clearsUnsetAction(t *testing.T) {
	input := BuildUpdateInput("service-123", 5, minimalNestedModel())

	// Update always sends Action, lowered to empty when unset, so a previously configured
	// action can be cleared back to unset - unlike Create, which omits it entirely.
	if assert.NotNil(t, input.Action) {
		assert.Equal(t, fastly.CacheSettingAction(""), *input.Action)
	}
}
