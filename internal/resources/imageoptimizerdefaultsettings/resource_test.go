package imageoptimizerdefaultsettings

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
)

func TestFlattenModel(t *testing.T) {
	var m Model
	flattenModel(&m, fullNestedModel(), "svc-123", 5)

	assert.Equal(t, types.StringValue("svc-123/5"), m.ID)
	assert.Equal(t, types.StringValue("svc-123"), m.Service)
	assert.Equal(t, types.Int64Value(5), m.Version)
	assert.Equal(t, fullNestedModel(), m.NestedModel)
}

func TestResourceAttributes(t *testing.T) {
	attrs := ResourceAttributes()

	for _, name := range []string{"id", "service_id", "version", "allow_video", "jpeg_quality", "jpeg_type", "resize_filter", "upscale", "webp", "webp_quality"} {
		assert.Contains(t, attrs, name)
	}

	assert.True(t, attrs["id"].IsComputed())
	assert.True(t, attrs["service_id"].IsRequired())
	assert.True(t, attrs["version"].IsRequired())

	assert.Len(t, attrs["service_id"].(schema.StringAttribute).PlanModifiers, 1)
	assert.Empty(t, attrs["version"].(schema.Int64Attribute).PlanModifiers)

	for name, attr := range CommonAttributes() {
		assert.True(t, attr.IsOptional(), "%s should be optional", name)
		assert.True(t, attr.IsComputed(), "%s should be computed so its default populates", name)
	}
}
