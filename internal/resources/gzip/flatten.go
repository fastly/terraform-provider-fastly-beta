package gzip

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"
)

// flatten converts the API response into Model state, normalizing content_types/extensions
// against prior so that a value prior never set doesn't drift against the Fastly API's
// silently-substituted default list (see ops.Equal). Pass nil for prior, such as on import, to
// skip normalization and keep the API's values as-is.
func flatten(ctx context.Context, g *fastly.Gzip, m *Model, prior *NestedModel) {
	if g == nil {
		tflog.Warn(ctx, "flatten called with nil gzip configuration")
		return
	}

	id := fastly.ToValue(g.ServiceID) + "-" + strconv.Itoa(fastly.ToValue(g.ServiceVersion)) + "-" + fastly.ToValue(g.Name)
	m.ID = types.StringValue(id)
	m.Service = types.StringValue(fastly.ToValue(g.ServiceID))
	m.Version = types.Int64Value(int64(fastly.ToValue(g.ServiceVersion)))

	model := ops{}.ToModel(g)
	if prior != nil {
		if listUnset(prior.ContentTypes) {
			model.ContentTypes = prior.ContentTypes
		}
		if listUnset(prior.Extensions) {
			model.Extensions = prior.Extensions
		}
	}
	m.NestedModel = model

	tflog.Debug(ctx, "Flattened service gzip configuration state", map[string]any{
		"id":      id,
		"service": m.Service.ValueString(),
		"version": m.Version.ValueInt64(),
		"name":    m.Name.ValueString(),
	})
}
