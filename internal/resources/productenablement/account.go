package productenablement

import (
	"context"
	"fmt"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/fastly/go-fastly/v19/fastly"
	"github.com/fastly/go-fastly/v19/fastly/products"
	"github.com/fastly/go-fastly/v19/fastly/products/aiaccelerator"
	"github.com/fastly/go-fastly/v19/fastly/products/airuntimecontrol"
	"github.com/fastly/go-fastly/v19/fastly/products/domainresearch"
	"github.com/fastly/go-fastly/v19/fastly/products/objectstorage"
)

// accountProductSpec describes a product enabled for the whole account.
// productID doubles as the resource ID.
type accountProductSpec struct {
	attrName    string
	displayName string
	productID   string
	get         func(ctx context.Context, c *fastly.Client) (products.EnableOutput, error)
	enable      func(ctx context.Context, c *fastly.Client) (products.EnableOutput, error)
	disable     func(ctx context.Context, c *fastly.Client) error
}

var (
	aiAcceleratorSpec = accountProductSpec{
		attrName:    "ai_accelerator",
		displayName: aiaccelerator.ProductName,
		productID:   aiaccelerator.ProductID,
		get:         aiaccelerator.Get,
		enable:      aiaccelerator.Enable,
		disable:     aiaccelerator.Disable,
	}
	aiRuntimeControlSpec = accountProductSpec{
		attrName:    "ai_runtime_control",
		displayName: airuntimecontrol.ProductName,
		productID:   airuntimecontrol.ProductID,
		get:         airuntimecontrol.Get,
		enable:      airuntimecontrol.Enable,
		disable:     airuntimecontrol.Disable,
	}
	domainResearchSpec = accountProductSpec{
		attrName:    "domain_research",
		displayName: domainresearch.ProductName,
		productID:   domainresearch.ProductID,
		get:         domainresearch.Get,
		enable:      domainresearch.Enable,
		disable:     domainresearch.Disable,
	}
	objectStorageSpec = accountProductSpec{
		attrName:    "object_storage",
		displayName: objectstorage.ProductName,
		productID:   objectstorage.ProductID,
		get:         objectstorage.Get,
		enable:      objectstorage.Enable,
		disable:     objectstorage.Disable,
	}
)

func NewAIAcceleratorResource() resource.Resource {
	return &accountProductResource{spec: aiAcceleratorSpec}
}

func NewAIRuntimeControlResource() resource.Resource {
	return &accountProductResource{spec: aiRuntimeControlSpec}
}

func NewDomainResearchResource() resource.Resource {
	return &accountProductResource{spec: domainResearchSpec}
}

func NewObjectStorageResource() resource.Resource {
	return &accountProductResource{spec: objectStorageSpec}
}

var (
	_ resource.Resource                = &accountProductResource{}
	_ resource.ResourceWithImportState = &accountProductResource{}
)

type accountProductModel struct {
	ID      types.String `tfsdk:"id"`
	Enabled types.Bool   `tfsdk:"enabled"`
}

type accountProductResource struct {
	spec   accountProductSpec
	client *fastly.Client
}

func (r *accountProductResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + r.spec.attrName
}

func (r *accountProductResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: fmt.Sprintf("Enables %s for the account. Destroying this resource disables the product.", r.spec.displayName),
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: fmt.Sprintf("The ID of this resource (always `%s`).", r.spec.productID),
			},
			"enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: fmt.Sprintf("Whether %s is enabled for the account. Defaults to `true`.", r.spec.displayName),
			},
		},
	}
}

func (r *accountProductResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	data, diags := fastlyclient.FromProviderData(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || data == nil {
		return
	}

	r.client = data.Client
}

func (r *accountProductResource) apply(ctx context.Context, enabled bool) error {
	if enabled {
		_, err := r.spec.enable(ctx, r.client)
		return err
	}
	if err := r.spec.disable(ctx, r.client); err != nil && !isEntitlementError(err) && !errors.IsNotFound(err) {
		return err
	}
	return nil
}

func (r *accountProductResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan accountProductModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("Creating Fastly Account Product Enablement (%s)", r.spec.attrName))

	enabled := plan.Enabled.IsNull() || plan.Enabled.ValueBool()
	if err := r.apply(ctx, enabled); err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Error configuring %s", r.spec.attrName), err.Error())
		return
	}

	plan.ID = types.StringValue(r.spec.productID)
	plan.Enabled = types.BoolValue(enabled)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read reports a 404 or "no product" response as enabled = false instead of
// dropping the resource, matching the service-level product resources.
func (r *accountProductResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state accountProductModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("Reading Fastly Account Product Enablement (%s)", r.spec.attrName))

	if _, err := r.spec.get(ctx, r.client); err != nil {
		if !errors.IsNotFound(err) && !isProductDisabledError(err) {
			resp.Diagnostics.AddError(fmt.Sprintf("Error reading %s", r.spec.attrName), err.Error())
			return
		}
		state.Enabled = types.BoolValue(false)
	} else {
		state.Enabled = types.BoolValue(true)
	}

	state.ID = types.StringValue(r.spec.productID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *accountProductResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan accountProductModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	enabled := plan.Enabled.IsNull() || plan.Enabled.ValueBool()

	tflog.Debug(ctx, fmt.Sprintf("Updating Fastly Account Product Enablement (%s)", r.spec.attrName), map[string]any{
		"enabled": enabled,
	})

	if err := r.apply(ctx, enabled); err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Error configuring %s", r.spec.attrName), err.Error())
		return
	}

	plan.ID = types.StringValue(r.spec.productID)
	plan.Enabled = types.BoolValue(enabled)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete disables the product account-wide. Entitlement errors are treated as
// success so `terraform destroy` can still complete.
func (r *accountProductResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state accountProductModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("Deleting Fastly Account Product Enablement (%s)", r.spec.attrName))

	if state.Enabled.IsNull() || state.Enabled.ValueBool() {
		if err := r.apply(ctx, false); err != nil {
			resp.Diagnostics.AddError(fmt.Sprintf("Error disabling %s", r.spec.attrName), err.Error())
		}
	}
}

// ImportState accepts only the product ID: an account has one instance of each.
func (r *accountProductResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID != r.spec.productID {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("%s is an account-level product; import it with the ID %q.", r.spec.displayName, r.spec.productID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
