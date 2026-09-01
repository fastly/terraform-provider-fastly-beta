package ngwafworkspacealertmicrosoftteamsintegration

import (
	"context"

	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafalertintegration"

	"github.com/hashicorp/terraform-plugin-framework/resource"

	fastly "github.com/fastly/go-fastly/v17/fastly"
	MicrosoftTeamsAlerts "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/alerts/microsoftteams"
)

const alertType = "microsoftteams"

func NewResource() resource.Resource {
	return ngwafalertintegration.NewWorkspaceResource(definition())
}

func DefinitionForDataSource() ngwafalertintegration.Definition {
	return definition()
}

func definition() ngwafalertintegration.Definition {
	return ngwafalertintegration.Definition{
		Type:        alertType,
		TypeSuffix:  "microsoft_teams",
		DisplayName: "Microsoft Teams",
		Description: "Manages a Fastly Next-Gen WAF Microsoft Teams alert integration scoped to a single workspace.",
		ConfigAttrs: []ngwafalertintegration.ConfigAttribute{
			{Name: "webhook", Description: "Microsoft Teams webhook URL.", Sensitive: true},
		},
		Operations: operations{},
	}
}

type operations struct{}

func (operations) Create(ctx context.Context, client *fastly.Client, plan ngwafalertintegration.Model) (*ngwafalertintegration.RemoteAlert, error) {
	input := &MicrosoftTeamsAlerts.CreateInput{
		Config: &MicrosoftTeamsAlerts.CreateConfig{
			Webhook: ngwafalertintegration.StringPointer(ngwafalertintegration.ValueForAttribute(plan, "webhook")),
		},
		Description: ngwafalertintegration.StringPointer(ngwafalertintegration.ValueForAttribute(plan, "description")),
		Events:      ngwafalertintegration.FlagEvents(),
		WorkspaceID: ngwafalertintegration.StringPointer(plan.WorkspaceID.ValueString()),
	}
	alert, err := MicrosoftTeamsAlerts.Create(ctx, client, input)
	if err != nil {
		return nil, err
	}
	return remoteAlert(alert)
}

func (operations) Get(ctx context.Context, client *fastly.Client, workspaceID, alertID string) (*ngwafalertintegration.RemoteAlert, error) {
	alert, err := MicrosoftTeamsAlerts.Get(ctx, client, &MicrosoftTeamsAlerts.GetInput{
		AlertID:     &alertID,
		WorkspaceID: &workspaceID,
	})
	if err != nil {
		return nil, err
	}
	return remoteAlert(alert)
}

func (operations) Update(ctx context.Context, client *fastly.Client, alertID string, plan ngwafalertintegration.Model) (*ngwafalertintegration.RemoteAlert, error) {
	input := &MicrosoftTeamsAlerts.UpdateInput{
		AlertID: &alertID,
		Config: &MicrosoftTeamsAlerts.UpdateConfig{
			Webhook: ngwafalertintegration.StringPointer(ngwafalertintegration.ValueForAttribute(plan, "webhook")),
		},
		Events:      ngwafalertintegration.FlagEvents(),
		WorkspaceID: ngwafalertintegration.StringPointer(plan.WorkspaceID.ValueString()),
	}
	alert, err := MicrosoftTeamsAlerts.Update(ctx, client, input)
	if err != nil {
		return nil, err
	}
	return remoteAlert(alert)
}

func (operations) Delete(ctx context.Context, client *fastly.Client, workspaceID, alertID string) error {
	return MicrosoftTeamsAlerts.Delete(ctx, client, &MicrosoftTeamsAlerts.DeleteInput{
		AlertID:     &alertID,
		WorkspaceID: &workspaceID,
	})
}

func (operations) List(ctx context.Context, client *fastly.Client, workspaceID string) ([]ngwafalertintegration.RemoteAlert, error) {
	alerts, err := MicrosoftTeamsAlerts.List(ctx, client, &MicrosoftTeamsAlerts.ListInput{WorkspaceID: &workspaceID})
	if err != nil {
		return nil, err
	}

	var data []MicrosoftTeamsAlerts.Alert
	if alerts != nil {
		data = alerts.Data
	}

	result := make([]ngwafalertintegration.RemoteAlert, 0, len(data))
	for i := range data {
		remote, err := remoteAlert(&data[i])
		if err != nil {
			return nil, err
		}
		result = append(result, *remote)
	}
	return result, nil
}

func remoteAlert(alert *MicrosoftTeamsAlerts.Alert) (*ngwafalertintegration.RemoteAlert, error) {
	if alert == nil {
		return nil, nil
	}
	return &ngwafalertintegration.RemoteAlert{
		ID:          alert.ID,
		Type:        alert.Type,
		Description: alert.Description,
		Config: map[string]string{
			"webhook": ngwafalertintegration.StringFromAny(alert.Config.Webhook),
		},
	}, nil
}
