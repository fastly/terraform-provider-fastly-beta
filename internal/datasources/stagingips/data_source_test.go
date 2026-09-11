package stagingips

import (
	"testing"

	"github.com/fastly/go-fastly/v17/fastly"
)

func TestFlattenStagingIPs(t *testing.T) {
	name := "www.example.com"
	stagingIP := "203.0.113.1"

	setVal, ids, diags := flattenStagingIPs([]*fastly.Domain{
		{
			Name:      &name,
			StagingIP: &stagingIP,
		},
	})
	if diags.HasError() {
		t.Fatalf("flattenStagingIPs returned diagnostics: %v", diags)
	}

	if len(ids) != 1 {
		t.Fatalf("ids length = %d, want 1", len(ids))
	}

	if setVal.IsNull() || setVal.IsUnknown() {
		t.Fatal("expected non-null, known set")
	}

	if len(setVal.Elements()) != 1 {
		t.Fatalf("set elements length = %d, want 1", len(setVal.Elements()))
	}
}

func TestFlattenStagingIPsEmpty(t *testing.T) {
	setVal, ids, diags := flattenStagingIPs(nil)
	if diags.HasError() {
		t.Fatalf("flattenStagingIPs returned diagnostics: %v", diags)
	}

	if len(ids) != 0 {
		t.Fatalf("ids length = %d, want 0", len(ids))
	}

	if len(setVal.Elements()) != 0 {
		t.Fatalf("set elements length = %d, want 0", len(setVal.Elements()))
	}
}
