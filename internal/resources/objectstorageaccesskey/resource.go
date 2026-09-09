package objectstorageaccesskey

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/fastly/go-fastly/v17/fastly"
	"github.com/fastly/go-fastly/v17/fastly/objectstorage/accesskeys"
)

var (
	_ resource.Resource                = &Resource{}
	_ resource.ResourceWithConfigure   = &Resource{}
	_ resource.ResourceWithImportState = &Resource{}
)

type Resource struct {
	client *fastly.Client
}

func NewResource() resource.Resource {
	return &Resource{}
}

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_object_storage_access_keys"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Provides an Object Storage Access Key, used to manage resources in various clouds. Access keys are versionless and independent of any service-version lifecycle. The resource is immutable: any change to `description`, `permission`, or `buckets` destroys and recreates it.",
		Attributes:  ResourceAttributes(),
	}
}

func (r *Resource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	data, diags := fastlyclient.FromProviderData(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || data == nil {
		return
	}

	r.client = data.Client
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating Fastly Object Storage Access Key", map[string]any{
		"description": plan.Description.ValueString(),
	})

	input, diags := buildCreateInput(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := accesskeys.Create(ctx, r.client, input)
	if err != nil {
		resp.Diagnostics.AddError("Error creating Object Storage Access Key", err.Error())
		return
	}

	if created.AccessKeyID == "" || created.SecretKey == "" {
		resp.Diagnostics.AddError("Error creating Object Storage Access Key", "API response is missing the access key ID or secret key")
		return
	}

	newState, diags := flattenToModel(ctx, created, plan.SecretKey(), plan.Buckets)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	tflog.Debug(ctx, "Reading Fastly Object Storage Access Key", map[string]any{"id": id})

	key, err := accesskeys.Get(ctx, r.client, &accesskeys.GetInput{AccessKeyID: new(id)})
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "Object Storage Access Key not found, removing from state", map[string]any{"id": id})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Object Storage Access Key", err.Error())
		return
	}

	newState, diags := flattenToModel(ctx, key, state.SecretKey(), state.Buckets)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

// Update is unreachable in practice: description, permission, and buckets all
// require replacement, so Terraform never plans an in-place update for this
// resource. Implemented only to satisfy resource.Resource.
func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	tflog.Debug(ctx, "Deleting Fastly Object Storage Access Key", map[string]any{"id": id})

	err := accesskeys.Delete(ctx, r.client, &accesskeys.DeleteInput{AccessKeyID: new(id)})
	if err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting Object Storage Access Key", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
