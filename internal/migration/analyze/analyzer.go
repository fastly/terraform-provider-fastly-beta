package analyze

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"

	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/compatibility"
)

var terraformMetaAttributes = map[string]struct{}{
	"count":      {},
	"depends_on": {},
	"for_each":   {},
	"provider":   {},
}

var terraformMetaBlocks = map[string]struct{}{
	"connection":  {},
	"lifecycle":   {},
	"provisioner": {},
}

// Analyzer performs read-only static analysis of Terraform configuration.
type Analyzer struct {
	registry *compatibility.Registry
}

// New constructs an Analyzer backed by the provided compatibility registry.
func New(registry *compatibility.Registry) *Analyzer {
	return &Analyzer{registry: registry}
}

type parsedFile struct {
	moduleDir string
	body      *hclsyntax.Body
}

type analysisState struct {
	root       string
	result     Result
	files      []parsedFile
	addressMap map[string]map[string]string
}

// Analyze inspects a Terraform working directory without modifying configuration or state.
func (a *Analyzer) Analyze(ctx context.Context, workingDir string) (Result, error) {
	if a == nil || a.registry == nil {
		return Result{}, errors.New("analyze: compatibility registry is required")
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	root, err := filepath.Abs(workingDir)
	if err != nil {
		return Result{}, fmt.Errorf("resolve working directory: %w", err)
	}
	root = filepath.Clean(root)

	info, err := os.Stat(root)
	if err != nil {
		return Result{}, fmt.Errorf("stat working directory %q: %w", workingDir, err)
	}
	if !info.IsDir() {
		return Result{}, fmt.Errorf("working directory %q is not a directory", workingDir)
	}

	state := &analysisState{
		root:       root,
		result:     Result{Root: root},
		addressMap: make(map[string]map[string]string),
	}

	if err := a.discover(ctx, state); err != nil {
		return Result{}, err
	}

	for _, file := range state.files {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		a.analyzeFile(state, file)
	}
	a.findReferences(state)

	state.result.sort()
	return state.result, nil
}

func (a *Analyzer) discover(ctx context.Context, state *analysisState) error {
	queue := []string{state.root}
	seen := make(map[string]struct{})

	for len(queue) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}

		moduleDir := filepath.Clean(queue[0])
		queue = queue[1:]
		if _, ok := seen[moduleDir]; ok {
			continue
		}
		seen[moduleDir] = struct{}{}

		info, err := os.Stat(moduleDir)
		if err != nil {
			return fmt.Errorf("stat local module %q: %w", relativePath(state.root, moduleDir), err)
		}
		if !info.IsDir() {
			return fmt.Errorf("local module %q is not a directory", relativePath(state.root, moduleDir))
		}

		state.result.Modules = append(state.result.Modules, relativeModulePath(state.root, moduleDir))

		entries, err := os.ReadDir(moduleDir)
		if err != nil {
			return fmt.Errorf("read local module %q: %w", relativePath(state.root, moduleDir), err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			path := filepath.Join(moduleDir, name)

			if strings.HasSuffix(name, ".tf.json") {
				state.result.Files = append(state.result.Files, relativePath(state.root, path))
				state.result.Findings = append(state.result.Findings, Finding{
					Severity: SeverityError,
					Range:    fileStartRange(path),
					Message:  "Terraform JSON configuration is not supported by the migration rewriter.",
					Remediation: "Convert this file to native Terraform .tf syntax before running " +
						"fastly-tf-migrate.",
				})
				continue
			}
			if !strings.HasSuffix(name, ".tf") {
				continue
			}

			source, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read Terraform file %q: %w", relativePath(state.root, path), err)
			}
			file, diags := hclsyntax.ParseConfig(source, path, hcl.Pos{Line: 1, Column: 1})
			state.result.Files = append(state.result.Files, relativePath(state.root, path))
			appendHCLDiagnostics(&state.result, path, diags)
			if file == nil {
				continue
			}

			body, ok := file.Body.(*hclsyntax.Body)
			if !ok {
				return fmt.Errorf("parse Terraform file %q: unexpected HCL body type %T",
					relativePath(state.root, path), file.Body)
			}
			state.files = append(state.files, parsedFile{
				moduleDir: moduleDir,
				body:      body,
			})
			for _, child := range localModuleSources(body) {
				if child.ambiguous {
					state.result.Findings = append(state.result.Findings, Finding{
						Severity: SeverityError,
						Range:    child.sourceRange,
						Message:  "Local module source cannot be determined statically.",
						Remediation: "Use a literal local module source before running fastly-tf-migrate " +
							"so the complete configuration can be analyzed.",
					})
					continue
				}
				if child.source == "" {
					continue
				}
				if !isLocalModuleSource(child.source) {
					state.result.Findings = append(state.result.Findings, Finding{
						Severity: SeverityWarning,
						Range:    child.sourceRange,
						Message: fmt.Sprintf(
							"Remote module source %q is outside the selected local module tree.",
							child.source,
						),
						Remediation: "If this module manages legacy Fastly resources, analyze and migrate " +
							"that module's source separately.",
					})
					continue
				}
				queue = append(queue, filepath.Clean(filepath.Join(moduleDir, child.source)))
			}
		}
	}

	return nil
}

