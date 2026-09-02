package ngwafrule

import (
	"context"
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
)

func TestExpandConditions(t *testing.T) {
	models := []ConditionModel{
		{Field: types.StringValue("ip"), Operator: types.StringValue("equals"), Value: types.StringValue("1.2.3.4")},
	}

	result := ExpandConditions(models, func(field, operator, value *string) *rules.CreateCondition {
		return &rules.CreateCondition{Field: field, Operator: operator, Value: value}
	})

	if assert.Len(t, result, 1) {
		assert.Equal(t, "ip", *result[0].Field)
		assert.Equal(t, "equals", *result[0].Operator)
		assert.Equal(t, "1.2.3.4", *result[0].Value)
	}
}

func TestExpandConditions_nilOnEmpty(t *testing.T) {
	result := ExpandConditions[rules.CreateCondition](nil, func(field, operator, value *string) *rules.CreateCondition {
		return &rules.CreateCondition{Field: field, Operator: operator, Value: value}
	})
	assert.Nil(t, result)
}

func TestExpandMultivalConditions(t *testing.T) {
	models := []MultivalConditionModel{
		{
			Field:         types.StringValue("post_parameter"),
			Operator:      types.StringValue("exists"),
			GroupOperator: types.StringValue("any"),
			Condition: []ConditionModel{
				{Field: types.StringValue("name"), Operator: types.StringValue("equals"), Value: types.StringValue("foo")},
			},
		},
	}

	newCondition := func(field, operator, value *string) *rules.CreateConditionMult {
		return &rules.CreateConditionMult{Field: field, Operator: operator, Value: value}
	}
	newMultival := func(field, operator, groupOperator *string, conditions []*rules.CreateConditionMult) *rules.CreateMultivalCondition {
		return &rules.CreateMultivalCondition{Field: field, Operator: operator, GroupOperator: groupOperator, Conditions: conditions}
	}

	result := ExpandMultivalConditions(models, newCondition, newMultival)

	if assert.Len(t, result, 1) {
		assert.Equal(t, "post_parameter", *result[0].Field)
		assert.Equal(t, "any", *result[0].GroupOperator)
		if assert.Len(t, result[0].Conditions, 1) {
			assert.Equal(t, "name", *result[0].Conditions[0].Field)
		}
	}
}

func TestExpandGroupConditions(t *testing.T) {
	models := []GroupConditionModel{
		{
			GroupOperator: types.StringValue("all"),
			Condition: []ConditionModel{
				{Field: types.StringValue("ip"), Operator: types.StringValue("equals"), Value: types.StringValue("1.2.3.4")},
			},
			MultivalCondition: []MultivalConditionModel{
				{
					Field:         types.StringValue("request_header"),
					Operator:      types.StringValue("does_not_exist"),
					GroupOperator: types.StringValue("all"),
					Condition: []ConditionModel{
						{Field: types.StringValue("name"), Operator: types.StringValue("equals"), Value: types.StringValue("X-Foo")},
					},
				},
			},
		},
	}

	newCondition := func(field, operator, value *string) *rules.CreateCondition {
		return &rules.CreateCondition{Field: field, Operator: operator, Value: value}
	}
	newMultivalCondition := func(field, operator, value *string) *rules.CreateConditionMult {
		return &rules.CreateConditionMult{Field: field, Operator: operator, Value: value}
	}
	newMultival := func(field, operator, groupOperator *string, conditions []*rules.CreateConditionMult) *rules.CreateMultivalCondition {
		return &rules.CreateMultivalCondition{Field: field, Operator: operator, GroupOperator: groupOperator, Conditions: conditions}
	}
	newGroup := func(groupOperator *string, conditions []*rules.CreateCondition, multivals []*rules.CreateMultivalCondition) *rules.CreateGroupCondition {
		return &rules.CreateGroupCondition{GroupOperator: groupOperator, Conditions: conditions, MultivalConditions: multivals}
	}

	result := ExpandGroupConditions(models, newCondition, newMultivalCondition, newMultival, newGroup)

	if assert.Len(t, result, 1) {
		assert.Equal(t, "all", *result[0].GroupOperator)
		assert.Len(t, result[0].Conditions, 1)
		if assert.Len(t, result[0].MultivalConditions, 1) {
			assert.Equal(t, "request_header", *result[0].MultivalConditions[0].Field)
		}
	}
}

