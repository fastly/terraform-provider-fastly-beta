package director

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"
)

func flatten(ctx context.Context, d *fastly.Director, m *Model) {
	if d == nil {
		tflog.Warn(ctx, "flatten called with nil director")
		return
	}

	id := fastly.ToValue(d.ServiceID) + "-" + strconv.Itoa(fastly.ToValue(d.ServiceVersion)) + "-" + fastly.ToValue(d.Name)
	m.ID = types.StringValue(id)
	m.Service = types.StringValue(fastly.ToValue(d.ServiceID))
	m.Version = types.Int64Value(int64(fastly.ToValue(d.ServiceVersion)))

	m.NestedModel = ops{}.ToModel(d)

	tflog.Debug(ctx, "Flattened service director state", map[string]any{
		"id":      id,
		"service": m.Service.ValueString(),
		"version": m.Version.ValueInt64(),
		"name":    m.Name.ValueString(),
	})
}
