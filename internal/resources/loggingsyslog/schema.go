package loggingsyslog

import (
	"context"
	"maps"

	"github.com/fastly/terraform-provider-fastly/internal/constants"
	"github.com/fastly/terraform-provider-fastly/internal/defaults"
	"github.com/fastly/terraform-provider-fastly/internal/reconcile"
	"github.com/fastly/terraform-provider-fastly/internal/service"
	"github.com/fastly/terraform-provider-fastly/internal/validation"

	fastly "github.com/fastly/go-fastly/v17/fastly"
	"github.com/hashicorp/terraform-plugin-framework/attr"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	fwdefaults "github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	DefaultFormatVersion     = 2
	DefaultResponseCondition = ""
	DefaultProcessingRegion  = "none"
	DefaultPort              = 514
	DefaultMessageType       = "classic"
	DefaultUseTLS            = false
	DefaultTLSHostname       = ""

	// maximumFormatLength is the maximum length the Fastly API accepts for a
	// logging endpoint `format` string. Exceeding it is only rejected by the
	// API at apply time, so it is enforced at plan/validate time instead.
	maximumFormatLength = 12288

	syslogTLSCACertEnvVar     = "FASTLY_SYSLOG_CA_CERT"
	syslogTLSClientCertEnvVar = "FASTLY_SYSLOG_CLIENT_CERT"
	syslogTLSClientKeyEnvVar  = "FASTLY_SYSLOG_CLIENT_KEY"
)

// commonModel holds the Syslog logging attributes shared by VCL and Compute
// services. format, format_version, placement, and response_condition only
// affect generated VCL, so they live on NestedModel only — Compute services use
// ComputeNestedModel, which embeds just this common set.
type commonModel struct {
	Name             types.String `tfsdk:"name"`
	Address          types.String `tfsdk:"address"`
	Port             types.Int64  `tfsdk:"port"`
	Authentication   types.Object `tfsdk:"authentication"`
	TLS              types.Object `tfsdk:"tls"`
	UseTLS           types.Bool   `tfsdk:"use_tls"`
	MessageType      types.String `tfsdk:"message_type"`
	ProcessingRegion types.String `tfsdk:"processing_region"`
}

// NestedModel is the Syslog logging model for the standalone
// fastly_service_logging_syslog resource and the VCL nested block
// (service_cdn_auto.logging_syslog).
type NestedModel struct {
	commonModel
	Format            types.String `tfsdk:"format"`
	FormatVersion     types.Int64  `tfsdk:"format_version"`
	Placement         types.String `tfsdk:"placement"`
	ResponseCondition types.String `tfsdk:"response_condition"`
}

// ComputeNestedModel is the Syslog logging model for the Compute nested block
// (service_compute_auto.logging_syslog). It omits format, format_version,
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

// defaultAuthenticationObject is the `authentication` block's value when the
// practitioner omits it entirely. Unlike some other logging endpoints, the
// live (SDKv2) provider has no environment variable for this token, so it
// simply defaults to "".
var defaultAuthenticationObject = NewAuthenticationObject(types.StringValue(""))

var tlsAttributeTypes = map[string]attr.Type{
	"ca_cert":     types.StringType,
	"client_cert": types.StringType,
	"client_key":  types.StringType,
	"hostname":    types.StringType,
}

func NewTLSObject(caCert, clientCert, clientKey, hostname types.String) types.Object {
	return types.ObjectValueMust(
		tlsAttributeTypes,
		map[string]attr.Value{
			"ca_cert":     caCert,
			"client_cert": clientCert,
			"client_key":  clientKey,
			"hostname":    hostname,
		},
	)
}

// tlsEnvDefault populates the `tls` object from the FASTLY_SYSLOG_CA_CERT,
// FASTLY_SYSLOG_CLIENT_CERT, and FASTLY_SYSLOG_CLIENT_KEY environment
// variables when the practitioner omits the whole `tls` block. The framework
// only walks into an object attribute's per-field Default handlers once the
// object itself already resolves to a known value; a Computed object
// attribute with no Default of its own is instead marked wholesale unknown,
// and its children's Defaults are never evaluated. Setting this Default on
// the parent gives the object a known value up front so the per-field
// defaults still run for a partially-configured object. hostname has no
// environment variable in the live provider, so it always defaults to "".
type tlsEnvDefault struct{}

