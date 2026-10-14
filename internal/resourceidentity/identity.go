package resourceidentity

import (
	"context"
	"fmt"

	fastly "github.com/fastly/go-fastly/v17/fastly"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ServiceModel identifies a top-level Fastly service by its stable Fastly
// service ID. The identity attribute is named service_id consistently with the
// versioned explicit resource family, even though the service resource stores
// the same value in its computed "id" state attribute.
type ServiceModel struct {
	ServiceID types.String `tfsdk:"service_id"`
}

// NamedVersionedModel identifies a logical object that lives on a versioned
// service. Version is deliberately absent: moving the object to another Fastly
// service version must not change its Terraform resource identity.
type NamedVersionedModel struct {
	ServiceID types.String `tfsdk:"service_id"`
	Name      types.String `tfsdk:"name"`
}

// ServiceScopedVersionedModel identifies a singleton object scoped to a
// service, such as service settings. Version is deliberately absent.
type ServiceScopedVersionedModel struct {
	ServiceID types.String `tfsdk:"service_id"`
}

// ResourceLinkModel identifies a resource link by the shared resource it
// targets rather than its alias name, since the name can be renamed in place.
// Version is deliberately absent.
type ResourceLinkModel struct {
	ServiceID  types.String `tfsdk:"service_id"`
	ResourceID types.String `tfsdk:"resource_id"`
}

// ACLCollectionModel identifies the collection of entries belonging to a Fastly
// ACL. ACL entries are addressed by service and ACL IDs rather than service
// version.
type ACLCollectionModel struct {
	ServiceID types.String `tfsdk:"service_id"`
	ACLID     types.String `tfsdk:"acl_id"`
}

func ServiceSchema() identityschema.Schema {
	return identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"service_id": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Fastly service ID.",
			},
		},
	}
}

func NamedVersionedSchema() identityschema.Schema {
	return identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"service_id": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Fastly service ID.",
			},
			"name": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Resource name.",
			},
		},
	}
}

func ServiceScopedVersionedSchema() identityschema.Schema {
	return identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"service_id": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Fastly service ID.",
			},
		},
	}
}

func ResourceLinkSchema() identityschema.Schema {
	return identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"service_id": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Fastly service ID.",
			},
			"resource_id": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "ID of the linked shared resource.",
			},
		},
	}
}

func ACLCollectionSchema() identityschema.Schema {
	return identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"service_id": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Fastly service ID.",
			},
			"acl_id": identityschema.StringAttribute{
				RequiredForImport: true,
				Description:       "Fastly ACL ID.",
			},
		},
	}
}

func Service(id types.String) *ServiceModel {
	return &ServiceModel{ServiceID: id}
}

func NamedVersioned(serviceID, name types.String) *NamedVersionedModel {
	return &NamedVersionedModel{ServiceID: serviceID, Name: name}
}

func ServiceScopedVersioned(serviceID types.String) *ServiceScopedVersionedModel {
	return &ServiceScopedVersionedModel{ServiceID: serviceID}
}

func ResourceLink(serviceID, resourceID types.String) *ResourceLinkModel {
	return &ResourceLinkModel{ServiceID: serviceID, ResourceID: resourceID}
}

func ACLCollection(serviceID, aclID types.String) *ACLCollectionModel {
	return &ACLCollectionModel{ServiceID: serviceID, ACLID: aclID}
}

func requireImportIdentity(req resource.ImportStateRequest, resp *resource.ImportStateResponse) bool {
	if req.Identity != nil {
		return true
	}

	resp.Diagnostics.AddError(
		"Missing resource identity",
		"The import request did not include a resource identity. Identity-based import requires an identity payload.",
	)
	return false
}

// ImportService seeds enough state for the resource's normal Read operation to
// finish an identity-based import of a top-level service.
func ImportService(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !requireImportIdentity(req, resp) {
		return
	}

	var identity ServiceModel
	resp.Diagnostics.Append(req.Identity.Get(ctx, &identity)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if identity.ServiceID.IsNull() || identity.ServiceID.IsUnknown() || identity.ServiceID.ValueString() == "" {
		resp.Diagnostics.AddError("Invalid resource identity", "The service identity must contain a non-empty service_id.")
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), identity.ServiceID)...)
	if resp.Diagnostics.HasError() || resp.Identity == nil {
		return
	}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, &identity)...)
}