type moduleSource struct {
	source      string
	sourceRange hcl.Range
	ambiguous   bool
}

func localModuleSources(body *hclsyntax.Body) []moduleSource {
	var result []moduleSource
	for _, block := range body.Blocks {
		if block.Type != "module" {
			continue
		}
		attr, ok := block.Body.Attributes["source"]
		if !ok {
			continue
		}
		value, diags := attr.Expr.Value(nil)
		if diags.HasErrors() || !value.IsKnown() || value.IsNull() || value.Type() != cty.String {
			result = append(result, moduleSource{
				sourceRange: attr.SrcRange,
				ambiguous:   true,
			})
			continue
		}
		result = append(result, moduleSource{
			source:      value.AsString(),
			sourceRange: attr.SrcRange,
		})
	}
	return result
}

func isLocalModuleSource(source string) bool {
	return source == "." ||
		source == ".." ||
		strings.HasPrefix(source, "./") ||
		strings.HasPrefix(source, "../")
}

func (a *Analyzer) analyzeFile(state *analysisState, parsed parsedFile) {
	body := parsed.body
	a.analyzeProviderRequirements(state, body)

	for _, block := range body.Blocks {
		if block.Type != "resource" || len(block.Labels) != 2 {
			continue
		}
		a.analyzeResource(state, parsed.moduleDir, block)
	}
}

func (a *Analyzer) analyzeProviderRequirements(state *analysisState, body *hclsyntax.Body) {
	for _, terraformBlock := range body.Blocks {
		if terraformBlock.Type != "terraform" {
			continue
		}
		for _, requiredBlock := range terraformBlock.Body.Blocks {
			if requiredBlock.Type != "required_providers" {
				continue
			}
			for localName, attr := range requiredBlock.Body.Attributes {
				requirement, relevant, ambiguous := providerRequirement(localName, attr)
				if ambiguous {
					if relevant || strings.Contains(localName, "fastly") {
						state.result.Findings = append(state.result.Findings, Finding{
							Severity: SeverityError,
							Range:    attr.SrcRange,
							Message: fmt.Sprintf(
								"Fastly provider requirement %q cannot be determined statically.",
								localName,
							),
							Remediation: "Use a literal required_providers entry so the migration target " +
								"can be verified.",
						})
					}
					continue
				}
				if relevant {
					state.result.ProviderRequirements = append(state.result.ProviderRequirements, requirement)
				}
			}
		}
	}
}

