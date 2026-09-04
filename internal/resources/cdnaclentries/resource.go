package cdnaclentries

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/fastly/go-fastly/v17/fastly"
	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"
	"github.com/fastly/terraform-provider-fastly-beta/internal/validation"
)

var (
	_ resource.Resource                = &Resource{}
	_ resource.ResourceWithImportState = &Resource{}
)

type Resource struct {
	providerData *fastlyclient.Data
}

func NewResource() resource.Resource {
	return &Resource{}
}

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_cdn_acl_entries"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages ACL entries for a Fastly service ACL. Terraform manages only the entries declared in the `entry` blocks and leaves other ACL entries unchanged.",
		Attributes:  ResourceAttributes(),
		Blocks:      ResourceBlocks(),
	}
}

func (r *Resource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	data, diags := fastlyclient.FromProviderData(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || data == nil {
		return
	}

	r.providerData = data
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serviceID := plan.ServiceID.ValueString()
	aclID := plan.ACLID.ValueString()

	if err := validation.EnsureServiceTypeSupported(ctx, r.providerData.TypeChecker, serviceID, "fastly_service_cdn_acl_entries", service.TypeVCL); err != nil {
		resp.Diagnostics.AddError("Unsupported Fastly service type", err.Error())
		return
	}

	desired := expandEntries(ctx, plan.Entry, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating Fastly service ACL entries", map[string]any{
		"service_id": serviceID,
		"acl_id":     aclID,
		"count":      len(desired),
	})

	remote, err := r.listEntries(ctx, serviceID, aclID)
	if err != nil {
		resp.Diagnostics.AddError("Error reading ACL entries before create", err.Error())
		return
	}

	batch := buildBatchEntries(ctx, remote, nil, desired)
	if err := r.executeBatch(ctx, serviceID, aclID, batch); err != nil {
		resp.Diagnostics.AddError(
			"Error creating ACL entries",
			fmt.Sprintf("service %s, ACL %s: %s", serviceID, aclID, err),
		)
		return
	}

	plan.ID = types.StringValue(fmt.Sprintf("%s/%s", serviceID, aclID))

	refreshed, err := r.listEntries(ctx, serviceID, aclID)
	if err != nil {
		resp.Diagnostics.AddError("Error refreshing ACL entries", err.Error())
		return
	}

	plan.Entry = flattenEntries(ctx, filterManagedRemoteEntries(refreshed, desired), plan.Entry, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serviceID := state.ServiceID.ValueString()
	aclID := state.ACLID.ValueString()

	tflog.Debug(ctx, "Reading Fastly service ACL entries", map[string]any{
		"service_id": serviceID,
		"acl_id":     aclID,
	})

	remote, err := r.listEntries(ctx, serviceID, aclID)
	if err != nil {
		if errors.IsNotFound(err) {
			tflog.Warn(ctx, "ACL not found, removing ACL entries from state", map[string]any{
				"service_id": serviceID,
				"acl_id":     aclID,
			})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading ACL entries", err.Error())
		return
	}

	// Import starts with only service_id/acl_id known, so entry is null. In that
	// case, adopt every existing entry into Terraform state. During ordinary
	// refreshes, preserve partial ownership by reading only entries that this
	// resource already manages.
	if state.Entry.IsNull() || state.Entry.IsUnknown() {
		state.Entry = flattenEntries(ctx, remote, state.Entry, &resp.Diagnostics)
	} else {
		managed := expandEntries(ctx, state.Entry, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
		state.Entry = flattenEntries(ctx, filterManagedRemoteEntries(remote, managed), state.Entry, &resp.Diagnostics)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	state.ID = types.StringValue(fmt.Sprintf("%s/%s", serviceID, aclID))
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

	serviceID := plan.ServiceID.ValueString()
	aclID := plan.ACLID.ValueString()

	if err := validation.EnsureServiceTypeSupported(ctx, r.providerData.TypeChecker, serviceID, "fastly_service_cdn_acl_entries", service.TypeVCL); err != nil {
		resp.Diagnostics.AddError("Unsupported Fastly service type", err.Error())
		return
	}

	desired := expandEntries(ctx, plan.Entry, &resp.Diagnostics)
	currentManaged := expandEntries(ctx, state.Entry, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating Fastly service ACL entries", map[string]any{
		"service_id": serviceID,
		"acl_id":     aclID,
		"count":      len(desired),
	})

	remote, err := r.listEntries(ctx, serviceID, aclID)
	if err != nil {
		if errors.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading ACL entries before update", err.Error())
		return
	}

	batch := buildBatchEntries(ctx, remote, currentManaged, desired)
	if err := r.executeBatch(ctx, serviceID, aclID, batch); err != nil {
		resp.Diagnostics.AddError(
			"Error updating ACL entries",
			fmt.Sprintf("service %s, ACL %s: %s", serviceID, aclID, err),
		)
		return
	}

	plan.ID = types.StringValue(fmt.Sprintf("%s/%s", serviceID, aclID))

	refreshed, err := r.listEntries(ctx, serviceID, aclID)
	if err != nil {
		resp.Diagnostics.AddError("Error refreshing ACL entries", err.Error())
		return
	}

	plan.Entry = flattenEntries(ctx, filterManagedRemoteEntries(refreshed, desired), plan.Entry, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	serviceID := state.ServiceID.ValueString()
	aclID := state.ACLID.ValueString()

	if err := validation.EnsureServiceTypeSupported(ctx, r.providerData.TypeChecker, serviceID, "fastly_service_cdn_acl_entries", service.TypeVCL); err != nil {
		resp.Diagnostics.AddError("Unsupported Fastly service type", err.Error())
		return
	}

	managed := expandEntries(ctx, state.Entry, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting Fastly service ACL entries", map[string]any{
		"service_id": serviceID,
		"acl_id":     aclID,
		"count":      len(managed),
	})

	remote, err := r.listEntries(ctx, serviceID, aclID)
	if err != nil {
		if errors.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error reading ACL entries before delete", err.Error())
		return
	}

	batch := buildBatchEntries(ctx, remote, managed, nil)
	if err := r.executeBatch(ctx, serviceID, aclID, batch); err != nil {
		if errors.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError(
			"Error deleting ACL entries",
			fmt.Sprintf("service %s, ACL %s: %s", serviceID, aclID, err),
		)
	}
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	split := strings.Split(req.ID, "/")

	if len(split) != 2 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Invalid id: %s. The ID should be in the format [service_id]/[acl_id]", req.ID),
		)
		return
	}

	serviceID := split[0]
	aclID := split[1]

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("service_id"), serviceID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("acl_id"), aclID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func (r *Resource) listEntries(ctx context.Context, serviceID, aclID string) ([]*fastly.ACLEntry, error) {
	paginator := r.providerData.Client.GetACLEntries(ctx, &fastly.GetACLEntriesInput{
		ServiceID: serviceID,
		ACLID:     aclID,
	})

	var entries []*fastly.ACLEntry
	for paginator.HasNext() {
		results, err := paginator.GetNext()
		if err != nil {
			return nil, err
		}
		entries = append(entries, results...)
	}
	return entries, nil
}

func (r *Resource) executeBatch(ctx context.Context, serviceID, aclID string, batch []*fastly.BatchACLEntry) error {
	if len(batch) == 0 {
		return nil
	}

	batchSize := fastly.BatchModifyMaximumOperations
	for i := 0; i < len(batch); i += batchSize {
		j := min(i+batchSize, len(batch))

		if err := r.providerData.Client.BatchModifyACLEntries(ctx, &fastly.BatchModifyACLEntriesInput{
			ServiceID: serviceID,
			ACLID:     aclID,
			Entries:   batch[i:j],
		}); err != nil {
			return err
		}
	}

	return nil
}

// buildBatchEntries reconciles Terraform-owned entries against current Fastly
// state. currentManaged determines which entries Terraform may delete; remote
// entries that were never managed by this resource are left untouched.
// Entries are matched by (ip, subnet), which is what Fastly's ACL enforces
// uniqueness on -- not the full content -- so a comment/negated-only change is
// diffed as an update to the existing entry rather than a delete-and-create
// pair at the same ip/subnet, which the batch API would reject.
func buildBatchEntries(ctx context.Context, remote []*fastly.ACLEntry, currentManaged, desired []EntryModel) []*fastly.BatchACLEntry {
	remoteByIdentity := make(map[string]*fastly.ACLEntry, len(remote))
	for _, e := range remote {
		remoteByIdentity[remoteEntryIdentityKey(e)] = e
	}

	desiredByIdentity := make(map[string]EntryModel, len(desired))
	for _, e := range desired {
		desiredByIdentity[entryIdentityKey(e)] = e
	}

	var batch []*fastly.BatchACLEntry

	deleteIDs := make([]string, 0)
	for _, e := range currentManaged {
		key := entryIdentityKey(e)
		if _, stillDesired := desiredByIdentity[key]; stillDesired {
			continue
		}
		if remoteEntry, exists := remoteByIdentity[key]; exists && remoteEntry.EntryID != nil {
			deleteIDs = append(deleteIDs, *remoteEntry.EntryID)
		}
	}
	sort.Strings(deleteIDs)
	for _, id := range deleteIDs {
		op := fastly.DeleteBatchOperation
		batch = append(batch, &fastly.BatchACLEntry{Operation: &op, EntryID: new(id)})
	}

	desiredKeys := make([]string, 0, len(desired))
	for key := range desiredByIdentity {
		desiredKeys = append(desiredKeys, key)
	}
	sort.Strings(desiredKeys)

	for _, key := range desiredKeys {
		entry := desiredByIdentity[key]
		remoteEntry, exists := remoteByIdentity[key]

		if !exists {
			batch = append(batch, buildBatchACLEntry(ctx, entry, fastly.CreateBatchOperation))
			continue
		}

		if !remoteEntryMatchesDesired(remoteEntry, entry) {
			adopted := entry
			if remoteEntry.EntryID != nil {
				adopted.ID = types.StringValue(*remoteEntry.EntryID)
			}
			batch = append(batch, buildBatchACLEntry(ctx, adopted, fastly.UpdateBatchOperation))
		}
	}

	return batch
}

// remoteEntryMatchesDesired reports whether the remote entry already reflects
// the desired negated/comment values. Fields left null/unknown in desired are
// not compared, since there is nothing to diff against.
func remoteEntryMatchesDesired(remote *fastly.ACLEntry, desired EntryModel) bool {
	if !desired.Comment.IsNull() && !desired.Comment.IsUnknown() {
		remoteComment := ""
		if remote.Comment != nil {
			remoteComment = *remote.Comment
		}
		if remoteComment != desired.Comment.ValueString() {
			return false
		}
	}

	if !desired.Negated.IsNull() && !desired.Negated.IsUnknown() {
		remoteNegated := false
		if remote.Negated != nil {
			remoteNegated = bool(*remote.Negated)
		}
		if remoteNegated != desired.Negated.ValueBool() {
			return false
		}
	}

	return true
}

// filterManagedRemoteEntries returns only remote entries already owned by
// this Terraform resource. Undeclared ACL entries are intentionally ignored.
func filterManagedRemoteEntries(remote []*fastly.ACLEntry, managed []EntryModel) []*fastly.ACLEntry {
	managedIdentities := make(map[string]struct{}, len(managed))
	for _, e := range managed {
		managedIdentities[entryIdentityKey(e)] = struct{}{}
	}

	var result []*fastly.ACLEntry
	for _, e := range remote {
		if _, ok := managedIdentities[remoteEntryIdentityKey(e)]; ok {
			result = append(result, e)
		}
	}
	return result
}

// entryIdentityKey returns the (ip, subnet) pair Fastly's ACL enforces
// uniqueness on, for entries sourced from Terraform configuration/state.
func entryIdentityKey(e EntryModel) string {
	ip := ""
	subnet := int64(0)

	if !e.IP.IsNull() && !e.IP.IsUnknown() {
		ip = e.IP.ValueString()
	}
	if !e.Subnet.IsNull() && !e.Subnet.IsUnknown() {
		subnet = e.Subnet.ValueInt64()
	}

	return fmt.Sprintf("%s|%d", ip, subnet)
}

// remoteEntryIdentityKey mirrors entryIdentityKey for entries sourced from the API.
func remoteEntryIdentityKey(e *fastly.ACLEntry) string {
	ip := ""
	subnet := 0

	if e.IP != nil {
		ip = *e.IP
	}
	if e.Subnet != nil {
		subnet = *e.Subnet
	}

	return fmt.Sprintf("%s|%d", ip, subnet)
}
