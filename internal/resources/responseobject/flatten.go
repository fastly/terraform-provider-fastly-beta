package responseobject

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v17/fastly"
)

func flatten(ctx context.Context, ro *fastly.ResponseObject, m *Model) {
	if ro == nil {
		tflog.Warn(ctx, "flatten called with nil response object")
		return
	}

	id := fastly.ToValue(ro.ServiceID) + "-" + strconv.Itoa(fastly.ToValue(ro.ServiceVersion)) + "-" + fastly.ToValue(ro.Name)
	m.ID = types.StringValue(id)
	m.Service = types.StringValue(fastly.ToValue(ro.ServiceID))
	m.Version = types.Int64Value(int64(fastly.ToValue(ro.ServiceVersion)))
	m.NestedModel = ops{}.ToModel(ro)

	tflog.Debug(ctx, "Flattened service response object state", map[string]any{
		"id":      id,
		"service": m.Service.ValueString(),
		"version": m.Version.ValueInt64(),
		"name":    m.Name.ValueString(),
	})
}
