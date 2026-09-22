package ratelimiter

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"

	fastly "github.com/fastly/go-fastly/v17/fastly"
)

func TestModelEmbedding(t *testing.T) {
	nested := fullNestedModel()
	m := Model{
		NestedModel: nested,
		ID:          types.StringValue("test-id"),
		Service:     types.StringValue("test-service"),
		Version:     types.Int64Value(1),
	}

	assert.Equal(t, nested.Name, m.Name)
	assert.Equal(t, nested.Action, m.Action)
	assert.Equal(t, nested.RpsLimit, m.RpsLimit)

	assert.Equal(t, types.StringValue("test-id"), m.ID)
	assert.Equal(t, types.StringValue("test-service"), m.Service)
	assert.Equal(t, types.Int64Value(1), m.Version)

	extracted := m.NestedModel
	assert.Equal(t, nested, extracted)
}

func TestFlatten(t *testing.T) {
	tests := []struct {
		name string
		erl  *fastly.ERL
		want func(t *testing.T, m *Model)
	}{
		{
			name: "nil rate limiter logs warning and leaves model untouched",
			erl:  nil,
			want: func(t *testing.T, m *Model) {
				assert.Equal(t, types.String{}, m.ID)
				assert.Equal(t, types.String{}, m.Service)
				assert.Equal(t, types.Int64{}, m.Version)
			},
		},
		{
			name: "rate limiter with service metadata",
			erl: func() *fastly.ERL {
				action := fastly.ERLActionLogOnly
				windowSize := fastly.ERLSize60
				return &fastly.ERL{
					ServiceID:          new("service-123"),
					Version:            new(5),
					Name:               new("rate-limiter"),
					Action:             &action,
					ClientKey:          []*string{new("req.http.Fastly-Client-IP")},
					FeatureRevision:    new(1),
					HTTPMethods:        []*string{new("GET")},
					PenaltyBoxDuration: new(10),
					RateLimiterID:      new("abc123"),
					RpsLimit:           new(100),
					WindowSize:         &windowSize,
				}
			}(),
			want: func(t *testing.T, m *Model) {
				assert.Equal(t, types.StringValue("service-123-5-rate-limiter"), m.ID)
				assert.Equal(t, types.StringValue("service-123"), m.Service)
				assert.Equal(t, types.Int64Value(5), m.Version)
				assert.Equal(t, types.StringValue("rate-limiter"), m.Name)
				assert.Equal(t, types.StringValue("abc123"), m.RateLimiterID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			m := &Model{}
			flatten(ctx, tt.erl, m)
			tt.want(t, m)
		})
	}
}
