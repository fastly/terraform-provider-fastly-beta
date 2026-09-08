package ngwafalertintegrations

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"

	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafalertintegration"
)

func TestFlattenAlerts(t *testing.T) {
	alerts := []ngwafalertintegration.RemoteAlert{
		{ID: "alert-b", Description: "beta"},
		{ID: "alert-a", Description: "alpha"},
	}

	listValue, ids, diags := FlattenAlerts(alerts)
	require.False(t, diags.HasError(), diags)
	require.Equal(t, []string{"alert-a", "alert-b"}, ids)
	require.Len(t, listValue.Elements(), 2)

	first, ok := listValue.Elements()[0].(types.Object)
	require.True(t, ok)
	firstID, ok := first.Attributes()["id"].(types.String)
	require.True(t, ok)
	require.Equal(t, "alert-a", firstID.ValueString())
}

func TestFlattenAlertsEmpty(t *testing.T) {
	listValue, ids, diags := FlattenAlerts(nil)
	require.False(t, diags.HasError(), diags)
	require.Empty(t, ids)
	require.Empty(t, listValue.Elements())
}
