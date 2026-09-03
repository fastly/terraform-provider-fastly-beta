package tlssubscriptionvalidation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/fastly/go-fastly/v17/fastly"
)

func TestFlattenToModel_issued(t *testing.T) {
	subscription := &fastly.TLSSubscription{
		State:        "issued",
		Certificates: []*fastly.TLSSubscriptionCertificate{{ID: "cert-1"}},
	}

	m := flattenToModel(subscription, "sub-1")
	assert.Equal(t, "sub-1", m.ID.ValueString())
	assert.Equal(t, "cert-1", m.CertificateID.ValueString())
	assert.Equal(t, "sub-1", m.SubscriptionID.ValueString())
}

// Renewing subscriptions can still have an issued certificate; that's what should determine
// validity, not the subscription's current state.
func TestFlattenToModel_renewingWithCertificate(t *testing.T) {
	subscription := &fastly.TLSSubscription{
		State:        "renewing",
		Certificates: []*fastly.TLSSubscriptionCertificate{{ID: "cert-1"}},
	}

	m := flattenToModel(subscription, "sub-1")
	assert.False(t, m.ID.IsNull())
	assert.Equal(t, "cert-1", m.CertificateID.ValueString())
}

func TestFlattenToModel_noCertificate(t *testing.T) {
	subscription := &fastly.TLSSubscription{State: "pending"}

	m := flattenToModel(subscription, "sub-1")
	assert.True(t, m.ID.IsNull())
	assert.True(t, m.CertificateID.IsNull())
	assert.Equal(t, "sub-1", m.SubscriptionID.ValueString())
}

func TestWaitForIssued_succeedsImmediately(t *testing.T) {
	calls := 0
	get := func(_ context.Context, id string) (*fastly.TLSSubscription, error) {
		calls++
		assert.Equal(t, "sub-1", id)
		return &fastly.TLSSubscription{State: "issued"}, nil
	}

	err := waitForIssued(context.Background(), get, "sub-1", time.Second, time.Millisecond)
	assert.NoError(t, err)
	assert.Equal(t, 1, calls)
}

func TestWaitForIssued_pollsUntilIssued(t *testing.T) {
	calls := 0
	get := func(context.Context, string) (*fastly.TLSSubscription, error) {
		calls++
		if calls < 3 {
			return &fastly.TLSSubscription{State: "pending"}, nil
		}
		return &fastly.TLSSubscription{State: "issued"}, nil
	}

	err := waitForIssued(context.Background(), get, "sub-1", time.Second, time.Millisecond)
	assert.NoError(t, err)
	assert.Equal(t, 3, calls)
}

func TestWaitForIssued_timesOut(t *testing.T) {
	get := func(context.Context, string) (*fastly.TLSSubscription, error) {
		return &fastly.TLSSubscription{State: "pending"}, nil
	}

	err := waitForIssued(context.Background(), get, "sub-1", 5*time.Millisecond, time.Millisecond)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pending")
}

func TestWaitForIssued_propagatesGetError(t *testing.T) {
	wantErr := errors.New("boom")
	get := func(context.Context, string) (*fastly.TLSSubscription, error) {
		return nil, wantErr
	}

	err := waitForIssued(context.Background(), get, "sub-1", time.Second, time.Millisecond)
	assert.ErrorIs(t, err, wantErr)
}

func TestWaitForIssued_respectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	get := func(context.Context, string) (*fastly.TLSSubscription, error) {
		cancel()
		return &fastly.TLSSubscription{State: "pending"}, nil
	}

	err := waitForIssued(ctx, get, "sub-1", time.Minute, time.Minute)
	assert.ErrorIs(t, err, context.Canceled)
}
