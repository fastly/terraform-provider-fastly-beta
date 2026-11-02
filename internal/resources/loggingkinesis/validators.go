package loggingkinesis

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
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
