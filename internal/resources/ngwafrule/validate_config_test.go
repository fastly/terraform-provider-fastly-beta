package ngwafrule_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"

	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafrequestrule"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafsignalrule"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspaceratelimitrule"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacerequestrule"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacesignalrule"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacetemplatedsignalrule"
	vt "github.com/fastly/terraform-provider-fastly-beta/internal/validation/validationtest"
)

func TestValidateConfig_unknownBlocks(t *testing.T) {
	resources := map[string]func() resource.Resource{
		"request_rule":                    ngwafrequestrule.NewResource,
		"signal_rule":                     ngwafsignalrule.NewResource,
		"workspace_rate_limit_rule":       ngwafworkspaceratelimitrule.NewResource,
		"workspace_request_rule":          ngwafworkspacerequestrule.NewResource,
		"workspace_signal_rule":           ngwafworkspacesignalrule.NewResource,
		"workspace_templated_signal_rule": ngwafworkspacetemplatedsignalrule.NewResource,
	}

	for rname, newResource := range resources {
		r := newResource()
		typ := vt.ConfigType(t, r)
		groupType := vt.BlockType(typ, "group_condition")

		t.Run(rname+"/no conditions still fails", func(t *testing.T) {
			assert.True(t, vt.ValidateConfig(t, r, nil).HasError())
		})

		for name, at := range typ.AttributeTypes {
			if l, ok := at.(tftypes.List); !ok || !l.ElementType.Is(tftypes.Object{}) {
				continue
			}
			t.Run(rname+"/"+name, func(t *testing.T) {
				diags := vt.ValidateConfig(t, r, map[string]tftypes.Value{
					"condition": vt.Unknown(typ, "condition"),
					name:        vt.Unknown(typ, name),
				})
				assert.False(t, diags.HasError(), "%v", diags)
			})
		}

		for _, nested := range []string{"condition", "multival_condition"} {
			t.Run(rname+"/group_condition."+nested, func(t *testing.T) {
				diags := vt.ValidateConfig(t, r, map[string]tftypes.Value{
					"group_condition": vt.Block(typ, "group_condition", vt.Object(t, groupType, map[string]tftypes.Value{
						nested: vt.Unknown(groupType, nested),
					})),
				})
				assert.False(t, diags.HasError(), "%v", diags)
			})
		}
	}
}

func TestValidateConfig_unknownClientIdentifiers(t *testing.T) {
	r := ngwafworkspaceratelimitrule.NewResource()
	typ := vt.ConfigType(t, r)
	rateLimitType := vt.BlockType(typ, "rate_limit")

	diags := vt.ValidateConfig(t, r, map[string]tftypes.Value{
		"condition": vt.Unknown(typ, "condition"),
		"rate_limit": vt.Block(typ, "rate_limit", vt.Object(t, rateLimitType, map[string]tftypes.Value{
			"client_identifiers": vt.Unknown(rateLimitType, "client_identifiers"),
		})),
	})
	assert.False(t, diags.HasError(), "%v", diags)
}
