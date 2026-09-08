package aclentries

import (
	"context"
	"fmt"
	"strings"
	"time"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/fastly/go-fastly/v17/fastly"
	"github.com/fastly/go-fastly/v17/fastly/computeacls"
)

// The Compute ACL batch update endpoint responds 202 Accepted and applies the
// batch asynchronously, so a list call immediately afterward can race the
// propagation. Poll until the listed entries reflect the batch we sent.
const (
	entriesPollInterval = 500 * time.Millisecond
	entriesPollTimeout  = 30 * time.Second
)

var (
	_ resource.Resource                = &Resource{}
	_ resource.ResourceWithImportState = &Resource{}
)

type Resource struct {
	client *fastly.Client
}

func NewResource() resource.Resource {
	return &Resource{}
}

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_acl_entries"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages CIDR-based allow/block entries within a Fastly ACL. Terraform manages only the prefixes declared in the entries map and leaves other ACL entries unchanged.",
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

	desired := expandEntries(ctx, plan.Entries, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	aclID := plan.ACLID.ValueString()
	tflog.Debug(ctx, "Creating Fastly ACL entries", map[string]any{
		"acl_id": aclID,
		"count":  len(desired),
	})

	remote, err := r.listEntries(ctx, aclID)
	if err != nil {
		resp.Diagnostics.AddError("Error reading ACL entries before create", err.Error())
		return
	}

	batch := buildBatchEntries(remote, nil, desired)
	if err := r.applyBatch(ctx, aclID, batch); err != nil {
		resp.Diagnostics.AddError(
			"Error creating ACL entries",
			fmt.Sprintf("ACL %s: %s", aclID, err),
		)
		return
	}

	plan.ID = types.StringValue(resourceID(aclID))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	aclID := state.ACLID.ValueString()
	tflog.Debug(ctx, "Reading Fastly ACL entries", map[string]any{
		"acl_id": aclID,
	})

	remote, err := r.listEntries(ctx, aclID)
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "ACL not found, removing entries from state", map[string]any{
				"acl_id": aclID,
			})
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Error reading ACL entries", err.Error())
		return
	}

	// Import starts with acl_id and id only. In that case, adopt every existing
	// entry into Terraform state. During ordinary refreshes, preserve partial
	// ownership by reading only prefixes that this resource already owns.
	if state.Entries.IsNull() || state.Entries.IsUnknown() {
		state.Entries = flattenEntries(ctx, remote, &resp.Diagnostics)
	} else {
		managed := expandEntries(ctx, state.Entries, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}

		state.Entries = flattenEntries(ctx, filterManagedRemoteEntries(remote, managed), &resp.Diagnostics)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	state.ID = types.StringValue(resourceID(aclID))
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

	desired := expandEntries(ctx, plan.Entries, &resp.Diagnostics)
	currentManaged := expandEntries(ctx, state.Entries, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	aclID := plan.ACLID.ValueString()
	tflog.Debug(ctx, "Updating Fastly ACL entries", map[string]any{
		"acl_id": aclID,
		"count":  len(desired),
	})

	remote, err := r.listEntries(ctx, aclID)
	if err != nil {
		if errors.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Error reading ACL entries before update", err.Error())
		return
	}

	batch := buildBatchEntries(remote, currentManaged, desired)
	if err := r.applyBatch(ctx, aclID, batch); err != nil {
		resp.Diagnostics.AddError(
			"Error updating ACL entries",
			fmt.Sprintf("ACL %s: %s", aclID, err),
		)
		return
	}

	plan.ID = types.StringValue(resourceID(aclID))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	managed := expandEntries(ctx, state.Entries, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	aclID := state.ACLID.ValueString()
	tflog.Debug(ctx, "Deleting Fastly ACL entries", map[string]any{
		"acl_id": aclID,
		"count":  len(managed),
	})

	remote, err := r.listEntries(ctx, aclID)
	if err != nil {
		if errors.IsNotFound(err) {
			return
		}

		resp.Diagnostics.AddError("Error reading ACL entries before delete", err.Error())
		return
	}

	batch := buildBatchEntries(remote, managed, nil)
	if err := r.applyBatch(ctx, aclID, batch); err != nil {
		if errors.IsNotFound(err) {
			return
		}

		resp.Diagnostics.AddError(
			"Error deleting ACL entries",
			fmt.Sprintf("ACL %s: %s", aclID, err),
		)
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	aclID, suffix, ok := strings.Cut(req.ID, "/")
	if !ok || aclID == "" || suffix != "entries" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Invalid id: %s. The ID should be in the format <acl_id>/entries", req.ID),
		)
		return
	}

	tflog.Debug(ctx, "Importing ACL entries", map[string]any{
		"acl_id": aclID,
	})

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), resourceID(aclID))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("acl_id"), aclID)...)
}

func (r *Resource) listEntries(ctx context.Context, aclID string) (map[string]string, error) {
	entries := make(map[string]string)
	var cursor *string

	for {
		page, err := computeacls.ListEntries(ctx, r.client, &computeacls.ListEntriesInput{
			ComputeACLID: &aclID,
			Cursor:       cursor,
		})
		if err != nil {
			return nil, err
		}

		for _, entry := range page.Entries {
			entries[entry.Prefix] = entry.Action
		}

		if page.Meta.NextCursor == "" {
			break
		}
		cursor = new(page.Meta.NextCursor)
	}

	return entries, nil
}

// applyBatch sends a batch update and waits for Fastly's asynchronous ACL API
// to converge before Terraform commits the planned managed entries to state.
func (r *Resource) applyBatch(ctx context.Context, aclID string, batch []*computeacls.BatchComputeACLEntry) error {
	if len(batch) == 0 {
		return nil
	}

	if err := computeacls.Update(ctx, r.client, &computeacls.UpdateInput{
		ComputeACLID: &aclID,
		Entries:      batch,
	}); err != nil {
		return err
	}

	return r.waitForEntries(ctx, aclID, batch)
}

// waitForEntries polls listEntries until the remote state reflects every
// operation in batch, since the batch update endpoint applies asynchronously.
func (r *Resource) waitForEntries(ctx context.Context, aclID string, batch []*computeacls.BatchComputeACLEntry) error {
	ctx, cancel := context.WithTimeout(ctx, entriesPollTimeout)
	defer cancel()

	for {
		remote, err := r.listEntries(ctx, aclID)
		if err != nil {
			return err
		}

		if entriesConverged(remote, batch) {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out after %s waiting for ACL entries to reflect the update: %w", entriesPollTimeout, ctx.Err())
		case <-time.After(entriesPollInterval):
		}
	}
}

// entriesConverged reports whether remote already reflects every operation
// in batch: deleted prefixes absent, created/updated prefixes present with
// the expected action.
func entriesConverged(remote map[string]string, batch []*computeacls.BatchComputeACLEntry) bool {
	for _, op := range batch {
		action, exists := remote[*op.Prefix]
		if *op.Operation == deleteOperation {
			if exists {
				return false
			}
			continue
		}

		if !exists || action != *op.Action {
			return false
		}
	}

	return true
}

func resourceID(aclID string) string {
	return fmt.Sprintf("%s/entries", aclID)
}
