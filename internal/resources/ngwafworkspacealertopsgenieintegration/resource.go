package ngwafworkspacealertopsgenieintegration

import (
	"context"

	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafalertintegration"

	"github.com/hashicorp/terraform-plugin-framework/resource"

	fastly "github.com/fastly/go-fastly/v17/fastly"
	OpsgenieAlerts "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/alerts/opsgenie"
)

const alertType = "opsgenie"

func NewResource() resource.Resource {
	return ngwafalertintegration.NewWorkspaceResource(definition())
}

func DefinitionForDataSource() ngwafalertintegration.Definition {
	return definition()
}

func definition() ngwafalertintegration.Definition {
	return ngwafalertintegration.Definition{
		Type:        alertType,
		TypeSuffix:  "opsgenie",
		DisplayName: "Opsgenie",
		Description: "Manages a Fastly Next-Gen WAF Opsgenie alert integration scoped to a single workspace.",
		ConfigAttrs: []ngwafalertintegration.ConfigAttribute{
			{Name: "key", Description: "Opsgenie API key.", Sensitive: true},
		},
		Operations: operations{},
	}
}

type operations struct{}

func (operations) Create(ctx context.Context, client *fastly.Client, plan ngwafalertintegration.Model) (*ngwafalertintegration.RemoteAlert, error) {
	input := &OpsgenieAlerts.CreateInput{
		Config: &OpsgenieAlerts.CreateConfig{
			Key: ngwafalertintegration.StringPointer(ngwafalertintegration.ValueForAttribute(plan, "key")),
		},
		Description: ngwafalertintegration.StringPointer(ngwafalertintegration.ValueForAttribute(plan, "description")),
		Events:      ngwafalertintegration.FlagEvents(),
		WorkspaceID: ngwafalertintegration.StringPointer(plan.WorkspaceID.ValueString()),
	}
	alert, err := OpsgenieAlerts.Create(ctx, client, input)
	if err != nil {
		return nil, err
	}
	return remoteAlert(alert)
}

func (operations) Get(ctx context.Context, client *fastly.Client, workspaceID, alertID string) (*ngwafalertintegration.RemoteAlert, error) {
	alert, err := OpsgenieAlerts.Get(ctx, client, &OpsgenieAlerts.GetInput{
		AlertID:     &alertID,
		WorkspaceID: &workspaceID,
	})
	if err != nil {
		return nil, err
	}
	return remoteAlert(alert)
}

func (operations) Update(ctx context.Context, client *fastly.Client, alertID string, plan ngwafalertintegration.Model) (*ngwafalertintegration.RemoteAlert, error) {
	input := &OpsgenieAlerts.UpdateInput{
		AlertID: &alertID,
		Config: &OpsgenieAlerts.UpdateConfig{
			Key: ngwafalertintegration.StringPointer(ngwafalertintegration.ValueForAttribute(plan, "key")),
		},
		WorkspaceID: ngwafalertintegration.StringPointer(plan.WorkspaceID.ValueString()),
	}
	alert, err := OpsgenieAlerts.Update(ctx, client, input)
	if err != nil {
		return nil, err
	}
	return remoteAlert(alert)
}

func (operations) Delete(ctx context.Context, client *fastly.Client, workspaceID, alertID string) error {
	return OpsgenieAlerts.Delete(ctx, client, &OpsgenieAlerts.DeleteInput{
		AlertID:     &alertID,
		WorkspaceID: &workspaceID,
	})
}

func (operations) List(ctx context.Context, client *fastly.Client, workspaceID string) ([]ngwafalertintegration.RemoteAlert, error) {
	alerts, err := OpsgenieAlerts.List(ctx, client, &OpsgenieAlerts.ListInput{WorkspaceID: &workspaceID})
	if err != nil {
		return nil, err
	}

	var data []OpsgenieAlerts.Alert
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

func remoteAlert(alert *OpsgenieAlerts.Alert) (*ngwafalertintegration.RemoteAlert, error) {
	if alert == nil {
		return nil, nil
	}
	return &ngwafalertintegration.RemoteAlert{
		ID:          alert.ID,
		Type:        alertType,
		Description: alert.Description,
		Config: map[string]string{
			"key": ngwafalertintegration.StringFromAny(alert.Config.Key),
		},
	}, nil
}
