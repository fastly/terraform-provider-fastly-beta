package servicecomputeauto

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func moveTarget(t *testing.T) resource.MoveStateResponse {
	t.Helper()
	ctx := context.Background()
	r := NewResource()
	var sr resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &sr)
	return resource.MoveStateResponse{TargetState: tfsdk.State{
		Raw:    tftypes.NewValue(sr.Schema.Type().TerraformType(ctx), nil),
		Schema: sr.Schema,
	}}
}

func TestMoveLegacyServiceComputeState(t *testing.T) {
	ctx := context.Background()
	resp := moveTarget(t)
	moveLegacyServiceComputeState(ctx, resource.MoveStateRequest{
		SourceProviderAddress: legacyFastlyProviderAddress,
		SourceTypeName:        legacyServiceComputeTypeName,
		SourceSchemaVersion:   0,
		SourceRawState:        &tfprotov6.RawState{JSON: []byte(`{"id":"SID1","force_destroy":true,"reuse":null,"product_enablement":[]}`)},
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}
	var id types.String
	var fd, reuse types.Bool
	resp.TargetState.GetAttribute(ctx, path.Root("id"), &id)
	resp.TargetState.GetAttribute(ctx, path.Root("force_destroy"), &fd)
	resp.TargetState.GetAttribute(ctx, path.Root("reuse"), &reuse)
	if id.ValueString() != "SID1" || !fd.ValueBool() || reuse.IsNull() || reuse.ValueBool() {
		t.Fatalf("got id=%v fd=%v reuse=%v", id, fd, reuse)
	}
}

func TestMoveLegacyServiceComputeStateRejects(t *testing.T) {
	ctx := context.Background()
	cases := map[string]resource.MoveStateRequest{
		"wrong version": {SourceProviderAddress: legacyFastlyProviderAddress, SourceTypeName: legacyServiceComputeTypeName, SourceSchemaVersion: 1, SourceRawState: &tfprotov6.RawState{JSON: []byte(`{"id":"x"}`)}},
		"no id":         {SourceProviderAddress: legacyFastlyProviderAddress, SourceTypeName: legacyServiceComputeTypeName, SourceRawState: &tfprotov6.RawState{JSON: []byte(`{"name":"x"}`)}},
		"no raw state":  {SourceProviderAddress: legacyFastlyProviderAddress, SourceTypeName: legacyServiceComputeTypeName},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			resp := moveTarget(t)
			moveLegacyServiceComputeState(ctx, req, &resp)
			if !resp.Diagnostics.HasError() {
				t.Fatal("expected error")
			}
		})
	}
}

func TestMoveLegacyServiceComputeStateIgnoresOtherSources(t *testing.T) {
	resp := moveTarget(t)
	moveLegacyServiceComputeState(context.Background(), resource.MoveStateRequest{
		SourceProviderAddress: legacyFastlyProviderAddress, SourceTypeName: "fastly_service_vcl",
	}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal("unexpected error")
	}
}
