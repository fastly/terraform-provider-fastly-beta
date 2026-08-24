package responseobject

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"

	fastly "github.com/fastly/go-fastly/v17/fastly"
)

func minimalNestedModel() NestedModel {
	return NestedModel{
		Name:             types.StringValue("response-object"),
		CacheCondition:   types.StringValue(""),
		Content:          types.StringValue(""),
		ContentType:      types.StringValue(""),
		RequestCondition: types.StringValue(""),
		Response:         types.StringValue(DefaultResponse),
		Status:           types.Int64Value(DefaultStatus),
	}
}

func fullNestedModel() NestedModel {
	return NestedModel{
		Name:             types.StringValue("response-object"),
		CacheCondition:   types.StringValue("cache-condition"),
		Content:          types.StringValue("test content"),
		ContentType:      types.StringValue("text/html"),
		RequestCondition: types.StringValue("request-condition"),
		Response:         types.StringValue("Not Found"),
		Status:           types.Int64Value(404),
	}
}

func TestToModel(t *testing.T) {
	api := &fastly.ResponseObject{
		Name:             new("response-object"),
		CacheCondition:   new("cache-condition"),
		Content:          new("test content"),
		ContentType:      new("text/html"),
		RequestCondition: new("request-condition"),
		Response:         new("Not Found"),
		Status:           new(404),
	}

	result := ops{}.ToModel(api)

	assert.True(t, fullNestedModel().ModelsEqual(result))
}

func TestToModel_defaultsAppliedForUnsetFields(t *testing.T) {
	api := &fastly.ResponseObject{
		Name: new("response-object"),
	}

	result := ops{}.ToModel(api)

	assert.Equal(t, "response-object", result.Name.ValueString())
	assert.Equal(t, "", result.CacheCondition.ValueString())
	assert.Equal(t, "", result.Content.ValueString())
	assert.Equal(t, "", result.ContentType.ValueString())
	assert.Equal(t, "", result.RequestCondition.ValueString())
	assert.Equal(t, DefaultResponse, result.Response.ValueString())
	assert.Equal(t, int64(DefaultStatus), result.Status.ValueInt64())
}

func TestOpsEqual(t *testing.T) {
	remote := &fastly.ResponseObject{
		Name:             new("response-object"),
		CacheCondition:   new("cache-condition"),
		Content:          new("test content"),
		ContentType:      new("text/html"),
		RequestCondition: new("request-condition"),
		Response:         new("Not Found"),
		Status:           new(404),
	}

	assert.True(t, ops{}.Equal(fullNestedModel(), remote))
}

func TestOpsEqual_mismatch(t *testing.T) {
	remote := &fastly.ResponseObject{
		Name: new("response-object"),
	}

	assert.False(t, ops{}.Equal(fullNestedModel(), remote))
}

func TestEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        []NestedModel
		b        []NestedModel
		expected bool
	}{
		{
			name:     "both empty",
			a:        []NestedModel{},
			b:        []NestedModel{},
			expected: true,
		},
		{
			name:     "same content",
			a:        []NestedModel{fullNestedModel()},
			b:        []NestedModel{fullNestedModel()},
			expected: true,
		},
		{
			name: "different lengths",
			a:    []NestedModel{minimalNestedModel()},
			b: []NestedModel{
				minimalNestedModel(),
				fullNestedModel(),
			},
			expected: false,
		},
		{
			name:     "different content",
			a:        []NestedModel{minimalNestedModel()},
			b:        []NestedModel{fullNestedModel()},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Equal(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchOrder(t *testing.T) {
	a := NestedModel{Name: types.StringValue("a")}
	b := NestedModel{Name: types.StringValue("b")}

	result := MatchOrder([]NestedModel{b, a}, []NestedModel{a, b})

	assert.Equal(t, []NestedModel{a, b}, result)
}

func TestValidateConditionReferences(t *testing.T) {
	conditionNames := map[string]struct{}{"my-condition": {}}

	t.Run("no conditions set", func(t *testing.T) {
		item := minimalNestedModel()
		assert.NoError(t, ValidateConditionReferences([]NestedModel{item}, nil))
	})

	t.Run("references configured conditions", func(t *testing.T) {
		item := minimalNestedModel()
		item.CacheCondition = types.StringValue("my-condition")
		item.RequestCondition = types.StringValue("my-condition")

		assert.NoError(t, ValidateConditionReferences([]NestedModel{item}, conditionNames))
	})

	t.Run("cache_condition references a condition that isn't configured", func(t *testing.T) {
		item := minimalNestedModel()
		item.CacheCondition = types.StringValue("missing-condition")

		err := ValidateConditionReferences([]NestedModel{item}, conditionNames)
		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), `"response-object"`)
			assert.Contains(t, err.Error(), `"missing-condition"`)
		}
	})

	t.Run("request_condition references a condition that isn't configured", func(t *testing.T) {
		item := minimalNestedModel()
		item.RequestCondition = types.StringValue("missing-condition")

		assert.Error(t, ValidateConditionReferences([]NestedModel{item}, conditionNames))
	})

	t.Run("skips unknown conditions", func(t *testing.T) {
		item := minimalNestedModel()
		item.CacheCondition = types.StringUnknown()
		item.RequestCondition = types.StringUnknown()

		assert.NoError(t, ValidateConditionReferences([]NestedModel{item}, nil))
	})
}
