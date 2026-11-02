package loggingsftp

import (
	"context"
	"strings"

	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

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

// notTrimmed rejects a string with leading or trailing whitespace (e.g.
// \n\t\r\f). The Fastly API silently mishandles a public_key or secret_key
// with such whitespace, so this is caught at plan/validate time instead.
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

// requireOneCredential enforces that authentication sets at least one of
// password or secret_key. The legacy provider only caught this at apply time.
type requireOneCredential struct{}

func (requireOneCredential) Description(_ context.Context) string {
	return "at least one of `password` or `secret_key` must be set"
}

func (v requireOneCredential) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (requireOneCredential) ValidateObject(_ context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	attrs := req.ConfigValue.Attributes()

	hasValue := func(name string) bool {
		v, ok := attrs[name]
		if !ok || v == nil || v.IsNull() || v.IsUnknown() {
			return false
		}
		s, ok := v.(types.String)
		return ok && s.ValueString() != ""
	}

	if hasValue("password") || hasValue("secret_key") {
		return
	}

	resp.Diagnostics.AddAttributeError(
		req.Path,
		"Missing SFTP credential",
		"Either `password` or `secret_key` must be set in `authentication`.",
	)
}
