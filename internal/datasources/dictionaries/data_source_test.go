package dictionaries

import (
	"testing"

	"github.com/fastly/go-fastly/v17/fastly"
)

func TestFlattenDictionaries(t *testing.T) {
	id := "dictionary-id"
	name := "my_dictionary"
	writeOnly := true

	setVal, ids, diags := flattenDictionaries([]*fastly.Dictionary{
		{
			DictionaryID: &id,
			Name:         &name,
			WriteOnly:    &writeOnly,
		},
	})
	if diags.HasError() {
		t.Fatalf("flattenDictionaries returned diagnostics: %v", diags)
	}

	if len(ids) != 1 || ids[0] != id {
		t.Fatalf("ids = %v, want [%s]", ids, id)
	}

	if setVal.IsNull() || setVal.IsUnknown() {
		t.Fatal("expected non-null, known set")
	}

	if len(setVal.Elements()) != 1 {
		t.Fatalf("set elements length = %d, want 1", len(setVal.Elements()))
	}
}

func TestFlattenDictionaries_nilAndEmpty(t *testing.T) {
	setVal, ids, diags := flattenDictionaries([]*fastly.Dictionary{nil})
	if diags.HasError() {
		t.Fatalf("flattenDictionaries returned diagnostics: %v", diags)
	}
	if len(ids) != 0 {
		t.Fatalf("ids = %v, want empty", ids)
	}
	if len(setVal.Elements()) != 0 {
		t.Fatalf("set elements length = %d, want 0", len(setVal.Elements()))
	}
}
