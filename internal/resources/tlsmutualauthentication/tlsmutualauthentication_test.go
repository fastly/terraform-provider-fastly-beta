package tlsmutualauthentication

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"

	"github.com/fastly/go-fastly/v17/fastly"
)

func TestBuildCreateInput_minimal(t *testing.T) {
	plan := Model{
		CertBundle: types.StringValue("bundle"),
		Enforced:   types.BoolNull(),
		Name:       types.StringNull(),
	}

	input := buildCreateInput(plan)
	assert.Equal(t, "bundle", input.CertBundle)
	assert.False(t, input.Enforced)
	assert.Empty(t, input.Name)
}

func TestBuildCreateInput_withEnforcedAndName(t *testing.T) {
	plan := Model{
		CertBundle: types.StringValue("bundle"),
		Enforced:   types.BoolValue(true),
		Name:       types.StringValue("my-mtls"),
	}

	input := buildCreateInput(plan)
	assert.True(t, input.Enforced)
	assert.Equal(t, "my-mtls", input.Name)
}

func TestBuildUpdateInput_alwaysSendsEnforced(t *testing.T) {
	plan := Model{
		CertBundle: types.StringValue("bundle"),
		Enforced:   types.BoolValue(false),
		Name:       types.StringValue("existing"),
	}
	state := plan

	input := buildUpdateInput("mtls-1", plan, state)
	assert.Equal(t, "mtls-1", input.ID)
	assert.Equal(t, "bundle", input.CertBundle)
	assert.False(t, input.Enforced)
	assert.Empty(t, input.Name)
}

func TestBuildUpdateInput_nameOnlySentWhenChanged(t *testing.T) {
	state := Model{Name: types.StringValue("old")}
	plan := Model{CertBundle: types.StringValue("bundle"), Name: types.StringValue("new")}

	input := buildUpdateInput("mtls-1", plan, state)
	assert.Equal(t, "new", input.Name)
}

func TestFlattenToModel(t *testing.T) {
	createdAt := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	updatedAt := time.Date(2024, 6, 7, 8, 9, 10, 0, time.UTC)
	mtls := &fastly.TLSMutualAuthentication{
		ID:        "mtls-1",
		Enforced:  true,
		Name:      "my-mtls",
		CreatedAt: &createdAt,
		UpdatedAt: &updatedAt,
		Activations: []*fastly.TLSActivation{
			{ID: "activation-2"},
			{ID: "activation-1"},
		},
	}

	m := flattenToModel(mtls)
	assert.Equal(t, "mtls-1", m.ID.ValueString())
	assert.True(t, m.Enforced.ValueBool())
	assert.Equal(t, "my-mtls", m.Name.ValueString())
	assert.Equal(t, createdAt.Format(time.RFC3339), m.CreatedAt.ValueString())
	assert.Equal(t, updatedAt.Format(time.RFC3339), m.UpdatedAt.ValueString())

	var activations []string
	m.TLSActivations.ElementsAs(context.Background(), &activations, false)
	assert.Equal(t, []string{"activation-1", "activation-2"}, activations)
}

func TestFlattenToModel_noActivations(t *testing.T) {
	mtls := &fastly.TLSMutualAuthentication{ID: "mtls-1"}

	m := flattenToModel(mtls)
	assert.True(t, m.TLSActivations.IsNull())
	assert.True(t, m.CreatedAt.IsNull())
	assert.True(t, m.UpdatedAt.IsNull())
}

func TestSetToStringSlice(t *testing.T) {
	ctx := context.Background()
	var diags diag.Diagnostics

	set, d := types.SetValueFrom(ctx, types.StringType, []string{"a", "b"})
	diags.Append(d...)

	got := setToStringSlice(ctx, set, &diags)
	assert.False(t, diags.HasError())
	assert.ElementsMatch(t, []string{"a", "b"}, got)
}

func TestSetToStringSlice_nullOrUnknown(t *testing.T) {
	ctx := context.Background()
	var diags diag.Diagnostics

	assert.Equal(t, []string{}, setToStringSlice(ctx, types.SetNull(types.StringType), &diags))
	assert.Equal(t, []string{}, setToStringSlice(ctx, types.SetUnknown(types.StringType), &diags))
	assert.False(t, diags.HasError())
}
