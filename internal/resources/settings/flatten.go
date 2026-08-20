package settings

import (
	"context"
	"fmt"
	"time"

	"github.com/fastly/terraform-provider-fastly/internal/errors"
	"github.com/fastly/terraform-provider-fastly/internal/service"

	fastly "github.com/fastly/go-fastly/v17/fastly"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	http3ConsistencyPollInterval = 500 * time.Millisecond
	http3ConsistencyTimeout      = 30 * time.Second
)

func FlattenToNestedModel(s *fastly.Settings, http3Enabled bool) NestedModel {
	if s == nil {
		return NestedModel{}
	}

	var defaultTTL, staleIfErrorTTL *int
	if s.DefaultTTL != nil {
		defaultTTL = new(int(*s.DefaultTTL))
	}
	if s.StaleIfErrorTTL != nil {
		staleIfErrorTTL = new(int(*s.StaleIfErrorTTL))
	}

	return NestedModel{
		DefaultHost:     service.StringPointerOrDefault(s.DefaultHost, DefaultDefaultHost),
		DefaultTTL:      service.Int64PointerOrDefault(defaultTTL, DefaultDefaultTTL),
		HTTP3:           types.BoolValue(http3Enabled),
		StaleIfError:    service.BoolPointerOrDefault(s.StaleIfError, DefaultStaleIfError),
		StaleIfErrorTTL: service.Int64PointerOrDefault(staleIfErrorTTL, DefaultStaleIfErrorTTL),
	}
}

// ReadForVersion reads the general settings for a service version.
//
// Unlike most singleton settings (e.g. Image Optimizer default settings, gated behind product
// enablement), GetSettings always succeeds for every service, configured or not - there is no
// remote signal distinguishing "never touched" from "explicitly set to platform defaults". So
// the remote API is only queried when current is non-empty, and - deliberately, unlike other
// singleton blocks - this is never bypassed on import: doing so would surface a settings block
// on every import of every service, even ones that never declared it.
func ReadForVersion(ctx context.Context, client *fastly.Client, serviceID string, version int, current []NestedModel) ([]NestedModel, error) {
	if len(current) == 0 {
		return nil, nil
	}

	m, err := readCurrent(ctx, client, serviceID, version)
	if err != nil {
		return nil, err
	}
	return []NestedModel{m}, nil
}

// Reconcile returns the resulting settings so the caller skips a second ReadForVersion
// round-trip - reconciling already fetches or produces them. Settings always exist server-side,
// so create and update are the same full-replace operation. Removing the block resets to API
// defaults, but only if previous shows it was configured before; either way, empty desired means
// no settings belong in state, so both paths return (nil, nil).
func Reconcile(ctx context.Context, client *fastly.Client, serviceID string, version int, previous, desired []NestedModel) ([]NestedModel, error) {
	if len(desired) == 0 {
		if len(previous) == 0 {
			return nil, nil
		}
		if err := resetToDefaults(ctx, client, serviceID, version); err != nil {
			return nil, err
		}
		return nil, nil
	}

	current, err := readCurrent(ctx, client, serviceID, version)
	if err != nil {
		return nil, err
	}
	if desired[0].ModelsEqual(current) {
		return []NestedModel{current}, nil
	}

	result, err := apply(ctx, client, serviceID, version, desired[0], service.BoolValue(current.HTTP3))
	if err != nil {
		return nil, err
	}
	return []NestedModel{result}, nil
}

func resetToDefaults(ctx context.Context, client *fastly.Client, serviceID string, version int) error {
	current, err := readCurrent(ctx, client, serviceID, version)
	if err != nil {
		return err
	}

	def := defaultNestedModel()
	if def.ModelsEqual(current) {
		return nil
	}

	_, err = apply(ctx, client, serviceID, version, def, service.BoolValue(current.HTTP3))
	return err
}

