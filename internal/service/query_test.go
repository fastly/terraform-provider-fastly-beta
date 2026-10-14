package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly"
)

func TestQueryListResourceConfigSchema(t *testing.T) {
	s := QueryListResourceConfigSchema("test description")

	if s.Description != "test description" {
		t.Fatalf("Description = %q, want %q", s.Description, "test description")
	}

	attr, ok := s.Attributes["service_id"]
	if !ok {
		t.Fatal("service_id list configuration attribute is missing")
	}

	serviceIDAttr, ok := attr.(listschema.StringAttribute)
	if !ok {
		t.Fatalf("service_id attribute type = %T, want listschema.StringAttribute", attr)
	}
	if !serviceIDAttr.Optional {
		t.Fatal("service_id list configuration attribute must be optional")
	}
}

func TestListServicesForServiceID(t *testing.T) {
	tests := []struct {
		name           string
		serviceID      types.String
		wantPaths      []string
		wantServiceIDs []string
		wantError      bool
	}{
		{
			name:           "omitted filter lists all services",
			serviceID:      types.StringNull(),
			wantPaths:      []string{"/service"},
			wantServiceIDs: []string{"service-a", "service-b"},
		},
		{
			name:           "configured filter reads only selected service",
			serviceID:      types.StringValue("service-b"),
			wantPaths:      []string{"/service/service-b"},
			wantServiceIDs: []string{"service-b"},
		},
		{
			name:      "empty filter is rejected",
			serviceID: types.StringValue(""),
			wantError: true,
		},
		{
			name:      "unknown filter is rejected",
			serviceID: types.StringUnknown(),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := &queryServiceTransport{}
			client, err := fastly.NewClient("test-token")
			if err != nil {
				t.Fatalf("fastly.NewClient() error = %v", err)
			}
			client.HTTPClient = &http.Client{Transport: transport}

			services, diags := listServicesForServiceID(context.Background(), client, tt.serviceID)
			if tt.wantError {
				if !diags.HasError() {
					t.Fatalf("expected diagnostics error, got none")
				}
				if len(transport.paths) != 0 {
					t.Fatalf("unexpected API requests for invalid filter: %v", transport.paths)
				}
				return
			}

			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if len(transport.paths) != len(tt.wantPaths) {
				t.Fatalf("request paths = %v, want %v", transport.paths, tt.wantPaths)
			}
			for i := range tt.wantPaths {
				if transport.paths[i] != tt.wantPaths[i] {
					t.Fatalf("request path %d = %q, want %q", i, transport.paths[i], tt.wantPaths[i])
				}
			}

			if len(services) != len(tt.wantServiceIDs) {
				t.Fatalf("services count = %d, want %d", len(services), len(tt.wantServiceIDs))
			}
			for i, wantID := range tt.wantServiceIDs {
				if got := fastly.ToValue(services[i].ServiceID); got != wantID {
					t.Fatalf("service %d ID = %q, want %q", i, got, wantID)
				}
			}
		})
	}
}

type queryServiceTransport struct {
	paths []string
}

func (t *queryServiceTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.paths = append(t.paths, req.URL.Path)

	body := `{}`
	switch req.URL.Path {
	case "/service":
		body = `[
			{"id":"service-a","name":"Service A","type":"vcl","active_version":1},
			{"id":"service-b","name":"Service B","type":"wasm","active_version":2}
		]`
	case "/service/service-b":
		body = `{"id":"service-b","name":"Service B","type":"wasm","active_version":2}`
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}