func providerRequirement(localName string, attr *hclsyntax.Attribute) (ProviderRequirement, bool, bool) {
	requirement := ProviderRequirement{
		LocalName: localName,
		Range:     attr.SrcRange,
	}

	if object, ok := attr.Expr.(*hclsyntax.ObjectConsExpr); ok {
		source, sourcePresent, sourceKnown := objectAttributeString(object, "source")
		if sourcePresent && !sourceKnown {
			if localName == "fastly" || localName == "fastly-beta" {
				return ProviderRequirement{}, true, true
			}
			return ProviderRequirement{}, false, false
		}
		if !sourcePresent {
			if localName != "fastly" && localName != "fastly-beta" {
				return ProviderRequirement{}, false, false
			}
			source = "<implicit>"
		}
		if !isFastlyProviderSource(source) && localName != "fastly" && localName != "fastly-beta" {
			return ProviderRequirement{}, false, false
		}

		requirement.Source = source
		version, versionPresent, versionKnown := objectAttributeString(object, "version")
		if versionPresent && !versionKnown {
			return requirement, true, true
		}

		requirement.Version = version
		return requirement, true, false
	}

	value, diags := attr.Expr.Value(nil)
	if diags.HasErrors() || !value.IsKnown() || value.IsNull() {
		relevant := strings.Contains(localName, "fastly")
		return ProviderRequirement{}, relevant, relevant
	}
	if value.Type() != cty.String {
		relevant := strings.Contains(localName, "fastly")
		return ProviderRequirement{}, relevant, relevant
	}
	if localName != "fastly" && localName != "fastly-beta" {
		return ProviderRequirement{}, false, false
	}

	requirement.Source = "<implicit>"
	requirement.Version = value.AsString()
	return requirement, true, false
}

func objectAttributeString(
	object *hclsyntax.ObjectConsExpr,
	name string,
) (value string, present bool, known bool) {
	for _, item := range object.Items {
		key, keyDiags := item.KeyExpr.Value(nil)
		if keyDiags.HasErrors() || !key.IsKnown() || key.IsNull() || key.Type() != cty.String {
			continue
		}
		if key.AsString() != name {
			continue
		}

		rawValue, valueDiags := item.ValueExpr.Value(nil)
		if valueDiags.HasErrors() || !rawValue.IsKnown() || rawValue.IsNull() || rawValue.Type() != cty.String {
			return "", true, false
		}
		return rawValue.AsString(), true, true
	}
	return "", false, true
}

func isFastlyProviderSource(source string) bool {
	switch source {
	case "fastly/fastly",
		"fastly/fastly-beta",
		"registry.terraform.io/fastly/fastly",
		"registry.terraform.io/fastly/fastly-beta":
		return true
	default:
		return false
	}
}

func (a *Analyzer) analyzeResource(
	state *analysisState,
	moduleDir string,
	block *hclsyntax.Block,
) {
	resourceType, resourceName := block.Labels[0], block.Labels[1]
	resourceRules := filterKind(a.registry.RulesForLegacy(resourceType), compatibility.ConstructResource)
	if len(resourceRules) == 0 {
		return
	}

	rule, ok := a.selectResourceRule(state, block, resourceRules)
	if !ok {
		return
	}

	a.applyRule(state, rule, block.TypeRange, resourceType, resourceName)
	if rule.Target != "" && rule.Target != resourceType {
		if state.addressMap[moduleDir] == nil {
			state.addressMap[moduleDir] = make(map[string]string)
		}
		state.addressMap[moduleDir][resourceType+"."+resourceName] = rule.Target + "." + resourceName
	}

	if resourceType != "fastly_service_vcl" && resourceType != "fastly_service_compute" {
		return
	}

	a.analyzeServiceBody(state, block.Body, resourceType, resourceName)
}

