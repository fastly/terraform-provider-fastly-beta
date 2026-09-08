package dynamicsnippet

import (
	"testing"

	regularsnippet "github.com/fastly/terraform-provider-fastly-beta/internal/resources/snippet"

	"github.com/hashicorp/terraform-plugin-framework/types"

	fastly "github.com/fastly/go-fastly/v17/fastly"
)

func TestBuildCreateInput(t *testing.T) {
	model := NestedModel{
		Name:     types.StringValue("dynamic_recv"),
		Type:     types.StringValue("recv"),
		Priority: types.Int64Value(50),
	}

	input := BuildCreateInput("service-id", 1, model)

	if input.ServiceID != "service-id" {
		t.Fatalf("ServiceID = %q, want service-id", input.ServiceID)
	}
	if input.ServiceVersion != 1 {
		t.Fatalf("ServiceVersion = %d, want 1", input.ServiceVersion)
	}
	if fastly.ToValue(input.Name) != "dynamic_recv" {
		t.Fatalf("Name = %q, want dynamic_recv", fastly.ToValue(input.Name))
	}
	if fastly.ToValue(input.Dynamic) != 1 {
		t.Fatalf("Dynamic = %d, want 1", fastly.ToValue(input.Dynamic))
	}
	if fastly.ToValue(input.Priority) != "50" {
		t.Fatalf("Priority = %q, want 50", fastly.ToValue(input.Priority))
	}
}

func TestBuildCreateInputDefaultsToEmptyContent(t *testing.T) {
	model := NestedModel{
		Name:     types.StringValue("dynamic_recv"),
		Type:     types.StringValue("recv"),
		Priority: types.Int64Value(50),
	}

	input := BuildCreateInput("service-id", 1, model)

	if fastly.ToValue(input.Content) != "" {
		t.Fatalf("Content = %q, want empty string when unconfigured", fastly.ToValue(input.Content))
	}
}

func TestBuildCreateInputSeedsConfiguredContent(t *testing.T) {
	model := NestedModel{
		Name:     types.StringValue("dynamic_recv"),
		Type:     types.StringValue("recv"),
		Priority: types.Int64Value(50),
		Content:  types.StringValue("sub my_helper {}"),
	}

	input := BuildCreateInput("service-id", 1, model)

	if fastly.ToValue(input.Content) != "sub my_helper {}" {
		t.Fatalf("Content = %q, want configured seed content", fastly.ToValue(input.Content))
	}
}

func TestBuildUpdateInputNeverIncludesContent(t *testing.T) {
	// The Fastly API rejects a content field on the per-version snippet Update endpoint for a
	// dynamic snippet ("Dynamic content can only be updated by the Dynamic Snippet API
	// endpoint"), regardless of whether the value is configured. PushConfiguredContent is the
	// only path allowed to write it, via UpdateDynamicSnippet.
	unconfigured := NestedModel{
		Name:     types.StringValue("dynamic_recv"),
		Type:     types.StringValue("recv"),
		Priority: types.Int64Value(50),
	}
	if got := BuildUpdateInput("service-id", 1, unconfigured).Content; got != nil {
		t.Fatalf("Content = %q, want nil when unconfigured", fastly.ToValue(got))
	}

	configured := NestedModel{
		Name:     types.StringValue("dynamic_recv"),
		Type:     types.StringValue("recv"),
		Priority: types.Int64Value(50),
		Content:  types.StringValue("sub my_helper {}"),
	}
	if got := BuildUpdateInput("service-id", 1, configured).Content; got != nil {
		t.Fatalf("Content = %q, want nil even when configured - see PushConfiguredContent", fastly.ToValue(got))
	}
}

func TestModelsEqualIgnoresUnconfiguredContent(t *testing.T) {
	desired := NestedModel{
		Name:     types.StringValue("dynamic_recv"),
		Type:     types.StringValue("recv"),
		Priority: types.Int64Value(50),
	}
	// FlattenToNestedModel never populates Content, so the "remote" side is always null/empty.
	remote := NestedModel{
		Name:     types.StringValue("dynamic_recv"),
		Type:     types.StringValue("recv"),
		Priority: types.Int64Value(50),
	}

	if !desired.ModelsEqual(remote) {
		t.Fatal("expected models to be equal when content is left unconfigured on both sides")
	}
}

func TestModelsEqualDetectsConfiguredContentChange(t *testing.T) {
	desired := NestedModel{
		Name:     types.StringValue("dynamic_recv"),
		Type:     types.StringValue("recv"),
		Priority: types.Int64Value(50),
		Content:  types.StringValue("sub my_helper {}"),
	}
	remote := NestedModel{
		Name:     types.StringValue("dynamic_recv"),
		Type:     types.StringValue("recv"),
		Priority: types.Int64Value(50),
	}

	if desired.ModelsEqual(remote) {
		t.Fatal("expected models to differ when content is configured but remote has none")
	}
}

