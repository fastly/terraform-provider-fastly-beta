package apisecurityoperationtag

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/fastly/go-fastly/v17/fastly"
	"github.com/fastly/go-fastly/v17/fastly/apisecurity/operations"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"
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
	resp.TypeName = req.ProviderTypeName + "_api_security_operation_tag"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an API Security operation tag for a Fastly service. Operation tags can be used to group and organize operations.",
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

	serviceID := plan.ServiceID.ValueString()

	tflog.Debug(ctx, "Creating Fastly API Security operation tag", map[string]any{
		"service_id": serviceID,
		"name":       plan.Name.ValueString(),
	})

	in := buildCreateInput(serviceID, plan)

	tag, err := operations.CreateTag(fastly.NewContextForResourceID(ctx, serviceID), r.client, in)
	if err != nil {
		resp.Diagnostics.AddError("Error creating API Security operation tag", err.Error())
		return
	}

	flatten(&plan, tag, serviceID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serviceID := state.ServiceID.ValueString()
	tagID := state.TagID.ValueString()

	tflog.Debug(ctx, "Reading Fastly API Security operation tag", map[string]any{
		"service_id": serviceID,
		"tag_id":     tagID,
	})

	tag, err := operations.DescribeTag(ctx, r.client, &operations.DescribeTagInput{
		ServiceID: &serviceID,
		TagID:     &tagID,
	})
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "API Security operation tag not found, removing from state", map[string]any{
				"service_id": serviceID,
				"tag_id":     tagID,
			})
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Error reading API Security operation tag", err.Error())
		return
	}

	flatten(&state, tag, serviceID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan Model
	var state Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serviceID := state.ServiceID.ValueString()
	tagID := state.TagID.ValueString()

	tflog.Debug(ctx, "Updating Fastly API Security operation tag", map[string]any{
		"service_id": serviceID,
		"tag_id":     tagID,
	})

	in := buildUpdateInput(serviceID, tagID, plan)

	tag, err := operations.UpdateTag(fastly.NewContextForResourceID(ctx, serviceID), r.client, in)
	if err != nil {
		if errors.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Error updating API Security operation tag", err.Error())
		return
	}

	flatten(&plan, tag, serviceID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serviceID := state.ServiceID.ValueString()
	tagID := state.TagID.ValueString()

	tflog.Debug(ctx, "Deleting Fastly API Security operation tag", map[string]any{
		"service_id": serviceID,
		"tag_id":     tagID,
	})

	err := operations.DeleteTag(fastly.NewContextForResourceID(ctx, serviceID), r.client, &operations.DeleteTagInput{
		ServiceID: &serviceID,
		TagID:     &tagID,
	})
	if err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting API Security operation tag", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Invalid id: %q. The ID should be in the format <service_id>/<tag_id>.", req.ID),
		)
		return
	}

	serviceID, tagID := parts[0], parts[1]

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("service_id"), serviceID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("tag_id"), tagID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(req.ID))...)
}
