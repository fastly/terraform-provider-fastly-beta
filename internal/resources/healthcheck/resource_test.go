package healthcheck

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fastly "github.com/fastly/go-fastly/v17/fastly"
)

func TestModelEmbedding(t *testing.T) {
	nested := fullNestedModel()
	m := Model{
		NestedModel: nested,
		ID:          types.StringValue("test-id"),
		Service:     types.StringValue("test-service"),
		Version:     types.Int64Value(1),
	}

	assert.Equal(t, nested.Name, m.Name)
	assert.Equal(t, nested.Host, m.Host)
	assert.Equal(t, nested.Headers, m.Headers)

	assert.Equal(t, types.StringValue("test-id"), m.ID)
	assert.Equal(t, types.StringValue("test-service"), m.Service)
	assert.Equal(t, types.Int64Value(1), m.Version)

	assert.Equal(t, nested, m.NestedModel)
}

func TestFlatten(t *testing.T) {
	emptyHeaders := types.SetValueMust(types.StringType, []attr.Value{})

	tests := []struct {
		name  string
		hc    *fastly.HealthCheck
		prior *NestedModel
		want  func(t *testing.T, m *Model)
	}{
		{
			name: "nil health check leaves model untouched",
			hc:   nil,
			want: func(t *testing.T, m *Model) {
				assert.Equal(t, types.String{}, m.ID)
				assert.Equal(t, types.String{}, m.Service)
				assert.Equal(t, types.Int64{}, m.Version)
			},
		},
		{
			name: "health check with service metadata",
			hc: &fastly.HealthCheck{
				ServiceID:      new("service-123"),
				ServiceVersion: new(5),
				Name:           new("origin-check"),
				Host:           new("example.com"),
				Path:           new("/healthz"),
				HTTPVersion:    new("1.0"),
				Headers:        []string{"X-Api-Key: abc123"},
			},
			want: func(t *testing.T, m *Model) {
				assert.Equal(t, types.StringValue("service-123-5-origin-check"), m.ID)
				assert.Equal(t, types.StringValue("service-123"), m.Service)
				assert.Equal(t, types.Int64Value(5), m.Version)
				assert.Equal(t, types.StringValue("origin-check"), m.Name)
				assert.Equal(t, types.StringValue("1.0"), m.HTTPVersion)
				assert.Equal(t, types.Int64Value(DefaultCheckInterval), m.CheckInterval)
				assert.Len(t, m.Headers.Elements(), 1)
			},
		},
		{
			name:  "explicit empty headers in prior is kept when remote has none",
			hc:    &fastly.HealthCheck{ServiceID: new("s"), ServiceVersion: new(1), Name: new("hc")},
			prior: &NestedModel{Headers: emptyHeaders},
			want: func(t *testing.T, m *Model) {
				assert.True(t, emptyHeaders.Equal(m.Headers))
			},
		},
		{
			name:  "null headers in prior stays null when remote has none",
			hc:    &fastly.HealthCheck{ServiceID: new("s"), ServiceVersion: new(1), Name: new("hc")},
			prior: &NestedModel{Headers: types.SetNull(types.StringType)},
			want: func(t *testing.T, m *Model) {
				assert.True(t, m.Headers.IsNull())
			},
		},
		{
			name: "prior headers kept when they differ from remote only by go-fastly's colon-space rewrite",
			hc: &fastly.HealthCheck{
				ServiceID: new("s"), ServiceVersion: new(1), Name: new("hc"),
				Headers: []string{"X-Api-Key:abc123", "X-Env:test"},
			},
			prior: &NestedModel{Headers: types.SetValueMust(types.StringType, []attr.Value{
				types.StringValue("X-Api-Key: abc123"), types.StringValue("X-Env: test"),
			})},
			want: func(t *testing.T, m *Model) {
				assert.ElementsMatch(t, []string{"X-Api-Key: abc123", "X-Env: test"}, headersToSlice(m.Headers))
			},
		},
		{
			name: "remote headers win when the value really changed",
			hc: &fastly.HealthCheck{
				ServiceID: new("s"), ServiceVersion: new(1), Name: new("hc"),
				Headers: []string{"X-Api-Key:def456"},
			},
			prior: &NestedModel{Headers: types.SetValueMust(types.StringType, []attr.Value{types.StringValue("X-Api-Key: abc123")})},
			want: func(t *testing.T, m *Model) {
				assert.Equal(t, []string{"X-Api-Key:def456"}, headersToSlice(m.Headers))
			},
		},
		{
			name: "remote headers win over an empty prior",
			hc: &fastly.HealthCheck{
				ServiceID: new("s"), ServiceVersion: new(1), Name: new("hc"),
				Headers: []string{"X-Drift: 1"},
			},
			prior: &NestedModel{Headers: emptyHeaders},
			want: func(t *testing.T, m *Model) {
				assert.Len(t, m.Headers.Elements(), 1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{}
			flatten(context.Background(), tt.hc, m, tt.prior)
			tt.want(t, m)
		})
	}
}

func TestResourceAttributes(t *testing.T) {
	attrs := ResourceAttributes()

	for _, name := range []string{"id", "service_id", "version", "name", "host", "path", "headers"} {
		assert.Contains(t, attrs, name)
	}

	assert.True(t, attrs["id"].IsComputed())
	assert.True(t, attrs["service_id"].IsRequired())
	assert.True(t, attrs["version"].IsRequired())

	assert.Len(t, attrs["service_id"].(schema.StringAttribute).PlanModifiers, 1)
	assert.Len(t, attrs["name"].(schema.StringAttribute).PlanModifiers, 1)
	assert.Len(t, attrs["headers"].(schema.SetAttribute).PlanModifiers, 1)

	// The nested block's name and headers must stay free of plan modifiers.
	common := CommonAttributes()
	assert.Empty(t, common["name"].(schema.StringAttribute).PlanModifiers)
	assert.Empty(t, common["headers"].(schema.SetAttribute).PlanModifiers)
}

func TestCheckIntervalValidator(t *testing.T) {
	v := CommonAttributes()["check_interval"].(schema.Int64Attribute).Validators
	require.Len(t, v, 1)

	for _, tt := range []struct {
		value   int64
		wantErr bool
	}{
		{999, true},
		{1000, false},
		{DefaultCheckInterval, false},
		{3600000, false},
		{3600001, true},
	} {
		resp := &validator.Int64Response{}
		v[0].ValidateInt64(context.Background(), validator.Int64Request{
			Path:        path.Root("check_interval"),
			ConfigValue: types.Int64Value(tt.value),
		}, resp)
		assert.Equal(t, tt.wantErr, resp.Diagnostics.HasError(), "value %d", tt.value)
	}
}

func TestHTTPVersionValidator(t *testing.T) {
	v := CommonAttributes()["http_version"].(schema.StringAttribute).Validators
	require.Len(t, v, 1)

	for _, tt := range []struct {
		value   string
		wantErr bool
	}{
		{"1.0", false},
		{"1.1", false},
		{"2", true},
		{"HTTP/1.1", true},
	} {
		resp := &validator.StringResponse{}
		v[0].ValidateString(context.Background(), validator.StringRequest{
			Path:        path.Root("http_version"),
			ConfigValue: types.StringValue(tt.value),
		}, resp)
		assert.Equal(t, tt.wantErr, resp.Diagnostics.HasError(), "value %q", tt.value)
	}
}

func TestHeadersClearedRequiresReplace(t *testing.T) {
	some := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("X-Api-Key: abc123")})
	other := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("X-Api-Key: def456")})
	empty := types.SetValueMust(types.StringType, []attr.Value{})
	null := types.SetNull(types.StringType)

	tests := []struct {
		name  string
		state types.Set
		plan  types.Set
		want  bool
	}{
		{"headers added", null, some, false},
		{"headers changed", some, other, false},
		{"headers removed to null", some, null, true},
		{"headers removed to empty", some, empty, true},
		{"headers unknown", some, types.SetUnknown(types.StringType), false},
		{"no headers before or after", null, empty, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &setplanmodifier.RequiresReplaceIfFuncResponse{}
			headersClearedRequiresReplace(context.Background(), planmodifier.SetRequest{
				StateValue: tt.state,
				PlanValue:  tt.plan,
			}, resp)
			assert.Equal(t, tt.want, resp.RequiresReplace)
		})
	}
}
