package migrate

import (
	"strings"

	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/compatibility"

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
		if sensitiveBody(parsed.Body) {
			return true
		}
	}
	return false
}

func sensitiveBody(body hcl.Body) bool {
	// Fail closed if the parser ever supplies an unexpected or nil body.
	// A source preview must not be emitted when sensitivity cannot be checked.
	syntaxBody, ok := body.(*hclsyntax.Body)
	if !ok || syntaxBody == nil {
		return true
	}
	for name := range syntaxBody.Attributes {
		if sensitiveFieldName(name) {
			return true
		}
	}
	for _, block := range syntaxBody.Blocks {
		if compatibility.IsTLSLoggingBlock(block.Type) || compatibility.IsStorageLoggingBlock(block.Type) || compatibility.IsRemainingCDNLoggingBlock(block.Type) || sensitiveFieldName(block.Type) || sensitiveBody(block.Body) {
			return true
		}
	}
	return false
}

func sensitiveFieldName(name string) bool {
	switch name {
	case "ssl_client_cert", "ssl_client_key", "ssl_client_secrets",
		"ssl_ca_cert", "private_key", "authentication", "tls",
		"api_key", "api_token", "header_value", "password", "token", "secret", "sensitive", "share_key":
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
