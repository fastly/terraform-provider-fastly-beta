package settings

import (
	"github.com/fastly/terraform-provider-fastly/internal/service"

	fastly "github.com/fastly/go-fastly/v17/fastly"
)

// BuildUpdateInput builds the API input to fully replace the general settings for a service
// version. All fields are always populated so that boolean fields (which cannot be represented
// as "unset" in Go) are never accidentally dropped from an update.
func BuildUpdateInput(serviceID string, version int, m NestedModel) *fastly.UpdateSettingsInput {
	defaultHost := service.StringValue(m.DefaultHost)
	defaultTTL := uint(service.Int64Value(m.DefaultTTL))
	staleIfError := service.BoolValue(m.StaleIfError)
	staleIfErrorTTL := uint(service.Int64Value(m.StaleIfErrorTTL))

	return &fastly.UpdateSettingsInput{
		ServiceID:       serviceID,
		ServiceVersion:  version,
		DefaultHost:     &defaultHost,
		DefaultTTL:      &defaultTTL,
		StaleIfError:    &staleIfError,
		StaleIfErrorTTL: &staleIfErrorTTL,
	}
}
