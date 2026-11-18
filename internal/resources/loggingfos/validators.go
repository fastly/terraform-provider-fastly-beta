package loggingfos

import (
	"context"
	"strings"

	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// gzipLevelCodecConflict rejects gzip_level with a non-blank compression_codec, which the API refuses.
type gzipLevelCodecConflict struct{}

func (gzipLevelCodecConflict) Description(_ context.Context) string {
	return "gzip_level cannot be set when compression_codec is set"
}

func (v gzipLevelCodecConflict) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (gzipLevelCodecConflict) ValidateInt64(ctx context.Context, req validator.Int64Request, resp *validator.Int64Response) {
	// Config values are null when unconfigured; the -1 default is applied at plan time.
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	var codec types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, req.Path.ParentPath().AtName("compression_codec"), &codec)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if codec.IsNull() || codec.IsUnknown() || service.StringValue(codec) == "" {
		return
	}

	resp.Diagnostics.AddAttributeError(
		req.Path,
		"Conflicting `gzip_level` and `compression_codec`",
		"`gzip_level` and `compression_codec` cannot be set together — setting both is rejected.\n\n"+
			"- To compress at a specific gzip level, leave `compression_codec` unset and set `gzip_level`.\n"+
			"- To use a codec (`zstd`, `snappy`, or `gzip`), remove `gzip_level`. With `compression_codec = \"gzip\"`, the level defaults to `3`.",
	)
}

// notTrimmed rejects leading/trailing whitespace, which the API mishandles in a PGP public_key.
type notTrimmed struct{}

func (notTrimmed) Description(_ context.Context) string {
	return "value must not contain leading or trailing whitespace"
}

func (v notTrimmed) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (notTrimmed) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	v := req.ConfigValue.ValueString()
	if v != strings.TrimSpace(v) {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Value",
			"must not contain leading or trailing whitespace characters (e.g., \\n\\t\\r\\f). Consider trimming the value.",
		)
	}
}
