package ngwafworkspacealertjiraintegration

import (
	"context"

	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafalertintegration"

	"github.com/hashicorp/terraform-plugin-framework/resource"

	fastly "github.com/fastly/go-fastly/v17/fastly"
	JiraAlerts "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/alerts/jira"
)

const alertType = "jira"

func NewResource() resource.Resource {
	return ngwafalertintegration.NewWorkspaceResource(definition())
}

func DefinitionForDataSource() ngwafalertintegration.Definition {
	return definition()
}

func definition() ngwafalertintegration.Definition {
	return ngwafalertintegration.Definition{
		Type:        alertType,
		TypeSuffix:  "jira",
		DisplayName: "Jira",
		Description: "Manages a Fastly Next-Gen WAF Jira alert integration scoped to a single workspace.",
		ConfigAttrs: []ngwafalertintegration.ConfigAttribute{
			{Name: "host", Description: "Jira instance host.", Sensitive: false},
			{Name: "username", Description: "Jira username.", Sensitive: false},
			{Name: "project", Description: "Jira project key.", Sensitive: false},
			{Name: "key", Description: "Jira API key.", Sensitive: true},
			{Name: "issue_type", Description: "Jira issue type. Defaults to `Task` when omitted by the API.", Sensitive: false},
		},
		Operations: operations{},
	}
}

type operations struct{}

func (operations) Create(ctx context.Context, client *fastly.Client, plan ngwafalertintegration.Model) (*ngwafalertintegration.RemoteAlert, error) {
	input := &JiraAlerts.CreateInput{
		Config: &JiraAlerts.CreateConfig{
			Host:      ngwafalertintegration.StringPointer(ngwafalertintegration.ValueForAttribute(plan, "host")),
			Username:  ngwafalertintegration.StringPointer(ngwafalertintegration.ValueForAttribute(plan, "username")),
			Project:   ngwafalertintegration.StringPointer(ngwafalertintegration.ValueForAttribute(plan, "project")),
			Key:       ngwafalertintegration.StringPointer(ngwafalertintegration.ValueForAttribute(plan, "key")),
			IssueType: ngwafalertintegration.StringPointer(ngwafalertintegration.ValueForAttribute(plan, "issue_type")),
		},
		Description: ngwafalertintegration.StringPointer(ngwafalertintegration.ValueForAttribute(plan, "description")),
		Events:      ngwafalertintegration.FlagEvents(),
		WorkspaceID: ngwafalertintegration.StringPointer(plan.WorkspaceID.ValueString()),
	}
	alert, err := JiraAlerts.Create(ctx, client, input)
	if err != nil {
		return nil, err
	}
	return remoteAlert(alert)
}

func (operations) Get(ctx context.Context, client *fastly.Client, workspaceID, alertID string) (*ngwafalertintegration.RemoteAlert, error) {
	alert, err := JiraAlerts.Get(ctx, client, &JiraAlerts.GetInput{
		AlertID:     &alertID,
		WorkspaceID: &workspaceID,
	})
	if err != nil {
		return nil, err
	}
	return remoteAlert(alert)
}

func (operations) Update(ctx context.Context, client *fastly.Client, alertID string, plan ngwafalertintegration.Model) (*ngwafalertintegration.RemoteAlert, error) {
	input := &JiraAlerts.UpdateInput{
		AlertID: &alertID,
		Config: &JiraAlerts.UpdateConfig{
			Host:      ngwafalertintegration.StringPointer(ngwafalertintegration.ValueForAttribute(plan, "host")),
			Username:  ngwafalertintegration.StringPointer(ngwafalertintegration.ValueForAttribute(plan, "username")),
			Project:   ngwafalertintegration.StringPointer(ngwafalertintegration.ValueForAttribute(plan, "project")),
			Key:       ngwafalertintegration.StringPointer(ngwafalertintegration.ValueForAttribute(plan, "key")),
			IssueType: ngwafalertintegration.StringPointer(ngwafalertintegration.ValueForAttribute(plan, "issue_type")),
		},
		WorkspaceID: ngwafalertintegration.StringPointer(plan.WorkspaceID.ValueString()),
	}
	alert, err := JiraAlerts.Update(ctx, client, input)
	if err != nil {
		return nil, err
	}
	return remoteAlert(alert)
}

func (operations) Delete(ctx context.Context, client *fastly.Client, workspaceID, alertID string) error {
	return JiraAlerts.Delete(ctx, client, &JiraAlerts.DeleteInput{
		AlertID:     &alertID,
		WorkspaceID: &workspaceID,
	})
}

func (operations) List(ctx context.Context, client *fastly.Client, workspaceID string) ([]ngwafalertintegration.RemoteAlert, error) {
	alerts, err := JiraAlerts.List(ctx, client, &JiraAlerts.ListInput{WorkspaceID: &workspaceID})
	if err != nil {
		return nil, err
	}

	var data []JiraAlerts.Alert
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

func remoteAlert(alert *JiraAlerts.Alert) (*ngwafalertintegration.RemoteAlert, error) {
	if alert == nil {
		return nil, nil
	}
	return &ngwafalertintegration.RemoteAlert{
		ID:          alert.ID,
		Type:        alertType,
		Description: alert.Description,
		Config: map[string]string{
			"host":       ngwafalertintegration.StringFromAny(alert.Config.Host),
			"username":   ngwafalertintegration.StringFromAny(alert.Config.Username),
			"project":    ngwafalertintegration.StringFromAny(alert.Config.Project),
			"key":        ngwafalertintegration.StringFromAny(alert.Config.Key),
			"issue_type": ngwafalertintegration.StringFromAny(alert.Config.IssueType),
		},
	}, nil
}
