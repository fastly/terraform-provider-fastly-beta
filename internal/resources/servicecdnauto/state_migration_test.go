package servicecdnauto

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestMoveStateDeclaresLegacyServiceVCLMover(t *testing.T) {
	t.Parallel()

	movers := NewResource().(*Resource).MoveState(context.Background())
	if len(movers) != 1 {
		t.Fatalf("MoveState() returned %d movers, want 1", len(movers))
	}
	if movers[0].SourceSchema == nil {
		t.Fatal("MoveState() source schema is nil")
	}
}

func TestMoveStateSourceSchemaMatchesLegacyServiceVCLSchemaV1(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	movers := NewResource().(*Resource).MoveState(ctx)
	if len(movers) != 1 || movers[0].SourceSchema == nil {
		t.Fatalf("MoveState() returned %#v, want one mover with a source schema", movers)
	}

	fixture := readLegacyServiceVCLTypeFixture(t)
	if fixture.Version != legacyServiceVCLSchemaV1 {
		t.Fatalf("fixture schema version = %d, want %d", fixture.Version, legacyServiceVCLSchemaV1)
	}

	got := movers[0].SourceSchema.Type().TerraformType(ctx)
	want := fixtureBlockTerraformType(t, fixture.Block)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy move-state Terraform type does not match SDKv2 schema-v1 fixture\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestMoveLegacyServiceVCLState(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := NewResource().(*Resource)
	mover := r.MoveState(ctx)[0]

	sourceState := newLegacyMoveSourceState(t, mover)
	targetState := newCDNAutoTargetState(ctx, t, r)

	req := resource.MoveStateRequest{
		SourceProviderAddress: legacyFastlyProviderAddress,
		SourceSchemaVersion:   legacyServiceVCLSchemaV1,
		SourceState:           sourceState,
		SourceTypeName:        legacyServiceVCLTypeName,
	}
	resp := resource.MoveStateResponse{
		TargetState: targetState,
	}

	mover.StateMover(ctx, req, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("state mover returned diagnostics: %v", resp.Diagnostics)
	}

	var gotID types.String
	if diags := resp.TargetState.GetAttribute(ctx, path.Root("id"), &gotID); diags.HasError() {
		t.Fatalf("GetAttribute(id) returned diagnostics: %v", diags)
	}
	if gotID.ValueString() != "SID123" {
		t.Errorf("id = %q, want SID123", gotID.ValueString())
	}

	var gotForceDestroy types.Bool
	if diags := resp.TargetState.GetAttribute(ctx, path.Root("force_destroy"), &gotForceDestroy); diags.HasError() {
		t.Fatalf("GetAttribute(force_destroy) returned diagnostics: %v", diags)
	}
	if !gotForceDestroy.ValueBool() {
		t.Error("force_destroy = false, want true")
	}

	var gotReuse types.Bool
	if diags := resp.TargetState.GetAttribute(ctx, path.Root("reuse"), &gotReuse); diags.HasError() {
		t.Fatalf("GetAttribute(reuse) returned diagnostics: %v", diags)
	}
	if gotReuse.ValueBool() {
		t.Error("reuse = true, want false")
	}
}

func TestMoveLegacyServiceVCLStateRejectsOldSchemaVersion(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := NewResource().(*Resource)
	mover := r.MoveState(ctx)[0]

	targetState := newCDNAutoTargetState(ctx, t, r)
	req := resource.MoveStateRequest{
		SourceProviderAddress: legacyFastlyProviderAddress,
		SourceSchemaVersion:   0,
		SourceState:           newLegacyMoveSourceState(t, mover),
		SourceTypeName:        legacyServiceVCLTypeName,
	}
	resp := resource.MoveStateResponse{TargetState: targetState}

	mover.StateMover(ctx, req, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("state mover accepted schema version 0, want an error")
	}
}

func TestMoveLegacyServiceVCLStateRejectsNilSourceState(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := NewResource().(*Resource)
	mover := r.MoveState(ctx)[0]

	req := resource.MoveStateRequest{
		SourceProviderAddress: legacyFastlyProviderAddress,
		SourceSchemaVersion:   legacyServiceVCLSchemaV1,
		SourceTypeName:        legacyServiceVCLTypeName,
	}
	resp := resource.MoveStateResponse{
		TargetState: newCDNAutoTargetState(ctx, t, r),
	}

	mover.StateMover(ctx, req, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("state mover accepted nil source state, want an error")
	}
}

func TestMoveLegacyServiceVCLStateRejectsProductEnablement(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := NewResource().(*Resource)
	mover := r.MoveState(ctx)[0]

	req := resource.MoveStateRequest{
		SourceProviderAddress: legacyFastlyProviderAddress,
		SourceSchemaVersion:   legacyServiceVCLSchemaV1,
		SourceState:           newLegacyMoveSourceStateWithProductEnablement(t, mover),
		SourceTypeName:        legacyServiceVCLTypeName,
	}
	resp := resource.MoveStateResponse{
		TargetState: newCDNAutoTargetState(ctx, t, r),
	}

	mover.StateMover(ctx, req, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("state mover accepted legacy product_enablement, want an error")
	}
}

func TestMoveLegacyServiceVCLStateIgnoresUnrelatedSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		providerAddress string
		typeName        string
	}{
		{
			name:            "provider",
			providerAddress: "registry.terraform.io/example/other",
			typeName:        legacyServiceVCLTypeName,
		},
		{
			name:            "resource type",
			providerAddress: legacyFastlyProviderAddress,
			typeName:        "fastly_service_compute",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			r := NewResource().(*Resource)
			mover := r.MoveState(ctx)[0]

			req := resource.MoveStateRequest{
				SourceProviderAddress: test.providerAddress,
				SourceSchemaVersion:   legacyServiceVCLSchemaV1,
				SourceState:           newLegacyMoveSourceState(t, mover),
				SourceTypeName:        test.typeName,
			}
			resp := resource.MoveStateResponse{TargetState: newCDNAutoTargetState(ctx, t, r)}

			mover.StateMover(ctx, req, &resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("state mover returned diagnostics for unrelated source: %v", resp.Diagnostics)
			}

			var gotID types.String
			if diags := resp.TargetState.GetAttribute(ctx, path.Root("id"), &gotID); diags.HasError() {
				t.Fatalf("GetAttribute(id) returned diagnostics: %v", diags)
			}
			if !gotID.IsNull() {
				t.Errorf("id = %v, want null for unrelated source", gotID)
			}
		})
	}
}

