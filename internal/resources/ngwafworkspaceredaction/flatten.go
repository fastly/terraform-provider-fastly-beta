package ngwafworkspaceredaction

import (
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/redactions"
)

func FlattenToModel(workspaceID string, redaction *redactions.Redaction) Model {
	return Model{
		ID:          types.StringValue(redaction.RedactionID),
		WorkspaceID: types.StringValue(workspaceID),
		Field:       types.StringValue(redaction.Field),
		Type:        types.StringValue(redaction.Type),
	}
}
