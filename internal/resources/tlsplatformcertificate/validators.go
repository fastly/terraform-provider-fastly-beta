package tlsplatformcertificate

import (
	"bytes"
	"context"
	"encoding/pem"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// pemBlockValidator checks that a string is a single PEM block of pemType.
type pemBlockValidator struct {
	pemType string
}

func (v pemBlockValidator) Description(_ context.Context) string {
	return fmt.Sprintf("value must be a single PEM-format block of type %q", v.pemType)
}

func (v pemBlockValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v pemBlockValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	block, rest := pem.Decode([]byte(req.ConfigValue.ValueString()))
	if block == nil {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid PEM Block",
			fmt.Sprintf("expected %s to be a valid PEM-format block", req.Path))
		return
	}
	if block.Type != v.pemType {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid PEM Block Type",
			fmt.Sprintf("expected %s to be a valid PEM-format block of type %q, got %q", req.Path, v.pemType, block.Type))
		return
	}
	// Trailing whitespace (e.g. a config heredoc's own closing newline) is
	// not a second block; only non-whitespace leftover content is.
	if len(bytes.TrimSpace(rest)) != 0 {
		resp.Diagnostics.AddAttributeError(req.Path, "Multiple PEM Blocks",
			fmt.Sprintf("expected %s to contain only one PEM-format block", req.Path))
	}
}

// pemBlocksValidator checks that a string is one or more concatenated PEM
// blocks, all of pemType.
type pemBlocksValidator struct {
	pemType string
}

func (v pemBlocksValidator) Description(_ context.Context) string {
	return fmt.Sprintf("value must be one or more PEM-format blocks of type %q", v.pemType)
}

func (v pemBlocksValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v pemBlocksValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	rest := []byte(req.ConfigValue.ValueString())
	var numBlocks int
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		numBlocks++
		if block.Type != v.pemType {
			resp.Diagnostics.AddAttributeError(req.Path, "Invalid PEM Block Type",
				fmt.Sprintf("expected %s to be valid PEM-format blocks of type %q, got %q", req.Path, v.pemType, block.Type))
			return
		}
	}

	if numBlocks < 1 {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid PEM Blocks",
			fmt.Sprintf("expected %s to be one or more valid PEM-format blocks of type %q", req.Path, v.pemType))
	}
}
