package migrate

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func TestSensitiveBodyFailsClosed(t *testing.T) {
	t.Parallel()
	var nilSyntaxBody *hclsyntax.Body
	for _, tc := range []struct {
		name string
		body hcl.Body
		want bool
	}{
		{"nil body", nil, true},
		{"typed nil syntax body", nilSyntaxBody, true},
		{"unexpected body type", hcl.EmptyBody(), true},
		{"empty native body", &hclsyntax.Body{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := sensitiveBody(tc.body); got != tc.want {
				t.Errorf("sensitiveBody() = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestContainsSensitiveConfiguration(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		files map[string][]byte
		want  bool
	}{
		{"empty inventory", nil, false},
		{"ordinary HCL", map[string][]byte{"main.tf": []byte("name = \"example\"\n")}, false},
		{"invalid HCL", map[string][]byte{"main.tf": []byte("name =\n")}, true},
		{"nested credential in another file", map[string][]byte{
			"main.tf":              []byte("name = \"example\"\n"),
			"modules/edge/main.tf": []byte("backend {\n  ssl_client_key = local.key\n}\n"),
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := containsSensitiveConfiguration(tc.files); got != tc.want {
				t.Errorf("containsSensitiveConfiguration() = %t, want %t", got, tc.want)
			}
		})
	}
}
