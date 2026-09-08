package tlsprivatekey

import (
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"

	"github.com/fastly/go-fastly/v17/fastly"
)

func TestBuildCreateInput(t *testing.T) {
	plan := Model{
		Name: types.StringValue("my-key"),
		PrivateKey: types.ObjectValueMust(privateKeyAttributeTypes, map[string]attr.Value{
			"pem": types.StringValue("-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----"),
		}),
	}

	input := buildCreateInput(plan)
	assert.Equal(t, "my-key", input.Name)
	assert.Equal(t, "-----BEGIN PRIVATE KEY-----\n...\n-----END PRIVATE KEY-----", input.Key)
}

func TestModel_PEM_nullObject(t *testing.T) {
	plan := Model{PrivateKey: types.ObjectNull(privateKeyAttributeTypes)}
	assert.True(t, plan.PEM().IsNull())
}

func TestFlattenToModel_full(t *testing.T) {
	createdAt := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	key := &fastly.PrivateKey{
		ID:            "key-1",
		Name:          "my-key",
		KeyLength:     2048,
		KeyType:       "RSA",
		PublicKeySHA1: "abc123",
		Replace:       false,
		CreatedAt:     &createdAt,
	}

	privateKey := types.ObjectValueMust(privateKeyAttributeTypes, map[string]attr.Value{
		"pem": types.StringValue("pem-contents"),
	})
	m := flattenToModel(key, privateKey)

	assert.Equal(t, "key-1", m.ID.ValueString())
	assert.Equal(t, "my-key", m.Name.ValueString())
	assert.Equal(t, int64(2048), m.KeyLength.ValueInt64())
	assert.Equal(t, "RSA", m.KeyType.ValueString())
	assert.Equal(t, "abc123", m.PublicKeySHA1.ValueString())
	assert.False(t, m.Replace.ValueBool())
	assert.Equal(t, createdAt.Format(time.RFC3339), m.CreatedAt.ValueString())
	assert.Equal(t, "pem-contents", m.PEM().ValueString())
}

// No CreatedAt returned: created_at must flatten to null, not "".
func TestFlattenToModel_noCreatedAt(t *testing.T) {
	key := &fastly.PrivateKey{ID: "key-1", Name: "my-key"}
	m := flattenToModel(key, types.ObjectNull(privateKeyAttributeTypes))
	assert.True(t, m.CreatedAt.IsNull())
}
