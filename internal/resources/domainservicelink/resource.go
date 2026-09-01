package domainservicelink

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/fastly/go-fastly/v17/fastly"
	"github.com/fastly/go-fastly/v17/fastly/domainmanagement/v1/domains"
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
	resp.TypeName = req.ProviderTypeName + "_domain_service_link"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Links a versionless `fastly_domain` to a service, independent of managing the domain itself.",
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

// upsert preserves the domain's description - Update has no omitempty,
// so leaving it unset would wipe it out as a side effect of the link.
func (r *Resource) upsert(ctx context.Context, domainID string, serviceID *string) (*domains.Data, error) {
	current, err := domains.Get(ctx, r.client, &domains.GetInput{DomainID: &domainID})
	if err != nil {
		return nil, err
	}

	input := &domains.UpdateInput{
		DomainID:  &domainID,
		ServiceID: serviceID,
	}
	if current.Description != "" {
		input.Description = &current.Description
	}

	return domains.Update(ctx, r.client, input)
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainID := plan.DomainID.ValueString()
	serviceID := plan.ServiceID.ValueString()
	tflog.Debug(ctx, "Creating Fastly Domain Service Link", map[string]any{
		"domain_id": domainID,
	})

	data, err := r.upsert(ctx, domainID, &serviceID)
	if err != nil {
		resp.Diagnostics.AddError("Error creating Domain Service Link", err.Error())
		return
	}

	newState := FlattenToModel(data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainID := state.DomainID.ValueString()
	tflog.Debug(ctx, "Reading Fastly Domain Service Link", map[string]any{
		"domain_id": domainID,
	})

	data, err := domains.Get(ctx, r.client, &domains.GetInput{DomainID: &domainID})
	if err != nil {
		if errors.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Domain Service Link", err.Error())
		return
	}

	// service_id cleared means the link is gone, by us or out-of-band.
	if data.ServiceID == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	newState := FlattenToModel(data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainID := plan.DomainID.ValueString()
	serviceID := plan.ServiceID.ValueString()
	tflog.Debug(ctx, "Updating Fastly Domain Service Link", map[string]any{
		"domain_id": domainID,
	})

	data, err := r.upsert(ctx, domainID, &serviceID)
	if err != nil {
		resp.Diagnostics.AddError("Error updating Domain Service Link", err.Error())
		return
	}

	newState := FlattenToModel(data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainID := state.DomainID.ValueString()
	tflog.Debug(ctx, "Deleting Fastly Domain Service Link", map[string]any{
		"domain_id": domainID,
	})

	if _, err := r.upsert(ctx, domainID, nil); err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting Domain Service Link", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("domain_id"), req, resp)
}
