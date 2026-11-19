package migrate

import (
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// containsSensitiveConfiguration intentionally classifies by schema field
// names, never by evaluating expressions. If any recognized sensitive field
// occurs anywhere in the tree, omit every source diff: a literal may be
// repeated in a local, variable default, comment or neighboring context line.
// This is not a general-purpose secret detector for arbitrary user HCL.
func containsSensitiveConfiguration(files map[string][]byte) bool {
	for filename, source := range files {
		parsed, diagnostics := hclsyntax.ParseConfig(source, filename, hcl.Pos{Line: 1, Column: 1})
		if diagnostics.HasErrors() || parsed == nil {
			return true
		}
		if sensitiveBody(parsed.Body.(*hclsyntax.Body)) {
			return true
		}
	}
	return false
}

func sensitiveBody(body *hclsyntax.Body) bool {
	for name := range body.Attributes {
		if sensitiveFieldName(name) {
			return true
		}
	}
	for _, block := range body.Blocks {
		if sensitiveFieldName(block.Type) || sensitiveBody(block.Body) {
			return true
		}
	}
	return false
}

func sensitiveFieldName(name string) bool {
	switch name {
	case "ssl_client_cert", "ssl_client_key", "ssl_client_secrets",
		"ssl_ca_cert", "private_key", "authentication", "tls",
		"api_key", "api_token", "password", "token", "secret", "sensitive", "share_key":
		return true
	}
	// Cover conventional prefixed credential names used by logging and
	// integrations, without evaluating or inspecting their values.
	for _, suffix := range []string{"_key", "_secret", "_token", "_password", "_cert", "_credentials"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}
