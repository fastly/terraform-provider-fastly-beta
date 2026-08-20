package settings

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"

	fastly "github.com/fastly/go-fastly/v17/fastly"
)

func fullNestedModel() NestedModel {
	return NestedModel{
		DefaultHost:     types.StringValue("host.example.com"),
		DefaultTTL:      types.Int64Value(120),
		HTTP3:           types.BoolValue(true),
		StaleIfError:    types.BoolValue(true),
		StaleIfErrorTTL: types.Int64Value(600),
	}
}

func TestFlattenToNestedModel(t *testing.T) {
	t.Run("nil settings", func(t *testing.T) {
		m := FlattenToNestedModel(nil, false)
		assert.Equal(t, NestedModel{}, m)
	})

	t.Run("full settings", func(t *testing.T) {
		s := &fastly.Settings{
			DefaultHost:     new("host.example.com"),
			DefaultTTL:      new(uint(120)),
			StaleIfError:    new(true),
			StaleIfErrorTTL: new(uint(600)),
		}
		m := FlattenToNestedModel(s, true)
		assert.Equal(t, fullNestedModel(), m)
	})

	t.Run("nil optional fields default", func(t *testing.T) {
		s := &fastly.Settings{}
		m := FlattenToNestedModel(s, false)
		assert.Equal(t, defaultNestedModel(), m)
	})
}

func TestDefaultNestedModel(t *testing.T) {
	d := defaultNestedModel()
	assert.Equal(t, types.StringValue(""), d.DefaultHost)
	assert.Equal(t, types.Int64Value(3600), d.DefaultTTL)
	assert.Equal(t, types.BoolValue(false), d.HTTP3)
	assert.Equal(t, types.BoolValue(false), d.StaleIfError)
	assert.Equal(t, types.Int64Value(43200), d.StaleIfErrorTTL)
}

func TestModelsEqual(t *testing.T) {
	a := fullNestedModel()
	b := fullNestedModel()
	assert.True(t, a.ModelsEqual(b))

	b.HTTP3 = types.BoolValue(false)
	assert.False(t, a.ModelsEqual(b))
}

func TestEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        []NestedModel
		b        []NestedModel
		expected bool
	}{
		{name: "both empty", a: nil, b: nil, expected: true},
		{name: "different lengths", a: []NestedModel{fullNestedModel()}, b: nil, expected: false},
		{name: "equal single", a: []NestedModel{fullNestedModel()}, b: []NestedModel{fullNestedModel()}, expected: true},
		{
			name:     "different single",
			a:        []NestedModel{fullNestedModel()},
			b:        []NestedModel{defaultNestedModel()},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, Equal(tt.a, tt.b))
		})
	}
}

func TestBuildUpdateInput(t *testing.T) {
	input := BuildUpdateInput("svc-123", 5, fullNestedModel())

	assert.Equal(t, "svc-123", input.ServiceID)
	assert.Equal(t, 5, input.ServiceVersion)
	assert.Equal(t, "host.example.com", *input.DefaultHost)
	assert.Equal(t, uint(120), *input.DefaultTTL)
	assert.Equal(t, true, *input.StaleIfError)
	assert.Equal(t, uint(600), *input.StaleIfErrorTTL)
}
