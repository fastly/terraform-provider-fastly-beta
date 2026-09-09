package tsigkey

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"

	"github.com/fastly/go-fastly/v17/fastly/dns/v1/tsigkeys"
)

func secretModel(value string) types.Object {
	return types.ObjectValueMust(secretAttributeTypes, map[string]attr.Value{
		"value": types.StringValue(value),
	})
}

func TestBuildCreateInput_minimal(t *testing.T) {
	plan := Model{
		Name:      types.StringValue("tsig-key"),
		Algorithm: types.StringValue("hmac-sha256"),
		Secret:    secretModel("c2VjcmV0"),
	}

	input := BuildCreateInput(plan)
	assert.Equal(t, "tsig-key", *input.Name)
	assert.Equal(t, "hmac-sha256", *input.Algorithm)
	assert.Equal(t, "c2VjcmV0", *input.Secret)
	assert.Nil(t, input.Description)
}

func TestBuildCreateInput_withDescription(t *testing.T) {
	plan := Model{
		Name:        types.StringValue("tsig-key"),
		Algorithm:   types.StringValue("hmac-sha256"),
		Secret:      secretModel("c2VjcmV0"),
		Description: types.StringValue("a key"),
	}

	input := BuildCreateInput(plan)
	assert.Equal(t, "a key", *input.Description)
}

func TestBuildUpdateInput_unchangedFieldsNotSent(t *testing.T) {
	state := Model{
		Name:      types.StringValue("tsig-key"),
		Algorithm: types.StringValue("hmac-sha256"),
		Secret:    secretModel("c2VjcmV0"),
	}
	plan := state

	input := BuildUpdateInput("key-id", plan, state)
	assert.Equal(t, "key-id", *input.TSIGKeyID)
	assert.Nil(t, input.Name)
	assert.Nil(t, input.Algorithm)
	assert.Nil(t, input.Secret)
	assert.Nil(t, input.Description)
}

func TestBuildUpdateInput_changedFieldsSent(t *testing.T) {
	state := Model{
		Name:      types.StringValue("tsig-key"),
		Algorithm: types.StringValue("hmac-sha256"),
		Secret:    secretModel("c2VjcmV0"),
	}
	plan := Model{
		Name:      types.StringValue("tsig-key-renamed"),
		Algorithm: types.StringValue("hmac-sha512"),
		Secret:    secretModel("bmV3c2VjcmV0"),
	}

	input := BuildUpdateInput("key-id", plan, state)
	assert.Equal(t, "tsig-key-renamed", *input.Name)
	assert.Equal(t, "hmac-sha512", *input.Algorithm)
	assert.Equal(t, "bmV3c2VjcmV0", *input.Secret)
}

func TestBuildUpdateInput_descriptionChangedSent(t *testing.T) {
	state := Model{Description: types.StringValue("old")}
	plan := Model{Description: types.StringValue("new")}

	input := BuildUpdateInput("key-id", plan, state)
	assert.Equal(t, `"new"`, mustMarshalNullable(t, input.Description))
}

// Removing description sends "", not null; ReconcileDescription handles the read side.
func TestBuildUpdateInput_descriptionClearedSendsEmptyString(t *testing.T) {
	state := Model{Description: types.StringValue("old")}
	plan := Model{Description: types.StringNull()}

	input := BuildUpdateInput("key-id", plan, state)
	assert.Equal(t, `""`, mustMarshalNullable(t, input.Description))
}

func TestFlattenToModel(t *testing.T) {
	key := &tsigkeys.TSIGKey{
		ID:        new("key-id"),
		Name:      new("tsig-key"),
		Algorithm: new("hmac-sha256"),
	}
	secret := secretModel("c2VjcmV0")

	m := FlattenToModel(key, secret)
	assert.Equal(t, types.StringValue("key-id"), m.ID)
	assert.Equal(t, types.StringValue("tsig-key"), m.Name)
	assert.Equal(t, types.StringValue("hmac-sha256"), m.Algorithm)
	assert.Equal(t, types.StringNull(), m.Description)
	assert.Equal(t, secret, m.Secret)
}

func TestSecretValue(t *testing.T) {
	m := Model{Secret: secretModel("c2VjcmV0")}
	assert.Equal(t, types.StringValue("c2VjcmV0"), m.SecretValue())

	empty := Model{Secret: types.ObjectNull(secretAttributeTypes)}
	assert.Equal(t, types.StringNull(), empty.SecretValue())
}

func TestNameValidator(t *testing.T) {
	attrs := ResourceAttributes()
	nameAttr, ok := attrs["name"].(schema.StringAttribute)
	assert.True(t, ok)

	cases := []struct {
		value string
		valid bool
	}{
		{"tsig-key", true},
		{"", false},
		{"has space", false},
	}

	for _, c := range cases {
		req := validator.StringRequest{ConfigValue: types.StringValue(c.value)}
		resp := &validator.StringResponse{}
		for _, v := range nameAttr.Validators {
			v.ValidateString(context.Background(), req, resp)
		}
		assert.Equal(t, c.valid, !resp.Diagnostics.HasError(), "value %q", c.value)
	}
}

func TestAlgorithmValidator(t *testing.T) {
	attrs := ResourceAttributes()
	algorithmAttr, ok := attrs["algorithm"].(schema.StringAttribute)
	assert.True(t, ok)

	cases := []struct {
		value string
		valid bool
	}{
		{"hmac-sha224", true},
		{"hmac-sha256", true},
		{"hmac-sha384", true},
		{"hmac-sha512", true},
		{"hmac-md5", false},
		{"", false},
	}

	for _, c := range cases {
		req := validator.StringRequest{ConfigValue: types.StringValue(c.value)}
		resp := &validator.StringResponse{}
		for _, v := range algorithmAttr.Validators {
			v.ValidateString(context.Background(), req, resp)
		}
		assert.Equal(t, c.valid, !resp.Diagnostics.HasError(), "value %q", c.value)
	}
}

func TestBase64Validator(t *testing.T) {
	v := base64Validator{}

	cases := []struct {
		value string
		valid bool
	}{
		{"c2VjcmV0", true},
		{"", false},
		{"not base64!!", false},
	}

	for _, c := range cases {
		req := validator.StringRequest{ConfigValue: types.StringValue(c.value)}
		resp := &validator.StringResponse{}
		v.ValidateString(context.Background(), req, resp)
		assert.Equal(t, c.valid, !resp.Diagnostics.HasError(), "value %q", c.value)
	}
}

func mustMarshalNullable(t *testing.T, n interface{ MarshalJSON() ([]byte, error) }) string {
	t.Helper()
	b, err := n.MarshalJSON()
	assert.NoError(t, err)
	return string(b)
}

func TestReconcileDescription_emptyCollapsedToNullIsRestored(t *testing.T) {
	got := ReconcileDescription(types.StringNull(), types.StringValue(""))
	assert.Equal(t, types.StringValue(""), got)
}

func TestReconcileDescription_genuineDriftIsNotMasked(t *testing.T) {
	got := ReconcileDescription(types.StringNull(), types.StringValue("a real description"))
	assert.Equal(t, types.StringNull(), got)
}

func TestReconcileDescription_nonNullReturnedIsUnchanged(t *testing.T) {
	got := ReconcileDescription(types.StringValue("from api"), types.StringValue(""))
	assert.Equal(t, types.StringValue("from api"), got)
}
