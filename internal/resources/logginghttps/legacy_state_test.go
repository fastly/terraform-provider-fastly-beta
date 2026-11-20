package logginghttps

import (
	"math/big"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func legacyHTTPSHintSet(values ...attr.Value) types.Set {
	return types.SetValueMust(types.ObjectType{AttrTypes: map[string]attr.Type{
		"name": types.StringType, "gzip_level": types.NumberType,
	}}, values)
}

func legacyHTTPSHint(name string, level types.Number) types.Object {
	return types.ObjectValueMust(map[string]attr.Type{"name": types.StringType, "gzip_level": types.NumberType},
		map[string]attr.Value{"name": types.StringValue(name), "gzip_level": level})
}

func TestLegacyHTTPSGzipHintsSurviveFirstRead(t *testing.T) {
	t.Parallel()
	level := func(n int64) types.Number { return types.NumberValue(new(big.Float).SetInt64(n)) }
	source := legacyHTTPSHintSet(legacyHTTPSHint("zero", level(0)), legacyHTTPSHint("codec", level(-1)),
		legacyHTTPSHint("default", types.NumberNull()), legacyHTTPSHint("six", level(6)))
	compute, diags := LegacyComputeMoveState(source)
	if diags.HasError() || len(compute) != 4 {
		t.Fatal("legacy HTTPS hints did not decode")
	}
	vcl, diags := LegacyMoveState(source)
	if diags.HasError() || len(vcl) != 4 {
		t.Fatal("legacy VCL HTTPS hints did not decode")
	}
	var remoteCompute []ComputeNestedModel
	var remoteVCL []NestedModel
	for _, item := range []struct {
		name, codec string
		level       int64
	}{
		{"zero", "", 0}, {"codec", "gzip", 3}, {"default", "", 0}, {"six", "", 6},
	} {
		remote := commonModel{
			Name: types.StringValue(item.name), CompressionCodec: types.StringValue(item.codec),
			GzipLevel: types.Int64Value(item.level),
		}
		remoteCompute = append(remoteCompute, ComputeNestedModel{commonModel: remote})
		remoteVCL = append(remoteVCL, NestedModel{commonModel: remote})
	}
	want := map[string]int64{"zero": 0, "codec": -1, "default": -1, "six": 6}
	for _, model := range ComputeMatchOrder(remoteCompute, compute) {
		if model.GzipLevel.ValueInt64() != want[model.Name.ValueString()] {
			t.Fatal("Compute HTTPS gzip hint lost on first refresh")
		}
	}
	for _, model := range MatchOrder(remoteVCL, vcl) {
		if model.GzipLevel.ValueInt64() != want[model.Name.ValueString()] {
			t.Fatal("VCL HTTPS gzip hint lost on first refresh")
		}
	}
}

func TestLegacyHTTPSGzipHintsRejectMalformedState(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		state types.Set
	}{
		{"duplicate", legacyHTTPSHintSet(legacyHTTPSHint("same", types.NumberValue(big.NewFloat(0))),
			legacyHTTPSHint("same", types.NumberValue(big.NewFloat(1))))},
		{"missing name", legacyHTTPSHintSet(legacyHTTPSHint("", types.NumberValue(big.NewFloat(0))))},
		{"unknown level", legacyHTTPSHintSet(legacyHTTPSHint("unknown", types.NumberUnknown()))},
		{"fractional", legacyHTTPSHintSet(legacyHTTPSHint("fractional", types.NumberValue(big.NewFloat(2.5))))},
		{"invalid range", legacyHTTPSHintSet(legacyHTTPSHint("invalid", types.NumberValue(big.NewFloat(10))))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, diags := LegacyComputeMoveState(tc.state)
			if !diags.HasError() || len(got) != 0 {
				t.Fatal("unsafe HTTPS hint accepted")
			}
		})
	}
}
