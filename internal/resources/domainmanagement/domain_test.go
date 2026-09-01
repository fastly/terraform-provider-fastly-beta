package domainmanagement

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"

	"github.com/fastly/go-fastly/v17/fastly/domainmanagement/v1/domains"
)

func TestBuildCreateInput_minimal(t *testing.T) {
	plan := Model{
		FQDN:        types.StringValue("www.example.com"),
		Description: types.StringNull(),
		ServiceID:   types.StringNull(),
	}

	input := BuildCreateInput(plan)
	assert.Equal(t, "www.example.com", *input.FQDN)
	assert.Nil(t, input.Description)
	assert.Nil(t, input.ServiceID)
}

func TestBuildCreateInput_full(t *testing.T) {
	plan := Model{
		FQDN:        types.StringValue("www.example.com"),
		Description: types.StringValue("my domain"),
		ServiceID:   types.StringValue("svc-123"),
	}

	input := BuildCreateInput(plan)
	assert.Equal(t, "my domain", *input.Description)
	assert.Equal(t, "svc-123", *input.ServiceID)
}

func TestBuildUpdateInput_omittedFieldsSentAsNull(t *testing.T) {
	plan := Model{}

	input := BuildUpdateInput("domain-id", plan)
	assert.Equal(t, "domain-id", *input.DomainID)
	assert.Nil(t, input.Description)
	assert.Nil(t, input.ServiceID)
}

func TestBuildUpdateInput_setFieldsSent(t *testing.T) {
	plan := Model{
		Description: types.StringValue("updated"),
		ServiceID:   types.StringValue("svc-456"),
	}

	input := BuildUpdateInput("domain-id", plan)
	assert.Equal(t, "updated", *input.Description)
	assert.Equal(t, "svc-456", *input.ServiceID)
}

func TestFlattenToModel_noServiceID(t *testing.T) {
	d := &domains.Data{
		DomainID:    "domain-id",
		FQDN:        "www.example.com",
		Description: "",
	}

	m := FlattenToModel(d)
	assert.Equal(t, types.StringValue("domain-id"), m.ID)
	assert.Equal(t, types.StringValue("www.example.com"), m.FQDN)
	assert.Equal(t, types.StringValue(""), m.Description)
	assert.Equal(t, types.StringNull(), m.ServiceID)
}

func TestFlattenToModel_withServiceID(t *testing.T) {
	serviceID := "svc-789"
	d := &domains.Data{
		DomainID:    "domain-id",
		FQDN:        "www.example.com",
		Description: "a domain",
		ServiceID:   &serviceID,
	}

	m := FlattenToModel(d)
	assert.Equal(t, types.StringValue("a domain"), m.Description)
	assert.Equal(t, types.StringValue("svc-789"), m.ServiceID)
}

// Guards the API's never-null description quirk (see ReconcileDescription).
func TestReconcileDescription_emptyCollapsedToNullWhenUnconfigured(t *testing.T) {
	got := ReconcileDescription(types.StringValue(""), types.StringNull())
	assert.Equal(t, types.StringNull(), got)
}

// An explicit "" is left alone, not collapsed to null.
func TestReconcileDescription_explicitEmptyIsKept(t *testing.T) {
	got := ReconcileDescription(types.StringValue(""), types.StringValue(""))
	assert.Equal(t, types.StringValue(""), got)
}

// Real drift (description cleared out-of-band) must still surface.
func TestReconcileDescription_genuineDriftIsNotMasked(t *testing.T) {
	got := ReconcileDescription(types.StringValue(""), types.StringValue("was set"))
	assert.Equal(t, types.StringValue(""), got)
}

func TestReconcileDescription_nonEmptyReturnedIsUnchanged(t *testing.T) {
	got := ReconcileDescription(types.StringValue("from api"), types.StringNull())
	assert.Equal(t, types.StringValue("from api"), got)
}