func readCurrent(ctx context.Context, client *fastly.Client, serviceID string, version int) (NestedModel, error) {
	remote, err := client.GetSettings(ctx, &fastly.GetSettingsInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
	})
	if err != nil {
		return NestedModel{}, err
	}

	http3Enabled, err := getHTTP3Enabled(ctx, client, serviceID, version)
	if err != nil {
		return NestedModel{}, err
	}

	return FlattenToNestedModel(remote, http3Enabled), nil
}

// apply writes desired and returns the resulting settings, built from UpdateSettings' response
// plus desired's own HTTP3 value - reconcileHTTP3 only returns successfully once the API
// confirms HTTP/3 has reached that value, so there's no need to re-fetch it.
func apply(ctx context.Context, client *fastly.Client, serviceID string, version int, desired NestedModel, currentHTTP3 bool) (NestedModel, error) {
	updated, err := client.UpdateSettings(ctx, BuildUpdateInput(serviceID, version, desired))
	if err != nil {
		return NestedModel{}, err
	}
	if err := reconcileHTTP3(ctx, client, serviceID, version, service.BoolValue(desired.HTTP3), currentHTTP3); err != nil {
		return NestedModel{}, err
	}
	return FlattenToNestedModel(updated, service.BoolValue(desired.HTTP3)), nil
}

// reconcileHTTP3 only calls Enable/Disable when the desired state differs from current: the
// API 400s if EnableHTTP3 is called while already enabled, and there's no reason to call
// DisableHTTP3 when it's already disabled.
//
// After a successful Enable/Disable call, it polls GetHTTP3 until the change is visible before
// returning. The Fastly API's HTTP/3 status endpoint is backed by a separate system from the
// Enable/Disable write path and can briefly lag behind it - without this, the caller's own very
// next read (ReadForVersion, called immediately after Reconcile in every _auto resource) can
// observe the pre-change value and report "Provider produced inconsistent result after apply".
func reconcileHTTP3(ctx context.Context, client *fastly.Client, serviceID string, version int, desired, current bool) error {
	if desired == current {
		return nil
	}

	if desired {
		if _, err := client.EnableHTTP3(ctx, &fastly.EnableHTTP3Input{
			FeatureRevision: new(http3FeatureRevision),
			ServiceID:       serviceID,
			ServiceVersion:  version,
		}); err != nil {
			return err
		}
	} else {
		if err := client.DisableHTTP3(ctx, &fastly.DisableHTTP3Input{
			ServiceID:      serviceID,
			ServiceVersion: version,
		}); err != nil {
			return err
		}
	}

	return waitForHTTP3Consistency(ctx, client, serviceID, version, desired)
}

// waitForHTTP3Consistency polls on a time budget, not a fixed attempt count: the prior
// 5-attempts/200ms (~1s) budget fired sporadically, since the status endpoint's lag isn't
// bounded to a small fixed window.
func waitForHTTP3Consistency(ctx context.Context, client *fastly.Client, serviceID string, version int, desired bool) error {
	ctx, cancel := context.WithTimeout(ctx, http3ConsistencyTimeout)
	defer cancel()

	var enabled bool
	for {
		var err error
		enabled, err = getHTTP3Enabled(ctx, client, serviceID, version)
		if err != nil {
			return err
		}
		if enabled == desired {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("HTTP/3 status did not reach desired state (%t) after %s; last observed state was %t", desired, http3ConsistencyTimeout, enabled)
		case <-time.After(http3ConsistencyPollInterval):
		}
	}
}

// getHTTP3Enabled reports whether HTTP/3 is enabled for a service version. The API returns a
// 404 when it isn't, which the client surfaces as an error - any other error is propagated
// rather than being treated as "disabled".
func getHTTP3Enabled(ctx context.Context, client *fastly.Client, serviceID string, version int) (bool, error) {
	_, err := client.GetHTTP3(ctx, &fastly.GetHTTP3Input{
		ServiceID:      serviceID,
		ServiceVersion: version,
	})
	if err == nil {
		return true, nil
	}
	if errors.IsNotFound(err) {
		return false, nil
	}
	return false, err
}
