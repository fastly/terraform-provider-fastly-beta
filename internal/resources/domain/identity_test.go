package domain

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/stretchr/testify/assert"
)

func requiresReplace(t *testing.T, attr schema.Attribute) bool {
	t.Helper()
	s, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("attribute is %T, want schema.StringAttribute", attr)
	}
	want := stringplanmodifier.RequiresReplace().Description(t.Context())
	for _, m := range s.PlanModifiers {
		if m.Description(t.Context()) == want {
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
