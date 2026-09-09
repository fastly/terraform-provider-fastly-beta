package objectstorageaccesskey

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"

	"github.com/fastly/go-fastly/v17/fastly/objectstorage/accesskeys"
)

func buildCreateInput(ctx context.Context, plan Model) (*accesskeys.CreateInput, diag.Diagnostics) {
	var diags diag.Diagnostics

	input := &accesskeys.CreateInput{
		Description: new(plan.Description.ValueString()),
		Permission:  new(plan.Permission.ValueString()),
	}

	if !plan.Buckets.IsNull() {
		var buckets []string
		diags.Append(plan.Buckets.ElementsAs(ctx, &buckets, false)...)
		if diags.HasError() {
			return nil, diags
		}
		input.Buckets = &buckets
	}

	return input, diags
}
