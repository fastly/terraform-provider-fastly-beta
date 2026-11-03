package logging

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// MaximumFormatLength is the API's limit on a `format` string, enforced at plan
// time rather than left to fail at apply.
const MaximumFormatLength = 12288

// ValidateNoVCLOnlyAttributesForCompute rejects format, format_version,
// placement, and response_condition on a Compute service. The standalone
// resources share one schema across service types, so unlike the nested blocks
// this can't be caught at schema level.
func ValidateNoVCLOnlyAttributesForCompute(ctx context.Context, cfg tfsdk.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	var format, placement, responseCondition types.String
	var formatVersion types.Int64

	diags.Append(cfg.GetAttribute(ctx, path.Root("format"), &format)...)
	diags.Append(cfg.GetAttribute(ctx, path.Root("format_version"), &formatVersion)...)
	diags.Append(cfg.GetAttribute(ctx, path.Root("placement"), &placement)...)
	diags.Append(cfg.GetAttribute(ctx, path.Root("response_condition"), &responseCondition)...)
	if diags.HasError() {
		return diags
	}

	var configured []string
	if !format.IsNull() {
		configured = append(configured, "format")
	}
	if !formatVersion.IsNull() {
		configured = append(configured, "format_version")
	}
	if !placement.IsNull() {
		configured = append(configured, "placement")
	}
	if !responseCondition.IsNull() {
		configured = append(configured, "response_condition")
	}

	if len(configured) > 0 {
		diags.AddError(
			"VCL-only attributes not supported on Compute services",
			"The following attributes only affect generated VCL and are not supported when `service_id` refers to a Compute service: "+
				strings.Join(configured, ", ")+". Remove them from this configuration.",
		)
	}

	return diags
}
