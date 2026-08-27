package configstoreitems

import (
	"context"
	"fmt"
	"sort"
	"strings"

	fastlyclient "github.com/fastly/terraform-provider-fastly/internal/client"
	"github.com/fastly/terraform-provider-fastly/internal/errors"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
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

type Model struct {
	ID      types.String `tfsdk:"id"`
	StoreID types.String `tfsdk:"store_id"`
	Items   types.Map    `tfsdk:"items"`
}

func NewResource() resource.Resource {
	return &Resource{}
}

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_configstore_items"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages key-value items in a Fastly Config Store. Terraform manages only the keys declared in the items map and leaves other Config Store items unchanged.",
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

	desired := expandItems(ctx, plan.Items, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	storeID := plan.StoreID.ValueString()
	tflog.Debug(ctx, "Creating Fastly Config Store items", map[string]any{
		"store_id": storeID,
		"count":    len(desired),
	})

	remote, err := r.listItems(ctx, storeID)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Config Store items before create", err.Error())
		return
	}

	operations := buildBatchOperations(remote, nil, desired)
	if err := r.executeBatchOperations(ctx, storeID, operations); err != nil {
		resp.Diagnostics.AddError(
			"Error creating Config Store items",
			fmt.Sprintf("Config Store %s: %s", storeID, err),
		)
		return
	}

	plan.ID = types.StringValue(resourceID(storeID))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	storeID := state.StoreID.ValueString()
	tflog.Debug(ctx, "Reading Fastly Config Store items", map[string]any{
		"store_id": storeID,
	})

	remote, err := r.listItems(ctx, storeID)
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "Config Store not found, removing Config Store items from state", map[string]any{
				"store_id": storeID,
			})
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Error reading Config Store items", err.Error())
		return
	}

	// Import starts with store_id and id only. In that case, adopt every
	// existing item into Terraform state. During ordinary refreshes, preserve
	// partial ownership by reading only keys that this resource already owns.
	if state.Items.IsNull() || state.Items.IsUnknown() {
		state.Items = flattenItems(ctx, remote, &resp.Diagnostics)
	} else {
		managed := expandItems(ctx, state.Items, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
		state.Items = flattenItems(ctx, filterManagedRemoteItems(remote, managed), &resp.Diagnostics)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	state.ID = types.StringValue(resourceID(storeID))
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	desired := expandItems(ctx, plan.Items, &resp.Diagnostics)
	currentManaged := expandItems(ctx, state.Items, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	storeID := plan.StoreID.ValueString()
	tflog.Debug(ctx, "Updating Fastly Config Store items", map[string]any{
		"store_id": storeID,
		"count":    len(desired),
	})

	remote, err := r.listItems(ctx, storeID)
	if err != nil {
		if errors.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Config Store items before update", err.Error())
		return
	}

	operations := buildBatchOperations(remote, currentManaged, desired)
	if err := r.executeBatchOperations(ctx, storeID, operations); err != nil {
		resp.Diagnostics.AddError(
			"Error updating Config Store items",
			fmt.Sprintf("Config Store %s: %s", storeID, err),
		)
		return
	}

	plan.ID = types.StringValue(resourceID(storeID))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	managed := expandItems(ctx, state.Items, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	storeID := state.StoreID.ValueString()
	tflog.Debug(ctx, "Deleting Fastly Config Store items", map[string]any{
		"store_id": storeID,
		"count":    len(managed),
	})

	remote, err := r.listItems(ctx, storeID)
	if err != nil {
		if errors.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error reading Config Store items before delete", err.Error())
		return
	}

	operations := buildBatchOperations(remote, managed, nil)
	if err := r.executeBatchOperations(ctx, storeID, operations); err != nil {
		resp.Diagnostics.AddError(
			"Error deleting Config Store items",
			fmt.Sprintf("Config Store %s: %s", storeID, err),
		)
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	storeID, suffix, ok := strings.Cut(req.ID, "/")
	if !ok || storeID == "" || suffix != "items" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Invalid id: %s. The ID should be in the format <store_id>/items", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), resourceID(storeID))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("store_id"), storeID)...)
}

func (r *Resource) listItems(ctx context.Context, storeID string) (map[string]string, error) {
	items, err := r.client.ListConfigStoreItems(ctx, &fastly.ListConfigStoreItemsInput{
		StoreID: storeID,
	})
	if err != nil {
		return nil, err
	}

	result := make(map[string]string, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result[item.Key] = item.Value
	}
	return result, nil
}

func (r *Resource) executeBatchOperations(ctx context.Context, storeID string, operations []*fastly.BatchConfigStoreItem) error {
	batchSize := fastly.BatchModifyMaximumOperations

	for i := 0; i < len(operations); i += batchSize {
		j := min(i+batchSize, len(operations))

		if err := r.client.BatchModifyConfigStoreItems(ctx, &fastly.BatchModifyConfigStoreItemsInput{
			StoreID: storeID,
			Items:   operations[i:j],
		}); err != nil {
			return err
		}
	}

	return nil
}

func expandItems(ctx context.Context, value types.Map, diags *diag.Diagnostics) map[string]string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}

	var result map[string]string
	diags.Append(value.ElementsAs(ctx, &result, false)...)
	return result
}

func flattenItems(ctx context.Context, items map[string]string, diags *diag.Diagnostics) types.Map {
	value, valueDiags := types.MapValueFrom(ctx, types.StringType, items)
	diags.Append(valueDiags...)
	return value
}

// filterManagedRemoteItems returns only remote keys already owned by this
// Terraform resource. Undeclared Config Store items are intentionally ignored.
func filterManagedRemoteItems(remote, managed map[string]string) map[string]string {
	result := make(map[string]string, len(managed))
	for key := range managed {
		if value, ok := remote[key]; ok {
			result[key] = value
		}
	}
	return result
}

// buildBatchOperations reconciles Terraform-owned keys against current Fastly
// state. currentManaged determines which keys Terraform may delete; remote keys
// that have never been managed by this resource are left untouched.
func buildBatchOperations(remote, currentManaged, desired map[string]string) []*fastly.BatchConfigStoreItem {
	var operations []*fastly.BatchConfigStoreItem

	deleteKeys := make([]string, 0)
	for key := range currentManaged {
		if _, stillDesired := desired[key]; stillDesired {
			continue
		}
		if _, exists := remote[key]; exists {
			deleteKeys = append(deleteKeys, key)
		}
	}
	sort.Strings(deleteKeys)
	for _, key := range deleteKeys {
		operations = append(operations, &fastly.BatchConfigStoreItem{
			Operation: fastly.DeleteBatchOperation,
			ItemKey:   key,
		})
	}

	desiredKeys := make([]string, 0, len(desired))
	for key := range desired {
		desiredKeys = append(desiredKeys, key)
	}
	sort.Strings(desiredKeys)

	for _, key := range desiredKeys {
		value := desired[key]
		remoteValue, exists := remote[key]

		switch {
		case !exists:
			operations = append(operations, &fastly.BatchConfigStoreItem{
				Operation: fastly.CreateBatchOperation,
				ItemKey:   key,
				ItemValue: value,
			})
		case remoteValue != value:
			operations = append(operations, &fastly.BatchConfigStoreItem{
				Operation: fastly.UpdateBatchOperation,
				ItemKey:   key,
				ItemValue: value,
			})
		}
	}

	return operations
}

func resourceID(storeID string) string {
	return fmt.Sprintf("%s/items", storeID)
}
