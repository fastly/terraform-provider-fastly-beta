package validation

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// IsUnknownFunc reports whether the config value at a path is unknown.
type IsUnknownFunc func(path.Path) bool

// GetConfig is tfsdk.Config.Get for ValidateConfig models that decode nested blocks into plain
// slices. A dynamic block with an unknown for_each (always the case under terraform validate)
// is an unknown list, which a slice cannot hold, so unknown lists and sets of objects are
// decoded as empty. Checks that would misread an unknown block as empty must consult the
// returned IsUnknownFunc and skip it.
func GetConfig(ctx context.Context, cfg tfsdk.Config, target any) (IsUnknownFunc, diag.Diagnostics) {
	var diags diag.Diagnostics

	raw, err := tftypes.Transform(cfg.Raw, func(_ *tftypes.AttributePath, v tftypes.Value) (tftypes.Value, error) {
		if !v.IsKnown() && isObjectCollection(v.Type()) {
			return tftypes.NewValue(v.Type(), nil), nil
		}
		return v, nil
	})
	if err != nil {
		diags.AddError("Unable to read configuration", err.Error())
		return nil, diags
	}

	nulled := tfsdk.Config{Schema: cfg.Schema, Raw: raw}
	diags.Append(nulled.Get(ctx, target)...)

	isUnknown := func(p path.Path) bool {
		var v attr.Value
		if d := cfg.GetAttribute(ctx, p, &v); d.HasError() || v == nil {
			return false
		}
		return v.IsUnknown()
	}

	return isUnknown, diags
}

func isObjectCollection(t tftypes.Type) bool {
	switch c := t.(type) {
	case tftypes.List:
		return c.ElementType.Is(tftypes.Object{})
	case tftypes.Set:
		return c.ElementType.Is(tftypes.Object{})
	}
	return false
}