func TestFlattenConditions(t *testing.T) {
	items := []rules.ConditionItem{
		{
			Type: "single",
			Fields: rules.SingleCondition{
				Field: "ip", Operator: "equals", Value: "1.2.3.4",
			},
		},
		{
			Type: "group",
			Fields: rules.GroupCondition{
				GroupOperator: "any",
				Conditions: []rules.GroupConditionItem{
					{Type: "single", Fields: rules.Condition{Field: "path", Operator: "contains", Value: "/admin"}},
					{
						Type: "multival",
						Fields: rules.MultivalCondition{
							Field: "post_parameter", Operator: "exists", GroupOperator: "all",
							Conditions: []rules.ConditionMul{{Field: "name", Operator: "equals", Value: "foo"}},
						},
					},
				},
			},
		},
		{
			Type: "multival",
			Fields: rules.MultivalCondition{
				Field: "request_header", Operator: "does_not_exist", GroupOperator: "any",
				Conditions: []rules.ConditionMul{{Field: "name", Operator: "equals", Value: "X-Foo"}},
			},
		},
	}

	conditions, groups, multivals := FlattenConditions(items)

	if assert.Len(t, conditions, 1) {
		assert.Equal(t, types.StringValue("ip"), conditions[0].Field)
	}

	if assert.Len(t, groups, 1) {
		assert.Equal(t, types.StringValue("any"), groups[0].GroupOperator)
		if assert.Len(t, groups[0].Condition, 1) {
			assert.Equal(t, types.StringValue("path"), groups[0].Condition[0].Field)
		}
		if assert.Len(t, groups[0].MultivalCondition, 1) {
			assert.Equal(t, types.StringValue("post_parameter"), groups[0].MultivalCondition[0].Field)
		}
	}

	if assert.Len(t, multivals, 1) {
		assert.Equal(t, types.StringValue("request_header"), multivals[0].Field)
		if assert.Len(t, multivals[0].Condition, 1) {
			assert.Equal(t, types.StringValue("X-Foo"), multivals[0].Condition[0].Value)
		}
	}
}

func TestHasAnyCondition(t *testing.T) {
	assert.False(t, HasAnyCondition(nil, nil, nil))
	assert.True(t, HasAnyCondition([]ConditionModel{{}}, nil, nil))
	assert.True(t, HasAnyCondition(nil, []GroupConditionModel{{}}, nil))
	assert.True(t, HasAnyCondition(nil, nil, []MultivalConditionModel{{}}))
}

func TestEmptyGroupConditionIndexes(t *testing.T) {
	groups := []GroupConditionModel{
		{Condition: []ConditionModel{{}}},
		{},
		{MultivalCondition: []MultivalConditionModel{{}}},
	}

	assert.Equal(t, []int{1}, EmptyGroupConditionIndexes(groups))
}

// TestRequiredBlocksRejectOmission covers blocks the API insists on: a size
// validator alone passes when the block is absent, so each one needs a
// validator that fires on a null list.
func TestRequiredBlocksRejectOmission(t *testing.T) {
	nested, ok := MultivalConditionBlock("test").NestedObject.Blocks["condition"].(schema.ListNestedBlock)
	require.True(t, ok)

	blocks := map[string]schema.ListNestedBlock{
		"action":                    ActionBlock([]string{"log_request"}, 1, 2),
		"signal action":             SignalActionBlock("test", "test"),
		"multival nested condition": nested,
	}

	for name, block := range blocks {
		assert.True(t, rejectsNullList(block), "%s: omitting the block raises no error", name)
	}
}

