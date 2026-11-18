package alert

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"

	vt "github.com/fastly/terraform-provider-fastly-beta/internal/validation/validationtest"
)

func TestValidateConfig_unknownBlocks(t *testing.T) {
	r := NewResource()
	typ := vt.ConfigType(t, r)

	for _, name := range []string{"dimensions", "evaluation_strategy"} {
		t.Run(name, func(t *testing.T) {
			diags := vt.ValidateConfig(t, r, map[string]tftypes.Value{
				"source":     vt.Unknown(typ, "source"),
				"service_id": vt.Unknown(typ, "service_id"),
				name:         vt.Unknown(typ, name),
			})
			assert.False(t, diags.HasError(), "%v", diags)
		})
	}
}
