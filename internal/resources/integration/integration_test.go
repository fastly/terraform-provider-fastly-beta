package integration

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fastly/go-fastly/v17/fastly"
)

func mustMap(t *testing.T, kv ...string) types.Map {
	t.Helper()
	elements := map[string]string{}
	for i := 0; i < len(kv); i += 2 {
		elements[kv[i]] = kv[i+1]
	}
	m, diags := types.MapValueFrom(context.Background(), types.StringType, elements)
	require.False(t, diags.HasError())
	return m
}

func TestBuildCreateInput(t *testing.T) {
	plan := Model{
		Name:        types.StringValue("my integration"),
		Description: types.StringValue("my description"),
		Type:        types.StringValue(fastly.IntegrationTypeDatadog),
		Config:      mustMap(t, "apikey", "abc123", "site", "datadoghq.eu"),
	}

	input, diags := BuildCreateInput(context.Background(), plan)
	require.False(t, diags.HasError())

	assert.Equal(t, "my integration", fastly.ToValue(input.Name))
	assert.Equal(t, "my description", fastly.ToValue(input.Description))
	assert.Equal(t, fastly.IntegrationTypeDatadog, fastly.ToValue(input.Type))
	assert.Equal(t, map[string]string{"apikey": "abc123", "site": "datadoghq.eu"}, input.Config)
}

func TestBuildCreateInput_omitsUnsetDescription(t *testing.T) {
	plan := Model{
		Name:   types.StringValue("my integration"),
		Type:   types.StringValue(typeWebhook),
		Config: mustMap(t, "url", "https://example.com/webhook"),
	}

	input, diags := BuildCreateInput(context.Background(), plan)
	require.False(t, diags.HasError())

	assert.Nil(t, input.Description)
}

func TestBuildUpdateInput(t *testing.T) {
	plan := Model{
		Name:        types.StringValue("renamed"),
		Description: types.StringValue("new description"),
		Type:        types.StringValue(typeSlack),
		Config:      mustMap(t, "url", "https://hooks.slack.com/x"),
	}

	input, diags := BuildUpdateInput(context.Background(), "int-id", plan)
	require.False(t, diags.HasError())

	assert.Equal(t, "int-id", input.ID)
	assert.Equal(t, "renamed", fastly.ToValue(input.Name))
	assert.Equal(t, "new description", fastly.ToValue(input.Description))
	assert.Equal(t, typeSlack, fastly.ToValue(input.Type))
	assert.Equal(t, map[string]string{"url": "https://hooks.slack.com/x"}, input.Config)
}

func TestFlattenToModel(t *testing.T) {
	prior := Model{
		Description: types.StringValue("kept if API omits it"),
		Config:      mustMap(t, "token", "secret-not-echoed", "baseurl", "https://old.example.com"),
	}

	i := &fastly.Integration{
		ID:   new("int-id"),
		Name: new("my integration"),
		Type: new(fastly.IntegrationTypeJiraIssue),
		Config: map[string]string{
			"baseurl": "https://new.example.com",
		},
	}

	m, diags := FlattenToModel(context.Background(), i, prior)
	require.False(t, diags.HasError())

	assert.Equal(t, "int-id", m.ID.ValueString())
	assert.Equal(t, "my integration", m.Name.ValueString())
	assert.Equal(t, fastly.IntegrationTypeJiraIssue, m.Type.ValueString())
	// Description isn't returned by the API here, so the prior value survives.
	assert.Equal(t, "kept if API omits it", m.Description.ValueString())

	var config map[string]string
	diags = m.Config.ElementsAs(context.Background(), &config, false)
	require.False(t, diags.HasError())
	assert.Equal(t, map[string]string{
		"token":   "secret-not-echoed",
		"baseurl": "https://new.example.com",
	}, config)
}

func TestFlattenToModel_descriptionReturnedByAPIOverridesPrior(t *testing.T) {
	prior := Model{Description: types.StringValue("stale")}

	i := &fastly.Integration{
		ID:          new("int-id"),
		Name:        new("my integration"),
		Type:        new(fastly.IntegrationTypeOpsGenie),
		Description: new("fresh from API"),
	}

	m, diags := FlattenToModel(context.Background(), i, prior)
	require.False(t, diags.HasError())
	assert.Equal(t, "fresh from API", m.Description.ValueString())
}

func TestFlattenToModel_nilRemoteConfigKeepsPrior(t *testing.T) {
	prior := Model{Config: mustMap(t, "apikey", "still-here")}

	i := &fastly.Integration{
		ID:   new("int-id"),
		Name: new("my integration"),
		Type: new(fastly.IntegrationTypeJSM),
	}

	m, diags := FlattenToModel(context.Background(), i, prior)
	require.False(t, diags.HasError())
	assert.True(t, m.Config.Equal(prior.Config))
}

func TestWarnIfMailingListUnconfirmed(t *testing.T) {
	cases := []struct {
		name     string
		i        *fastly.Integration
		wantWarn bool
	}{
		{
			name:     "mailinglist unconfirmed warns",
			i:        &fastly.Integration{Type: new(typeMailingList), Status: new("pending")},
			wantWarn: true,
		},
		{
			name:     "mailinglist confirmed does not warn",
			i:        &fastly.Integration{Type: new(typeMailingList), Status: new("confirmed")},
			wantWarn: false,
		},
		{
			name:     "non-mailinglist type does not warn",
			i:        &fastly.Integration{Type: new(fastly.IntegrationTypeDatadog), Status: new("pending")},
			wantWarn: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var diags diag.Diagnostics
			warnIfMailingListUnconfirmed(&diags, c.i)
			assert.Equal(t, c.wantWarn, diags.WarningsCount() > 0)
		})
	}
}
