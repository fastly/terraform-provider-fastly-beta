package tlscertificate

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly"
)

func flattenToModel(ctx context.Context, c *fastly.CustomTLSCertificate) (Model, diag.Diagnostics) {
	domains := make([]string, len(c.Domains))
	for i, d := range c.Domains {
		domains[i] = d.ID
	}
	domainsSet, diags := types.SetValueFrom(ctx, types.StringType, domains)

	m := Model{
		ID:                 types.StringValue(c.ID),
		Domains:            domainsSet,
		IssuedTo:           types.StringValue(c.IssuedTo),
		Issuer:             types.StringValue(c.Issuer),
		Name:               types.StringValue(c.Name),
		Replace:            types.BoolValue(c.Replace),
		SerialNumber:       types.StringValue(c.SerialNumber),
		SignatureAlgorithm: types.StringValue(c.SignatureAlgorithm),
		CreatedAt:          types.StringNull(),
		UpdatedAt:          types.StringNull(),
	}

	if c.CreatedAt != nil {
		m.CreatedAt = types.StringValue(c.CreatedAt.Format(time.RFC3339))
	}
	if c.UpdatedAt != nil {
		m.UpdatedAt = types.StringValue(c.UpdatedAt.Format(time.RFC3339))
	}

	return m, diags
}
