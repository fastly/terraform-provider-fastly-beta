package dnszone

import (
	"context"
	"fmt"
	"net"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// ipv4AddressValidator rejects anything that isn't a valid IPv4 address:
// Fastly's DNS zone transfer feature doesn't support IPv6 for primary
// servers, so this catches a hostname, IPv6 address, or typo at plan time
// instead of a failed apply.
type ipv4AddressValidator struct{}

func (ipv4AddressValidator) Description(_ context.Context) string {
	return "value must be a valid IPv4 address"
}

func (v ipv4AddressValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (ipv4AddressValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	v := req.ConfigValue.ValueString()
	if v == "" {
		return
	}

	ip := net.ParseIP(v)
	if ip == nil || ip.To4() == nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Attribute Value",
			fmt.Sprintf("must be a valid IPv4 address (IPv6 is not supported for DNS zone transfers), got: %q", v),
		)
	}
}
