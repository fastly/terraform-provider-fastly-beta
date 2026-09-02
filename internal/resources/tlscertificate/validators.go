package tlscertificate

import (
	"context"
	"encoding/pem"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// pemBlocks requires the value be one or more concatenated PEM blocks of type CERTIFICATE.
type pemBlocks struct{}

func (pemBlocks) Description(_ context.Context) string {
	return "must be valid PEM-format blocks of type 'CERTIFICATE'"
}

func (v pemBlocks) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (pemBlocks) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	rest := []byte(req.ConfigValue.ValueString())
	numBlocks := 0
	for {
		block, remainder := pem.Decode(rest)
		if block == nil {
			break
		}
		numBlocks++
		if block.Type != "CERTIFICATE" {
			resp.Diagnostics.AddAttributeError(
				req.Path,
				"Invalid certificate_body",
				fmt.Sprintf("expected certificate_body to be valid PEM-format blocks of type 'CERTIFICATE', got block of type %q", block.Type),
			)
			return
		}
		rest = remainder
	}

	if numBlocks < 1 {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid certificate_body",
			"expected certificate_body to be valid PEM-format blocks of type 'CERTIFICATE'",
		)
	}
}
