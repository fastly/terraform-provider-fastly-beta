package dynamicvclsnippet

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	fastly "github.com/fastly/go-fastly/v17/fastly"
)

func TestID(t *testing.T) {
	got := ID("service123", 3, "block_scrapers")
	want := "service123-3-block_scrapers"

	if got != want {
		t.Fatalf("ID() = %q, want %q", got, want)
	}
}

func TestFlattenPreservesConfiguredContent(t *testing.T) {
	name := "block_scrapers"
	snippetType := fastly.SnippetType("recv")
	priority := "50"
	dynamic := 1

	m := &Model{}
	m.Content = types.StringValue("sub my_helper {}")

	err := flatten(context.Background(), &fastly.Snippet{
		Name:     &name,
		Type:     &snippetType,
		Priority: &priority,
		Dynamic:  &dynamic,
	}, m)
	if err != nil {
		t.Fatalf("flatten returned error: %s", err)
	}

	if got := m.Content.ValueString(); got != "sub my_helper {}" {
		t.Fatalf("Content = %q, want the previously configured content to survive flatten", got)
	}
}
