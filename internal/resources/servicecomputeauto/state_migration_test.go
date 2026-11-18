package servicecomputeauto

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestMoveStateDeclaresLegacyServiceComputeMover(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := NewResource().(*Resource)

	_ = legacyServiceComputeSchemaV0Mover(ctx, t, r)
}

func TestMoveStateSourceSchemaMatchesLegacyServiceComputeSchemaV1(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := NewResource().(*Resource)
	mover := legacyServiceComputeSchemaV0Mover(ctx, t, r)

	fixture := readLegacyServiceComputeTypeFixture(t)
	if fixture.Version != legacyServiceComputeSchemaV0 {
		t.Fatalf("fixture schema version = %d, want %d", fixture.Version, legacyServiceComputeSchemaV0)
	}

	got := mover.SourceSchema.Type().TerraformType(ctx)
	want := fixtureBlockTerraformType(t, fixture.Block)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy move-state Terraform type does not match SDKv2 schema-v0 fixture\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestMoveLegacyServiceComputeState(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := NewResource().(*Resource)
	mover := legacyServiceComputeSchemaV0Mover(ctx, t, r)

	sourceState := newLegacyMoveSourceState(t, mover)
	targetState := newComputeAutoTargetState(ctx, t, r)

	req := resource.MoveStateRequest{
		SourceProviderAddress: legacyFastlyProviderAddress,
		SourceSchemaVersion:   legacyServiceComputeSchemaV0,
		SourceState:           sourceState,
		SourceTypeName:        legacyServiceComputeTypeName,
	}
	resp := resource.MoveStateResponse{
		TargetState: targetState,
	}

	mover.StateMover(ctx, req, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("state mover returned diagnostics: %v", resp.Diagnostics)
	}
	if got := len(resp.Diagnostics); got != 0 {
		t.Fatalf("state mover returned %d diagnostics for service without product_enablement, want 0: %v", got, resp.Diagnostics)
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

func TestMoveLegacyServiceComputeStateRejectsUnsupportedSchemaVersion(t *testing.T) {
	t.Parallel()

	for _, version := range []int64{1, 2} {
		t.Run(fmt.Sprintf("schema version %d", version), func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			r := NewResource().(*Resource)
			mover := legacyServiceComputeSchemaV0Mover(ctx, t, r)

			req := resource.MoveStateRequest{
				SourceProviderAddress: legacyFastlyProviderAddress,
				SourceSchemaVersion:   version,
				SourceState:           newLegacyMoveSourceState(t, mover),
				SourceTypeName:        legacyServiceComputeTypeName,
			}
			resp := resource.MoveStateResponse{TargetState: newComputeAutoTargetState(ctx, t, r)}

			mover.StateMover(ctx, req, &resp)
			if !resp.Diagnostics.HasError() {
				t.Fatalf("state mover accepted schema version %d, want an error", version)
			}
		})
	}
}

func TestMoveLegacyServiceComputeStateRejectsUnsupportedLifecycleSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		attribute string
		value     bool
		wantError string
	}{
		{name: "activate false", attribute: "activate", value: false, wantError: "activate = false"},
		{name: "stage true", attribute: "stage", value: true, wantError: "stage = true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			r := NewResource().(*Resource)
			mover := legacyServiceComputeSchemaV0Mover(ctx, t, r)

			source := newLegacyMoveSourceState(t, mover)
			if diags := source.SetAttribute(ctx, path.Root(tt.attribute), tt.value); diags.HasError() {
				t.Fatalf("SetAttribute(%s) returned diagnostics: %v", tt.attribute, diags)
			}

			req := resource.MoveStateRequest{
				SourceProviderAddress: legacyFastlyProviderAddress,
				SourceSchemaVersion:   legacyServiceComputeSchemaV0,
				SourceState:           source,
				SourceTypeName:        legacyServiceComputeTypeName,
			}
			resp := resource.MoveStateResponse{TargetState: newComputeAutoTargetState(ctx, t, r)}

			mover.StateMover(ctx, req, &resp)
			if !resp.Diagnostics.HasError() {
				t.Fatalf("state mover accepted %s, want an error", tt.name)
			}
			if got := resp.Diagnostics.Errors()[0].Summary(); !strings.Contains(got, tt.wantError) {
				t.Errorf("diagnostic summary = %q, want it to mention %q", got, tt.wantError)
			}
			var id types.String
			if diags := resp.TargetState.GetAttribute(ctx, path.Root("id"), &id); diags.HasError() || !id.IsNull() {
				t.Errorf("target state was written despite the error (id = %v, diags = %v)", id, diags)
			}
		})
	}
}

