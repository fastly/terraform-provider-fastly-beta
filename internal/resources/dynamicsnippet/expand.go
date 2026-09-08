package dynamicsnippet

import (
	"context"
	"strconv"

	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	fastly "github.com/fastly/go-fastly/v17/fastly"
)

func BuildCreateInput(serviceID string, version int, m NestedModel) *fastly.CreateSnippetInput {
	name := service.StringValue(m.Name)
	content := service.StringValue(m.Content)
	priority := strconv.FormatInt(service.Int64Value(m.Priority), 10)
	snippetType := fastly.SnippetType(normalizeType(service.StringValue(m.Type)))
	dynamic := 1

	return &fastly.CreateSnippetInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
		Name:           &name,
		Content:        &content,
		Priority:       &priority,
		Type:           &snippetType,
		Dynamic:        &dynamic,
	}
}

// BuildUpdateInput never sets Content: the Fastly API rejects a content field here for a dynamic
// snippet ("Dynamic content can only be updated by the Dynamic Snippet API endpoint"), even when
// the value would be unchanged. An existing dynamic snippet's content can only be updated through
// the versionless Dynamic Snippet API - see PushConfiguredContent, which BuildCreateInput's Create
// path doesn't need, since the API does accept an initial Content there.
func BuildUpdateInput(serviceID string, version int, m NestedModel) *fastly.UpdateSnippetInput {
	name := service.StringValue(m.Name)
	priority := strconv.FormatInt(service.Int64Value(m.Priority), 10)
	snippetType := fastly.SnippetType(normalizeType(service.StringValue(m.Type)))

	return &fastly.UpdateSnippetInput{
		ServiceID:      serviceID,
		ServiceVersion: version,
		Name:           name,
		NewName:        &name,
		Priority:       &priority,
		Type:           &snippetType,
	}
}

// PushConfiguredContent pushes desired.Content to the dynamic snippet identified by snippetID,
// via the versionless Dynamic Snippet API endpoint, when the model configures a value. It is a
// no-op when Content is unset, leaving whatever fastly_service_dynamic_snippet_content has set
// untouched. Callers use this after a BuildUpdateInput-based UpdateSnippet call, which cannot
// carry content itself - see BuildUpdateInput.
func PushConfiguredContent(ctx context.Context, client *fastly.Client, serviceID, snippetID string, desired NestedModel) error {
	if desired.Content.IsNull() || desired.Content.IsUnknown() {
		return nil
	}

	content := desired.Content.ValueString()
	_, err := client.UpdateDynamicSnippet(ctx, &fastly.UpdateDynamicSnippetInput{
		ServiceID: serviceID,
		SnippetID: snippetID,
		Content:   &content,
	})
	return err
}
