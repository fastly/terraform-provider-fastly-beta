package validation

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testItem struct {
	Name types.String `tfsdk:"name"`
}

type testModel struct {
	Items []testItem `tfsdk:"item"`
	Tags  types.List `tfsdk:"tags"`
}

func testConfig(items, tags tftypes.Value) tfsdk.Config {
	ctx := context.Background()
	s := schema.Schema{
		Attributes: map[string]schema.Attribute{
			"tags": schema.ListAttribute{ElementType: types.StringType, Optional: true},
		},
		Blocks: map[string]schema.Block{
			"item": schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{Optional: true},
					},
				},
			},
		},
	}
	typ := s.Type().TerraformType(ctx).(tftypes.Object)
	return tfsdk.Config{Schema: s, Raw: tftypes.NewValue(typ, map[string]tftypes.Value{"item": items, "tags": tags})}
}

func TestGetConfig(t *testing.T) {
	itemType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"name": tftypes.String}}
	itemsType := tftypes.List{ElementType: itemType}
	tagsType := tftypes.List{ElementType: tftypes.String}

	t.Run("unknown block decodes as empty and is reported unknown", func(t *testing.T) {
		cfg := testConfig(tftypes.NewValue(itemsType, tftypes.UnknownValue), tftypes.NewValue(tagsType, tftypes.UnknownValue))

		var m testModel
		isUnknown, diags := GetConfig(context.Background(), cfg, &m)
		require.False(t, diags.HasError(), "%v", diags)

		assert.Empty(t, m.Items)
		assert.True(t, isUnknown(path.Root("item")))
		assert.True(t, m.Tags.IsUnknown(), "unknown primitive lists stay unknown")
		assert.True(t, isUnknown(path.Root("tags")))
	})

	t.Run("known block decodes unchanged", func(t *testing.T) {
		cfg := testConfig(
			tftypes.NewValue(itemsType, []tftypes.Value{
				tftypes.NewValue(itemType, map[string]tftypes.Value{"name": tftypes.NewValue(tftypes.String, tftypes.UnknownValue)}),
			}),
			tftypes.NewValue(tagsType, nil),
		)

		var m testModel
		isUnknown, diags := GetConfig(context.Background(), cfg, &m)
		require.False(t, diags.HasError(), "%v", diags)

		require.Len(t, m.Items, 1)
		assert.True(t, m.Items[0].Name.IsUnknown())
		assert.False(t, isUnknown(path.Root("item")))
		assert.True(t, isUnknown(path.Root("item").AtListIndex(0).AtName("name")))
		assert.False(t, isUnknown(path.Root("tags")))
	})
}
