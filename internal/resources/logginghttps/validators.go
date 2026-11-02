package logginghttps

import (
	"context"
	"fmt"
	"net/url"

	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// httpsURL requires url to be a valid URL with a non-empty host and the
// https:// scheme, matching the Fastly API's documented requirement ("Must
// use HTTPS") and the SDKv2 provider's validation.IsURLWithHTTPS behavior. A
// bare scheme prefix like "https://" without a host is rejected.
type httpsURL struct{}

func (httpsURL) Description(_ context.Context) string {
	return "must be a valid URL using the https:// scheme"
}

func (v httpsURL) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (httpsURL) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.Scheme != "https" {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid URL",
			fmt.Sprintf("must be a valid URL using the https:// scheme, got %q.", value),
		)
	}
}

// gzipLevelCodecConflict enforces that gzip_level and compression_codec are not
// configured together. The Fastly API rejects a request that sets both, and the
// codec implies its own level (gzip defaults to 3), so the two are alternative
// ways to request compression. A blank compression_codec is not a conflict.
type gzipLevelCodecConflict struct{}

func (gzipLevelCodecConflict) Description(_ context.Context) string {
	return "gzip_level cannot be set when compression_codec is set"
}

func (v gzipLevelCodecConflict) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (gzipLevelCodecConflict) ValidateInt64(ctx context.Context, req validator.Int64Request, resp *validator.Int64Response) {
	// Only relevant when the user explicitly set gzip_level. Config values are
	// null when unconfigured (the -1 default is applied later, at plan time).
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	var codec types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, req.Path.ParentPath().AtName("compression_codec"), &codec)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// A blank codec is allowed alongside gzip_level.
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