func (tlsEnvDefault) Description(_ context.Context) string {
	return "ca_cert, client_cert, and client_key default to the FASTLY_SYSLOG_CA_CERT, FASTLY_SYSLOG_CLIENT_CERT, and FASTLY_SYSLOG_CLIENT_KEY environment variables"
}

func (d tlsEnvDefault) MarkdownDescription(ctx context.Context) string {
	return d.Description(ctx)
}

func (tlsEnvDefault) DefaultObject(ctx context.Context, _ fwdefaults.ObjectRequest, resp *fwdefaults.ObjectResponse) {
	resp.PlanValue = NewTLSObject(
		envStringDefault(ctx, syslogTLSCACertEnvVar),
		envStringDefault(ctx, syslogTLSClientCertEnvVar),
		envStringDefault(ctx, syslogTLSClientKeyEnvVar),
		types.StringValue(DefaultTLSHostname),
	)
}

func envStringDefault(ctx context.Context, envVar string) types.String {
	var resp fwdefaults.StringResponse
	defaults.EnvString(envVar, "").DefaultString(ctx, fwdefaults.StringRequest{}, &resp)
	return resp.PlanValue
}

func objectValue(obj types.Object, name string) types.String {
	if obj.IsNull() || obj.IsUnknown() {
		return types.StringValue("")
	}
	value, ok := obj.Attributes()[name]
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
	return objectValue(n.Authentication, "token")
}

func (n commonModel) TLSCACert() types.String {
	return objectValue(n.TLS, "ca_cert")
}

func (n commonModel) TLSClientCert() types.String {
	return objectValue(n.TLS, "client_cert")
}

func (n commonModel) TLSClientKey() types.String {
	return objectValue(n.TLS, "client_key")
}

func (n commonModel) TLSHostname() types.String {
	return objectValue(n.TLS, "hostname")
}

func (n commonModel) equal(other commonModel) bool {
	return service.StringValue(n.Name) == service.StringValue(other.Name) &&
		service.StringValue(n.Address) == service.StringValue(other.Address) &&
		service.Int64Value(n.Port) == service.Int64Value(other.Port) &&
		service.StringValue(n.Token()) == service.StringValue(other.Token()) &&
		service.StringValue(n.TLSCACert()) == service.StringValue(other.TLSCACert()) &&
		service.StringValue(n.TLSClientCert()) == service.StringValue(other.TLSClientCert()) &&
		service.StringValue(n.TLSClientKey()) == service.StringValue(other.TLSClientKey()) &&
		service.StringValue(n.TLSHostname()) == service.StringValue(other.TLSHostname()) &&
		service.BoolValue(n.UseTLS) == service.BoolValue(other.UseTLS) &&
		service.StringValue(n.MessageType) == service.StringValue(other.MessageType) &&
		service.StringValue(n.ProcessingRegion) == service.StringValue(other.ProcessingRegion)
}

func (n NestedModel) ModelsEqual(other NestedModel) bool {
	return n.commonModel.equal(other.commonModel) &&
		service.StringValue(n.Format) == service.StringValue(other.Format) &&
		service.Int64Value(n.FormatVersion) == service.Int64Value(other.FormatVersion) &&
		service.StringValue(n.Placement) == service.StringValue(other.Placement) &&
		service.StringValue(n.ResponseCondition) == service.StringValue(other.ResponseCondition)
}

func (c ComputeNestedModel) ModelsEqual(other ComputeNestedModel) bool {
	return c.commonModel.equal(other.commonModel)
}

// CommonAttributes returns the full Syslog logging attribute set — the shared
// attributes plus the VCL-only ones (format, format_version, placement,
// response_condition). Used by the standalone fastly_service_logging_syslog
// resource (which can attach to either service type) and the VCL nested block
// (NestedBlockSchema). Compute services use ComputeAttributes instead.
func CommonAttributes() map[string]schema.Attribute {
	attrs := sharedAttributes()
	maps.Copy(attrs, vclOnlyAttributes())
	return attrs
}

// ComputeAttributes returns the Syslog logging attribute set for Compute
// services, omitting the VCL-only attributes exposed by CommonAttributes.
func ComputeAttributes() map[string]schema.Attribute {
	return sharedAttributes()
}

