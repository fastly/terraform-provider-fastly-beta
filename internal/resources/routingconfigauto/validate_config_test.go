package routingconfigauto

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"

	vt "github.com/fastly/terraform-provider-fastly-beta/internal/validation/validationtest"
)

func TestValidateConfig_unknownBlocks(t *testing.T) {
	r := NewResource()
	typ := vt.ConfigType(t, r)
	pathType := vt.BlockType(typ, "path")
	ruleType := vt.BlockType(pathType, "rule")
	conditionType := vt.BlockType(ruleType, "condition")

	pathWithRules := func(rules ...tftypes.Value) map[string]tftypes.Value {
		return map[string]tftypes.Value{
			"path": vt.Block(typ, "path", vt.Object(t, pathType, map[string]tftypes.Value{
				"path": vt.String("/"),
				"rule": vt.Block(pathType, "rule", rules...),
			})),
		}
	}
	conditionalRule := vt.Object(t, ruleType, map[string]tftypes.Value{
		"condition": vt.Block(ruleType, "condition", vt.Object(t, conditionType, map[string]tftypes.Value{
			"type": vt.String("path"),
		})),
	})

	t.Run("unknown path", func(t *testing.T) {
		diags := vt.ValidateConfig(t, r, map[string]tftypes.Value{"path": vt.Unknown(typ, "path")})
		assert.False(t, diags.HasError(), "%v", diags)
	})

	t.Run("unknown rule", func(t *testing.T) {
		diags := vt.ValidateConfig(t, r, map[string]tftypes.Value{
			"path": vt.Block(typ, "path", vt.Object(t, pathType, map[string]tftypes.Value{
				"path": vt.String("/"),
				"rule": vt.Unknown(pathType, "rule"),
			})),
		})
		assert.False(t, diags.HasError(), "%v", diags)
	})

	t.Run("unknown condition may be the default rule", func(t *testing.T) {
		diags := vt.ValidateConfig(t, r, pathWithRules(
			conditionalRule,
			vt.Object(t, ruleType, map[string]tftypes.Value{"condition": vt.Unknown(ruleType, "condition")}),
		))
		assert.False(t, diags.HasError(), "%v", diags)
	})

	t.Run("known rules still require a default", func(t *testing.T) {
		diags := vt.ValidateConfig(t, r, pathWithRules(conditionalRule))
		assert.True(t, diags.HasError())
	})
}