func rejectsNullList(block schema.ListNestedBlock) bool {
	req := validator.ListRequest{
		Path:        path.Root("test"),
		ConfigValue: types.ListNull(block.NestedObject.Type()),
	}

	for _, v := range block.Validators {
		var resp validator.ListResponse
		v.ValidateList(context.Background(), req, &resp)
		if resp.Diagnostics.HasError() {
			return true
		}
	}

	return false
}

func TestAppliesToWildcardExclusive(t *testing.T) {
	for name, tc := range map[string]struct {
		entries   []string
		wantError bool
	}{
		"wildcard alone":           {entries: []string{AppliesToWildcard}, wantError: false},
		"named workspaces":         {entries: []string{"ws1", "ws2"}, wantError: false},
		"wildcard with named":      {entries: []string{AppliesToWildcard, "ws1"}, wantError: true},
		"named with wildcard last": {entries: []string{"ws1", AppliesToWildcard}, wantError: true},
	} {
		t.Run(name, func(t *testing.T) {
			elements := make([]attr.Value, 0, len(tc.entries))
			for _, e := range tc.entries {
				elements = append(elements, types.StringValue(e))
			}

			var resp validator.SetResponse
			appliesToWildcardExclusive{}.ValidateSet(context.Background(), validator.SetRequest{
				Path:        path.Root("applies_to"),
				ConfigValue: types.SetValueMust(types.StringType, elements),
			}, &resp)

			assert.Equal(t, tc.wantError, resp.Diagnostics.HasError(), resp.Diagnostics)
		})
	}
}

// TestAppliesToWildcardExclusiveIgnoresUnknown keeps the validator quiet when
// the set is not yet resolved, so a workspace ID coming from another resource
// cannot produce a spurious plan-time error.
func TestAppliesToWildcardExclusiveIgnoresUnknown(t *testing.T) {
	var resp validator.SetResponse
	appliesToWildcardExclusive{}.ValidateSet(context.Background(), validator.SetRequest{
		Path:        path.Root("applies_to"),
		ConfigValue: types.SetUnknown(types.StringType),
	}, &resp)

	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
}

// TestConditionLimitDocumented guards the one constraint whose number is
// repeated in prose rather than derived from the constant. Every other
// constraint reaches the docs through the same value that feeds its validator -
// OneOfDescriptor for enums, actionFieldDescriptor for per-action fields,
// ActionBlock's own bounds for action counts - so it cannot drift. The
// condition cap cannot: ValidateConditions enforces it in ValidateConfig, which
// tfplugindocs cannot see, so it is spelled out by hand. If MaxConditions
// moves, these sentences have to move with it, and this test is what says so.
func TestConditionLimitDocumented(t *testing.T) {
	// Relative to this package, internal/resources/ngwafrule. Every rule
	// resource's template is listed: the cap applies at both scopes.
	paths := []string{
		"../../../templates/resources/ngwaf_request_rule.md.tmpl",
		"../../../templates/resources/ngwaf_signal_rule.md.tmpl",
		"../../../templates/resources/ngwaf_workspace_request_rule.md.tmpl",
		"../../../templates/resources/ngwaf_workspace_signal_rule.md.tmpl",
		"../../../templates/resources/ngwaf_workspace_rate_limit_rule.md.tmpl",
		"../../../templates/resources/ngwaf_workspace_templated_signal_rule.md.tmpl",
		"../../../examples/ngwaf-rules/README.md",
		"../ngwafrequestrule/resource.go",
		"../ngwafsignalrule/resource.go",
	}

	want := strconv.Itoa(MaxConditions)
	stated := regexp.MustCompile(`(?i)(?:no more than|between 1 and) (\d+)[^.]*combined`)

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			raw, err := os.ReadFile(path)
			require.NoError(t, err)

			found := stated.FindAllStringSubmatch(string(raw), -1)
			require.NotEmpty(t, found, "combined condition limit not documented here; it must state MaxConditions (%s)", want)

			for _, m := range found {
				assert.Equal(t, want, m[1], "documented condition limit is stale: MaxConditions is now %s", want)
			}
		})
	}
}
