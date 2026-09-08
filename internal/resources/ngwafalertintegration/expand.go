package ngwafalertintegration

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func stringValue(value types.String) string {
	if value.IsNull() || value.IsUnknown() {
		return ""
	}
	return value.ValueString()
}

func ValueForAttribute(plan Model, name string) string {
	switch name {
	case "description":
		return stringValue(plan.Description)
	case "address":
		return stringValue(plan.Address)
	case "host":
		return stringValue(plan.Host)
	case "issue_type":
		return stringValue(plan.IssueType)
	case "key":
		return stringValue(plan.Key)
	case "project":
		return stringValue(plan.Project)
	case "site":
		return stringValue(plan.Site)
	case "username":
		return stringValue(plan.Username)
	case "webhook":
		return stringValue(plan.Webhook)
	default:
		return ""
	}
}

func StringPointer(value string) *string {
	return &value
}

func FlagEvents() *[]string {
	events := []string{"flag"}
	return &events
}

func StringFromAny(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case *string:
		if v == nil {
			return ""
		}
		return *v
	default:
		return fmt.Sprint(v)
	}
}

func ensureRemoteType(def Definition, remote *RemoteAlert) error {
	if remote == nil {
		return fmt.Errorf("cannot flatten nil NGWAF workspace %s alert integration", def.Type)
	}
	if remote.ID == "" {
		return fmt.Errorf("invalid NGWAF workspace %s alert integration: id is missing", def.Type)
	}
	if remote.Type != "" && remote.Type != def.Type {
		return fmt.Errorf("alert integration %s is a %q integration, not %q; use the resource for %q alert integrations instead", remote.ID, remote.Type, def.Type, remote.Type)
	}
	return nil
}
