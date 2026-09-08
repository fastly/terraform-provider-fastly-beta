package servicecdnauto

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestPartialCreateState(t *testing.T) {
	plan := &Model{
		Name:         types.StringValue("test-service"),
		Comment:      types.StringValue("a comment"),
		ForceDestroy: types.BoolValue(true),
		Reuse:        types.BoolValue(false),
	}

	got := partialCreateState("SID123", 1, plan)

	if got.ID != types.StringValue("SID123") {
		t.Errorf("ID = %v, want %v", got.ID, types.StringValue("SID123"))
	}
	if got.Name != plan.Name {
		t.Errorf("Name = %v, want %v", got.Name, plan.Name)
	}
	if got.Comment != plan.Comment {
		t.Errorf("Comment = %v, want %v", got.Comment, plan.Comment)
	}
	if got.ForceDestroy != plan.ForceDestroy {
		t.Errorf("ForceDestroy = %v, want %v", got.ForceDestroy, plan.ForceDestroy)
	}
	if got.Reuse != plan.Reuse {
		t.Errorf("Reuse = %v, want %v", got.Reuse, plan.Reuse)
	}
	if got.ManagedVersion != types.Int64Value(1) {
		t.Errorf("ManagedVersion = %v, want %v", got.ManagedVersion, types.Int64Value(1))
	}
	if !got.ActiveVersion.IsNull() {
		t.Errorf("ActiveVersion = %v, want null", got.ActiveVersion)
	}
	if len(got.Backend) != 0 || len(got.Domain) != 0 {
		t.Errorf("expected nested blocks to be empty, got Backend=%v Domain=%v", got.Backend, got.Domain)
	}
}

// TestPartialCreateStateMarshalsToSchema exercises the same state.Set call recordOrphanSafeState
// relies on at runtime, against a State built from the resource's real schema. resp.State.Set
// only adds a diagnostic on a schema/model mismatch - it doesn't panic or otherwise fail loud -
// so without this, such a mismatch would silently defeat the orphan fix (Create's error
// diagnostics still discard the returned state) and only surface, if at all, in the acceptance
// test.
func TestPartialCreateStateMarshalsToSchema(t *testing.T) {
	ctx := context.Background()
	r := NewResource()

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("Schema() returned diagnostics: %v", schemaResp.Diagnostics)
	}

	state := tfsdk.State{
		Raw:    tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), nil),
		Schema: schemaResp.Schema,
	}

	plan := &Model{
		Name:         types.StringValue("test-service"),
		Comment:      types.StringValue("a comment"),
		ForceDestroy: types.BoolValue(true),
		Reuse:        types.BoolValue(false),
	}

	diags := state.Set(ctx, partialCreateState("SID123", 1, plan))
	if diags.HasError() {
		t.Fatalf("state.Set(partialCreateState(...)) returned diagnostics: %v", diags)
	}

	var gotID types.String
	if diags := state.GetAttribute(ctx, path.Root("id"), &gotID); diags.HasError() {
		t.Fatalf("GetAttribute(id) returned diagnostics: %v", diags)
	}
	if gotID.ValueString() != "SID123" {
		t.Errorf("id = %v, want SID123", gotID)
	}

	var gotManagedVersion types.Int64
	if diags := state.GetAttribute(ctx, path.Root("managed_version"), &gotManagedVersion); diags.HasError() {
		t.Fatalf("GetAttribute(managed_version) returned diagnostics: %v", diags)
	}
	if gotManagedVersion.ValueInt64() != 1 {
		t.Errorf("managed_version = %v, want 1", gotManagedVersion)
	}

	var gotActiveVersion types.Int64
	if diags := state.GetAttribute(ctx, path.Root("active_version"), &gotActiveVersion); diags.HasError() {
		t.Fatalf("GetAttribute(active_version) returned diagnostics: %v", diags)
	}
	if !gotActiveVersion.IsNull() {
		t.Errorf("active_version = %v, want null", gotActiveVersion)
	}
}
