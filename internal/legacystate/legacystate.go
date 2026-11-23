// Package legacystate holds helpers shared by state movers that decode state
// written by the legacy SDKv2 Fastly provider.
package legacystate

import "github.com/hashicorp/terraform-plugin-framework/types"

// ProviderAddress is the registry address of the legacy SDKv2 provider.
const ProviderAddress = "registry.terraform.io/fastly/fastly"

// HasValue reports whether v is known, non-null and non-empty.
func HasValue(v types.String) bool {
	return !v.IsNull() && !v.IsUnknown() && v.ValueString() != ""
}
