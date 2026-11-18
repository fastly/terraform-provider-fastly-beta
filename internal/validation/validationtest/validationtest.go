// Package validationtest builds resource configs for unit-testing ValidateConfig.
package validationtest

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// ConfigType returns the Terraform object type of r's schema.
func ConfigType(t *testing.T, r resource.Resource) tftypes.Object {
	t.Helper()
	var resp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema: %v", resp.Diagnostics)
	}
	return resp.Schema.Type().TerraformType(context.Background()).(tftypes.Object)
}

// Object returns a value of typ with every attribute null except those in set.
func Object(t *testing.T, typ tftypes.Object, set map[string]tftypes.Value) tftypes.Value {
	t.Helper()
	vals := make(map[string]tftypes.Value, len(typ.AttributeTypes))
	for name, at := range typ.AttributeTypes {
		vals[name] = tftypes.NewValue(at, nil)
	}
	for name, v := range set {
		if _, ok := typ.AttributeTypes[name]; !ok {
			t.Fatalf("unknown attribute %q", name)
		}
		vals[name] = v
	}
	return tftypes.NewValue(typ, vals)
}

// Unknown returns an unknown value for attribute name of typ.
func Unknown(typ tftypes.Object, name string) tftypes.Value {
	return tftypes.NewValue(typ.AttributeTypes[name], tftypes.UnknownValue)
}

// String returns a known string value.
func String(s string) tftypes.Value {
	return tftypes.NewValue(tftypes.String, s)
}

// Block returns attribute name of typ as a list of the given block values.
func Block(typ tftypes.Object, name string, elems ...tftypes.Value) tftypes.Value {
	return tftypes.NewValue(typ.AttributeTypes[name], elems)
}

// BlockType returns the element object type of block attribute name of typ.
func BlockType(typ tftypes.Object, name string) tftypes.Object {
	switch c := typ.AttributeTypes[name].(type) {
	case tftypes.List:
		return c.ElementType.(tftypes.Object)
	case tftypes.Set:
		return c.ElementType.(tftypes.Object)
	}
	panic("not a block: " + name)
}

// ValidateConfig runs r's ValidateConfig against a top-level object with the given attributes set.
func ValidateConfig(t *testing.T, r resource.Resource, set map[string]tftypes.Value) diag.Diagnostics {
	t.Helper()
	ctx := context.Background()

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	vr, ok := r.(resource.ResourceWithValidateConfig)
	if !ok {
		t.Fatalf("%T does not implement ValidateConfig", r)
	}

	cfg := tfsdk.Config{Schema: schemaResp.Schema, Raw: Object(t, ConfigType(t, r), set)}
	var resp resource.ValidateConfigResponse
	vr.ValidateConfig(ctx, resource.ValidateConfigRequest{Config: cfg}, &resp)
	return resp.Diagnostics
}
