package responseobject

import (
	"context"

	"github.com/fastly/terraform-provider-fastly-beta/internal/reconcile"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"
	"github.com/fastly/terraform-provider-fastly-beta/internal/validation"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	fastly "github.com/fastly/go-fastly/v17/fastly"
)

const (
	DefaultResponse = "OK"
	DefaultStatus   = 200
)

type NestedModel struct {
	Name             types.String `tfsdk:"name"`
	CacheCondition   types.String `tfsdk:"cache_condition"`
	Content          types.String `tfsdk:"content"`
	ContentType      types.String `tfsdk:"content_type"`
	RequestCondition types.String `tfsdk:"request_condition"`
	Response         types.String `tfsdk:"response"`
	Status           types.Int64  `tfsdk:"status"`
}

func (n NestedModel) ModelsEqual(other NestedModel) bool {
	return service.StringValue(n.Name) == service.StringValue(other.Name) &&
		service.StringValue(n.CacheCondition) == service.StringValue(other.CacheCondition) &&
		service.StringValue(n.Content) == service.StringValue(other.Content) &&
		service.StringValue(n.ContentType) == service.StringValue(other.ContentType) &&
		service.StringValue(n.RequestCondition) == service.StringValue(other.RequestCondition) &&
		service.StringValue(n.Response) == service.StringValue(other.Response) &&
		service.Int64Value(n.Status) == service.Int64Value(other.Status)
}

func CommonAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name": schema.StringAttribute{
			Required:    true,
			Description: "A unique name to identify this Response Object. Changing this attribute will delete and recreate the resource.",
		},
		"cache_condition": schema.StringAttribute{
			Optional:    true,
			Computed:    true,
			Default:     stringdefault.StaticString(""),
			Description: "Name of already defined `condition` to check after we have retrieved an object. If the condition passes then deliver this Response Object instead. This `condition` must be of type `CACHE`. For detailed information about Conditionals, see [Fastly's Documentation on Conditionals](https://docs.fastly.com/en/guides/using-conditions)",
		},
		"content": schema.StringAttribute{
			Optional:    true,
			Computed:    true,
			Default:     stringdefault.StaticString(""),
			Description: "The content to deliver for the response object.",
		},
		"content_type": schema.StringAttribute{
			Optional: true,
			Computed: true,
			// content_type is nullable on the API and omitted from create/read responses when
			// unset; a static default would diverge from what flatten writes to state.
			// UseStateForUnknown preserves the prior state value when the field is omitted from
			// config, preventing (known after apply) drift on unrelated updates.
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
			Description: "The MIME type of the content, can be empty.",
		},
		"request_condition": schema.StringAttribute{
			Optional:    true,
			Computed:    true,
			Default:     stringdefault.StaticString(""),
			Description: "Name of already defined `condition` to be checked during the request phase. If the condition passes then this object will be delivered. This `condition` must be of type `REQUEST`.",
		},
		"response": schema.StringAttribute{
			Optional:    true,
			Computed:    true,
			Default:     stringdefault.StaticString(DefaultResponse),
			Description: "The HTTP Response. Default `OK`.",
		},
		"status": schema.Int64Attribute{
			Optional:    true,
			Computed:    true,
			Default:     int64default.StaticInt64(DefaultStatus),
			Description: "The HTTP Status Code. Default `200`.",
		},
	}
}

func NestedBlockSchema() schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: "Response objects attached to this service.",
		NestedObject: schema.NestedBlockObject{
			Attributes: CommonAttributes(),
		},
	}
}

type ops struct{}

func (o ops) List(ctx context.Context, client *fastly.Client, serviceID string, version int) ([]*fastly.ResponseObject, error) {
	return client.ListResponseObjects(ctx, &fastly.ListResponseObjectsInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
	})
}

func (o ops) GetName(api *fastly.ResponseObject) string {
	return fastly.ToValue(api.Name)
}

func (o ops) Delete(ctx context.Context, client *fastly.Client, serviceID string, version int, name string) error {
	return client.DeleteResponseObject(ctx, &fastly.DeleteResponseObjectInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
		Name:           name,
	})
}

func (o ops) Create(ctx context.Context, client *fastly.Client, serviceID string, version int, desired NestedModel) (*fastly.ResponseObject, error) {
	return client.CreateResponseObject(ctx, &fastly.CreateResponseObjectInput{
		ServiceID:        serviceID,
		ServiceVersion:   version,
		Name:             new(service.StringValue(desired.Name)),
		CacheCondition:   new(service.StringValue(desired.CacheCondition)),
		Content:          new(service.StringValue(desired.Content)),
		ContentType:      fastly.NullString(service.StringValue(desired.ContentType)),
		RequestCondition: new(service.StringValue(desired.RequestCondition)),
		Response:         new(service.StringValue(desired.Response)),
		Status:           new(int(service.Int64Value(desired.Status))),
	})
}

func (o ops) Equal(desired NestedModel, remote *fastly.ResponseObject) bool {
	return desired.ModelsEqual(o.ToModel(remote))
}

func (o ops) Update(ctx context.Context, client *fastly.Client, serviceID string, version int, desired NestedModel) (*fastly.ResponseObject, error) {
	return client.UpdateResponseObject(ctx, &fastly.UpdateResponseObjectInput{
		ServiceID:        serviceID,
		ServiceVersion:   version,
		Name:             service.StringValue(desired.Name),
		CacheCondition:   new(service.StringValue(desired.CacheCondition)),
		Content:          new(service.StringValue(desired.Content)),
		ContentType:      fastly.NullString(service.StringValue(desired.ContentType)),
		RequestCondition: new(service.StringValue(desired.RequestCondition)),
		Response:         new(service.StringValue(desired.Response)),
		Status:           new(int(service.Int64Value(desired.Status))),
	})
}

func (o ops) ToModel(api *fastly.ResponseObject) NestedModel {
	return NestedModel{
		Name:             types.StringValue(fastly.ToValue(api.Name)),
		CacheCondition:   service.StringPointerOrDefault(api.CacheCondition, ""),
		Content:          service.StringPointerOrDefault(api.Content, ""),
		ContentType:      service.StringPointerOrNull(api.ContentType),
		RequestCondition: service.StringPointerOrDefault(api.RequestCondition, ""),
		Response:         service.StringPointerOrDefault(api.Response, DefaultResponse),
		Status:           service.Int64PointerOrDefault(api.Status, DefaultStatus),
	}
}

var reconciler = &reconcile.Resource[NestedModel, fastly.ResponseObject]{
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

func MatchOrder(items, order []NestedModel) []NestedModel {
	return reconcile.MatchOrder(items, order, func(m NestedModel) string { return service.StringValue(m.Name) })
}

// ValidateConditionReferences rejects a cache_condition/request_condition naming a condition block absent from config.
func ValidateConditionReferences(responseObjects []NestedModel, conditionNames map[string]struct{}) error {
	getName := func(m NestedModel) types.String { return m.Name }

	if err := validation.References(responseObjects, "response object", getName, "cache_condition",
		func(m NestedModel) []string {
			if m.CacheCondition.IsUnknown() || m.CacheCondition.IsNull() {
				return nil
			}
			return []string{service.StringValue(m.CacheCondition)}
		},
		"condition", conditionNames); err != nil {
		return err
	}

	return validation.References(responseObjects, "response object", getName, "request_condition",
		func(m NestedModel) []string {
			if m.RequestCondition.IsUnknown() || m.RequestCondition.IsNull() {
				return nil
			}
			return []string{service.StringValue(m.RequestCondition)}
		},
		"condition", conditionNames)
}
