package ngwafworkspacealertslackintegration

import (
	"context"

	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafalertintegration"

	"github.com/hashicorp/terraform-plugin-framework/resource"

	fastly "github.com/fastly/go-fastly/v17/fastly"
	SlackAlerts "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/alerts/slack"
)

const alertType = "slack"

func NewResource() resource.Resource {
	return ngwafalertintegration.NewWorkspaceResource(definition())
}

func DefinitionForDataSource() ngwafalertintegration.Definition {
	return definition()
}

func definition() ngwafalertintegration.Definition {
	return ngwafalertintegration.Definition{
		Type:        alertType,
		TypeSuffix:  "slack",
		DisplayName: "Slack",
		Description: "Manages a Fastly Next-Gen WAF Slack alert integration scoped to a single workspace.",
		ConfigAttrs: []ngwafalertintegration.ConfigAttribute{
			{Name: "webhook", Description: "Slack webhook URL.", Sensitive: true},
		},
		Operations: operations{},
	}
}

type operations struct{}

func (operations) Create(ctx context.Context, client *fastly.Client, plan ngwafalertintegration.Model) (*ngwafalertintegration.RemoteAlert, error) {
	input := &SlackAlerts.CreateInput{
		Config: &SlackAlerts.CreateConfig{
			Webhook: ngwafalertintegration.StringPointer(ngwafalertintegration.ValueForAttribute(plan, "webhook")),
		},
		Description: ngwafalertintegration.StringPointer(ngwafalertintegration.ValueForAttribute(plan, "description")),
		Events:      ngwafalertintegration.FlagEvents(),
		WorkspaceID: ngwafalertintegration.StringPointer(plan.WorkspaceID.ValueString()),
	}
	alert, err := SlackAlerts.Create(ctx, client, input)
	if err != nil {
		return nil, err
	}
	return remoteAlert(alert)
}

func (operations) Get(ctx context.Context, client *fastly.Client, workspaceID, alertID string) (*ngwafalertintegration.RemoteAlert, error) {
	alert, err := SlackAlerts.Get(ctx, client, &SlackAlerts.GetInput{
		AlertID:     &alertID,
		WorkspaceID: &workspaceID,
	})
	if err != nil {
		return nil, err
	}
	return remoteAlert(alert)
}

func (operations) Update(ctx context.Context, client *fastly.Client, alertID string, plan ngwafalertintegration.Model) (*ngwafalertintegration.RemoteAlert, error) {
	input := &SlackAlerts.UpdateInput{
		AlertID: &alertID,
		Config: &SlackAlerts.UpdateConfig{
			Webhook: ngwafalertintegration.StringPointer(ngwafalertintegration.ValueForAttribute(plan, "webhook")),
		},
		WorkspaceID: ngwafalertintegration.StringPointer(plan.WorkspaceID.ValueString()),
	}
	alert, err := SlackAlerts.Update(ctx, client, input)
	if err != nil {
		return nil, err
	}
	return remoteAlert(alert)
}

func (operations) Delete(ctx context.Context, client *fastly.Client, workspaceID, alertID string) error {
	return SlackAlerts.Delete(ctx, client, &SlackAlerts.DeleteInput{
		AlertID:     &alertID,
		WorkspaceID: &workspaceID,
	})
}

func (operations) List(ctx context.Context, client *fastly.Client, workspaceID string) ([]ngwafalertintegration.RemoteAlert, error) {
	alerts, err := SlackAlerts.List(ctx, client, &SlackAlerts.ListInput{WorkspaceID: &workspaceID})
	if err != nil {
		return nil, err
	}

	var data []SlackAlerts.Alert
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

func remoteAlert(alert *SlackAlerts.Alert) (*ngwafalertintegration.RemoteAlert, error) {
	if alert == nil {
		return nil, nil
	}
	return &ngwafalertintegration.RemoteAlert{
		ID:          alert.ID,
		Type:        alertType,
		Description: alert.Description,
		Config: map[string]string{
			"webhook": ngwafalertintegration.StringFromAny(alert.Config.Webhook),
		},
	}, nil
}
