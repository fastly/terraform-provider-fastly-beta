package validation

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
)

type namedItem struct {
	name string
	refs []string
}

func (i namedItem) Name() types.String {
	if i.name == "" {
		return types.StringNull()
	}
	return types.StringValue(i.name)
}

func TestUniqueNames(t *testing.T) {
	t.Run("unique names", func(t *testing.T) {
		items := []namedItem{{name: "a"}, {name: "b"}}
		assert.NoError(t, UniqueNames(items, "widget", namedItem.Name))
	})

	t.Run("duplicate names", func(t *testing.T) {
		items := []namedItem{{name: "dup"}, {name: "dup"}}
		err := UniqueNames(items, "widget", namedItem.Name)
		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), `multiple widgets with the same name "dup"`)
		}
	})

	t.Run("skips unknown or null names", func(t *testing.T) {
		items := []namedItem{{name: ""}, {name: ""}}
		assert.NoError(t, UniqueNames(items, "widget", namedItem.Name))
	})
}

func TestNameSet(t *testing.T) {
	items := []namedItem{{name: "a"}, {name: "b"}, {name: ""}}
	set := NameSet(items, namedItem.Name)

	assert.Equal(t, map[string]struct{}{"a": {}, "b": {}}, set)
}

func TestReferences(t *testing.T) {
	validNames := map[string]struct{}{"origin1": {}}

	t.Run("references a valid name", func(t *testing.T) {
		items := []namedItem{{name: "item", refs: []string{"origin1"}}}
		assert.NoError(t, References(items, "widget", namedItem.Name, "backend", func(i namedItem) []string { return i.refs }, "backend", validNames))
	})

	t.Run("references a name that doesn't exist", func(t *testing.T) {
		items := []namedItem{{name: "item", refs: []string{"missing"}}}
		err := References(items, "widget", namedItem.Name, "backend", func(i namedItem) []string { return i.refs }, "backend", validNames)
		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), `widget "item": backend "missing" does not match any configured backend`)
		}
	})

	t.Run("skips empty references", func(t *testing.T) {
		items := []namedItem{{name: "item", refs: []string{""}}}
		assert.NoError(t, References(items, "widget", namedItem.Name, "backend", func(i namedItem) []string { return i.refs }, "backend", validNames))
	})
}

func TestSingleRef(t *testing.T) {
	type item struct {
		name types.String
		ref  types.String
	}
	getName := func(i item) types.String { return i.name }
	getRef := func(i item) types.String { return i.ref }
	validNames := map[string]struct{}{"cond1": {}}

	t.Run("references a valid name", func(t *testing.T) {
		items := []item{{name: types.StringValue("item"), ref: types.StringValue("cond1")}}
		assert.NoError(t, SingleRef(items, "widget", getName, getRef, "cache_condition", "condition", validNames))
	})

	t.Run("references a name that doesn't exist", func(t *testing.T) {
		items := []item{{name: types.StringValue("item"), ref: types.StringValue("missing")}}
		err := SingleRef(items, "widget", getName, getRef, "cache_condition", "condition", validNames)
		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), `widget "item": cache_condition "missing" does not match any configured condition`)
		}
	})

	t.Run("skips null, unknown, and empty references", func(t *testing.T) {
		items := []item{
			{name: types.StringValue("a"), ref: types.StringNull()},
			{name: types.StringValue("b"), ref: types.StringUnknown()},
			{name: types.StringValue("c"), ref: types.StringValue("")},
		}
		assert.NoError(t, SingleRef(items, "widget", getName, getRef, "cache_condition", "condition", validNames))
	})
}
