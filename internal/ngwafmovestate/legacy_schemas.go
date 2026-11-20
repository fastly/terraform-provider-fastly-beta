// These decode-only schemas snapshot the Terraform value types of the legacy
// fastly/fastly NGWAF resources at schema version 0, unchanged from 9.1.0
// through 10.3.0. TestLegacySchemasMatchFixtures checks them against
// testdata/legacy_*_schema_v0_10_3_0_type.json, exported from 10.3.0.
package ngwafmovestate

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func legacyAccountListSchema() schema.Schema {
	return schema.Schema{Attributes: map[string]schema.Attribute{
		"applies_to":  schema.ListAttribute{Computed: true, ElementType: types.StringType},
		"description": schema.StringAttribute{Computed: true},
		"entries":     schema.ListAttribute{Computed: true, ElementType: types.StringType},
		"id":          schema.StringAttribute{Computed: true},
		"name":        schema.StringAttribute{Computed: true},
		"type":        schema.StringAttribute{Computed: true},
	}}
}

func legacyWorkspaceListSchema() schema.Schema {
	return schema.Schema{Attributes: map[string]schema.Attribute{
		"description":  schema.StringAttribute{Computed: true},
		"entries":      schema.ListAttribute{Computed: true, ElementType: types.StringType},
		"id":           schema.StringAttribute{Computed: true},
		"name":         schema.StringAttribute{Computed: true},
		"type":         schema.StringAttribute{Computed: true},
		"workspace_id": schema.StringAttribute{Computed: true},
	}}
}

func legacyAccountRuleSchema() schema.Schema {
	return schema.Schema{Attributes: map[string]schema.Attribute{
		"applies_to":         schema.ListAttribute{Computed: true, ElementType: types.StringType},
		"description":        schema.StringAttribute{Computed: true},
		"enabled":            schema.BoolAttribute{Computed: true},
		"group_operator":     schema.StringAttribute{Computed: true},
		"id":                 schema.StringAttribute{Computed: true},
		"request_logging":    schema.StringAttribute{Computed: true},
		"type":               schema.StringAttribute{Computed: true},
		"action":             schema.ListAttribute{Computed: true, ElementType: types.ObjectType{AttrTypes: map[string]attr.Type{"signal": types.StringType, "type": types.StringType}}},
		"condition":          schema.ListAttribute{Computed: true, ElementType: types.ObjectType{AttrTypes: map[string]attr.Type{"field": types.StringType, "operator": types.StringType, "value": types.StringType}}},
		"group_condition":    schema.ListAttribute{Computed: true, ElementType: types.ObjectType{AttrTypes: map[string]attr.Type{"group_operator": types.StringType, "condition": types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{"field": types.StringType, "operator": types.StringType, "value": types.StringType}}}, "multival_condition": types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{"field": types.StringType, "group_operator": types.StringType, "operator": types.StringType, "condition": types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{"field": types.StringType, "operator": types.StringType, "value": types.StringType}}}}}}}}},
		"multival_condition": schema.ListAttribute{Computed: true, ElementType: types.ObjectType{AttrTypes: map[string]attr.Type{"field": types.StringType, "group_operator": types.StringType, "operator": types.StringType, "condition": types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{"field": types.StringType, "operator": types.StringType, "value": types.StringType}}}}}},
	}}
}

func legacyWorkspaceRuleSchema() schema.Schema {
	return schema.Schema{Attributes: map[string]schema.Attribute{
		"description":        schema.StringAttribute{Computed: true},
		"enabled":            schema.BoolAttribute{Computed: true},
		"group_operator":     schema.StringAttribute{Computed: true},
		"id":                 schema.StringAttribute{Computed: true},
		"request_logging":    schema.StringAttribute{Computed: true},
		"type":               schema.StringAttribute{Computed: true},
		"workspace_id":       schema.StringAttribute{Computed: true},
		"action":             schema.ListAttribute{Computed: true, ElementType: types.ObjectType{AttrTypes: map[string]attr.Type{"allow_interactive": types.BoolType, "deception_type": types.StringType, "redirect_url": types.StringType, "response_code": types.NumberType, "signal": types.StringType, "type": types.StringType}}},
		"condition":          schema.ListAttribute{Computed: true, ElementType: types.ObjectType{AttrTypes: map[string]attr.Type{"field": types.StringType, "operator": types.StringType, "value": types.StringType}}},
		"group_condition":    schema.ListAttribute{Computed: true, ElementType: types.ObjectType{AttrTypes: map[string]attr.Type{"group_operator": types.StringType, "condition": types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{"field": types.StringType, "operator": types.StringType, "value": types.StringType}}}, "multival_condition": types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{"field": types.StringType, "group_operator": types.StringType, "operator": types.StringType, "condition": types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{"field": types.StringType, "operator": types.StringType, "value": types.StringType}}}}}}}}},
		"multival_condition": schema.ListAttribute{Computed: true, ElementType: types.ObjectType{AttrTypes: map[string]attr.Type{"field": types.StringType, "group_operator": types.StringType, "operator": types.StringType, "condition": types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{"field": types.StringType, "operator": types.StringType, "value": types.StringType}}}}}},
		"rate_limit":         schema.ListAttribute{Computed: true, ElementType: types.ObjectType{AttrTypes: map[string]attr.Type{"duration": types.NumberType, "interval": types.NumberType, "signal": types.StringType, "threshold": types.NumberType, "client_identifiers": types.SetType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{"key": types.StringType, "name": types.StringType, "type": types.StringType}}}}}},
	}}
}