// ImportNamedVersioned seeds service_id, the currently readable service
// version, and name. Terraform will then invoke the resource's normal Read
// operation to populate the remaining state.
func ImportNamedVersioned(ctx context.Context, client *fastly.Client, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !requireImportIdentity(req, resp) {
		return
	}

	var identity NamedVersionedModel
	resp.Diagnostics.Append(req.Identity.Get(ctx, &identity)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if identity.ServiceID.IsNull() || identity.ServiceID.IsUnknown() || identity.ServiceID.ValueString() == "" {
		resp.Diagnostics.AddError("Invalid resource identity", "The resource identity must contain a non-empty service_id.")
		return
	}
	if identity.Name.IsNull() || identity.Name.IsUnknown() || identity.Name.ValueString() == "" {
		resp.Diagnostics.AddError("Invalid resource identity", "The resource identity must contain a non-empty name.")
		return
	}

	version, _, err := service.SelectReadVersion(ctx, client, identity.ServiceID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error selecting service version for import",
			fmt.Sprintf("Could not select an active or latest version for Fastly service %q: %s", identity.ServiceID.ValueString(), err),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("service_id"), identity.ServiceID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("version"), int64(version))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), identity.Name)...)
	if resp.Diagnostics.HasError() || resp.Identity == nil {
		return
	}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, &identity)...)
}

// ImportServiceScopedVersioned is the singleton counterpart of
// ImportNamedVersioned. It resolves active/latest version but has no name key.
func ImportServiceScopedVersioned(ctx context.Context, client *fastly.Client, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !requireImportIdentity(req, resp) {
		return
	}

	var identity ServiceScopedVersionedModel
	resp.Diagnostics.Append(req.Identity.Get(ctx, &identity)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if identity.ServiceID.IsNull() || identity.ServiceID.IsUnknown() || identity.ServiceID.ValueString() == "" {
		resp.Diagnostics.AddError("Invalid resource identity", "The resource identity must contain a non-empty service_id.")
		return
	}

	version, _, err := service.SelectReadVersion(ctx, client, identity.ServiceID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error selecting service version for import",
			fmt.Sprintf("Could not select an active or latest version for Fastly service %q: %s", identity.ServiceID.ValueString(), err),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("service_id"), identity.ServiceID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("version"), int64(version))...)
	if resp.Diagnostics.HasError() || resp.Identity == nil {
		return
	}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, &identity)...)
}

// ImportResourceLink seeds service_id, the currently readable service version,
// and resource_id. The resource's Read resolves the link from resource_id.
func ImportResourceLink(ctx context.Context, client *fastly.Client, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !requireImportIdentity(req, resp) {
		return
	}

	var identity ResourceLinkModel
	resp.Diagnostics.Append(req.Identity.Get(ctx, &identity)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if identity.ServiceID.IsNull() || identity.ServiceID.IsUnknown() || identity.ServiceID.ValueString() == "" {
		resp.Diagnostics.AddError("Invalid resource identity", "The resource identity must contain a non-empty service_id.")
		return
	}
	if identity.ResourceID.IsNull() || identity.ResourceID.IsUnknown() || identity.ResourceID.ValueString() == "" {
		resp.Diagnostics.AddError("Invalid resource identity", "The resource identity must contain a non-empty resource_id.")
		return
	}

	version, _, err := service.SelectReadVersion(ctx, client, identity.ServiceID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error selecting service version for import",
			fmt.Sprintf("Could not select an active or latest version for Fastly service %q: %s", identity.ServiceID.ValueString(), err),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("service_id"), identity.ServiceID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("version"), int64(version))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("resource_id"), identity.ResourceID)...)
	if resp.Diagnostics.HasError() || resp.Identity == nil {
		return
	}

	resp.Diagnostics.Append(resp.Identity.Set(ctx, &identity)...)
}

// ImportACLCollection seeds the stable service/ACL identity and deterministic
// resource ID. The normal Read operation adopts the current ACL entries.
func ImportACLCollection(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !requireImportIdentity(req, resp) {
		return
	}

	var identity ACLCollectionModel
	resp.Diagnostics.Append(req.Identity.Get(ctx, &identity)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if identity.ServiceID.IsNull() || identity.ServiceID.IsUnknown() || identity.ServiceID.ValueString() == "" {
		resp.Diagnostics.AddError("Invalid resource identity", "The resource identity must contain a non-empty service_id.")
		return
	}
	if identity.ACLID.IsNull() || identity.ACLID.IsUnknown() || identity.ACLID.ValueString() == "" {
		resp.Diagnostics.AddError("Invalid resource identity", "The resource identity must contain a non-empty acl_id.")
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("service_id"), identity.ServiceID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("acl_id"), identity.ACLID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(
		ctx,
		path.Root("id"),
		fmt.Sprintf("%s/%s", identity.ServiceID.ValueString(), identity.ACLID.ValueString()),
	)...)
	if resp.Diagnostics.HasError() || resp.Identity == nil {
		return
	}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, &identity)...)
}