func (a *Analyzer) selectResourceRule(
	state *analysisState,
	block *hclsyntax.Block,
	rules []compatibility.Rule,
) (compatibility.Rule, bool) {
	if len(rules) == 1 && rules[0].Selector == "" {
		return rules[0], true
	}

	selectorAttribute := ""
	for _, rule := range rules {
		name, _, ok := strings.Cut(rule.Selector, " = ")
		if !ok || name == "" {
			a.appendAmbiguousRuleFinding(
				state,
				block.TypeRange,
				rules,
				"Compatibility selector cannot be evaluated for this legacy resource.",
			)
			return compatibility.Rule{}, false
		}
		if selectorAttribute == "" {
			selectorAttribute = name
		} else if selectorAttribute != name {
			a.appendAmbiguousRuleFinding(
				state,
				block.TypeRange,
				rules,
				"Compatibility rules use multiple selectors for this legacy resource.",
			)
			return compatibility.Rule{}, false
		}
	}

	attr, ok := block.Body.Attributes[selectorAttribute]
	if !ok {
		a.appendAmbiguousRuleFinding(
			state,
			block.TypeRange,
			rules,
			fmt.Sprintf("Legacy resource selector %q is not configured.", selectorAttribute),
		)
		return compatibility.Rule{}, false
	}

	value, known := staticString(attr.Expr)
	if !known {
		a.appendAmbiguousRuleFinding(
			state,
			attr.SrcRange,
			rules,
			fmt.Sprintf("Legacy resource selector %q cannot be determined statically.", selectorAttribute),
		)
		return compatibility.Rule{}, false
	}
	selector := selectorAttribute + " = " + strconv.Quote(value)
	for _, rule := range rules {
		if rule.Selector == selector {
			return rule, true
		}
	}

	a.appendAmbiguousRuleFinding(
		state,
		attr.SrcRange,
		rules,
		fmt.Sprintf("No compatibility rule matches %s.", selector),
	)
	return compatibility.Rule{}, false
}

func (a *Analyzer) analyzeServiceBody(
	state *analysisState,
	body *hclsyntax.Body,
	resourceType string,
	resourceName string,
) {
	for name, attr := range body.Attributes {
		if _, meta := terraformMetaAttributes[name]; meta {
			continue
		}
		legacy := resourceType + "." + name
		rules := filterKind(a.registry.RulesForLegacy(legacy), compatibility.ConstructAttribute)
		if len(rules) == 0 {
			state.result.Findings = append(state.result.Findings, Finding{
				Severity: SeverityError,
				Range:    attr.SrcRange,
				Message:  fmt.Sprintf("No compatibility rule exists for legacy service attribute %q.", legacy),
				Remediation: "Remove or replace this attribute, or add a reviewed compatibility rule " +
					"before automatic migration.",
			})
			continue
		}
		rule, ok := selectAttributeRule(rules, attr.Expr)
		if !ok {
			a.appendAmbiguousRuleFinding(
				state,
				attr.SrcRange,
				rules,
				fmt.Sprintf("Value of %q cannot be classified statically.", legacy),
			)
			continue
		}
		a.applyRule(state, rule, attr.SrcRange, resourceType, resourceName)
	}

	for _, block := range body.Blocks {
		if _, meta := terraformMetaBlocks[block.Type]; meta {
			continue
		}
		if block.Type == "dynamic" {
			a.analyzeDynamicServiceBlock(state, block, resourceType)
			continue
		}

		legacy := resourceType + "." + block.Type
		rules := filterKind(a.registry.RulesForLegacy(legacy), compatibility.ConstructServiceBlock)
		if len(rules) == 0 {
			state.result.Findings = append(state.result.Findings, Finding{
				Severity: SeverityError,
				Range:    block.TypeRange,
				Message:  fmt.Sprintf("No compatibility rule exists for legacy service block %q.", legacy),
				Remediation: "Remove or replace this block, or add a reviewed compatibility rule " +
					"before automatic migration.",
			})
			continue
		}

		if len(rules) != 1 {
			a.appendAmbiguousRuleFinding(
				state,
				block.TypeRange,
				rules,
				fmt.Sprintf("Multiple compatibility rules match legacy service block %q.", legacy),
			)
			continue
		}
		rule := rules[0]
		if block.Type != "product_enablement" {
			a.applyRule(state, rule, block.TypeRange, resourceType, resourceName)
		}
		a.analyzeNestedBlock(state, block, resourceType, resourceName)
	}
}

