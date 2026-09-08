package aclentries

import (
	"context"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly/computeacls"
)

const (
	createOperation = "create"
	updateOperation = "update"
	deleteOperation = "delete"
)

func expandEntries(ctx context.Context, entries types.Map, diags *diag.Diagnostics) map[string]string {
	if entries.IsNull() || entries.IsUnknown() {
		return nil
	}

	var result map[string]string
	diags.Append(entries.ElementsAs(ctx, &result, false)...)
	return result
}

// filterManagedRemoteEntries returns only remote prefixes already owned by this
// Terraform resource. Undeclared ACL entries are intentionally ignored.
func filterManagedRemoteEntries(remote, managed map[string]string) map[string]string {
	result := make(map[string]string, len(managed))
	for prefix := range managed {
		if action, ok := remote[prefix]; ok {
			result[prefix] = action
		}
	}
	return result
}

// buildBatchEntries reconciles Terraform-owned prefixes against current Fastly
// state. currentManaged determines which prefixes Terraform may delete; remote
// prefixes that have never been managed by this resource are left untouched.
func buildBatchEntries(remote, currentManaged, desired map[string]string) []*computeacls.BatchComputeACLEntry {
	var batch []*computeacls.BatchComputeACLEntry

	deletePrefixes := make([]string, 0)
	for prefix := range currentManaged {
		if _, stillDesired := desired[prefix]; stillDesired {
			continue
		}
		if _, exists := remote[prefix]; exists {
			deletePrefixes = append(deletePrefixes, prefix)
		}
	}
	sort.Strings(deletePrefixes)
	for _, prefix := range deletePrefixes {
		batch = append(batch, &computeacls.BatchComputeACLEntry{
			Prefix:    new(prefix),
			Operation: new(deleteOperation),
		})
	}

	desiredPrefixes := make([]string, 0, len(desired))
	for prefix := range desired {
		desiredPrefixes = append(desiredPrefixes, prefix)
	}
	sort.Strings(desiredPrefixes)

	for _, prefix := range desiredPrefixes {
		action := desired[prefix]
		remoteAction, exists := remote[prefix]

		switch {
		case !exists:
			batch = append(batch, &computeacls.BatchComputeACLEntry{
				Prefix:    new(prefix),
				Action:    new(action),
				Operation: new(createOperation),
			})
		case remoteAction != action:
			batch = append(batch, &computeacls.BatchComputeACLEntry{
				Prefix:    new(prefix),
				Action:    new(action),
				Operation: new(updateOperation),
			})
		}
	}

	return batch
}
