package tlsprivatekey

import (
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly"
)

// flattenToModel maps the API response onto Model. privateKey is carried over from
// the caller (plan or prior state) rather than the API response, since the API
// never returns key material.
func flattenToModel(k *fastly.PrivateKey, privateKey types.Object) Model {
	m := Model{
		ID:            types.StringValue(k.ID),
		Name:          types.StringValue(k.Name),
		PrivateKey:    privateKey,
		CreatedAt:     types.StringNull(),
		KeyLength:     types.Int64Value(int64(k.KeyLength)),
		KeyType:       types.StringValue(k.KeyType),
		PublicKeySHA1: types.StringValue(k.PublicKeySHA1),
		Replace:       types.BoolValue(k.Replace),
	}

	if k.CreatedAt != nil {
		m.CreatedAt = types.StringValue(k.CreatedAt.Format(time.RFC3339))
	}

	return m
}