func (a *Analyzer) analyzeDynamicServiceBlock(
	state *analysisState,
	block *hclsyntax.Block,
	resourceType string,
) {
	if len(block.Labels) != 1 {
		state.result.Findings = append(state.result.Findings, Finding{
			Severity:    SeverityError,
			Range:       block.TypeRange,
			Message:     "Dynamic service block has an invalid label shape.",
			Remediation: "Use statically declared service blocks before running fastly-tf-migrate.",
		})
		return
	}
	legacy := resourceType + "." + block.Labels[0]
	rules := filterKind(a.registry.RulesForLegacy(legacy), compatibility.ConstructServiceBlock)
	if len(rules) == 0 {
		return
	}
	state.result.Findings = append(state.result.Findings, Finding{
		Severity: SeverityError,
		Range:    block.TypeRange,
		Message: fmt.Sprintf(
			"Dynamic legacy service block %q cannot be expanded safely by static analysis.",
			legacy,
		),
		Remediation: "Replace the dynamic block with statically declared blocks, or migrate this " +
			"construct manually.",
	})
}

func (a *Analyzer) analyzeNestedBlock(
	state *analysisState,
	block *hclsyntax.Block,
	resourceType string,
	resourceName string,
) {
	for name, attr := range block.Body.Attributes {
		legacy := resourceType + "." + block.Type + "." + name

		if block.Type == "product_enablement" {
			productRules := filterKind(a.registry.RulesForLegacy(legacy), compatibility.ConstructProduct)
			if len(productRules) > 0 {
				if isStaticallyNull(attr.Expr) {
					continue
				}
				if len(productRules) != 1 {
					a.appendAmbiguousRuleFinding(
						state,
						attr.SrcRange,
						productRules,
						fmt.Sprintf("Multiple compatibility rules match product field %q.", legacy),
					)
					continue
				}
				a.applyRule(state, productRules[0], attr.SrcRange, resourceType, resourceName)
				continue
			}
		}

		attributeRules := filterKind(a.registry.RulesForLegacy(legacy), compatibility.ConstructAttribute)
		if len(attributeRules) == 0 {
			if block.Type == "product_enablement" {
				state.result.Findings = append(state.result.Findings, Finding{
					Severity:    SeverityError,
					Range:       attr.SrcRange,
					Message:     fmt.Sprintf("No compatibility rule exists for product enablement field %q.", legacy),
					Remediation: "Remove or migrate this product setting manually before automatic migration.",
				})
			}
			continue
		}

		rule, ok := selectAttributeRule(attributeRules, attr.Expr)
		if !ok {
			a.appendAmbiguousRuleFinding(
				state,
				attr.SrcRange,
				attributeRules,
				fmt.Sprintf("Value of %q cannot be classified statically.", legacy),
			)
			continue
		}
		a.applyRule(state, rule, attr.SrcRange, resourceType, resourceName)
	}

	for _, nested := range block.Body.Blocks {
		legacy := resourceType + "." + block.Type + "." + nested.Type
		rules := a.registry.RulesForLegacy(legacy)

		if block.Type == "product_enablement" {
			productRules := filterKind(rules, compatibility.ConstructProduct)
			if len(productRules) > 0 {
				if len(productRules) != 1 {
					a.appendAmbiguousRuleFinding(
						state,
						nested.TypeRange,
						productRules,
						fmt.Sprintf("Multiple compatibility rules match product block %q.", legacy),
					)
					continue
				}
				a.applyRule(state, productRules[0], nested.TypeRange, resourceType, resourceName)
				continue
			}
		}

		if len(rules) == 0 {
			state.result.Findings = append(state.result.Findings, Finding{
				Severity: SeverityError,
				Range:    nested.TypeRange,
				Message:  fmt.Sprintf("Nested legacy construct %q has no compatibility rule.", legacy),
				Remediation: "Flatten, remove, or migrate this nested construct manually before " +
					"automatic migration.",
			})
		}
	}
}

func isStaticallyNull(expression hclsyntax.Expression) bool {
	value, diags := expression.Value(nil)
	return !diags.HasErrors() && value.IsKnown() && value.IsNull()
}

