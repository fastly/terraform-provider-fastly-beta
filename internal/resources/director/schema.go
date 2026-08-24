package director

import (
	"context"
	"fmt"
	"sort"

	"github.com/fastly/terraform-provider-fastly/internal/reconcile"
	"github.com/fastly/terraform-provider-fastly/internal/resources/backend"
	"github.com/fastly/terraform-provider-fastly/internal/service"
	"github.com/fastly/terraform-provider-fastly/internal/validation"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	fastly "github.com/fastly/go-fastly/v17/fastly"
)

const (
	DefaultComment = ""
	DefaultQuorum  = 75
	DefaultRetries = 5
	DefaultShield  = ""
	DefaultType    = "random"
)

// directorTypeByString maps both the friendly name and numeric alias to the fastly.DirectorType
// the API expects (director.yaml: enum [1, 3, 4], x-enum-varnames [random, hash, client]).
// round_robin/"2" is included so Create/Update can round-trip a director that already has it
// without panicking, even though the validator below still rejects it in new config.
var directorTypeByString = map[string]fastly.DirectorType{
	"random":      fastly.DirectorTypeRandom,
	"1":           fastly.DirectorTypeRandom,
	"round_robin": fastly.DirectorTypeRoundRobin,
	"2":           fastly.DirectorTypeRoundRobin,
	"hash":        fastly.DirectorTypeHash,
	"3":           fastly.DirectorTypeHash,
	"client":      fastly.DirectorTypeClient,
	"4":           fastly.DirectorTypeClient,
}

// directorTypeByAPI maps the integer DirectorType back to its friendly-name string for reading
// state. Includes round_robin, unlike directorTypeByString: a pre-existing director can have
// that type, and misreporting it as DefaultType could drive an unintended change on next apply.
var directorTypeByAPI = map[fastly.DirectorType]string{
	fastly.DirectorTypeRandom:     "random",
	fastly.DirectorTypeRoundRobin: "round_robin",
	fastly.DirectorTypeHash:       "hash",
	fastly.DirectorTypeClient:     "client",
}

type NestedModel struct {
	Name     types.String `tfsdk:"name"`
	Backends types.Set    `tfsdk:"backends"`
	Comment  types.String `tfsdk:"comment"`
	Quorum   types.Int64  `tfsdk:"quorum"`
	Retries  types.Int64  `tfsdk:"retries"`
	Shield   types.String `tfsdk:"shield"`
	Type     types.String `tfsdk:"type"`
}

func (n NestedModel) ModelsEqual(other NestedModel) bool {
	return service.StringValue(n.Name) == service.StringValue(other.Name) &&
		n.Backends.Equal(other.Backends) &&
		service.StringValue(n.Comment) == service.StringValue(other.Comment) &&
		service.Int64Value(n.Quorum) == service.Int64Value(other.Quorum) &&
		service.Int64Value(n.Retries) == service.Int64Value(other.Retries) &&
		service.StringValue(n.Shield) == service.StringValue(other.Shield) &&
		service.StringValue(n.Type) == service.StringValue(other.Type)
}

func CommonAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name": schema.StringAttribute{
			Required:    true,
			Description: "Unique name for this Director. It is important to note that changing this attribute will delete and recreate the resource",
		},
		"backends": schema.SetAttribute{
			ElementType: types.StringType,
			Required:    true,
			Description: "Names of defined backends to map the director to. Example: `[\"origin1\", \"origin2\"]`.",
			Validators: []validator.Set{
				setvalidator.SizeAtLeast(1),
			},
		},
		"comment": schema.StringAttribute{
			Optional:    true,
			Computed:    true,
			Default:     stringdefault.StaticString(DefaultComment),
			Description: "An optional comment about the Director.",
		},
		"quorum": schema.Int64Attribute{
			Optional:    true,
			Computed:    true,
			Default:     int64default.StaticInt64(DefaultQuorum),
			Description: "Percentage of capacity that needs to be up for the director itself to be considered up. Default `75`.",
			Validators: []validator.Int64{
				int64validator.Between(0, 100),
			},
		},
		"retries": schema.Int64Attribute{
			Optional:    true,
			Computed:    true,
			Default:     int64default.StaticInt64(DefaultRetries),
			Description: "How many backends to search if it fails. Default `5`.",
			Validators: []validator.Int64{
				int64validator.AtLeast(0),
			},
		},
		"shield": schema.StringAttribute{
			Optional:    true,
			Computed:    true,
			Default:     stringdefault.StaticString(DefaultShield),
			Description: "Selected POP to serve as a \"shield\" for backends. Valid values for `shield` are included in the [`GET /datacenters`](https://developer.fastly.com/reference/api/utils/datacenter/) API response.",
		},
		"type": schema.StringAttribute{
			Optional:    true,
			Computed:    true,
			Description: "Type of load balance group to use. One of `random`, `hash`, or `client` (the numeric equivalents `1`, `3`, and `4` are also accepted). Default `random`.",
			Validators: []validator.String{
				stringvalidator.OneOf("random", "hash", "client", "1", "3", "4"),
			},
		},
	}
}

