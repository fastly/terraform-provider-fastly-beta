package ngwafworkspacealertwebhookintegration

import (
	"context"

	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafalertintegration"

	"github.com/hashicorp/terraform-plugin-framework/resource"

	fastly "github.com/fastly/go-fastly/v17/fastly"
	WebhookAlerts "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/alerts/webhook"
)

const alertType = "webhook"

func NewResource() resource.Resource {
	return ngwafalertintegration.NewWorkspaceResource(definition())
}

func DefinitionForDataSource() ngwafalertintegration.Definition {
	return definition()
}

func definition() ngwafalertintegration.Definition {
	return ngwafalertintegration.Definition{
		Type:        alertType,
		TypeSuffix:  "webhook",
		DisplayName: "Webhook",
		Description: "Manages a Fastly Next-Gen WAF Webhook alert integration scoped to a single workspace.",
		ConfigAttrs: []ngwafalertintegration.ConfigAttribute{
			{Name: "webhook", Description: "Webhook URL.", Sensitive: true},
		},
		Operations: operations{},
	}
}

type operations struct{}

func (operations) Create(ctx context.Context, client *fastly.Client, plan ngwafalertintegration.Model) (*ngwafalertintegration.RemoteAlert, error) {
	input := &WebhookAlerts.CreateInput{
		Config: &WebhookAlerts.CreateConfig{
			Webhook: ngwafalertintegration.StringPointer(ngwafalertintegration.ValueForAttribute(plan, "webhook")),
		},
		Description: ngwafalertintegration.StringPointer(ngwafalertintegration.ValueForAttribute(plan, "description")),
		Events:      ngwafalertintegration.FlagEvents(),
		WorkspaceID: ngwafalertintegration.StringPointer(plan.WorkspaceID.ValueString()),
	}
	alert, err := WebhookAlerts.Create(ctx, client, input)
	if err != nil {
		return nil, err
	}
	return remoteAlert(alert)
}

func (operations) Get(ctx context.Context, client *fastly.Client, workspaceID, alertID string) (*ngwafalertintegration.RemoteAlert, error) {
	alert, err := WebhookAlerts.Get(ctx, client, &WebhookAlerts.GetInput{
		AlertID:     &alertID,
		WorkspaceID: &workspaceID,
	})
	if err != nil {
		return nil, err
	}
	return remoteAlert(alert)
}

func (operations) Update(ctx context.Context, client *fastly.Client, alertID string, plan ngwafalertintegration.Model) (*ngwafalertintegration.RemoteAlert, error) {
	input := &WebhookAlerts.UpdateInput{
		AlertID: &alertID,
		Config: &WebhookAlerts.UpdateConfig{
			Webhook: ngwafalertintegration.StringPointer(ngwafalertintegration.ValueForAttribute(plan, "webhook")),
		},
		Events:      ngwafalertintegration.FlagEvents(),
		WorkspaceID: ngwafalertintegration.StringPointer(plan.WorkspaceID.ValueString()),
	}
	alert, err := WebhookAlerts.Update(ctx, client, input)
	if err != nil {
		return nil, err
	}
	return remoteAlert(alert)
}

func (operations) Delete(ctx context.Context, client *fastly.Client, workspaceID, alertID string) error {
	return WebhookAlerts.Delete(ctx, client, &WebhookAlerts.DeleteInput{
		AlertID:     &alertID,
		WorkspaceID: &workspaceID,
	})
}

func (operations) List(ctx context.Context, client *fastly.Client, workspaceID string) ([]ngwafalertintegration.RemoteAlert, error) {
	alerts, err := WebhookAlerts.List(ctx, client, &WebhookAlerts.ListInput{WorkspaceID: &workspaceID})
	if err != nil {
		return nil, err
	}

	var data []WebhookAlerts.Alert
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

func remoteAlert(alert *WebhookAlerts.Alert) (*ngwafalertintegration.RemoteAlert, error) {
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