func TestMoveLegacyServiceComputeStateAllowsDefaultLifecycleSettings(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := NewResource().(*Resource)
	mover := legacyServiceComputeSchemaV0Mover(ctx, t, r)

	source := newLegacyMoveSourceState(t, mover)
	for attribute, value := range map[string]bool{"activate": true, "stage": false} {
		if diags := source.SetAttribute(ctx, path.Root(attribute), value); diags.HasError() {
			t.Fatalf("SetAttribute(%s) returned diagnostics: %v", attribute, diags)
		}
	}

	req := resource.MoveStateRequest{
		SourceProviderAddress: legacyFastlyProviderAddress,
		SourceSchemaVersion:   legacyServiceComputeSchemaV0,
		SourceState:           source,
		SourceTypeName:        legacyServiceComputeTypeName,
	}
	resp := resource.MoveStateResponse{TargetState: newComputeAutoTargetState(ctx, t, r)}

	mover.StateMover(ctx, req, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("state mover rejected default lifecycle settings: %v", resp.Diagnostics)
	}
}

type fakePrivateState map[string][]byte

func (f fakePrivateState) SetKey(_ context.Context, key string, value []byte) diag.Diagnostics {
	f[key] = value
	return nil
}

func TestMarkComputePackageForRefreshWritesImportedMarker(t *testing.T) {
	t.Parallel()

	private := fakePrivateState{}
	if diags := markComputePackageForRefresh(context.Background(), private); diags.HasError() {
		t.Fatalf("markComputePackageForRefresh returned diagnostics: %v", diags)
	}
	if got := string(private[computePackageImportedPrivateKey]); got != "true" {
		t.Fatalf("private key %q = %q, want \"true\"", computePackageImportedPrivateKey, got)
	}
}

func TestMoveLegacyServiceComputeStateRejectsNilSourceState(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := NewResource().(*Resource)
	mover := legacyServiceComputeSchemaV0Mover(ctx, t, r)

	req := resource.MoveStateRequest{
		SourceProviderAddress: legacyFastlyProviderAddress,
		SourceSchemaVersion:   legacyServiceComputeSchemaV0,
		SourceTypeName:        legacyServiceComputeTypeName,
	}
	resp := resource.MoveStateResponse{
		TargetState: newComputeAutoTargetState(ctx, t, r),
	}

	mover.StateMover(ctx, req, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("state mover accepted nil source state, want an error")
	}
}

func TestMoveLegacyServiceComputeStateAllowsSimpleProductEnablement(t *testing.T) {
	t.Parallel()

	for _, productName := range legacyServiceComputeSimpleProductNames {
		t.Run(productName, func(t *testing.T) {
			testMoveLegacyServiceComputeStateWithSimpleProducts(
				t,
				map[string]bool{productName: true},
				1,
			)
		})
	}

	tests := []struct {
		name            string
		products        map[string]bool
		wantDiagnostics int
	}{
		{
			name: "multiple simple products",
			products: map[string]bool{
				"domain_inspector": true,
				"fanout":           true,
				"websockets":       true,
			},
			wantDiagnostics: 1,
		},
		{
			name:            "explicitly disabled simple product",
			products:        map[string]bool{"domain_inspector": false},
			wantDiagnostics: 1,
		},
		{
			name:            "product block with no simple product values",
			products:        map[string]bool{},
			wantDiagnostics: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testMoveLegacyServiceComputeStateWithSimpleProducts(t, test.products, test.wantDiagnostics)
		})
	}
}