func selectAttributeRule(
	rules []compatibility.Rule,
	expression hclsyntax.Expression,
) (compatibility.Rule, bool) {
	if len(rules) == 1 && rules[0].Selector == "" {
		return rules[0], true
	}

	value, diags := expression.Value(nil)
	if !diags.HasErrors() && value.IsKnown() && value.IsNull() {
		for _, rule := range rules {
			if strings.Contains(rule.Selector, "omitted") {
				return rule, true
			}
		}
		return compatibility.Rule{}, false
	}

	for _, rule := range rules {
		switch rule.Selector {
		case "true or omitted", "false or omitted", "true", "false":
			value, known := staticBool(expression)
			if !known {
				return compatibility.Rule{}, false
			}
			if (value && (rule.Selector == "true" || rule.Selector == "true or omitted")) ||
				(!value && (rule.Selector == "false" || rule.Selector == "false or omitted")) {
				return rule, true
			}
		case "empty or omitted", "non-empty":
			value, known := staticString(expression)
			if !known {
				return compatibility.Rule{}, false
			}
			if (value == "" && rule.Selector == "empty or omitted") ||
				(value != "" && rule.Selector == "non-empty") {
				return rule, true
			}
		}
	}
	return compatibility.Rule{}, false
}

func staticBool(expression hclsyntax.Expression) (bool, bool) {
	value, diags := expression.Value(nil)
	if diags.HasErrors() || !value.IsKnown() || value.IsNull() || value.Type() != cty.Bool {
		return false, false
	}
	return value.True(), true
}

func staticString(expression hclsyntax.Expression) (string, bool) {
	value, diags := expression.Value(nil)
	if diags.HasErrors() || !value.IsKnown() || value.IsNull() || value.Type() != cty.String {
		return "", false
	}
	return value.AsString(), true
}

func (a *Analyzer) applyRule(
	state *analysisState,
	rule compatibility.Rule,
	sourceRange hcl.Range,
	resourceType string,
	resourceName string,
) {
	switch rule.Support {
	case compatibility.SupportAutomatic:
		if shouldPlan(rule) {
			state.result.Actions = append(state.result.Actions, Action{
				RuleID: rule.ID,
				Kind:   rule.Kind,
				From:   qualifyLegacy(rule.Legacy, resourceType, resourceName),
				To:     qualifyTarget(rule.Target, resourceName),
				HCL:    rule.HCL,
				State:  rule.State,
				Range:  sourceRange,
			})
		}
	case compatibility.SupportBlocked, compatibility.SupportManual, compatibility.SupportUnsupported:
		state.result.Findings = append(state.result.Findings, findingForRule(rule, sourceRange))
	}
}

func shouldPlan(rule compatibility.Rule) bool {
	switch rule.Kind {
	case compatibility.ConstructResource, compatibility.ConstructProduct:
		return rule.HCL != compatibility.HCLUnchanged ||
			rule.State != compatibility.StateUnchanged
	case compatibility.ConstructServiceBlock, compatibility.ConstructAttribute:
		return rule.HCL != compatibility.HCLUnchanged ||
			rule.State == compatibility.StateImport
	}
	return false
}

func findingForRule(rule compatibility.Rule, sourceRange hcl.Range) Finding {
	message := fmt.Sprintf(
		"%s is %s for automatic migration",
		rule.Legacy,
		rule.Support,
	)
	if rule.Selector != "" {
		message += fmt.Sprintf(" when %s", rule.Selector)
	}
	message += "."

	remediation := rule.Notes
	if rule.Support == compatibility.SupportBlocked && rule.ProviderPrerequisite != "" {
		remediation = rule.ProviderPrerequisite
		if rule.Notes != "" {
			remediation += " " + rule.Notes
		}
	}
	if remediation == "" {
		remediation = "Resolve this construct manually before running migration."
	}

	return Finding{
		Severity:    SeverityError,
		RuleID:      rule.ID,
		Range:       sourceRange,
		Message:     message,
		Remediation: remediation,
	}
}

