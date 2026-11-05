package backend

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
)

// requiresReplace runs the attribute's plan modifiers against an in-place
// change of an existing resource and reports whether any forces replacement.
func requiresReplace(t *testing.T, attr schema.Attribute) bool {
	t.Helper()
	s, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("attribute is %T, want schema.StringAttribute", attr)
	}

	existing := tftypes.NewValue(tftypes.Object{}, map[string]tftypes.Value{})
	req := planmodifier.StringRequest{
		State:       tfsdk.State{Raw: existing},
		Plan:        tfsdk.Plan{Raw: existing},
		StateValue:  types.StringValue("old"),
		PlanValue:   types.StringValue("new"),
		ConfigValue: types.StringValue("new"),
	}
	for _, m := range s.PlanModifiers {
		resp := &planmodifier.StringResponse{PlanValue: req.PlanValue}
		m.PlanModifyString(t.Context(), req, resp)
		if resp.RequiresReplace {
			return true
		}
	}
	return false
}

// service_id + name locate the object in the API, so an in-place update would
// address the planned identity instead of the one in state.
func TestResourceIdentityRequiresReplace(t *testing.T) {
	attrs := ResourceAttributes()
	assert.True(t, requiresReplace(t, attrs["service_id"]), "service_id must force replacement")
	assert.True(t, requiresReplace(t, attrs["name"]), "name must force replacement")
}

func TestNestedNameStaysMutable(t *testing.T) {
	name := NestedBlockSchema().NestedObject.Attributes["name"]
	assert.False(t, requiresReplace(t, name), "nested name must stay mutable for the reconciler")
}