// sharedAttributes returns the Syslog logging attributes common to both VCL
// and Compute services.
func sharedAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		// Required
		"name": schema.StringAttribute{
			Required:    true,
			Description: "The name for the real-time logging configuration. Must be unique within the service.",
		},
		"address": schema.StringAttribute{
			Required:    true,
			Description: "A hostname or IPv4 address of the Syslog endpoint.",
		},
		// Optional
		"port": schema.Int64Attribute{
			Optional:    true,
			Computed:    true,
			Default:     int64default.StaticInt64(DefaultPort),
			Description: "The port associated with the address where the Syslog endpoint can be accessed. Default `514`.",
		},
		// Grouped under `authentication` to match the other logging endpoints, even
		// though Syslog has a single credential.
		"authentication": schema.SingleNestedAttribute{
			Optional:    true,
			Computed:    true,
			Default:     objectdefault.StaticValue(defaultAuthenticationObject),
			Description: "Syslog authentication credentials.",
			Attributes: map[string]schema.Attribute{
				"token": schema.StringAttribute{
					Optional:    true,
					Computed:    true,
					Sensitive:   true,
					Default:     stringdefault.StaticString(""),
					Description: "Whether to prepend each message with a specific token.",
				},
			},
		},
		// Grouped under `tls` since client_key is credential material used to
		// authenticate this endpoint to the Syslog server via mutual TLS, and
		// ca_cert/client_cert/hostname are used alongside it to configure that same
		// connection.
		"tls": schema.SingleNestedAttribute{
			Optional:    true,
			Computed:    true,
			Default:     tlsEnvDefault{},
			Description: "TLS configuration used when `use_tls` is enabled. When this block is omitted entirely, `ca_cert`, `client_cert`, and `client_key` default to the `FASTLY_SYSLOG_CA_CERT`, `FASTLY_SYSLOG_CLIENT_CERT`, and `FASTLY_SYSLOG_CLIENT_KEY` environment variables.",
			Attributes: map[string]schema.Attribute{
				"ca_cert": schema.StringAttribute{
					Optional:    true,
					Computed:    true,
					Default:     defaults.EnvString(syslogTLSCACertEnvVar, ""),
					Description: "A secure certificate to authenticate the server with. Must be in PEM format. Can be set via the `FASTLY_SYSLOG_CA_CERT` environment variable.",
				},
				"client_cert": schema.StringAttribute{
					Optional:    true,
					Computed:    true,
					Default:     defaults.EnvString(syslogTLSClientCertEnvVar, ""),
					Description: "The client certificate used to make authenticated requests. Must be in PEM format. Can be set via the `FASTLY_SYSLOG_CLIENT_CERT` environment variable.",
				},
				"client_key": schema.StringAttribute{
					Optional:    true,
					Computed:    true,
					Sensitive:   true,
					Default:     defaults.EnvString(syslogTLSClientKeyEnvVar, ""),
					Description: "The client private key used to make authenticated requests. Must be in PEM format. Can be set via the `FASTLY_SYSLOG_CLIENT_KEY` environment variable.",
				},
				"hostname": schema.StringAttribute{
					Optional:    true,
					Computed:    true,
					Default:     stringdefault.StaticString(DefaultTLSHostname),
					Description: "The hostname used to verify the server's certificate. This should be one of the Subject Alternative Name (SAN) fields for the certificate. Common Names (CN) are not supported.",
				},
			},
		},
		"use_tls": schema.BoolAttribute{
			Optional:    true,
			Computed:    true,
			Default:     booldefault.StaticBool(DefaultUseTLS),
			Description: "Whether to use TLS for secure logging. Default: `false`.",
		},
		"message_type": schema.StringAttribute{
			Optional: true,
			Computed: true,
			Default:  stringdefault.StaticString(DefaultMessageType),
			Validators: []validator.String{
				stringvalidator.OneOf("classic", "loggly", "logplex", "blank"),
			},
			Description: "How the message should be formatted. Valid values are `classic`, `loggly`, `logplex`, and `blank`. Default `classic`.",
		},
		"processing_region": schema.StringAttribute{
			Optional: true,
			Computed: true,
			Default:  stringdefault.StaticString(DefaultProcessingRegion),
			Validators: []validator.String{
				stringvalidator.OneOf("none", "us", "eu"),
			},
			Description: "The geographic region where the logs will be processed before streaming. Valid values are `us`, `eu`, and `none` for global. Default: `none`.",
		},
	}
}

