package loggingkinesis

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// authenticationRequired enforces the Fastly API's own requirement that a
// Kinesis endpoint have either an IAM role or an access/secret key pair —
// unlike S3, Kinesis rejects a request carrying none of the three at apply
// time ("either an IAM role or an access key and secret key pair must be
// provided"), so this catches the same mistake at plan time instead. A
// completely omitted `authentication` block resolves to all-empty defaults,
// which is exactly the same invalid case, so it is rejected too rather than
// skipped.
type authenticationRequired struct{}

func (authenticationRequired) Description(_ context.Context) string {
	return "requires either `iam_role`, or both `access_key` and `secret_key`"
}

func (v authenticationRequired) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (authenticationRequired) ValidateObject(_ context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
	if req.ConfigValue.IsUnknown() {
		return
	}

	var accessKey, secretKey, iamRole string
	if !req.ConfigValue.IsNull() {
		attrs := req.ConfigValue.Attributes()
		access, ok := attrs["access_key"].(types.String)
		if !ok || access.IsUnknown() {
			return
		}
		secret, ok := attrs["secret_key"].(types.String)
		if !ok || secret.IsUnknown() {
			return
		}
		role, ok := attrs["iam_role"].(types.String)
		if !ok || role.IsUnknown() {
			return
		}
		accessKey = access.ValueString()
		secretKey = secret.ValueString()
		iamRole = role.ValueString()
	}
	// A null block (omitted entirely) resolves to the schema's all-empty
	// defaults, which is the same invalid state as an explicit empty block.

	if iamRole != "" {
		return
	}
	if accessKey != "" && secretKey != "" {
		return
	}

	resp.Diagnostics.AddAttributeError(
		req.Path,
		"Missing Kinesis authentication credentials",
		"Provide either `iam_role`, or both `access_key` and `secret_key`, in the `authentication` block. "+
			"The Fastly API rejects a Kinesis logging endpoint with none of the three set.",
	)
}

// ValidateNoVCLOnlyAttributesForCompute returns an error diagnostic if format,
// format_version, placement, or response_condition are explicitly configured on
// a Compute service. The standalone fastly_service_logging_kinesis resource
// has one schema shared by both service types — unlike the nested blocks,
// which have distinct VCL (NestedBlockSchema) and Compute
// (ComputeNestedBlockSchema) schemas — so this is the only way to catch the
// mistake before it silently sends unsupported VCL-only attributes to a
// Compute service.
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