type legacyServiceVCLTypeFixture struct {
	Version int64                  `json:"version"`
	Block   legacyBlockTypeFixture `json:"block"`
}

type legacyBlockTypeFixture struct {
	Attributes map[string]json.RawMessage          `json:"attributes"`
	BlockTypes map[string]legacyNestedBlockFixture `json:"block_types"`
}

type legacyNestedBlockFixture struct {
	NestingMode string                 `json:"nesting_mode"`
	Block       legacyBlockTypeFixture `json:"block"`
}

func readLegacyServiceVCLTypeFixture(t *testing.T) legacyServiceVCLTypeFixture {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("testdata", "legacy_service_vcl_schema_v1_type.json"))
	if err != nil {
		t.Fatalf("read legacy schema fixture: %v", err)
	}

	var fixture legacyServiceVCLTypeFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("decode legacy schema fixture: %v", err)
	}

	return fixture
}

func fixtureBlockTerraformType(t *testing.T, block legacyBlockTypeFixture) tftypes.Type {
	t.Helper()

	attributeTypes := make(map[string]tftypes.Type, len(block.Attributes)+len(block.BlockTypes))

	for name, rawType := range block.Attributes {
		attributeTypes[name] = fixtureAttributeTerraformType(t, rawType)
	}

	for name, nested := range block.BlockTypes {
		elementType := fixtureBlockTerraformType(t, nested.Block)

		switch nested.NestingMode {
		case "set":
			attributeTypes[name] = tftypes.Set{ElementType: elementType}
		case "list":
			attributeTypes[name] = tftypes.List{ElementType: elementType}
		case "map":
			attributeTypes[name] = tftypes.Map{ElementType: elementType}
		default:
			t.Fatalf("unsupported legacy nested block mode %q for %q", nested.NestingMode, name)
		}
	}

	return tftypes.Object{AttributeTypes: attributeTypes}
}

func fixtureAttributeTerraformType(t *testing.T, raw json.RawMessage) tftypes.Type {
	t.Helper()

	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("decode Terraform type %s: %v", raw, err)
	}

	return fixtureTerraformType(t, value)
}

