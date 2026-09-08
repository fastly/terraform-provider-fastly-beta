package ngwafworkspacetemplatedsignalrule

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafrule"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
)

func FlattenToModel(rule *rules.Rule) Model {
	return Model{
		CommonModel: ngwafrule.FlattenCommon(rule),
		WorkspaceID: ngwafrule.FlattenWorkspaceID(rule),
		Action:      ngwafrule.FlattenSignalActions(rule.Actions),
	}
}
