package requestsetting

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"
)

func flatten(ctx context.Context, rs *fastly.RequestSetting, m *Model) {
	if rs == nil {
		tflog.Warn(ctx, "flatten called with nil request setting")
		return
	}

	id := fastly.ToValue(rs.ServiceID) + "-" + strconv.Itoa(fastly.ToValue(rs.ServiceVersion)) + "-" + fastly.ToValue(rs.Name)
	m.ID = types.StringValue(id)
	m.Service = types.StringValue(fastly.ToValue(rs.ServiceID))
	m.Version = types.Int64Value(int64(fastly.ToValue(rs.ServiceVersion)))
	m.NestedModel = ops{}.ToModel(rs)

	tflog.Debug(ctx, "Flattened service request setting state", map[string]any{
		"id":      id,
		"service": m.Service.ValueString(),
		"version": m.Version.ValueInt64(),
		"name":    m.Name.ValueString(),
	})
}
