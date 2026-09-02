package tlscertificate

import (
	"context"
	"fmt"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/fastly/go-fastly/v17/fastly"
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
	resp.TypeName = req.ProviderTypeName + "_tls_certificate"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Uploads a custom TLS certificate. TLS certificates are versionless and independent of any service-version lifecycle.",
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

	tflog.Debug(ctx, "Creating Fastly TLS certificate", map[string]any{"name": service.StringValue(plan.Name)})

	cert, err := r.client.CreateCustomTLSCertificate(ctx, &fastly.CreateCustomTLSCertificateInput{
		CertBlob: service.StringValue(plan.CertificateBody),
		Name:     service.StringValue(plan.Name),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating TLS certificate", err.Error())
		return
	}

	newState, diags := flattenToModel(ctx, cert)
	resp.Diagnostics.Append(diags...)
	newState.CertificateBody = plan.CertificateBody
	warnIfReplaceRecommended(cert, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	tflog.Debug(ctx, "Reading Fastly TLS certificate", map[string]any{"id": id})

	cert, err := r.client.GetCustomTLSCertificate(ctx, &fastly.GetCustomTLSCertificateInput{ID: id})
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "TLS certificate not found, removing from state", map[string]any{"id": id})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading TLS certificate", err.Error())
		return
	}

	newState, diags := flattenToModel(ctx, cert)
	resp.Diagnostics.Append(diags...)
	newState.CertificateBody = state.CertificateBody
	warnIfReplaceRecommended(cert, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	tflog.Debug(ctx, "Updating Fastly TLS certificate", map[string]any{"id": id})

	cert, err := r.client.UpdateCustomTLSCertificate(ctx, &fastly.UpdateCustomTLSCertificateInput{
		ID:       id,
		CertBlob: service.StringValue(plan.CertificateBody),
		Name:     service.StringValue(plan.Name),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating TLS certificate", err.Error())
		return
	}

	newState, diags := flattenToModel(ctx, cert)
	resp.Diagnostics.Append(diags...)
	newState.CertificateBody = plan.CertificateBody
	warnIfReplaceRecommended(cert, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	tflog.Debug(ctx, "Deleting Fastly TLS certificate", map[string]any{"id": id})

	err := r.client.DeleteCustomTLSCertificate(ctx, &fastly.DeleteCustomTLSCertificateInput{ID: id})
	if err != nil && !errors.IsNotFound(err) {
		resp.Diagnostics.AddError("Error deleting TLS certificate", err.Error())
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func warnIfReplaceRecommended(c *fastly.CustomTLSCertificate, diags *diag.Diagnostics) {
	if c.Replace {
		diags.AddWarning("Certificate replacement recommended", fmt.Sprintf("Fastly recommends that this certificate (%s) be replaced", c.ID))
	}
}