func testMoveLegacyServiceComputeStateWithSimpleProducts(
	t *testing.T,
	products map[string]bool,
	wantDiagnostics int,
) {
	t.Helper()

	ctx := context.Background()
	r := NewResource().(*Resource)
	mover := legacyServiceComputeSchemaV0Mover(ctx, t, r)

	req := resource.MoveStateRequest{
		SourceProviderAddress: legacyFastlyProviderAddress,
		SourceSchemaVersion:   legacyServiceComputeSchemaV0,
		SourceState:           newLegacyMoveSourceStateWithSimpleProducts(t, mover, products),
		SourceTypeName:        legacyServiceComputeTypeName,
	}
	resp := resource.MoveStateResponse{
		TargetState: newComputeAutoTargetState(ctx, t, r),
	}

	mover.StateMover(ctx, req, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("state mover rejected supported simple product_enablement: %v", resp.Diagnostics)
	}
	if got := len(resp.Diagnostics); got != wantDiagnostics {
		t.Fatalf("state mover returned %d diagnostics, want %d: %v", got, wantDiagnostics, resp.Diagnostics)
	}

	var gotID types.String
	if diags := resp.TargetState.GetAttribute(ctx, path.Root("id"), &gotID); diags.HasError() {
		t.Fatalf("GetAttribute(id) returned diagnostics: %v", diags)
	}
	if gotID.ValueString() != "SID123" {
		t.Errorf("id = %q, want SID123", gotID.ValueString())
	}
}

func TestMoveLegacyServiceComputeStateAllowsConfigurableProductEnablement(t *testing.T) {
	t.Parallel()

	for _, productName := range legacyServiceComputeConfigurableProductNames {
		t.Run(productName, func(t *testing.T) {
			ctx := context.Background()
			r := NewResource().(*Resource)
			mover := legacyServiceComputeSchemaV0Mover(ctx, t, r)

			req := resource.MoveStateRequest{
				SourceProviderAddress: legacyFastlyProviderAddress,
				SourceSchemaVersion:   legacyServiceComputeSchemaV0,
				SourceState:           newLegacyMoveSourceStateWithConfiguredProduct(t, mover, productName),
				SourceTypeName:        legacyServiceComputeTypeName,
			}
			resp := resource.MoveStateResponse{
				TargetState: newComputeAutoTargetState(ctx, t, r),
			}

			mover.StateMover(ctx, req, &resp)
			if resp.Diagnostics.HasError() {
				t.Fatalf("state mover rejected legacy configurable product %q: %v", productName, resp.Diagnostics)
			}
			if got := len(resp.Diagnostics); got != 1 {
				t.Fatalf("state mover returned %d diagnostics, want 1 standalone-resource warning: %v", got, resp.Diagnostics)
			}

			var gotID types.String
			if diags := resp.TargetState.GetAttribute(ctx, path.Root("id"), &gotID); diags.HasError() {
				t.Fatalf("GetAttribute(id) returned diagnostics: %v", diags)
			}
			if gotID.ValueString() != "SID123" {
				t.Errorf("id = %q, want SID123", gotID.ValueString())
			}
		})
	}
}

func TestMoveLegacyServiceComputeStateRejectsMultipleProductEnablementElements(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := NewResource().(*Resource)
	mover := legacyServiceComputeSchemaV0Mover(ctx, t, r)

	req := resource.MoveStateRequest{
		SourceProviderAddress: legacyFastlyProviderAddress,
		SourceSchemaVersion:   legacyServiceComputeSchemaV0,
		SourceState: newLegacyMoveSourceStateWithProductEnablement(
			t,
			mover,
			func(productType tftypes.Object, values map[string]tftypes.Value) {
				setLegacySimpleProductValue(t, productType, values, "domain_inspector", true)
			},
			func(productType tftypes.Object, values map[string]tftypes.Value) {
				setLegacySimpleProductValue(t, productType, values, "fanout", true)
			},
		),
		SourceTypeName: legacyServiceComputeTypeName,
	}
	resp := resource.MoveStateResponse{
		TargetState: newComputeAutoTargetState(ctx, t, r),
	}

	mover.StateMover(ctx, req, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("state mover accepted multiple legacy product_enablement elements, want an error")
	}
}