// vclOnlyAttributes returns the Syslog logging attributes that only affect
// generated VCL and have no meaning for Compute services.
func vclOnlyAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"format": schema.StringAttribute{
			Optional: true,
			Computed: true,
			Default:  stringdefault.StaticString(constants.LoggingSyslogDefaultFormat),
			Validators: []validator.String{
				stringvalidator.LengthAtMost(maximumFormatLength),
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

// NestedBlockSchema returns the Syslog logging nested block schema for VCL
// services (service_cdn_auto.logging_syslog), including the VCL-only
// attributes.
func NestedBlockSchema() schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: "Syslog logging endpoints attached to this service.",
		NestedObject: schema.NestedBlockObject{
			Attributes: CommonAttributes(),
		},
	}
}

// ComputeNestedBlockSchema returns the Syslog logging nested block schema for
// Compute services (service_compute_auto.logging_syslog), omitting the
// VCL-only attributes.
func ComputeNestedBlockSchema() schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: "Syslog logging endpoints attached to this service.",
		NestedObject: schema.NestedBlockObject{
			Attributes: ComputeAttributes(),
		},
	}
}

type ops struct{}

func (o ops) List(ctx context.Context, client *fastly.Client, serviceID string, version int) ([]*fastly.Syslog, error) {
	return client.ListSyslogs(ctx, &fastly.ListSyslogsInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
	})
}

func (o ops) GetName(api *fastly.Syslog) string {
	return fastly.ToValue(api.Name)
}

func (o ops) Delete(ctx context.Context, client *fastly.Client, serviceID string, version int, name string) error {
	return client.DeleteSyslog(ctx, &fastly.DeleteSyslogInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
		Name:           name,
	})
}

func (o ops) Create(ctx context.Context, client *fastly.Client, serviceID string, version int, desired NestedModel) (*fastly.Syslog, error) {
	input := BuildCreateInput(serviceID, version, desired)
	return client.CreateSyslog(ctx, input)
}

func (o ops) Equal(desired NestedModel, remote *fastly.Syslog) bool {
	return desired.ModelsEqual(FlattenToNestedModel(remote))
}

func (o ops) Update(ctx context.Context, client *fastly.Client, serviceID string, version int, desired NestedModel) (*fastly.Syslog, error) {
	return client.UpdateSyslog(ctx, BuildUpdateInput(serviceID, version, desired))
}

func (o ops) ToModel(api *fastly.Syslog) NestedModel {
	return FlattenToNestedModel(api)
}

var reconciler = &reconcile.Resource[NestedModel, fastly.Syslog]{
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
	return validation.References(endpoints, "Syslog logging endpoint", func(m NestedModel) types.String { return m.Name }, "response_condition",
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

func (o computeOps) List(ctx context.Context, client *fastly.Client, serviceID string, version int) ([]*fastly.Syslog, error) {
	return client.ListSyslogs(ctx, &fastly.ListSyslogsInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
	})
}

func (o computeOps) GetName(api *fastly.Syslog) string {
	return fastly.ToValue(api.Name)
}

func (o computeOps) Delete(ctx context.Context, client *fastly.Client, serviceID string, version int, name string) error {
	return client.DeleteSyslog(ctx, &fastly.DeleteSyslogInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
		Name:           name,
	})
}

func (o computeOps) Create(ctx context.Context, client *fastly.Client, serviceID string, version int, desired ComputeNestedModel) (*fastly.Syslog, error) {
	input := BuildComputeCreateInput(serviceID, version, desired)
	return client.CreateSyslog(ctx, input)
}

func (o computeOps) Equal(desired ComputeNestedModel, remote *fastly.Syslog) bool {
	return desired.ModelsEqual(FlattenToComputeNestedModel(remote))
}

func (o computeOps) Update(ctx context.Context, client *fastly.Client, serviceID string, version int, desired ComputeNestedModel) (*fastly.Syslog, error) {
	return client.UpdateSyslog(ctx, BuildComputeUpdateInput(serviceID, version, desired))
}

func (o computeOps) ToModel(api *fastly.Syslog) ComputeNestedModel {
	return FlattenToComputeNestedModel(api)
}

var computeReconciler = &reconcile.Resource[ComputeNestedModel, fastly.Syslog]{
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
