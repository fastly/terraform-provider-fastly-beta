package ratelimiter

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"
)

func flatten(ctx context.Context, e *fastly.ERL, m *Model) {
	if e == nil {
		tflog.Warn(ctx, "flatten called with nil rate limiter")
		return
	}

	id := fastly.ToValue(e.ServiceID) + "/" + strconv.Itoa(fastly.ToValue(e.Version)) + "/" + fastly.ToValue(e.Name)
	m.ID = types.StringValue(id)
	m.Service = types.StringValue(fastly.ToValue(e.ServiceID))
	m.Version = types.Int64Value(int64(fastly.ToValue(e.Version)))

	m.NestedModel = ops{}.ToModel(e)

	tflog.Debug(ctx, "Flattened service rate limiter state", map[string]any{
		"id":      id,
		"service": m.Service.ValueString(),
		"version": m.Version.ValueInt64(),
		"name":    m.Name.ValueString(),
	})
}
