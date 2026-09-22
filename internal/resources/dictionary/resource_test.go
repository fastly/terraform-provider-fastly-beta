package dictionary

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
	assert.Equal(t, nested.DictionaryID, m.DictionaryID)
	assert.Equal(t, nested.WriteOnly, m.WriteOnly)
	assert.Equal(t, nested.ForceDestroy, m.ForceDestroy)

	assert.Equal(t, types.StringValue("test-id"), m.ID)
	assert.Equal(t, types.StringValue("test-service"), m.Service)
	assert.Equal(t, types.Int64Value(1), m.Version)
}

func TestFlatten(t *testing.T) {
	tests := []struct {
		name     string
		dict     *fastly.Dictionary
		validate func(t *testing.T, m *Model)
	}{
		{
			name: "nil dictionary logs warning and leaves model untouched",
			dict: nil,
			validate: func(t *testing.T, m *Model) {
				assert.Equal(t, types.String{}, m.ID)
				assert.Equal(t, types.String{}, m.Service)
				assert.Equal(t, types.Int64{}, m.Version)
			},
		},
		{
			name: "dictionary with service metadata",
			dict: &fastly.Dictionary{
				ServiceID:      new("service_123"),
				ServiceVersion: new(5),
				Name:           new("test_dictionary"),
				DictionaryID:   new("dict_abc123"),
				WriteOnly:      new(false),
			},
			validate: func(t *testing.T, m *Model) {
				assert.Equal(t, types.StringValue("service_123-5-test_dictionary"), m.ID)
				assert.Equal(t, types.StringValue("service_123"), m.Service)
				assert.Equal(t, types.Int64Value(5), m.Version)
				assert.Equal(t, types.StringValue("test_dictionary"), m.Name)
				assert.Equal(t, types.StringValue("dict_abc123"), m.DictionaryID)
				assert.Equal(t, types.BoolValue(false), m.WriteOnly)
			},
		},
		{
			name: "existing force_destroy is preserved across flatten",
			dict: &fastly.Dictionary{
				ServiceID:      new("service_456"),
				ServiceVersion: new(10),
				Name:           new("full_dictionary"),
				DictionaryID:   new("dict_xyz789"),
				WriteOnly:      new(true),
			},
			validate: func(t *testing.T, m *Model) {
				assert.Equal(t, types.BoolValue(true), m.ForceDestroy)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			m := &Model{}
			if tt.name == "existing force_destroy is preserved across flatten" {
				m.ForceDestroy = types.BoolValue(true)
			}
			flatten(ctx, tt.dict, m)
			tt.validate(t, m)
		})
	}
}

func TestResourceAttributes(t *testing.T) {
	attrs := ResourceAttributes()

	for _, key := range []string{"id", "service_id", "version", "name", "dictionary_id", "write_only", "force_destroy"} {
		if _, ok := attrs[key]; !ok {
			t.Errorf("ResourceAttributes() missing %q", key)
		}
	}
}
