package loggingfos

import (
	"context"
	"maps"

	"github.com/fastly/terraform-provider-fastly-beta/internal/logging"

	"github.com/fastly/terraform-provider-fastly-beta/internal/constants"
	"github.com/fastly/terraform-provider-fastly-beta/internal/reconcile"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"
	"github.com/fastly/terraform-provider-fastly-beta/internal/validation"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	fastly "github.com/fastly/go-fastly/v19/fastly"
)

const (
	DefaultFormatVersion = 2
	// DefaultGzipLevel is a sentinel for "gzip_level not configured"; -1 is distinct from an explicit 0.
	// An unset level is never sent because the API rejects it alongside compression_codec.
	DefaultGzipLevel         = -1
	DefaultMessageType       = "classic"
	DefaultPath              = ""
	DefaultPeriod            = 3600
	DefaultTimestampFormat   = "%Y-%m-%dT%H:%M:%S.000"
	DefaultCompressionCodec  = ""
	DefaultResponseCondition = ""
	DefaultPublicKey         = ""
	DefaultFileMaxBytes      = 0
)

// commonModel holds the attributes shared by VCL and Compute. The VCL-only attributes live on NestedModel.
type commonModel struct {
	Name             types.String `tfsdk:"name"`
	BucketName       types.String `tfsdk:"bucket_name"`
	Region           types.String `tfsdk:"region"`
	Path             types.String `tfsdk:"path"`
	Period           types.Int64  `tfsdk:"period"`
	GzipLevel        types.Int64  `tfsdk:"gzip_level"`
	CompressionCodec types.String `tfsdk:"compression_codec"`
	MessageType      types.String `tfsdk:"message_type"`
	TimestampFormat  types.String `tfsdk:"timestamp_format"`
	FileMaxBytes     types.Int64  `tfsdk:"file_max_bytes"`
	PublicKey        types.String `tfsdk:"public_key"`
}

// NestedModel is the model for the standalone resource and the VCL nested block.
type NestedModel struct {
	commonModel
	Format            types.String `tfsdk:"format"`
	FormatVersion     types.Int64  `tfsdk:"format_version"`
	Placement         types.String `tfsdk:"placement"`
	ResponseCondition types.String `tfsdk:"response_condition"`
}

// ComputeNestedModel is the Compute nested block model, without the VCL-only attributes.
type ComputeNestedModel struct {
	commonModel
}

