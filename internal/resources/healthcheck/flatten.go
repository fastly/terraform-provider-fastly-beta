package healthcheck

import (
	"context"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"
)

// flatten converts the API response into Model state, keeping prior's headers when they match
// the remote ones after normalization (see headersEquivalent). Pass nil for prior, such as on
// import, to keep the API's values as-is.
func flatten(ctx context.Context, h *fastly.HealthCheck, m *Model, prior *NestedModel) {
	if h == nil {
		tflog.Warn(ctx, "flatten called with nil health check")
		return
	}

	id := fastly.ToValue(h.ServiceID) + "-" + strconv.Itoa(fastly.ToValue(h.ServiceVersion)) + "-" + fastly.ToValue(h.Name)
	m.ID = types.StringValue(id)
	m.Service = types.StringValue(fastly.ToValue(h.ServiceID))
	m.Version = types.Int64Value(int64(fastly.ToValue(h.ServiceVersion)))

	model := ops{}.ToModel(h)
	if prior != nil && !prior.Headers.IsUnknown() && headersEquivalent(prior.Headers, model.Headers) {
		model.Headers = prior.Headers
	}
	m.NestedModel = model

	tflog.Debug(ctx, "Flattened service health check state", map[string]any{
		"id":      id,
		"service": m.Service.ValueString(),
		"version": m.Version.ValueInt64(),
		"name":    m.Name.ValueString(),
	})
}

// headersEquivalent reports whether a and b hold the same headers once normalized the way
// go-fastly sends them: its health check form encoding rewrites every ": " to ":", so the API
// stores and returns "Foo:Bar" for a configured "Foo: Bar". Null and empty sets are equivalent.
func headersEquivalent(a, b types.Set) bool {
	an, bn := normalizedHeaders(a), normalizedHeaders(b)
	if len(an) != len(bn) {
		return false
	}
	for h := range an {
		if _, ok := bn[h]; !ok {
			return false
		}
	}
	return true
}

func normalizedHeaders(s types.Set) map[string]struct{} {
	headers := headersToSlice(s)
	out := make(map[string]struct{}, len(headers))
	for _, h := range headers {
		out[strings.ReplaceAll(h, ": ", ":")] = struct{}{}
	}
	return out
}