func TestMatchOrderPreservePlanFieldsPreservesContent(t *testing.T) {
	items := []NestedModel{
		{Name: types.StringValue("dynamic_recv"), Type: types.StringValue("recv"), Priority: types.Int64Value(50), SnippetID: types.StringValue("snippet-1")},
	}
	plan := []NestedModel{
		{Name: types.StringValue("dynamic_recv"), Type: types.StringValue("recv"), Priority: types.Int64Value(50), Content: types.StringValue("sub my_helper {}")},
	}

	result := MatchOrderPreservePlanFields(items, plan)

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}
	if got := result[0].Content.ValueString(); got != "sub my_helper {}" {
		t.Fatalf("Content = %q, want the plan's configured content", got)
	}
	if got := result[0].SnippetID.ValueString(); got != "snippet-1" {
		t.Fatalf("SnippetID = %q, want the API's computed snippet_id", got)
	}
}

func TestMatchOrderPreserveContentCarriesForwardPreviousValue(t *testing.T) {
	items := []NestedModel{
		{Name: types.StringValue("dynamic_recv"), Type: types.StringValue("deliver"), Priority: types.Int64Value(25), SnippetID: types.StringValue("snippet-1")},
	}
	previous := []NestedModel{
		{Name: types.StringValue("dynamic_recv"), Type: types.StringValue("recv"), Priority: types.Int64Value(50), Content: types.StringValue("sub my_helper {}")},
	}

	result := MatchOrderPreserveContent(items, previous)

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}
	if got := result[0].Content.ValueString(); got != "sub my_helper {}" {
		t.Fatalf("Content = %q, want the previously known content carried forward", got)
	}
	// Fields the API actually tracks should still refresh to the latest remote values.
	if got := result[0].Type.ValueString(); got != "deliver" {
		t.Fatalf("Type = %q, want the freshly read remote value", got)
	}
}

func TestFlattenToNestedModelRejectsRegularSnippet(t *testing.T) {
	dynamic := 0
	name := "regular_recv"

	_, err := FlattenToNestedModel(&fastly.Snippet{
		Name:    &name,
		Dynamic: &dynamic,
	})
	if err == nil {
		t.Fatal("expected regular snippet to be rejected")
	}
}

func TestParsePriority(t *testing.T) {
	value := "25"
	got, err := parsePriority(&value)
	if err != nil {
		t.Fatalf("parsePriority returned error: %s", err)
	}
	if got != 25 {
		t.Fatalf("parsePriority = %d, want 25", got)
	}

	empty := ""
	got, err = parsePriority(&empty)
	if err != nil {
		t.Fatalf("parsePriority empty returned error: %s", err)
	}
	if got != DefaultPriority {
		t.Fatalf("parsePriority empty = %d, want %d", got, DefaultPriority)
	}

	invalid := "invalid"
	if _, err := parsePriority(&invalid); err == nil {
		t.Fatal("expected invalid priority to return an error")
	}
}

func TestValidateNoNameConflicts(t *testing.T) {
	dynamic := []NestedModel{
		{Name: types.StringValue("shared")},
	}
	regular := []regularsnippet.NestedModel{
		{Name: types.StringValue("shared")},
	}

	if err := ValidateNoNameConflicts(dynamic, regular); err == nil {
		t.Fatal("expected shared regular and dynamic snippet name to return an error")
	}
}

func TestValidateNoNameConflictsWhitespace(t *testing.T) {
	dynamic := []NestedModel{
		{Name: types.StringValue("shared")},
	}
	regular := []regularsnippet.NestedModel{
		{Name: types.StringValue(" shared ")},
	}

	if err := ValidateNoNameConflicts(dynamic, regular); err == nil {
		t.Fatal("expected regular and dynamic snippet names differing only in whitespace to return an error")
	}
}

func TestValidateConfigDuplicateWhitespace(t *testing.T) {
	items := []NestedModel{
		{Name: types.StringValue("one"), Type: types.StringValue("recv"), Priority: types.Int64Value(100)},
		{Name: types.StringValue(" one "), Type: types.StringValue("recv"), Priority: types.Int64Value(100)},
	}

	if err := ValidateConfig(items); err == nil {
		t.Fatal("expected duplicate names differing only in whitespace to return an error")
	}
}

func TestValidateDuplicateWhitespace(t *testing.T) {
	items := []NestedModel{
		{Name: types.StringValue("one"), Type: types.StringValue("recv"), Priority: types.Int64Value(100)},
		{Name: types.StringValue(" one "), Type: types.StringValue("recv"), Priority: types.Int64Value(100)},
	}

	if err := Validate(items); err == nil {
		t.Fatal("expected duplicate names differing only in whitespace to return an error")
	}
}