func (n commonModel) equal(other commonModel) bool {
	return service.StringValue(n.Name) == service.StringValue(other.Name) &&
		service.StringValue(n.BucketName) == service.StringValue(other.BucketName) &&
		service.StringValue(n.Region) == service.StringValue(other.Region) &&
		service.StringValue(n.Path) == service.StringValue(other.Path) &&
		service.Int64Value(n.Period) == service.Int64Value(other.Period) &&
		service.Int64Value(n.GzipLevel) == service.Int64Value(other.GzipLevel) &&
		service.StringValue(n.CompressionCodec) == service.StringValue(other.CompressionCodec) &&
		service.StringValue(n.MessageType) == service.StringValue(other.MessageType) &&
		service.StringValue(n.TimestampFormat) == service.StringValue(other.TimestampFormat) &&
		service.Int64Value(n.FileMaxBytes) == service.Int64Value(other.FileMaxBytes) &&
		service.StringValue(n.PublicKey) == service.StringValue(other.PublicKey)
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

// CommonAttributes returns the shared and VCL-only attributes (standalone resource, VCL nested block).
func CommonAttributes() map[string]schema.Attribute {
	attrs := sharedAttributes()
	maps.Copy(attrs, vclOnlyAttributes())
	return attrs
}

// ComputeAttributes returns the shared attributes only.
func ComputeAttributes() map[string]schema.Attribute {
	return sharedAttributes()
}

func sharedAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"bucket_name": schema.StringAttribute{
			Required:    true,
			Description: "The name of the Fastly Object Storage bucket.",
		},
		"region": schema.StringAttribute{
			Required:    true,
			Description: "The region of the Fastly Object Storage bucket (e.g. `us-east`).",
		},
		"name": schema.StringAttribute{
			Required:    true,
			Description: "The name for the real-time logging configuration. Must be unique within the service.",
		},
		"compression_codec": schema.StringAttribute{
			Optional: true,
			Computed: true,
			Default:  stringdefault.StaticString(DefaultCompressionCodec),
			Validators: []validator.String{
				stringvalidator.OneOf("zstd", "snappy", "gzip"),
			},
			Description: "The codec used for compressing your logs. Valid values are `zstd`, `snappy`, and `gzip`. If the codec is `gzip`, `gzip_level` defaults to `3`; to use a different level, leave `compression_codec` unset and set `gzip_level` instead. Conflicts with `gzip_level`: setting both in the same request will result in an error.",
		},
		"file_max_bytes": schema.Int64Attribute{
			Optional:    true,
			Computed:    true,
			Default:     int64default.StaticInt64(DefaultFileMaxBytes),
			Description: "The maximum number of bytes for each uploaded file. A value of 0 can be used to indicate there is no limit on the size of uploaded files, otherwise the minimum value is 1048576 bytes (1 MiB.).",
		},
		"gzip_level": schema.Int64Attribute{
			Optional: true,
			Computed: true,
			Default:  int64default.StaticInt64(DefaultGzipLevel),
			// Validated against config, where an unset gzip_level is null rather than the -1 default.
			// The internal -1 sentinel is rejected by Between; omit the attribute instead.
			Validators: []validator.Int64{
				gzipLevelCodecConflict{},
				int64validator.Between(0, 9),
			},
			Description: "The level of gzip encoding when sending logs. Valid values are `0` (no compression) through `9`. To compress at a specific gzip level, leave `compression_codec` unset and set this. Conflicts with `compression_codec`: setting both in the same request will result in an error.",
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
		"path": schema.StringAttribute{
			Optional:    true,
			Computed:    true,
			Default:     stringdefault.StaticString(DefaultPath),
			Description: "Path to store the files. Must end with a trailing slash. If this field is left empty, the files will be saved in the bucket's root path.",
		},
		"period": schema.Int64Attribute{
			Optional:    true,
			Computed:    true,
			Default:     int64default.StaticInt64(DefaultPeriod),
			Description: "How frequently log files are finalized so they can be available for reading in seconds. Default `3600`.",
		},
		"public_key": schema.StringAttribute{
			Optional: true,
			Computed: true,
			Default:  stringdefault.StaticString(DefaultPublicKey),
			Validators: []validator.String{
				notTrimmed{},
			},
			Description: "PGP public key that Fastly will use to encrypt your log files before writing them to disk.",
		},
		"timestamp_format": schema.StringAttribute{
			Optional: true,
			Computed: true,
			Default:  stringdefault.StaticString(DefaultTimestampFormat),
			Validators: []validator.String{
				validation.NotBlank("timestamp_format"),
			},
			Description: "strftime-specified timestamp format for log filename.",
		},
	}
}

// vclOnlyAttributes returns the attributes that only affect generated VCL.
func vclOnlyAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"format": schema.StringAttribute{
			Optional: true,
			Computed: true,
			Default:  stringdefault.StaticString(constants.LoggingFOSDefaultFormat),
			Validators: []validator.String{
				stringvalidator.LengthAtMost(logging.MaximumFormatLength),
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
			Description: "The version of the custom logging format used for the configured endpoint. The logging call gets placed by default in vcl_log if format_version is set to `2` and in `vcl_deliver` if `format_version` is set to `1`.",
		},
		"placement": schema.StringAttribute{
			// Not Computed: unset (API auto-places) and "none" (suppresses the call) are distinct. The API
			// forces "none" on Compute services, so ResetVCLOnlyToDefaults discards it there.
			Optional: true,
			Validators: []validator.String{
				stringvalidator.OneOf("none"),
			},
			Description: "Where in the generated VCL the logging call should be placed. If not set, endpoints with format_version of 2 are placed in vcl_log and those with format_version of 1 are placed in vcl_deliver. Valid value is `none`.",
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
	// service_id and name locate the endpoint, so changing either forces replacement. version does not:
	// clone copies the endpoint forward. Applied here, not in CommonAttributes, so nested blocks are unaffected.
	nameAttr := attrs["name"].(schema.StringAttribute)
	nameAttr.PlanModifiers = []planmodifier.String{
		stringplanmodifier.RequiresReplace(),
	}
	attrs["name"] = nameAttr
	return attrs
}

// NestedBlockSchema returns the VCL nested block schema.
func NestedBlockSchema() schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: "Object Storage logging endpoints attached to this service.",
		NestedObject: schema.NestedBlockObject{
			Attributes: CommonAttributes(),
		},
	}
}

