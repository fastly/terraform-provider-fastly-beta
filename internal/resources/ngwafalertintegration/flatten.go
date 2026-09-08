package ngwafalertintegration

import "github.com/hashicorp/terraform-plugin-framework/types"

func FlattenToModel(def Definition, workspaceID string, remote *RemoteAlert) (Model, error) {
	if err := ensureRemoteType(def, remote); err != nil {
		return Model{}, err
	}

	m := Model{
		ID:          types.StringValue(remote.ID),
		WorkspaceID: types.StringValue(workspaceID),
		Description: types.StringValue(remote.Description),
	}

	for _, attr := range def.ConfigAttrs {
		value := remote.Config[attr.Name]
		switch attr.Name {
		case "address":
			m.Address = types.StringValue(value)
		case "host":
			m.Host = types.StringValue(value)
		case "issue_type":
			m.IssueType = types.StringValue(value)
		case "key":
			m.Key = types.StringValue(value)
		case "project":
			m.Project = types.StringValue(value)
		case "site":
			m.Site = types.StringValue(value)
		case "username":
			m.Username = types.StringValue(value)
		case "webhook":
			m.Webhook = types.StringValue(value)
		}
	}

	return m, nil
}
