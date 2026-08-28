package ngwafworkspacethreshold

import (
	"github.com/fastly/terraform-provider-fastly/internal/service"

	th "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/thresholds"
)

func BuildCreateInput(plan Model) *th.CreateInput {
	return &th.CreateInput{
		WorkspaceID: new(service.StringValue(plan.WorkspaceID)),
		Action:      new(service.StringValue(plan.Action)),
		DontNotify:  new(service.BoolValue(plan.DontNotify)),
		Duration:    new(int(service.Int64Value(plan.Duration))),
		Enabled:     new(service.BoolValue(plan.Enabled)),
		Interval:    new(int(service.Int64Value(plan.Interval))),
		Limit:       new(int(service.Int64Value(plan.Limit))),
		Name:        new(service.StringValue(plan.Name)),
		Signal:      new(service.StringValue(plan.Signal)),
	}
}

func BuildUpdateInput(workspaceID, thresholdID string, plan Model) *th.UpdateInput {
	return &th.UpdateInput{
		WorkspaceID: &workspaceID,
		ThresholdID: &thresholdID,
		Action:      new(service.StringValue(plan.Action)),
		DontNotify:  new(service.BoolValue(plan.DontNotify)),
		Duration:    new(int(service.Int64Value(plan.Duration))),
		Enabled:     new(service.BoolValue(plan.Enabled)),
		Interval:    new(int(service.Int64Value(plan.Interval))),
		Limit:       new(int(service.Int64Value(plan.Limit))),
		Name:        new(service.StringValue(plan.Name)),
		Signal:      new(service.StringValue(plan.Signal)),
	}
}