func legacyAccountSignalSchema() schema.Schema {
	return schema.Schema{Attributes: map[string]schema.Attribute{
		"applies_to":   schema.ListAttribute{Computed: true, ElementType: types.StringType},
		"description":  schema.StringAttribute{Computed: true},
		"id":           schema.StringAttribute{Computed: true},
		"name":         schema.StringAttribute{Computed: true},
		"reference_id": schema.StringAttribute{Computed: true},
	}}
}

func legacyRedactionSchema() schema.Schema {
	return schema.Schema{Attributes: map[string]schema.Attribute{
		"field":        schema.StringAttribute{Computed: true},
		"id":           schema.StringAttribute{Computed: true},
		"type":         schema.StringAttribute{Computed: true},
		"workspace_id": schema.StringAttribute{Computed: true},
	}}
}

func legacyThresholdsSchema() schema.Schema {
	return schema.Schema{Attributes: map[string]schema.Attribute{
		"action":       schema.StringAttribute{Computed: true},
		"dont_notify":  schema.BoolAttribute{Computed: true},
		"duration":     schema.NumberAttribute{Computed: true},
		"enabled":      schema.BoolAttribute{Computed: true},
		"id":           schema.StringAttribute{Computed: true},
		"interval":     schema.NumberAttribute{Computed: true},
		"limit":        schema.NumberAttribute{Computed: true},
		"name":         schema.StringAttribute{Computed: true},
		"signal":       schema.StringAttribute{Computed: true},
		"workspace_id": schema.StringAttribute{Computed: true},
	}}
}

func legacyVirtualPatchesSchema() schema.Schema {
	return schema.Schema{Attributes: map[string]schema.Attribute{
		"action":           schema.StringAttribute{Computed: true},
		"enabled":          schema.BoolAttribute{Computed: true},
		"id":               schema.StringAttribute{Computed: true},
		"virtual_patch_id": schema.StringAttribute{Computed: true},
		"workspace_id":     schema.StringAttribute{Computed: true},
	}}
}

func legacyAlertDatadogIntegrationSchema() schema.Schema {
	return schema.Schema{Attributes: map[string]schema.Attribute{
		"description":  schema.StringAttribute{Computed: true},
		"id":           schema.StringAttribute{Computed: true},
		"key":          schema.StringAttribute{Computed: true},
		"site":         schema.StringAttribute{Computed: true},
		"workspace_id": schema.StringAttribute{Computed: true},
	}}
}

func legacyAlertJiraIntegrationSchema() schema.Schema {
	return schema.Schema{Attributes: map[string]schema.Attribute{
		"description":  schema.StringAttribute{Computed: true},
		"host":         schema.StringAttribute{Computed: true},
		"id":           schema.StringAttribute{Computed: true},
		"issue_type":   schema.StringAttribute{Computed: true},
		"key":          schema.StringAttribute{Computed: true},
		"project":      schema.StringAttribute{Computed: true},
		"username":     schema.StringAttribute{Computed: true},
		"workspace_id": schema.StringAttribute{Computed: true},
	}}
}

func legacyAlertMailingListIntegrationSchema() schema.Schema {
	return schema.Schema{Attributes: map[string]schema.Attribute{
		"address":      schema.StringAttribute{Computed: true},
		"description":  schema.StringAttribute{Computed: true},
		"id":           schema.StringAttribute{Computed: true},
		"workspace_id": schema.StringAttribute{Computed: true},
	}}
}

func legacyAlertMicrosoftTeamsIntegrationSchema() schema.Schema {
	return schema.Schema{Attributes: map[string]schema.Attribute{
		"description":  schema.StringAttribute{Computed: true},
		"id":           schema.StringAttribute{Computed: true},
		"webhook":      schema.StringAttribute{Computed: true},
		"workspace_id": schema.StringAttribute{Computed: true},
	}}
}

func legacyAlertOpsgenieIntegrationSchema() schema.Schema {
	return schema.Schema{Attributes: map[string]schema.Attribute{
		"description":  schema.StringAttribute{Computed: true},
		"id":           schema.StringAttribute{Computed: true},
		"key":          schema.StringAttribute{Computed: true},
		"workspace_id": schema.StringAttribute{Computed: true},
	}}
}

func legacyAlertPagerdutyIntegrationSchema() schema.Schema {
	return schema.Schema{Attributes: map[string]schema.Attribute{
		"description":  schema.StringAttribute{Computed: true},
		"id":           schema.StringAttribute{Computed: true},
		"key":          schema.StringAttribute{Computed: true},
		"workspace_id": schema.StringAttribute{Computed: true},
	}}
}

func legacyAlertSlackIntegrationSchema() schema.Schema {
	return schema.Schema{Attributes: map[string]schema.Attribute{
		"description":  schema.StringAttribute{Computed: true},
		"id":           schema.StringAttribute{Computed: true},
		"webhook":      schema.StringAttribute{Computed: true},
		"workspace_id": schema.StringAttribute{Computed: true},
	}}
}

func legacyAlertWebhookIntegrationSchema() schema.Schema {
	return schema.Schema{Attributes: map[string]schema.Attribute{
		"description":  schema.StringAttribute{Computed: true},
		"id":           schema.StringAttribute{Computed: true},
		"webhook":      schema.StringAttribute{Computed: true},
		"workspace_id": schema.StringAttribute{Computed: true},
	}}
}
