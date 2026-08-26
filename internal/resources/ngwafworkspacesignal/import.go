package ngwafworkspacesignal

import (
	"fmt"
	"strings"
)

func ParseImportID(id string) (workspaceID, signalID string, err error) {
	parts := strings.Split(id, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("expected import ID in format WORKSPACE_ID/SIGNAL_ID")
	}

	return parts[0], parts[1], nil
}
