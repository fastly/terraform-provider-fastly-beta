package gzip

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var legacyGzipType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"name": types.StringType, "cache_condition": types.StringType,
	"content_types": types.ListType{ElemType: types.StringType},
	"extensions":    types.ListType{ElemType: types.StringType},
}}

func legacyGzip(name string, contentTypes, extensions types.List) attr.Value {
	return types.ObjectValueMust(legacyGzipType.AttrTypes, map[string]attr.Value{
		"name": types.StringValue(name), "cache_condition": types.StringValue(""),
		"content_types": contentTypes, "extensions": extensions,
	})
}

func TestLegacyGzipHintsSurviveFirstRead(t *testing.T) {
	t.Parallel()
	source := types.SetValueMust(legacyGzipType, []attr.Value{
		legacyGzip("text", stringList("text/html"), types.ListNull(types.StringType)),
		legacyGzip("defaults", stringList(), stringList()),
	})
	moved, diags := LegacyMoveState(source)
	if diags.HasError() || len(moved) != 2 {
		t.Fatal("legacy gzip hints did not decode")
	}
	if moved[0].Name.ValueString() != "defaults" || moved[1].Name.ValueString() != "text" {
		t.Fatal("legacy gzip hints are not sorted by name")
	}
	if !moved[0].ContentTypes.IsNull() || !moved[0].Extensions.IsNull() || !moved[1].Extensions.IsNull() {
		t.Fatal("unset legacy gzip lists were not kept null")
	}
	if moved[1].ContentTypes.IsNull() {
		t.Fatal("configured legacy gzip list was lost")
	}
}

func TestLegacyGzipHintsRejectMalformedState(t *testing.T) {
	t.Parallel()
	for _, source := range []types.Set{
		types.SetUnknown(legacyGzipType),
		types.SetValueMust(legacyGzipType, []attr.Value{legacyGzip("", stringList(), stringList())}),
		types.SetValueMust(legacyGzipType, []attr.Value{legacyGzip("a", types.ListUnknown(types.StringType), stringList())}),
	} {
		if _, diags := LegacyMoveState(source); !diags.HasError() {
			t.Fatal("malformed legacy gzip state accepted")
		}
	}
	if moved, diags := LegacyMoveState(types.SetNull(legacyGzipType)); diags.HasError() || moved != nil {
		t.Fatal("absent legacy gzip state was not ignored")
	}
}
