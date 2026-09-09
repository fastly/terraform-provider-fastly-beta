package objectstorageaccesskey

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly/objectstorage/accesskeys"
)

// flattenToModel builds the resource state from an API response. The API only
// returns secret_key at creation time, so priorSecretKey is carried forward
// on every later read.
func flattenToModel(ctx context.Context, key *accesskeys.AccessKey, priorSecretKey types.String) (Model, diag.Diagnostics) {
	var diags diag.Diagnostics

	buckets := types.ListNull(types.StringType)
	if len(key.Buckets) > 0 {
		var d diag.Diagnostics
		buckets, d = types.ListValueFrom(ctx, types.StringType, key.Buckets)
		diags.Append(d...)
	}

	secretKey := priorSecretKey
	if key.SecretKey != "" {
		secretKey = types.StringValue(key.SecretKey)
	}

	return Model{
		ID:             types.StringValue(key.AccessKeyID),
		AccessKeyID:    types.StringValue(key.AccessKeyID),
		Authentication: NewAuthenticationObject(secretKey),
		Description:    types.StringValue(key.Description),
		Permission:     types.StringValue(key.Permission),
		Buckets:        buckets,
	}, diags
}