func NestedBlockSchema() schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: "Directors attached to this service.",
		NestedObject: schema.NestedBlockObject{
			Attributes: CommonAttributes(),
		},
		PlanModifiers: []planmodifier.List{
			typeStickyDefault{},
		},
	}
}

// typeStickyDefault resets a director's type to DefaultType when omitted from config, matching
// the legacy provider's schema-level default. round_robin is the exception: config can never set
// it, so a director that already has it (pre-existing this schema, or set out-of-band) must keep
// it when type is unset. Matching is done by name across the whole list rather than per-attribute,
// because the framework pairs a ListNestedBlock plan modifier's prior StateValue by list index,
// not by name - a per-attribute modifier would attribute round_robin to the wrong director
// whenever the list is reordered or an element is inserted.
//
// It also canonicalizes a numeric type alias ("1"/"3"/"4") to its friendly name in the plan, so
// state always matches what was planned regardless of which alias form was configured - avoiding
// a "Provider produced inconsistent result after apply" error.
type typeStickyDefault struct{}

func (m typeStickyDefault) Description(_ context.Context) string {
	return fmt.Sprintf("resets each director's type to %q when omitted from config, unless its existing value (matched by name) is round_robin, which isn't settable via config and is preserved instead; canonicalizes a numeric type alias to its friendly name", DefaultType)
}

func (m typeStickyDefault) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m typeStickyDefault) PlanModifyList(ctx context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	if req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		return
	}

	priorTypeByName := make(map[string]attr.Value)
	if !req.StateValue.IsNull() && !req.StateValue.IsUnknown() {
		for _, elem := range req.StateValue.Elements() {
			obj, ok := elem.(types.Object)
			if !ok {
				continue
			}
			attrs := obj.Attributes()
			name, ok := attrs["name"].(types.String)
			if !ok || name.IsNull() || name.IsUnknown() {
				continue
			}
			priorTypeByName[name.ValueString()] = attrs["type"]
		}
	}

	configElems := req.ConfigValue.Elements()
	planElems := req.PlanValue.Elements()
	if len(configElems) != len(planElems) {
		return
	}

	changed := false
	newElems := make([]attr.Value, len(planElems))
	for i, elem := range planElems {
		newElems[i] = elem

		planObj, ok := elem.(types.Object)
		if !ok {
			continue
		}
		configObj, ok := configElems[i].(types.Object)
		if !ok {
			continue
		}

		configType, ok := configObj.Attributes()["type"].(types.String)
		if !ok {
			continue
		}

		var newType attr.Value
		switch {
		case configType.IsNull():
			newType = types.StringValue(DefaultType)
			if name, ok := planObj.Attributes()["name"].(types.String); ok && !name.IsNull() && !name.IsUnknown() {
				if prior, ok := priorTypeByName[name.ValueString()]; ok {
					if priorStr, ok := prior.(types.String); ok && !priorStr.IsNull() && !priorStr.IsUnknown() && priorStr.ValueString() == "round_robin" {
						newType = prior
					}
				}
			}
		case configType.IsUnknown():
			continue
		default:
			canonical, ok := directorTypeCanonical(configType.ValueString())
			if !ok || canonical == configType.ValueString() {
				continue
			}
			newType = types.StringValue(canonical)
		}

		attrs := planObj.Attributes()
		attrs["type"] = newType
		newObj, diags := types.ObjectValue(planObj.AttributeTypes(ctx), attrs)
		resp.Diagnostics.Append(diags...)
		if diags.HasError() {
			return
		}
		newElems[i] = newObj
		changed = true
	}

	if !changed {
		return
	}

	newList, diags := types.ListValue(req.PlanValue.ElementType(ctx), newElems)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	resp.PlanValue = newList
}

// directorTypeCanonical returns the friendly-name form of a valid type value (friendly name or
// numeric alias); ok is false if s isn't recognized.
func directorTypeCanonical(s string) (string, bool) {
	t, ok := directorTypeByString[s]
	if !ok {
		return "", false
	}
	return directorTypeByAPI[t], true
}

