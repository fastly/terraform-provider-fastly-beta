package ngwafworkspacealertdatadogintegration

import (
	"context"

	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafalertintegration"

	"github.com/hashicorp/terraform-plugin-framework/resource"

	fastly "github.com/fastly/go-fastly/v17/fastly"
	DatadogAlerts "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/alerts/datadog"
)

const alertType = "datadog"

func NewResource() resource.Resource {
	return ngwafalertintegration.NewWorkspaceResource(definition())
}

func DefinitionForDataSource() ngwafalertintegration.Definition {
	return definition()
}

func definition() ngwafalertintegration.Definition {
	return ngwafalertintegration.Definition{
		Type:        alertType,
		TypeSuffix:  "datadog",
		DisplayName: "Datadog",
		Description: "Manages a Fastly Next-Gen WAF Datadog alert integration scoped to a single workspace.",
		ConfigAttrs: []ngwafalertintegration.ConfigAttribute{
			{Name: "key", Description: "Datadog integration key.", Sensitive: true},
			{Name: "site", Description: "Datadog site. One of `us1`, `us3`, `us5`, or `eu1`.", Sensitive: false},
		},
		Operations: operations{},
	}
}

type operations struct{}

func (operations) Create(ctx context.Context, client *fastly.Client, plan ngwafalertintegration.Model) (*ngwafalertintegration.RemoteAlert, error) {
	input := &DatadogAlerts.CreateInput{
		Config: &DatadogAlerts.CreateConfig{
			Key:  ngwafalertintegration.StringPointer(ngwafalertintegration.ValueForAttribute(plan, "key")),
			Site: ngwafalertintegration.StringPointer(ngwafalertintegration.ValueForAttribute(plan, "site")),
		},
		Description: ngwafalertintegration.StringPointer(ngwafalertintegration.ValueForAttribute(plan, "description")),
		Events:      ngwafalertintegration.FlagEvents(),
		WorkspaceID: ngwafalertintegration.StringPointer(plan.WorkspaceID.ValueString()),
	}
	alert, err := DatadogAlerts.Create(ctx, client, input)
	if err != nil {
		return nil, err
	}
	return remoteAlert(alert)
}

func (operations) Get(ctx context.Context, client *fastly.Client, workspaceID, alertID string) (*ngwafalertintegration.RemoteAlert, error) {
	alert, err := DatadogAlerts.Get(ctx, client, &DatadogAlerts.GetInput{
		AlertID:     &alertID,
		WorkspaceID: &workspaceID,
	})
	if err != nil {
		return nil, err
	}
	return remoteAlert(alert)
}

func (operations) Update(ctx context.Context, client *fastly.Client, alertID string, plan ngwafalertintegration.Model) (*ngwafalertintegration.RemoteAlert, error) {
	input := &DatadogAlerts.UpdateInput{
		AlertID: &alertID,
		Config: &DatadogAlerts.UpdateConfig{
			Key:  ngwafalertintegration.StringPointer(ngwafalertintegration.ValueForAttribute(plan, "key")),
			Site: ngwafalertintegration.StringPointer(ngwafalertintegration.ValueForAttribute(plan, "site")),
		},
		WorkspaceID: ngwafalertintegration.StringPointer(plan.WorkspaceID.ValueString()),
	}
	alert, err := DatadogAlerts.Update(ctx, client, input)
	if err != nil {
		return nil, err
	}
	return remoteAlert(alert)
}

func (operations) Delete(ctx context.Context, client *fastly.Client, workspaceID, alertID string) error {
	return DatadogAlerts.Delete(ctx, client, &DatadogAlerts.DeleteInput{
		AlertID:     &alertID,
		WorkspaceID: &workspaceID,
	})
}

func (operations) List(ctx context.Context, client *fastly.Client, workspaceID string) ([]ngwafalertintegration.RemoteAlert, error) {
	alerts, err := DatadogAlerts.List(ctx, client, &DatadogAlerts.ListInput{WorkspaceID: &workspaceID})
	if err != nil {
		return nil, err
	}

	var data []DatadogAlerts.Alert
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

func remoteAlert(alert *DatadogAlerts.Alert) (*ngwafalertintegration.RemoteAlert, error) {
	if alert == nil {
		return nil, nil
	}
	return &ngwafalertintegration.RemoteAlert{
		ID:          alert.ID,
		Type:        alertType,
		Description: alert.Description,
		Config: map[string]string{
			"key":  ngwafalertintegration.StringFromAny(alert.Config.Key),
			"site": ngwafalertintegration.StringFromAny(alert.Config.Site),
		},
	}, nil
}
