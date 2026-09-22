package cachesetting

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"
)

func flatten(ctx context.Context, c *fastly.CacheSetting, m *Model) {
	if c == nil {
		tflog.Warn(ctx, "flatten called with nil cache setting")
		return
	}

	id := fastly.ToValue(c.ServiceID) + "-" + strconv.Itoa(fastly.ToValue(c.ServiceVersion)) + "-" + fastly.ToValue(c.Name)
	m.ID = types.StringValue(id)
	m.Service = types.StringValue(fastly.ToValue(c.ServiceID))
	m.Version = types.Int64Value(int64(fastly.ToValue(c.ServiceVersion)))

	m.NestedModel = ops{}.ToModel(c)

	tflog.Debug(ctx, "Flattened service cache setting state", map[string]any{
		"id":      id,
		"service": m.Service.ValueString(),
		"version": m.Version.ValueInt64(),
		"name":    m.Name.ValueString(),
	})
}
