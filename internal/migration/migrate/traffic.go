package migrate

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/compatibility"
	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/rewrite"
)

// compileVCLBlockEdits orders repeated legacy set blocks by name and applies
// the value rewrites. It must run before any block rename, since it
// addresses blocks by their legacy names. Each edit is a no-op where the
// block or attribute is absent; the analyzer has already rejected values
// these edits cannot represent.
func compileVCLBlockEdits(key resourceKey) []rewrite.Edit {
	if key.typ != "fastly_service_vcl" {
		return nil
	}
	var edits []rewrite.Edit
	for _, change := range compatibility.VCLLiteralRewrites {
		if len(change.Values) != 0 {
			edits = append(edits, rewrite.ReplaceLiteral{
				File: key.file, ResourceType: key.typ, ResourceName: key.name,
				Parents: []string{change.Block}, Attribute: change.Attribute,
				Values: change.Values, CaseInsensitive: change.CaseInsensitive,
			})
		}
		if change.RemoveEmpty {
			edits = append(edits, rewrite.RemoveLiteralString{
				File: key.file, ResourceType: key.typ, ResourceName: key.name,
				Parents: []string{change.Block}, Attribute: change.Attribute,
			})
		}
	}
	for _, block := range compatibility.VCLNameOrderedBlocks {
		edits = append(edits, rewrite.SortBlocksByName{
			File: key.file, ResourceType: key.typ, ResourceName: key.name, Block: block,
		})
	}
	return edits
}
