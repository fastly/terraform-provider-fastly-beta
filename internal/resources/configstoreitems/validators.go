package configstoreitems

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	maxItemKeyLength   = 256
	maxItemValueLength = 8000
)

type validItemsValidator struct{}

func ValidItems() validator.Map {
	return validItemsValidator{}
}

func (v validItemsValidator) Description(_ context.Context) string {
	return "Config Store item keys must contain 1 to 256 characters and values must contain at most 8000 characters."
}

func (v validItemsValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v validItemsValidator) ValidateMap(_ context.Context, req validator.MapRequest, resp *validator.MapResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	for key, value := range req.ConfigValue.Elements() {
		keyLength := utf8.RuneCountInString(key)
		if keyLength == 0 || keyLength > maxItemKeyLength {
			resp.Diagnostics.AddAttributeError(
				req.Path,
				"Invalid Config Store Item Key",
				fmt.Sprintf("Config Store item key %q must contain between 1 and %d characters; got %d.", key, maxItemKeyLength, keyLength),
			)
		}

		strValue, ok := value.(types.String)
		if !ok || strValue.IsNull() || strValue.IsUnknown() {
			continue
		}

		valueLength := utf8.RuneCountInString(strValue.ValueString())
		if valueLength > maxItemValueLength {
			resp.Diagnostics.AddAttributeError(
				req.Path,
				"Invalid Config Store Item Value",
				fmt.Sprintf("Config Store item %q value must contain at most %d characters; got %d.", key, maxItemValueLength, valueLength),
			)
		}
	}
}
