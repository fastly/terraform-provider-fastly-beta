package loggingheroku

import (
	"context"
	"maps"

	"github.com/fastly/terraform-provider-fastly-beta/internal/constants"
	"github.com/fastly/terraform-provider-fastly-beta/internal/reconcile"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"
	"github.com/fastly/terraform-provider-fastly-beta/internal/validation"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	fastly "github.com/fastly/go-fastly/v17/fastly"
)

const (
	DefaultFormatVersion     = 2
	DefaultResponseCondition = ""
	DefaultProcessingRegion  = "none"

	// maximumFormatLength is the maximum length the Fastly API accepts for a
	// logging endpoint `format` string. Exceeding it is only rejected by the
	// API at apply time, so it is enforced at plan/validate time instead.
	maximumFormatLength = 12288
)

// commonModel holds the Heroku logging attributes shared by VCL and Compute
// services. format, format_version, placement, and response_condition only
// affect generated VCL, so they live on NestedModel only — Compute services use
// ComputeNestedModel, which embeds just this common set.
type commonModel struct {
	Name             types.String `tfsdk:"name"`
	URL              types.String `tfsdk:"url"`
	Authentication   types.Object `tfsdk:"authentication"`
	ProcessingRegion types.String `tfsdk:"processing_region"`
}

// NestedModel is the Heroku logging model for the standalone
// fastly_service_logging_heroku resource and the VCL nested block
// (service_cdn_auto.logging_heroku).
type NestedModel struct {
	commonModel
	Format            types.String `tfsdk:"format"`
	FormatVersion     types.Int64  `tfsdk:"format_version"`
	Placement         types.String `tfsdk:"placement"`
	ResponseCondition types.String `tfsdk:"response_condition"`
}

// ComputeNestedModel is the Heroku logging model for the Compute nested block
// (service_compute_auto.logging_heroku). It omits format, format_version,
// placement, and response_condition, which only apply to VCL services.
type ComputeNestedModel struct {
	commonModel
}

var authenticationAttributeTypes = map[string]attr.Type{
	"token": types.StringType,
}

func NewAuthenticationObject(token types.String) types.Object {
	return types.ObjectValueMust(
		authenticationAttributeTypes,
		map[string]attr.Value{
			"token": token,
		},
	)
}

func authenticationValue(auth types.Object, name string) types.String {
	if auth.IsNull() || auth.IsUnknown() {
		return types.StringValue("")
	}
	value, ok := auth.Attributes()[name]
	if !ok || value == nil || value.IsNull() || value.IsUnknown() {
		return types.StringValue("")
	}
	stringValue, ok := value.(types.String)
	if !ok {
		return types.StringValue("")
	}
	return stringValue
}

func (n commonModel) Token() types.String {
	return authenticationValue(n.Authentication, "token")
}

func (n commonModel) equal(other commonModel) bool {
	return service.StringValue(n.Name) == service.StringValue(other.Name) &&
		service.StringValue(n.URL) == service.StringValue(other.URL) &&
		service.StringValue(n.Token()) == service.StringValue(other.Token()) &&
		service.StringValue(n.ProcessingRegion) == service.StringValue(other.ProcessingRegion)
}

func (n NestedModel) ModelsEqual(other NestedModel) bool {
	return n.equal(other.commonModel) &&
		service.StringValue(n.Format) == service.StringValue(other.Format) &&
		service.Int64Value(n.FormatVersion) == service.Int64Value(other.FormatVersion) &&
		service.StringValue(n.Placement) == service.StringValue(other.Placement) &&
		service.StringValue(n.ResponseCondition) == service.StringValue(other.ResponseCondition)
}

func (c ComputeNestedModel) ModelsEqual(other ComputeNestedModel) bool {
	return c.equal(other.commonModel)
}

// CommonAttributes returns the full Heroku logging attribute set — the shared
// attributes plus the VCL-only ones (format, format_version, placement,
// response_condition). Used by the standalone fastly_service_logging_heroku
// resource (which can attach to either service type) and the VCL nested block
// (NestedBlockSchema). Compute services use ComputeAttributes instead.
func CommonAttributes() map[string]schema.Attribute {
	attrs := sharedAttributes()
	maps.Copy(attrs, vclOnlyAttributes())
	return attrs
}

// ComputeAttributes returns the Heroku logging attribute set for Compute
// services, omitting the VCL-only attributes exposed by CommonAttributes.
func ComputeAttributes() map[string]schema.Attribute {
	return sharedAttributes()
}

