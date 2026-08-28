package ngwafthresholds

import (
	"github.com/hashicorp/terraform-plugin-framework/types"

	th "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/thresholds"
)

// FlattenToModel populates state from the API response. duration, interval,
// and limit are Optional+Computed with a static schema default, so a
// threshold created through this resource always has a real value - the
// framework fills in the default at plan time before any write reaches the
// API. A threshold created out-of-band (e.g. a `block_immediately`
// threshold, for which the API accepts these fields as unset) can come back
// from a bare GET with a literal 0, which fails this schema's own
// validators on the next plan; zero values are overridden with the schema
// defaults here to avoid that, matching how ngwafworkspace resolves its
// attack_signal_thresholds zero values on import.
func FlattenToModel(workspaceID string, threshold *th.Threshold) Model {
	duration := threshold.Duration
	if duration == 0 {
		duration = DefaultDuration
	}

	interval := threshold.Interval
	if interval == 0 {
		interval = DefaultInterval
	}

	limit := threshold.Limit
	if limit == 0 {
		limit = DefaultLimit
	}

	return Model{
		ID:          types.StringValue(threshold.ThresholdID),
		WorkspaceID: types.StringValue(workspaceID),
		Action:      types.StringValue(threshold.Action),
		DontNotify:  types.BoolValue(threshold.DontNotify),
		Duration:    types.Int64Value(int64(duration)),
		Enabled:     types.BoolValue(threshold.Enabled),
		Interval:    types.Int64Value(int64(interval)),
		Limit:       types.Int64Value(int64(limit)),
		Name:        types.StringValue(threshold.Name),
		Signal:      types.StringValue(threshold.Signal),
	}
}
