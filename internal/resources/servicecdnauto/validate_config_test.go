package servicecdnauto

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"

	vt "github.com/fastly/terraform-provider-fastly-beta/internal/validation/validationtest"
)

// A dynamic block with an unknown for_each makes the whole block list unknown, which
// terraform validate always hits since input variables are unknown there.
func TestValidateConfig_unknownBlocks(t *testing.T) {
	r := NewResource()
	typ := vt.ConfigType(t, r)

	for name, at := range typ.AttributeTypes {
		if _, ok := at.(tftypes.List); !ok {
			continue
		}
		if !at.(tftypes.List).ElementType.Is(tftypes.Object{}) {
			continue
		}
		t.Run(name, func(t *testing.T) {
			diags := vt.ValidateConfig(t, r, map[string]tftypes.Value{name: vt.Unknown(typ, name)})
			assert.False(t, diags.HasError(), "%v", diags)
		})
	}
}

func TestValidateConfig_unknownReferencedBlock(t *testing.T) {
	r := NewResource()
	typ := vt.ConfigType(t, r)
	backendType := vt.BlockType(typ, "backend")

	cases := map[string]struct {
		backendAttr string
		referenced  string
	}{
		"condition":   {backendAttr: "request_condition", referenced: "condition"},
		"healthcheck": {backendAttr: "healthcheck", referenced: "healthcheck"},
	}

	for name, tc := range cases {
		backends := vt.Block(typ, "backend", vt.Object(t, backendType, map[string]tftypes.Value{
			"name":         vt.String("origin"),
			tc.backendAttr: vt.String("missing"),
		}))

		t.Run(name+" unknown skips the reference check", func(t *testing.T) {
			diags := vt.ValidateConfig(t, r, map[string]tftypes.Value{
				"backend":     backends,
				tc.referenced: vt.Unknown(typ, tc.referenced),
			})
			assert.False(t, diags.HasError(), "%v", diags)
		})

		t.Run(name+" absent still fails the reference check", func(t *testing.T) {
			diags := vt.ValidateConfig(t, r, map[string]tftypes.Value{"backend": backends})
			assert.True(t, diags.HasError())
		})
	}
}