// ops caches the most recent List result by name so Update can diff backend associations without
// an extra round-trip. A fresh ops must be used per Reconcile/ReadForVersion call.
type ops struct {
	remoteByName map[string]*fastly.Director
}

func (o *ops) List(ctx context.Context, client *fastly.Client, serviceID string, version int) ([]*fastly.Director, error) {
	directors, err := client.ListDirectors(ctx, &fastly.ListDirectorsInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
	})
	if err != nil {
		return nil, err
	}

	o.remoteByName = make(map[string]*fastly.Director, len(directors))
	for _, d := range directors {
		o.remoteByName[fastly.ToValue(d.Name)] = d
	}

	return directors, nil
}

func (o ops) GetName(api *fastly.Director) string {
	return fastly.ToValue(api.Name)
}

func (o *ops) Delete(ctx context.Context, client *fastly.Client, serviceID string, version int, name string) error {
	return client.DeleteDirector(ctx, &fastly.DeleteDirectorInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
		Name:           name,
	})
}

func (o ops) Create(ctx context.Context, client *fastly.Client, serviceID string, version int, desired NestedModel) (*fastly.Director, error) {
	name := service.StringValue(desired.Name)

	d, err := client.CreateDirector(ctx, &fastly.CreateDirectorInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
		Name:           new(name),
		Comment:        new(service.StringValue(desired.Comment)),
		Shield:         new(service.StringValue(desired.Shield)),
		Quorum:         new(int(service.Int64Value(desired.Quorum))),
		Retries:        new(int(service.Int64Value(desired.Retries))),
		Type:           directorTypePointer(desired.Type),
	})
	if err != nil {
		return nil, err
	}

	for _, backendName := range setToStringSlice(desired.Backends) {
		if _, err := client.CreateDirectorBackend(ctx, &fastly.CreateDirectorBackendInput{
			ServiceID:      serviceID,
			ServiceVersion: version,
			Director:       name,
			Backend:        backendName,
		}); err != nil {
			return nil, err
		}
	}

	return d, nil
}

func (o ops) Equal(desired NestedModel, remote *fastly.Director) bool {
	return desired.ModelsEqual(o.ToModel(remote))
}

// metadataFieldsEqual is ops.Equal minus Backends, letting Update skip UpdateDirector when only
// backend membership changed.
func (o ops) metadataFieldsEqual(desired NestedModel, remote *fastly.Director) bool {
	m := o.ToModel(remote)
	return service.StringValue(desired.Name) == service.StringValue(m.Name) &&
		service.StringValue(desired.Comment) == service.StringValue(m.Comment) &&
		service.Int64Value(desired.Quorum) == service.Int64Value(m.Quorum) &&
		service.Int64Value(desired.Retries) == service.Int64Value(m.Retries) &&
		service.StringValue(desired.Shield) == service.StringValue(m.Shield) &&
		service.StringValue(desired.Type) == service.StringValue(m.Type)
}

func (o *ops) Update(ctx context.Context, client *fastly.Client, serviceID string, version int, desired NestedModel) (*fastly.Director, error) {
	name := service.StringValue(desired.Name)

	remote := o.remoteByName[name]

	d := remote
	if remote == nil || !o.metadataFieldsEqual(desired, remote) {
		var err error
		d, err = client.UpdateDirector(ctx, &fastly.UpdateDirectorInput{
			ServiceID:      serviceID,
			ServiceVersion: version,
			Name:           name,
			Comment:        new(service.StringValue(desired.Comment)),
			Shield:         new(service.StringValue(desired.Shield)),
			Quorum:         new(int(service.Int64Value(desired.Quorum))),
			Retries:        new(int(service.Int64Value(desired.Retries))),
			Type:           directorTypePointer(desired.Type),
		})
		if err != nil {
			return nil, err
		}
	}

	var currentBackends []string
	if remote != nil {
		currentBackends = remote.Backends
	}

	err := reconcile.DiffSet(currentBackends, setToStringSlice(desired.Backends),
		func(backendName string) error {
			_, err := client.CreateDirectorBackend(ctx, &fastly.CreateDirectorBackendInput{
				ServiceID:      serviceID,
				ServiceVersion: version,
				Director:       name,
				Backend:        backendName,
			})
			return err
		},
		func(backendName string) error {
			return client.DeleteDirectorBackend(ctx, &fastly.DeleteDirectorBackendInput{
				ServiceID:      serviceID,
				ServiceVersion: version,
				Director:       name,
				Backend:        backendName,
			})
		},
	)
	if err != nil {
		return nil, err
	}

	return d, nil
}