func (a *Analyzer) appendAmbiguousRuleFinding(
	state *analysisState,
	sourceRange hcl.Range,
	rules []compatibility.Rule,
	message string,
) {
	selectors := make([]string, 0, len(rules))
	for _, rule := range rules {
		if rule.Selector != "" {
			selectors = append(selectors, rule.Selector)
		}
	}
	remediation := "Replace the selector with a literal or otherwise resolve it before running migration."
	if len(selectors) > 0 {
		remediation = fmt.Sprintf(
			"Resolve the value to one of the compatibility selectors: %s.",
			strings.Join(selectors, ", "),
		)
	}
	state.result.Findings = append(state.result.Findings, Finding{
		Severity:    SeverityError,
		Range:       sourceRange,
		Message:     message,
		Remediation: remediation,
	})
}

func filterKind(rules []compatibility.Rule, kind compatibility.ConstructKind) []compatibility.Rule {
	result := make([]compatibility.Rule, 0, len(rules))
	for _, rule := range rules {
		if rule.Kind == kind {
			result = append(result, rule)
		}
	}
	return result
}

func qualifyLegacy(legacy, resourceType, resourceName string) string {
	if legacy == resourceType {
		return legacy + "." + resourceName
	}
	prefix := resourceType + "."
	if strings.HasPrefix(legacy, prefix) {
		return resourceType + "." + resourceName + "." + strings.TrimPrefix(legacy, prefix)
	}
	return legacy
}

func qualifyTarget(target, resourceName string) string {
	if target == "" {
		return "<removed>"
	}
	resource, rest, found := strings.Cut(target, ".")
	if !found {
		return target + "." + resourceName
	}
	return resource + "." + resourceName + "." + rest
}

func (a *Analyzer) findReferences(state *analysisState) {
	if len(state.addressMap) == 0 {
		return
	}

	for _, parsed := range state.files {
		moduleAddresses := state.addressMap[parsed.moduleDir]
		if len(moduleAddresses) == 0 {
			continue
		}
		walkExpressions(parsed.body, func(expression hclsyntax.Expression) {
			for _, traversal := range expression.Variables() {
				if len(traversal) < 2 {
					continue
				}
				root, ok := traversal[0].(hcl.TraverseRoot)
				if !ok {
					continue
				}
				name, ok := traversal[1].(hcl.TraverseAttr)
				if !ok {
					continue
				}
				address := root.Name + "." + name.Name
				target, ok := moduleAddresses[address]
				if !ok {
					continue
				}
				state.result.References = append(state.result.References, Reference{
					From:  address,
					To:    target,
					Range: traversal.SourceRange(),
				})
			}
		})
	}
}

func walkExpressions(body *hclsyntax.Body, visit func(hclsyntax.Expression)) {
	for _, attr := range body.Attributes {
		visit(attr.Expr)
	}
	for _, block := range body.Blocks {
		walkExpressions(block.Body, visit)
	}
}

func appendHCLDiagnostics(result *Result, filename string, diags hcl.Diagnostics) {
	for _, diagnostic := range diags {
		if diagnostic.Severity != hcl.DiagError {
			continue
		}
		sourceRange := fileStartRange(filename)
		if diagnostic.Subject != nil {
			sourceRange = *diagnostic.Subject
		}
		result.Findings = append(result.Findings, Finding{
			Severity: SeverityError,
			Range:    sourceRange,
			Message:  diagnostic.Summary + ": " + diagnostic.Detail,
			Remediation: "Fix the Terraform syntax error before running " +
				"fastly-tf-migrate analyze again.",
		})
	}
}

func fileStartRange(filename string) hcl.Range {
	return hcl.Range{
		Filename: filename,
		Start:    hcl.Pos{Line: 1, Column: 1},
		End:      hcl.Pos{Line: 1, Column: 1},
	}
}

func relativeModulePath(root, path string) string {
	relative := relativePath(root, path)
	if relative == "" {
		return "."
	}
	return relative
}

func relativePath(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.Clean(path)
	}
	if relative == "." {
		return "."
	}
	return filepath.ToSlash(relative)
}