// ComputeNestedBlockSchema returns the Compute nested block schema.
func ComputeNestedBlockSchema() schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: "Object Storage logging endpoints attached to this service.",
		NestedObject: schema.NestedBlockObject{
			Attributes: ComputeAttributes(),
		},
	}
}

type endpointOps struct{}

func (o endpointOps) List(ctx context.Context, client *fastly.Client, serviceID string, version int) ([]*fastly.FOS, error) {
	return client.ListFOSes(ctx, &fastly.ListFOSesInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
	})
}

func (o endpointOps) GetName(api *fastly.FOS) string {
	return fastly.ToValue(api.Name)
}

func (o endpointOps) Delete(ctx context.Context, client *fastly.Client, serviceID string, version int, name string) error {
	return client.DeleteFOS(ctx, &fastly.DeleteFOSInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
		Name:           name,
	})
}

type ops struct{ endpointOps }

func (o ops) Create(ctx context.Context, client *fastly.Client, serviceID string, version int, desired NestedModel) (*fastly.FOS, error) {
	input := BuildCreateInput(serviceID, version, desired)
	return client.CreateFOS(ctx, input)
}

func (o ops) Equal(desired NestedModel, remote *fastly.FOS) bool {
	remoteModel := FlattenToNestedModel(remote)
	preserveGzipSentinel(&remoteModel, desired)
	return desired.ModelsEqual(remoteModel)
}

func (o ops) Update(ctx context.Context, client *fastly.Client, serviceID string, version int, desired NestedModel) (*fastly.FOS, error) {
	input := BuildUpdateInput(serviceID, version, desired)
	return client.UpdateFOS(ctx, input)
}

func (o ops) ToModel(api *fastly.FOS) NestedModel {
	return FlattenToNestedModel(api)
}

var reconciler = &reconcile.Resource[NestedModel, fastly.FOS]{
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

func ValidateConditionReferences(endpoints []NestedModel, conditionNames map[string]struct{}) error {
	return validation.SingleRef(endpoints, "Object Storage logging endpoint",
		func(m NestedModel) types.String { return m.Name },
		func(m NestedModel) types.String { return m.ResponseCondition },
		"response_condition", "condition", conditionNames)
}

func MatchOrder(items, order []NestedModel) []NestedModel {
	result := reconcile.MatchOrder(items, order, func(m NestedModel) string { return service.StringValue(m.Name) })
	// order holds the gzip_level sentinel for endpoints the user left unset.
	preserveGzipSentinelList(result, order)
	return result
}

type computeOps struct{ endpointOps }

func (o computeOps) Create(ctx context.Context, client *fastly.Client, serviceID string, version int, desired ComputeNestedModel) (*fastly.FOS, error) {
	input := BuildComputeCreateInput(serviceID, version, desired)
	return client.CreateFOS(ctx, input)
}

func (o computeOps) Equal(desired ComputeNestedModel, remote *fastly.FOS) bool {
	remoteModel := FlattenToComputeNestedModel(remote)
	preserveGzipSentinelCompute(&remoteModel, desired)
	return desired.ModelsEqual(remoteModel)
}

func (o computeOps) Update(ctx context.Context, client *fastly.Client, serviceID string, version int, desired ComputeNestedModel) (*fastly.FOS, error) {
	input := BuildComputeUpdateInput(serviceID, version, desired)
	return client.UpdateFOS(ctx, input)
}

func (o computeOps) ToModel(api *fastly.FOS) ComputeNestedModel {
	return FlattenToComputeNestedModel(api)
}

var computeReconciler = &reconcile.Resource[ComputeNestedModel, fastly.FOS]{
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
	result := reconcile.MatchOrder(items, order, func(m ComputeNestedModel) string { return service.StringValue(m.Name) })
	// order holds the gzip_level sentinel for endpoints the user left unset.
	preserveGzipSentinelListCompute(result, order)
	return result
}
