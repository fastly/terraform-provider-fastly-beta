package ngwafworkspaceredaction

import (
	"fmt"
	"strings"
)

func ParseImportID(id string) (workspaceID, redactionID string, err error) {
	parts := strings.Split(id, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid composite import ID format: expected workspace_id/redaction_id, got %q", id)
	}

	if parts[0] == "" {
		return "", "", fmt.Errorf("workspace_id cannot be empty in import ID %q", id)
	}

	if parts[1] == "" {
		return "", "", fmt.Errorf("redaction_id cannot be empty in import ID %q", id)
	}

	return parts[0], parts[1], nil
}
