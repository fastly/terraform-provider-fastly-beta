package settings

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	DefaultDefaultHost     = ""
	DefaultDefaultTTL      = 3600
	DefaultHTTP3           = false
	DefaultStaleIfError    = false
	DefaultStaleIfErrorTTL = 43200
	http3FeatureRevision   = 1
)

type NestedModel struct {
	DefaultHost     types.String `tfsdk:"default_host"`
	DefaultTTL      types.Int64  `tfsdk:"default_ttl"`
	HTTP3           types.Bool   `tfsdk:"http3"`
	StaleIfError    types.Bool   `tfsdk:"stale_if_error"`
	StaleIfErrorTTL types.Int64  `tfsdk:"stale_if_error_ttl"`
}

func (n NestedModel) ModelsEqual(other NestedModel) bool {
	return service.StringValue(n.DefaultHost) == service.StringValue(other.DefaultHost) &&
		service.Int64Value(n.DefaultTTL) == service.Int64Value(other.DefaultTTL) &&
		service.BoolValue(n.HTTP3) == service.BoolValue(other.HTTP3) &&
		service.BoolValue(n.StaleIfError) == service.BoolValue(other.StaleIfError) &&
		service.Int64Value(n.StaleIfErrorTTL) == service.Int64Value(other.StaleIfErrorTTL)
}

func defaultNestedModel() NestedModel {
	return NestedModel{
		DefaultHost:     types.StringValue(DefaultDefaultHost),
		DefaultTTL:      types.Int64Value(DefaultDefaultTTL),
		HTTP3:           types.BoolValue(DefaultHTTP3),
		StaleIfError:    types.BoolValue(DefaultStaleIfError),
		StaleIfErrorTTL: types.Int64Value(DefaultStaleIfErrorTTL),
	}
}

func CommonAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"default_host": schema.StringAttribute{
			Optional:    true,
			Computed:    true,
			Default:     stringdefault.StaticString(DefaultDefaultHost),
			Description: "The default hostname.",
		},
		"default_ttl": schema.Int64Attribute{
			Optional:    true,
			Computed:    true,
			Default:     int64default.StaticInt64(DefaultDefaultTTL),
			Description: "The default Time-to-live (TTL) for requests. Default `3600`.",
			Validators: []validator.Int64{
				int64validator.AtLeast(0),
			},
		},
		"http3": schema.BoolAttribute{
			Optional:    true,
			Computed:    true,
			Default:     booldefault.StaticBool(DefaultHTTP3),
			Description: "Enables support for the HTTP/3 (QUIC) protocol. Default `false`.",
		},
		"stale_if_error": schema.BoolAttribute{
			Optional:    true,
			Computed:    true,
			Default:     booldefault.StaticBool(DefaultStaleIfError),
			Description: "Enables serving a stale object if there is an error. Default `false`.",
		},
		"stale_if_error_ttl": schema.Int64Attribute{
			Optional:    true,
			Computed:    true,
			Default:     int64default.StaticInt64(DefaultStaleIfErrorTTL),
			Description: "The default time-to-live (TTL) for serving the stale object for the version. Default `43200`.",
			Validators: []validator.Int64{
				int64validator.AtLeast(0),
			},
		},
	}
}

// NestedBlockSchema returns the settings block for use inside _auto aggregate resources. At
// most one block is supported per service, since these settings are a singleton per service
// version.
func NestedBlockSchema() schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: "General settings for this service version. At most one block is supported. " +
			"Removing this block from configuration resets these settings back to their API defaults.",
		NestedObject: schema.NestedBlockObject{
			Attributes: CommonAttributes(),
		},
		Validators: []validator.List{
			listvalidator.SizeAtMost(1),
		},
	}
}

func Equal(a, b []NestedModel) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 {
		return true
	}
	return a[0].ModelsEqual(b[0])
}
