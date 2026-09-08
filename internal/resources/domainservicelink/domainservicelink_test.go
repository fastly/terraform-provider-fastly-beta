package domainservicelink

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"

	"github.com/fastly/go-fastly/v17/fastly/domainmanagement/v1/domains"
)

func TestFlattenToModel(t *testing.T) {
	serviceID := "svc-123"
	d := &domains.Data{
		DomainID:  "domain-id",
		ServiceID: &serviceID,
	}

	m := FlattenToModel(d)
	assert.Equal(t, types.StringValue("domain-id"), m.ID)
	assert.Equal(t, types.StringValue("domain-id"), m.DomainID)
	assert.Equal(t, types.StringValue("svc-123"), m.ServiceID)
}

func TestFlattenToModel_noServiceID(t *testing.T) {
	d := &domains.Data{
		DomainID: "domain-id",
	}

	m := FlattenToModel(d)
	assert.Equal(t, types.StringNull(), m.ServiceID)
}