func TestInspectLegacyProductAttributesRejectsUnknownProduct(t *testing.T) {
	t.Parallel()

	attributes := make(map[string]attr.Value, len(legacyServiceComputeSimpleProductNames)+len(legacyServiceComputeConfigurableProductNames)+2)
	attributes["name"] = types.StringValue("products")

	for _, name := range legacyServiceComputeSimpleProductNames {
		attributes[name] = types.BoolNull()
	}
	for _, name := range legacyServiceComputeConfigurableProductNames {
		attributes[name] = types.ListNull(types.StringType)
	}

	attributes["future_product"] = types.BoolValue(true)

	supported, _ := inspectLegacyProductAttributes(attributes)
	if supported {
		t.Fatal("legacy product enablement inspection accepted an unknown product attribute, want rejection")
	}
}

func TestMoveLegacyServiceComputeStateIgnoresUnrelatedSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		providerAddress string
		typeName        string
	}{
		{
			name:            "provider",
			providerAddress: "registry.terraform.io/example/other",
			typeName:        legacyServiceComputeTypeName,
		},
		{
			name:            "resource type",
			providerAddress: legacyFastlyProviderAddress,
			typeName:        "fastly_service_vcl",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			r := NewResource().(*Resource)
			mover := legacyServiceComputeSchemaV0Mover(ctx, t, r)

			req := resource.MoveStateRequest{
				SourceProviderAddress: test.providerAddress,
				SourceSchemaVersion:   legacyServiceComputeSchemaV0,
				SourceState:           newLegacyMoveSourceState(t, mover),
				SourceTypeName:        test.typeName,
			}
			resp := resource.MoveStateResponse{TargetState: newComputeAutoTargetState(ctx, t, r)}

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

func legacyServiceComputeSchemaV0Mover(ctx context.Context, t *testing.T, r *Resource) resource.StateMover {
	t.Helper()

	fixture := readLegacyServiceComputeTypeFixture(t)
	if fixture.Version != legacyServiceComputeSchemaV0 {
		t.Fatalf("fixture schema version = %d, want %d", fixture.Version, legacyServiceComputeSchemaV0)
	}
	want := fixtureBlockTerraformType(t, fixture.Block)

	var matches []resource.StateMover
	for _, mover := range r.MoveState(ctx) {
		if mover.SourceSchema == nil {
			continue
		}
		if reflect.DeepEqual(mover.SourceSchema.Type().TerraformType(ctx), want) {
			matches = append(matches, mover)
		}
	}

	if len(matches) != 1 {
		t.Fatalf("MoveState() returned %d movers matching legacy fastly_service_compute schema version 0, want 1", len(matches))
	}

	return matches[0]
}

type legacyServiceComputeTypeFixture struct {
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

func readLegacyServiceComputeTypeFixture(t *testing.T) legacyServiceComputeTypeFixture {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("testdata", "legacy_service_compute_schema_v0_type.json"))
	if err != nil {
		t.Fatalf("read legacy schema fixture: %v", err)
	}

	var fixture legacyServiceComputeTypeFixture
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
	fixture := readLegacyServiceComputeTypeFixture(t)
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

func newLegacyMoveSourceStateWithSimpleProducts(
	t *testing.T,
	mover resource.StateMover,
	products map[string]bool,
) *tfsdk.State {
	t.Helper()

	return newLegacyMoveSourceStateWithProductEnablement(t, mover, func(productType tftypes.Object, values map[string]tftypes.Value) {
		for name, enabled := range products {
			setLegacySimpleProductValue(t, productType, values, name, enabled)
		}
	})
}

func setLegacySimpleProductValue(
	t *testing.T,
	productType tftypes.Object,
	values map[string]tftypes.Value,
	name string,
	enabled bool,
) {
	t.Helper()

	attrType, ok := productType.AttributeTypes[name]
	if !ok {
		t.Fatalf("legacy product_enablement has no %q attribute", name)
	}
	if !reflect.DeepEqual(attrType, tftypes.Bool) {
		t.Fatalf("legacy product_enablement.%s type = %T, want tftypes.Bool", name, attrType)
	}
	values[name] = tftypes.NewValue(attrType, enabled)
}

func newLegacyMoveSourceStateWithConfiguredProduct(
	t *testing.T,
	mover resource.StateMover,
	productName string,
) *tfsdk.State {
	t.Helper()

	return newLegacyMoveSourceStateWithProductEnablement(t, mover, func(productType tftypes.Object, values map[string]tftypes.Value) {
		listType, ok := productType.AttributeTypes[productName].(tftypes.List)
		if !ok {
			t.Fatalf("legacy product_enablement.%s type = %T, want tftypes.List", productName, productType.AttributeTypes[productName])
		}
		elementType, ok := listType.ElementType.(tftypes.Object)
		if !ok {
			t.Fatalf("legacy product_enablement.%s element type = %T, want tftypes.Object", productName, listType.ElementType)
		}

		elementValues := make(map[string]tftypes.Value, len(elementType.AttributeTypes))
		for name, attrType := range elementType.AttributeTypes {
			elementValues[name] = tftypes.NewValue(attrType, nil)
		}
		if enabledType, ok := elementType.AttributeTypes["enabled"]; ok {
			elementValues["enabled"] = tftypes.NewValue(enabledType, true)
		}

		values[productName] = tftypes.NewValue(
			listType,
			[]tftypes.Value{tftypes.NewValue(elementType, elementValues)},
		)
	})
}

func newLegacyMoveSourceStateWithProductEnablement(
	t *testing.T,
	mover resource.StateMover,
	configures ...func(productType tftypes.Object, values map[string]tftypes.Value),
) *tfsdk.State {
	t.Helper()

	sourceSchema := *mover.SourceSchema
	fixture := readLegacyServiceComputeTypeFixture(t)
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

	productElements := make([]tftypes.Value, 0, len(configures))
	for _, configure := range configures {
		productValues := make(map[string]tftypes.Value, len(productObjectType.AttributeTypes))
		for name, attrType := range productObjectType.AttributeTypes {
			if _, ok := attrType.(tftypes.List); ok {
				productValues[name] = tftypes.NewValue(attrType, []tftypes.Value{})
				continue
			}
			productValues[name] = tftypes.NewValue(attrType, nil)
		}
		productValues["name"] = tftypes.NewValue(tftypes.String, "products")
		configure(productObjectType, productValues)
		productElements = append(productElements, tftypes.NewValue(productObjectType, productValues))
	}

	rootValues := make(map[string]tftypes.Value, len(rootType.AttributeTypes))
	for name, attrType := range rootType.AttributeTypes {
		rootValues[name] = tftypes.NewValue(attrType, nil)
	}
	rootValues["id"] = tftypes.NewValue(tftypes.String, "SID123")
	rootValues["force_destroy"] = tftypes.NewValue(tftypes.Bool, true)
	rootValues["reuse"] = tftypes.NewValue(tftypes.Bool, false)
	rootValues["product_enablement"] = tftypes.NewValue(productSetType, productElements)

	return &tfsdk.State{
		Raw:    tftypes.NewValue(rootType, rootValues),
		Schema: sourceSchema,
	}
}

func newComputeAutoTargetState(ctx context.Context, t *testing.T, r *Resource) tfsdk.State {
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
