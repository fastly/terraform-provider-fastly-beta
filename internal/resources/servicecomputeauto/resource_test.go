package servicecomputeauto

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
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