// sharedAttributes returns the Heroku logging attributes common to both VCL
// and Compute services.
func sharedAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		// Required
		"name": schema.StringAttribute{
			Required:    true,
			Description: "The name for the real-time logging configuration. Must be unique within the service.",
		},
		"url": schema.StringAttribute{
			Required:    true,
			Description: "The URL to stream logs to.",
		},
		// Grouped under `authentication` to match loggings3/loggingdatadog, even
		// though Heroku has a single credential.
		"authentication": schema.SingleNestedAttribute{
			Required:    true,
			Description: "Heroku authentication credentials.",
			Attributes: map[string]schema.Attribute{
				"token": schema.StringAttribute{
					Required:    true,
					Sensitive:   true,
					Description: "The token to use for authentication. See [Heroku's log integration documentation](https://devcenter.heroku.com/articles/add-on-partner-log-integration).",
				},
			},
		},
		// Optional
		"processing_region": schema.StringAttribute{
			Optional: true,
			Computed: true,
			Default:  stringdefault.StaticString(DefaultProcessingRegion),
			Validators: []validator.String{
				stringvalidator.OneOf("none", "us", "eu"),
			},
			Description: "The geographic region where the logs will be processed before streaming to Heroku. Valid values are `us`, `eu`, and `none` for global. Default: `none`.",
		},
	}
}

// vclOnlyAttributes returns the Heroku logging attributes that only affect
// generated VCL and have no meaning for Compute services.
func vclOnlyAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"format": schema.StringAttribute{
			Optional: true,
			Computed: true,
			Default:  stringdefault.StaticString(constants.LoggingHerokuDefaultFormat),
			Validators: []validator.String{
				stringvalidator.LengthAtMost(maximumFormatLength),
				validation.NotBlank("format"),
			},
			Description: "A Fastly [log format string](https://www.fastly.com/documentation/guides/integrations/streaming-logs/custom-log-formats/).",
		},
		"format_version": schema.Int64Attribute{
			Optional: true,
			Computed: true,
			Default:  int64default.StaticInt64(DefaultFormatVersion),
			Validators: []validator.Int64{
				int64validator.Between(1, 2),
			},
			Description: "The version of the custom logging format used for the configured endpoint. The logging call gets placed by default in `vcl_log` if `format_version` is set to `2` and in `vcl_deliver` if `format_version` is set to `1`.",
		},
		"placement": schema.StringAttribute{
			// Not Computed: unset and explicitly "none" are distinct states on the
			// API (unset lets the API auto-place the logging call; "none" suppresses
			// it entirely). Keeping it plain Optional means removing placement from
			// config is a real, visible diff from Terraform core's own plan proposal,
			// with no plan modifier required to force it.
			//
			// On a VCL service the API resolves this to exactly what was configured.
			// Compute (wasm) services are the exception — the API forces "none" there
			// regardless of what was sent, which can never match the planned null, so
			// ResetVCLOnlyToDefaults discards it after apply for those services.
			Optional: true,
			Validators: []validator.String{
				stringvalidator.OneOf("none"),
			},
			Description: "Where in the generated VCL the logging call should be placed. If not set, endpoints with `format_version` of `2` are placed in `vcl_log` and those with `format_version` of `1` are placed in `vcl_deliver`. Valid value is `none`.",
		},
		"response_condition": schema.StringAttribute{
			Optional:    true,
			Computed:    true,
			Default:     stringdefault.StaticString(DefaultResponseCondition),
			Description: "The name of an existing condition in the configured endpoint, or leave blank to always execute.",
		},
	}
}

func ResourceAttributes() map[string]schema.Attribute {
	attrs := map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "Terraform resource identifier.",
		},
		"service_id": schema.StringAttribute{
			Required:    true,
			Description: "Fastly service ID.",
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"version": schema.Int64Attribute{
			Required:    true,
			Description: "Writable Fastly service version to modify.",
		},
	}
	maps.Copy(attrs, CommonAttributes())
	// For the standalone resource, service_id + name locate the endpoint in the
	// API, so a change to either cannot be an in-place update. version is not
	// replacement-forcing: the explicit clone workflow copies the endpoint into
	// the new version, so an in-place update there succeeds. Applied to name
	// here (not in CommonAttributes) so the nested block, where name is only a
	// list key, is unaffected.
	nameAttr := attrs["name"].(schema.StringAttribute)
	nameAttr.PlanModifiers = []planmodifier.String{
		stringplanmodifier.RequiresReplace(),
	}
	attrs["name"] = nameAttr
	return attrs
}

// NestedBlockSchema returns the Heroku logging nested block schema for VCL
// services (service_cdn_auto.logging_heroku), including the VCL-only
// attributes.
func NestedBlockSchema() schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: "Heroku logging endpoints attached to this service.",
		NestedObject: schema.NestedBlockObject{
			Attributes: CommonAttributes(),
		},
	}
}

// ComputeNestedBlockSchema returns the Heroku logging nested block schema for
// Compute services (service_compute_auto.logging_heroku), omitting the
// VCL-only attributes.
func ComputeNestedBlockSchema() schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: "Heroku logging endpoints attached to this service.",
		NestedObject: schema.NestedBlockObject{
			Attributes: ComputeAttributes(),
		},
	}
}

type ops struct{}

func (o ops) List(ctx context.Context, client *fastly.Client, serviceID string, version int) ([]*fastly.Heroku, error) {
	return client.ListHerokus(ctx, &fastly.ListHerokusInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
	})
}

func (o ops) GetName(api *fastly.Heroku) string {
	return fastly.ToValue(api.Name)
}

