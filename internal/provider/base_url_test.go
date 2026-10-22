package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestResolveBaseURL(t *testing.T) {
	tests := []struct {
		name       string
		configured types.String
		env        string
		want       string
		wantErr    bool
	}{
		{"default", types.StringNull(), "", "https://api.fastly.com", false},
		{"env overrides default", types.StringNull(), "https://env.example.com", "https://env.example.com", false},
		{"config overrides env", types.StringValue("https://cfg.example.com"), "https://env.example.com", "https://cfg.example.com", false},
		{"empty config falls back to env", types.StringValue(""), "https://env.example.com", "https://env.example.com", false},
		{"http rejected", types.StringValue("http://cfg.example.com"), "", "", true},
		{"http env rejected", types.StringNull(), "http://env.example.com", "", true},
		{"missing host rejected", types.StringValue("https://"), "", "", true},
		{"no scheme rejected", types.StringValue("api.example.com"), "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveBaseURL(tt.configured, tt.env)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