func fixtureTerraformType(t *testing.T, value any) tftypes.Type {
	t.Helper()

	switch value := value.(type) {
	case string:
		switch value {
		case "string":
			return tftypes.String
		case "bool":
			return tftypes.Bool
		case "number":
			return tftypes.Number
		default:
			t.Fatalf("unsupported legacy scalar type %q", value)
		}

	case []any:
		if len(value) != 2 {
			t.Fatalf("invalid legacy collection type %#v", value)
		}

		kind, ok := value[0].(string)
		if !ok {
			t.Fatalf("invalid legacy collection kind %#v", value[0])
		}
		elementType := fixtureTerraformType(t, value[1])

		switch kind {
		case "set":
			return tftypes.Set{ElementType: elementType}
		case "list":
			return tftypes.List{ElementType: elementType}
		case "map":
			return tftypes.Map{ElementType: elementType}
		default:
			t.Fatalf("unsupported legacy collection type %q", kind)
		}

	default:
		t.Fatalf("unsupported legacy Terraform type fixture value %#v", value)
	}

	return nil
}

func newLegacyMoveSourceState(t *testing.T, mover resource.StateMover) *tfsdk.State {
	t.Helper()

	sourceSchema := *mover.SourceSchema
	fixture := readLegacyServiceVCLTypeFixture(t)
	terraformType := fixtureBlockTerraformType(t, fixture.Block)
	objectType, ok := terraformType.(tftypes.Object)
	if !ok {
		t.Fatalf("legacy SDKv2 fixture type = %T, want tftypes.Object", terraformType)
	}

	values := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
	for name, attrType := range objectType.AttributeTypes {
		values[name] = tftypes.NewValue(attrType, nil)
	}
	values["id"] = tftypes.NewValue(tftypes.String, "SID123")
	values["force_destroy"] = tftypes.NewValue(tftypes.Bool, true)
	values["reuse"] = tftypes.NewValue(tftypes.Bool, false)

	return &tfsdk.State{
		Raw:    tftypes.NewValue(objectType, values),
		Schema: sourceSchema,
	}
}

func newLegacyMoveSourceStateWithProductEnablement(t *testing.T, mover resource.StateMover) *tfsdk.State {
	t.Helper()

	sourceSchema := *mover.SourceSchema
	fixture := readLegacyServiceVCLTypeFixture(t)
	terraformType := fixtureBlockTerraformType(t, fixture.Block)
	rootType, ok := terraformType.(tftypes.Object)
	if !ok {
		t.Fatalf("legacy SDKv2 fixture type = %T, want tftypes.Object", terraformType)
	}

	productSetType, ok := rootType.AttributeTypes["product_enablement"].(tftypes.Set)
	if !ok {
		t.Fatalf("product_enablement type = %T, want tftypes.Set", rootType.AttributeTypes["product_enablement"])
	}
	productObjectType, ok := productSetType.ElementType.(tftypes.Object)
	if !ok {
		t.Fatalf("product_enablement element type = %T, want tftypes.Object", productSetType.ElementType)
	}

	productValues := make(map[string]tftypes.Value, len(productObjectType.AttributeTypes))
	for name, attrType := range productObjectType.AttributeTypes {
		productValues[name] = tftypes.NewValue(attrType, nil)
	}
	productValues["api_discovery"] = tftypes.NewValue(tftypes.Bool, true)

	rootValues := make(map[string]tftypes.Value, len(rootType.AttributeTypes))
	for name, attrType := range rootType.AttributeTypes {
		rootValues[name] = tftypes.NewValue(attrType, nil)
	}
	rootValues["id"] = tftypes.NewValue(tftypes.String, "SID123")
	rootValues["force_destroy"] = tftypes.NewValue(tftypes.Bool, true)
	rootValues["reuse"] = tftypes.NewValue(tftypes.Bool, false)
	rootValues["product_enablement"] = tftypes.NewValue(
		productSetType,
		[]tftypes.Value{tftypes.NewValue(productObjectType, productValues)},
	)

	return &tfsdk.State{
		Raw:    tftypes.NewValue(rootType, rootValues),
		Schema: sourceSchema,
	}
}

func newCDNAutoTargetState(ctx context.Context, t *testing.T, r *Resource) tfsdk.State {
	t.Helper()

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("Schema() returned diagnostics: %v", schemaResp.Diagnostics)
	}

	return tfsdk.State{
		Raw:    tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), nil),
		Schema: schemaResp.Schema,
	}
}