func (o ops) Delete(ctx context.Context, client *fastly.Client, serviceID string, version int, name string) error {
	return client.DeleteHeroku(ctx, &fastly.DeleteHerokuInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
		Name:           name,
	})
}

func (o ops) Create(ctx context.Context, client *fastly.Client, serviceID string, version int, desired NestedModel) (*fastly.Heroku, error) {
	input := BuildCreateInput(serviceID, version, desired)
	return client.CreateHeroku(ctx, input)
}

func (o ops) Equal(desired NestedModel, remote *fastly.Heroku) bool {
	return desired.ModelsEqual(FlattenToNestedModel(remote))
}

func (o ops) Update(ctx context.Context, client *fastly.Client, serviceID string, version int, desired NestedModel) (*fastly.Heroku, error) {
	input := BuildUpdateInput(serviceID, version, desired)
	return client.UpdateHeroku(ctx, input)
}

func (o ops) ToModel(api *fastly.Heroku) NestedModel {
	return FlattenToNestedModel(api)
}

var reconciler = &reconcile.Resource[NestedModel, fastly.Heroku]{
	Ops: ops{},
	GetName: func(m NestedModel) string {
		return service.StringValue(m.Name)
	},
	Sortable: true,
}

func ReadForVersion(ctx context.Context, client *fastly.Client, serviceID string, version int) ([]NestedModel, error) {
	return reconciler.ReadForVersion(ctx, client, serviceID, version)
}

func Reconcile(ctx context.Context, client *fastly.Client, serviceID string, version int, desired []NestedModel) error {
	return reconciler.Run(ctx, client, serviceID, version, desired)
}

func Equal(a, b []NestedModel) bool {
	return reconcile.ModelsEqual(a, b, func(m NestedModel) string { return service.StringValue(m.Name) }, NestedModel.ModelsEqual, true)
}

// ValidateConditionReferences rejects a response_condition naming a condition block absent from config.
func ValidateConditionReferences(endpoints []NestedModel, conditionNames map[string]struct{}) error {
	return validation.References(endpoints, "Heroku logging endpoint", func(m NestedModel) types.String { return m.Name }, "response_condition",
		func(m NestedModel) []string {
			if m.ResponseCondition.IsUnknown() || m.ResponseCondition.IsNull() {
				return nil
			}
			return []string{service.StringValue(m.ResponseCondition)}
		},
		"condition", conditionNames)
}

func MatchOrder(items, order []NestedModel) []NestedModel {
	return reconcile.MatchOrder(items, order, func(m NestedModel) string { return service.StringValue(m.Name) })
}

type computeOps struct{}

func (o computeOps) List(ctx context.Context, client *fastly.Client, serviceID string, version int) ([]*fastly.Heroku, error) {
	return client.ListHerokus(ctx, &fastly.ListHerokusInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
	})
}

func (o computeOps) GetName(api *fastly.Heroku) string {
	return fastly.ToValue(api.Name)
}

func (o computeOps) Delete(ctx context.Context, client *fastly.Client, serviceID string, version int, name string) error {
	return client.DeleteHeroku(ctx, &fastly.DeleteHerokuInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
		Name:           name,
	})
}

func (o computeOps) Create(ctx context.Context, client *fastly.Client, serviceID string, version int, desired ComputeNestedModel) (*fastly.Heroku, error) {
	input := BuildComputeCreateInput(serviceID, version, desired)
	return client.CreateHeroku(ctx, input)
}

func (o computeOps) Equal(desired ComputeNestedModel, remote *fastly.Heroku) bool {
	return desired.ModelsEqual(FlattenToComputeNestedModel(remote))
}

func (o computeOps) Update(ctx context.Context, client *fastly.Client, serviceID string, version int, desired ComputeNestedModel) (*fastly.Heroku, error) {
	input := BuildComputeUpdateInput(serviceID, version, desired)
	return client.UpdateHeroku(ctx, input)
}

func (o computeOps) ToModel(api *fastly.Heroku) ComputeNestedModel {
	return FlattenToComputeNestedModel(api)
}

var computeReconciler = &reconcile.Resource[ComputeNestedModel, fastly.Heroku]{
	Ops: computeOps{},
	GetName: func(m ComputeNestedModel) string {
		return service.StringValue(m.Name)
	},
	Sortable: true,
}

func ComputeReadForVersion(ctx context.Context, client *fastly.Client, serviceID string, version int) ([]ComputeNestedModel, error) {
	return computeReconciler.ReadForVersion(ctx, client, serviceID, version)
}

func ComputeReconcile(ctx context.Context, client *fastly.Client, serviceID string, version int, desired []ComputeNestedModel) error {
	return computeReconciler.Run(ctx, client, serviceID, version, desired)
}

func ComputeEqual(a, b []ComputeNestedModel) bool {
	return reconcile.ModelsEqual(a, b, func(m ComputeNestedModel) string { return service.StringValue(m.Name) }, ComputeNestedModel.ModelsEqual, true)
}

func ComputeMatchOrder(items, order []ComputeNestedModel) []ComputeNestedModel {
	return reconcile.MatchOrder(items, order, func(m ComputeNestedModel) string { return service.StringValue(m.Name) })
}