func (o ops) ToModel(api *fastly.Director) NestedModel {
	return NestedModel{
		Name:     types.StringValue(fastly.ToValue(api.Name)),
		Backends: stringSliceToSet(api.Backends),
		Comment:  types.StringValue(fastly.ToValue(api.Comment)),
		Quorum:   types.Int64Value(int64(fastly.ToValue(api.Quorum))),
		Retries:  types.Int64Value(int64(fastly.ToValue(api.Retries))),
		Shield:   types.StringValue(fastly.ToValue(api.Shield)),
		Type:     types.StringValue(directorTypeString(api.Type)),
	}
}

// directorTypeString falls back to DefaultType for nil or unrecognized values rather than an
// empty string, which would panic in directorTypePointer on the next write.
func directorTypeString(t *fastly.DirectorType) string {
	if t == nil {
		return DefaultType
	}
	s, ok := directorTypeByAPI[*t]
	if !ok {
		return DefaultType
	}
	return s
}

// directorTypePointer panics on an unrecognized type rather than silently sending the API an
// undefined DirectorType zero-value; every value the schema can produce has an entry in
// directorTypeByString, so this should never trigger.
func directorTypePointer(v types.String) *fastly.DirectorType {
	s := service.StringValue(v)
	t, ok := directorTypeByString[s]
	if !ok {
		panic(fmt.Sprintf("director: unrecognized type %q; must be one of random, hash, client, round_robin, or their numeric aliases 1, 3, 4, 2", s))
	}
	return &t
}

// setToStringSlice skips unknown elements (e.g. `backends = [some_resource.x.name]` before
// x.name is known) rather than turning them into a bogus empty-string backend name.
func setToStringSlice(s types.Set) []string {
	elems := s.Elements()
	parts := make([]string, 0, len(elems))
	for _, e := range elems {
		v, ok := e.(types.String)
		if !ok || v.IsUnknown() || v.IsNull() {
			continue
		}
		parts = append(parts, service.StringValue(v))
	}
	return parts
}

// stringSliceToSet sorts first for deterministic state, and drops duplicates rather than passing
// them to SetValueMust, which panics on a duplicate element.
func stringSliceToSet(s []string) types.Set {
	sorted := make([]string, len(s))
	copy(sorted, s)
	sort.Strings(sorted)

	elems := make([]attr.Value, 0, len(sorted))
	for i, v := range sorted {
		if i > 0 && v == sorted[i-1] {
			continue
		}
		elems = append(elems, types.StringValue(v))
	}
	return types.SetValueMust(types.StringType, elems)
}

// newReconciler builds a fresh Resource per call since ops.remoteByName must not be shared
// across concurrent reconciles.
func newReconciler() *reconcile.Resource[NestedModel, fastly.Director] {
	return &reconcile.Resource[NestedModel, fastly.Director]{
		Ops: &ops{},
		GetName: func(m NestedModel) string {
			return service.StringValue(m.Name)
		},
		Sortable: true,
	}
}

func ReadForVersion(ctx context.Context, client *fastly.Client, serviceID string, version int) ([]NestedModel, error) {
	return newReconciler().ReadForVersion(ctx, client, serviceID, version)
}

func Reconcile(ctx context.Context, client *fastly.Client, serviceID string, version int, desired []NestedModel) error {
	return newReconciler().Run(ctx, client, serviceID, version, desired)
}

func Equal(a, b []NestedModel) bool {
	return reconcile.ModelsEqual(a, b, func(m NestedModel) string { return service.StringValue(m.Name) }, NestedModel.ModelsEqual, true)
}

func MatchOrder(items, order []NestedModel) []NestedModel {
	return reconcile.MatchOrder(items, order, func(m NestedModel) string { return service.StringValue(m.Name) })
}

// ValidateConfig enforces director name uniqueness at plan time; reconcile.Run keys directors by
// name, so duplicates would otherwise silently collapse to one.
func ValidateConfig(directors []NestedModel) error {
	return validation.UniqueNames(directors, "director", func(m NestedModel) types.String { return m.Name })
}

// ValidateBackendReferences catches a director referencing a renamed/removed backend at plan
// time, instead of a 404 on the director-backend association at apply time.
func ValidateBackendReferences(directors []NestedModel, backends []backend.NestedModel) error {
	backendNames := validation.NameSet(backends, func(b backend.NestedModel) types.String { return b.Name })

	return validation.References(directors, "director", func(m NestedModel) types.String { return m.Name }, "backend",
		func(m NestedModel) []string {
			if m.Backends.IsUnknown() || m.Backends.IsNull() {
				return nil
			}
			return setToStringSlice(m.Backends)
		},
		"backend", backendNames)
}
