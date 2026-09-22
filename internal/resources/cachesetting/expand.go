package cachesetting

import (
	"strings"

	fastly "github.com/fastly/go-fastly/v17/fastly"

	"github.com/fastly/terraform-provider-fastly-beta/internal/service"
)

// BuildCreateInput omits Action entirely when unset, rather than sending an empty string, since
// the Fastly API validates Action as one of cache/pass/restart and may reject a blank value on
// creation. BuildUpdateInput (below) always sends Action, since that's the only way to clear a
// previously configured value back to unset.
func BuildCreateInput(serviceID string, version int, m NestedModel) *fastly.CreateCacheSettingInput {
	name := service.StringValue(m.Name)
	cacheCondition := service.StringValue(m.CacheCondition)
	ttl := int(service.Int64Value(m.TTL))
	staleTTL := int(service.Int64Value(m.StaleTTL))

	return &fastly.CreateCacheSettingInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
		Name:           &name,
		Action:         actionPointer(m.Action),
		CacheCondition: &cacheCondition,
		TTL:            &ttl,
		StaleTTL:       &staleTTL,
	}
}

func BuildUpdateInput(serviceID string, version int, m NestedModel) *fastly.UpdateCacheSettingInput {
	action := fastly.CacheSettingAction(strings.ToLower(service.StringValue(m.Action)))
	cacheCondition := service.StringValue(m.CacheCondition)
	ttl := int(service.Int64Value(m.TTL))
	staleTTL := int(service.Int64Value(m.StaleTTL))

	return &fastly.UpdateCacheSettingInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
		Name:           service.StringValue(m.Name),
		Action:         &action,
		CacheCondition: &cacheCondition,
		TTL:            &ttl,
		StaleTTL:       &staleTTL,
	}
}
