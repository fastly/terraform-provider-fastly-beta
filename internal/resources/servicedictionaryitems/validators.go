package servicedictionaryitems

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly"
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
	return fmt.Sprintf(
		"Dictionary item keys must contain 1 to %d characters, values must contain at most %d characters, and the map must contain at most %d items.",
		maxItemKeyLength, maxItemValueLength, fastly.MaximumDictionarySize,
	)
}

func (v validItemsValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v validItemsValidator) ValidateMap(_ context.Context, req validator.MapRequest, resp *validator.MapResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	if size := len(req.ConfigValue.Elements()); size > fastly.MaximumDictionarySize {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Too Many Dictionary Items",
			fmt.Sprintf("A dictionary may contain at most %d items; got %d.", fastly.MaximumDictionarySize, size),
		)
		return
	}

	for key, value := range req.ConfigValue.Elements() {
		keyLength := utf8.RuneCountInString(key)
		if keyLength == 0 || keyLength > maxItemKeyLength {
			resp.Diagnostics.AddAttributeError(
				req.Path,
				"Invalid Dictionary Item Key",
				fmt.Sprintf("Dictionary item key %q must contain between 1 and %d characters; got %d.", key, maxItemKeyLength, keyLength),
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
				"Invalid Dictionary Item Value",
				fmt.Sprintf("Dictionary item %q value must contain at most %d characters; got %d.", key, maxItemValueLength, valueLength),
			)
		}
	}
}
