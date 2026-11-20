package rewrite

import (
	"fmt"
	"maps"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"

	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/compatibility"
)

// ReferenceTarget describes an enabled resource conversion in one module.
// Settings is reserved for a proven provider-side singleton settings migration.
type ReferenceTarget struct {
	To       string
	Settings bool
	Indexed  bool // count or for_each is present on the declaration
}

// ResourceReferenceTarget records the source instance shape. The current
// CDN mover does not hydrate the optional settings list before Read, so even
// explicitly configured legacy settings cannot yet authorize output rewrites.
// Settings must remain false until provider-side no-op migration is proven.
func ResourceReferenceTarget(to string, block *hclsyntax.Block) ReferenceTarget {
	return ReferenceTarget{
		To: to, Indexed: block.Body.Attributes["count"] != nil || block.Body.Attributes["for_each"] != nil,
	}
}

// ReferenceOccurrence describes a matched reference using module-local addresses.
type ReferenceOccurrence struct {
	From, To string
	Range    hcl.Range
	// Changed reports that this occurrence needs a source edit. A reference to
	// a resource whose address is unchanged is matched, but needs no edit
	// unless it reads an attribute the target schema renamed.
	Changed bool
}

// ReferenceProblem is a fail-closed diagnostic about an ambiguous reference.
type ReferenceProblem struct {
	Range   hcl.Range
	Message string
}

// ReferenceInspection is the read-only result of scanning one native HCL file.
type ReferenceInspection struct {
	References []ReferenceOccurrence
	Problems   []ReferenceProblem
	edits      []replacement
}

// References rewrites only syntax-confirmed references. Routes are local
// to this module. AllRoutes is used solely to detect ambiguous cross-module
// state-address constructs; normal expressions never use another module's map.
type References struct {
	File      string
	Routes    map[string]ReferenceTarget
	AllRoutes map[string]bool
	Registry  *compatibility.Registry
}

func (e References) file() string    { return e.File }
func (e References) canCreate() bool { return false }

func (e References) apply(source []byte, filename string) ([]byte, error) {
	body, err := parseBody(source, filename)
	if err != nil {
		return nil, err
	}
	result := e.Inspect(body)
	if len(result.Problems) > 0 {
		p := result.Problems[0]
		return nil, fmt.Errorf("%s: %s", p.Range.String(), p.Message)
	}
	return replace(source, result.edits)
}

// Inspect is shared by analyze and the writer. It never evaluates configuration
// expressions or guesses the identity of a resource passed as a whole object.
func (e References) Inspect(body *hclsyntax.Body) ReferenceInspection {
	s := referenceScanner{edit: e}
	s.body(body, "", "", nil, nil)
	sort.Slice(s.result.Problems, func(i, j int) bool {
		return s.result.Problems[i].Range.Start.Byte < s.result.Problems[j].Range.Start.Byte
	})
	sort.Slice(s.result.References, func(i, j int) bool {
		return s.result.References[i].Range.Start.Byte < s.result.References[j].Range.Start.Byte
	})
	return s.result
}

type referenceScanner struct {
	edit   References
	result ReferenceInspection
}

func (s *referenceScanner) problem(r hcl.Range, message string) {
	s.result.Problems = append(s.result.Problems, ReferenceProblem{Range: r, Message: message})
}

func (s *referenceScanner) token(r hcl.Range, value string) {
	s.result.edits = append(s.result.edits, replacement{r.Start.Byte, r.End.Byte, []byte(value)})
}

// Attribute ranges include the leading dot. Replace only the identifier so
// intervening whitespace/comments survive.
func (s *referenceScanner) attribute(step referenceStep, value string) {
	r := step.rng
	r.Start.Byte = r.End.Byte - len(step.name)
	s.token(r, value)
}

type referenceStep struct {
	name  string // empty for an index or splat
	rng   hcl.Range
	splat bool
}

type referenceChain struct {
	root  string
	rng   hcl.Range
	steps []referenceStep
	end   int
}

func appendTraversal(chain referenceChain, traversal hcl.Traversal) (referenceChain, bool) {
	for _, part := range traversal {
		switch p := part.(type) {
		case hcl.TraverseRoot:
			chain.root, chain.rng = p.Name, p.SrcRange
		case hcl.TraverseAttr:
			chain.steps = append(chain.steps, referenceStep{name: p.Name, rng: p.SrcRange})
		case hcl.TraverseIndex:
			chain.steps = append(chain.steps, referenceStep{rng: p.SrcRange})
		default:
			return referenceChain{}, false
		}
	}
	return chain, true
}

// expressionChain follows access syntax, including dynamic indexes and both
// kinds of splat. It does not follow arbitrary function results or evaluate keys.
func expressionChain(expression hclsyntax.Expression, anon *hclsyntax.AnonSymbolExpr) (referenceChain, bool) {
	var chain referenceChain
	var ok bool
	switch e := expression.(type) {
	case *hclsyntax.ScopeTraversalExpr:
		chain, ok = appendTraversal(chain, e.Traversal)
	case *hclsyntax.RelativeTraversalExpr:
		chain, ok = expressionChain(e.Source, anon)
		if ok {
			chain, ok = appendTraversal(chain, e.Traversal)
		}
	case *hclsyntax.IndexExpr:
		chain, ok = expressionChain(e.Collection, anon)
		if ok {
			chain.steps = append(chain.steps, referenceStep{})
		}
	case *hclsyntax.SplatExpr:
		chain, ok = expressionChain(e.Source, anon)
		if ok {
			var each referenceChain
			each, ok = expressionChain(e.Each, e.Item)
			if ok && each.root == "" {
				chain.steps = append(chain.steps, referenceStep{splat: true})
				chain.steps = append(chain.steps, each.steps...)
			} else {
				ok = false
			}
		}
	case *hclsyntax.AnonSymbolExpr:
		ok = anon != nil && e == anon
	case *hclsyntax.ParenthesesExpr:
		chain, ok = expressionChain(e.Expression, anon)
	default:
		return referenceChain{}, false
	}
	chain.end = expression.Range().End.Byte
	return chain, ok
}

// freeChains combines HCL's lexical-scope-aware Variables with maximal access
// chains from the syntax tree. Variables alone truncates dynamic index/splat
// references before their suffix, which could conceal a removed attribute.
func freeChains(expression hclsyntax.Expression) []referenceChain {
	free := make(map[int]referenceChain)
	for _, traversal := range expression.Variables() {
		chain, ok := appendTraversal(referenceChain{}, traversal)
		if ok && chain.root != "" {
			chain.end = traversal.SourceRange().End.Byte
			free[chain.rng.Start.Byte] = chain
		}
	}
	_ = hclsyntax.VisitAll(expression, func(node hclsyntax.Node) hcl.Diagnostics {
		if expr, ok := node.(hclsyntax.Expression); ok {
			chain, valid := expressionChain(expr, nil)
			if previous, found := free[chain.rng.Start.Byte]; valid && found && chain.root == previous.root &&
				(chain.end > previous.end || (chain.end == previous.end && len(chain.steps) > len(previous.steps))) {
				free[chain.rng.Start.Byte] = chain
			}
		}
		return nil
	})
	chains := make([]referenceChain, 0, len(free))
	for _, chain := range free {
		chains = append(chains, chain)
	}
	sort.Slice(chains, func(i, j int) bool { return chains[i].rng.Start.Byte < chains[j].rng.Start.Byte })
	return chains
}

func (c referenceChain) address() (address string, typeRange hcl.Range, tail []referenceStep) {
	if c.root == "resource" && len(c.steps) >= 2 && c.steps[0].name != "" && c.steps[1].name != "" {
		r := c.steps[0].rng
		r.Start.Byte = r.End.Byte - len(c.steps[0].name)
		return c.steps[0].name + "." + c.steps[1].name, r, c.steps[2:]
	}
	if len(c.steps) == 0 || c.steps[0].name == "" {
		return "", hcl.Range{}, nil
	}
	return c.root + "." + c.steps[0].name, c.rng, c.steps[1:]
}

func instanceTail(tail []referenceStep) []referenceStep {
	if len(tail) != 0 && tail[0].name == "" {
		return tail[1:] // one resource instance selector (index or splat)
	}
	return tail
}

func (s *referenceScanner) body(body *hclsyntax.Body, blockType, self string, shadow map[string]bool, iterators map[string]bool) {
	for name, attr := range body.Attributes {
		if ((blockType == "resource" || blockType == "data" || blockType == "import") && name == "provider") ||
			(blockType == "module" && name == "providers") || blockType == "required_providers" ||
			(blockType == "dynamic" && name == "iterator") {
			continue // provider references and dynamic iterator declarations are not value expressions
		}
		if (blockType == "import" && name == "to") || (blockType == "removed" && name == "from") {
			s.stateAddress(attr.Expr)
			continue
		}
		if blockType == "lifecycle" && name == "ignore_changes" {
			if self != "" {
				s.ignoreChanges(attr.Expr, self)
			}
			continue // ignore_changes paths are relative to the enclosing resource
		}
		activeShadow := shadow
		if blockType == "dynamic" && name == "labels" {
			activeShadow = iterators
		}
		dependency := (name == "depends_on" && (blockType == "resource" || blockType == "data" ||
			blockType == "module" || blockType == "output")) || (blockType == "lifecycle" && name == "replace_triggered_by")
		s.expression(attr.Expr, activeShadow, self, dependency)
	}
	for _, block := range body.Blocks {
		if block.Type == "moved" {
			s.moved(block)
			continue
		}
		childSelf := self
		if block.Type == "resource" && len(block.Labels) == 2 {
			childSelf = block.Labels[0] + "." + block.Labels[1]
			if _, ok := s.edit.Routes[childSelf]; !ok {
				childSelf = ""
			}
		}
		childShadow := shadow
		var dynamicShadow map[string]bool
		if blockType == "dynamic" && block.Type == "content" {
			childShadow = iterators
		}
		if block.Type == "dynamic" && len(block.Labels) == 1 {
			iterator := block.Labels[0]
			if attr := block.Body.Attributes["iterator"]; attr != nil {
				traversal, diags := hcl.AbsTraversalForExpr(attr.Expr)
				if diags.HasErrors() || len(traversal) != 1 {
					s.problem(attr.SrcRange, "dynamic iterator cannot be resolved safely")
					continue
				}
				iterator = traversal.RootName()
			}
			dynamicShadow = make(map[string]bool)
			maps.Copy(dynamicShadow, childShadow)
			dynamicShadow[iterator] = true
		}
		s.body(block.Body, block.Type, childSelf, childShadow, dynamicShadow)
	}
}

func (s *referenceScanner) expression(expr hclsyntax.Expression, shadow map[string]bool, self string, dependency bool) {
	// An order-independent name=>ID map is a safe way to consume the
	// legacy set-backed ACL/dictionary/snippet collections.
	projections := make(map[int]*hclsyntax.ForExpr)
	_ = hclsyntax.VisitAll(expr, func(node hclsyntax.Node) hcl.Diagnostics {
		if loop, ok := node.(*hclsyntax.ForExpr); ok {
			if c, valid := expressionChain(loop.CollExpr, nil); valid {
				projections[c.rng.Start.Byte] = loop
			}
		}
		return nil
	})
	for _, c := range freeChains(expr) {
		s.chain(c, shadow, self, dependency, projections)
	}
}

func (s *referenceScanner) chain(c referenceChain, shadow map[string]bool, self string, dependency bool, projections map[int]*hclsyntax.ForExpr) {
	if shadow[c.root] {
		return
	}
	editsBefore := len(s.result.edits)
	address, typeRange, tail := c.address()
	isSelf := c.root == "self" && self != ""
	if isSelf {
		address, tail = self, c.steps
	}
	target, found := s.edit.Routes[address]
	if !found {
		return
	}
	s.result.References = append(s.result.References, ReferenceOccurrence{From: address, To: target.To, Range: c.rng})
	occurrence := len(s.result.References) - 1
	defer func() { s.result.References[occurrence].Changed = len(s.result.edits) > editsBefore }()
	splat := !isSelf && len(tail) != 0 && tail[0].splat
	instanceSelected := !isSelf && len(tail) != 0 && tail[0].name == ""
	if !isSelf {
		tail = instanceTail(tail)
	}
	sourceType, _, _ := strings.Cut(address, ".")
	targetType, _, _ := strings.Cut(target.To, ".")
	switch {
	case len(tail) == 0 && !isSelf && (dependency ||
		(target.Indexed && !instanceSelected && s.verifiedInstanceProjection(projections[c.rng.Start.Byte], sourceType, targetType))):
		// Terraform dependency meta-arguments consume resource addresses;
		// verified scalar projections consume only unchanged instance values.
		if sourceType != targetType {
			s.token(typeRange, targetType)
		}
		return
	case len(tail) == 1 && tail[0].name != "":
		if path, ok := s.edit.Registry.ReferenceAttribute(sourceType, targetType, tail[0].name); ok {
			if strings.HasPrefix(path, "settings[0].") && (!target.Settings || splat) {
				s.problem(c.rng, fmt.Sprintf("reference to %s.%s requires a proven singleton settings migration", address, tail[0].name))
				return
			}
			if path != tail[0].name {
				s.attribute(tail[0], path)
			}
		} else if block, id, newID, ok := s.edit.Registry.ReferenceIdentityCollection(sourceType, targetType, tail[0].name); ok &&
			!splat && s.rewriteIdentityProjection(projections[c.rng.Start.Byte], id, newID) {
			if block != tail[0].name {
				s.attribute(tail[0], block)
			}
		} else {
			s.problem(c.rng, fmt.Sprintf("reference to %s.%s has no verified equivalent output or collection projection", address, tail[0].name))
			return
		}
	default:
		s.problem(c.rng, fmt.Sprintf("reference to %s uses an unsupported whole-resource value or nested attribute path", address))
		return
	}
	if !isSelf && sourceType != targetType {
		s.token(typeRange, targetType)
	}
}

func (s *referenceScanner) rewriteIdentityProjection(loop *hclsyntax.ForExpr, identity, targetIdentity string) bool {
	if loop == nil || loop.KeyVar != "" || loop.KeyExpr == nil || loop.CondExpr != nil || loop.Group {
		return false
	}
	key, okKey := expressionChain(loop.KeyExpr, nil)
	value, okValue := expressionChain(loop.ValExpr, nil)
	if !okKey || !okValue || key.root != loop.ValVar || value.root != loop.ValVar ||
		len(key.steps) != 1 || key.steps[0].name != "name" || len(value.steps) != 1 ||
		(value.steps[0].name != identity && value.steps[0].name != "name") {
		return false
	}
	if value.steps[0].name == identity && targetIdentity != identity {
		s.attribute(value.steps[0], targetIdentity)
	}
	return true
}

// Projections over count/for_each instances retain their instance ordering and
// keys; accept only direct unchanged scalar values, never whole service objects.
func (s *referenceScanner) verifiedInstanceProjection(loop *hclsyntax.ForExpr, sourceType, targetType string) bool {
	if loop == nil || loop.CondExpr != nil || loop.Group {
		return false
	}
	scalar := func(expr hclsyntax.Expression) bool {
		c, ok := expressionChain(expr, nil)
		if !ok {
			return false
		}
		if loop.KeyVar != "" && c.root == loop.KeyVar && len(c.steps) == 0 {
			return true
		}
		if c.root != loop.ValVar || len(c.steps) != 1 || c.steps[0].name == "" {
			return false
		}
		path, ok := s.edit.Registry.ReferenceAttribute(sourceType, targetType, c.steps[0].name)
		return ok && path == c.steps[0].name
	}
	return scalar(loop.ValExpr) && (loop.KeyExpr == nil || scalar(loop.KeyExpr))
}

func (s *referenceScanner) ignoreChanges(expr hclsyntax.Expression, address string) {
	if c, ok := expressionChain(expr, nil); ok && c.root == "all" && len(c.steps) == 0 {
		return
	}
	tuple, ok := expr.(*hclsyntax.TupleConsExpr)
	if !ok {
		s.problem(expr.Range(), "ignore_changes must use all or a static attribute list")
		return
	}
	target := s.edit.Routes[address]
	fromType, _, _ := strings.Cut(address, ".")
	toType, _, _ := strings.Cut(target.To, ".")
	for _, item := range tuple.Exprs {
		c, valid := expressionChain(item, nil)
		if !valid || c.root == "" || len(c.steps) != 0 {
			s.problem(item.Range(), "nested or dynamic ignore_changes paths require manual migration")
			continue
		}
		path, mapped := s.edit.Registry.ReferenceAttribute(fromType, toType, c.root)
		if !mapped || (strings.HasPrefix(path, "settings[0].") && !target.Settings) {
			s.problem(item.Range(), fmt.Sprintf("ignore_changes attribute %s has no verified equivalent", c.root))
			continue
		}
		if path != c.root {
			s.token(c.rng, path)
		}
	}
}

// State-address constructs have state semantics, not normal value semantics.
// Preserve an exact existing local move; refuse conflicting/ambiguous moves,
// imports or removals rather than silently changing their historical addresses.
func (s *referenceScanner) moved(block *hclsyntax.Block) {
	from, to := block.Body.Attributes["from"], block.Body.Attributes["to"]
	if from != nil && to != nil {
		a, okA := expressionChain(from.Expr, nil)
		b, okB := expressionChain(to.Expr, nil)
		fromAddress, _, fromTail := a.address()
		toAddress, _, toTail := b.address()
		if target, found := s.edit.Routes[fromAddress]; okA && okB && found &&
			a.root != "resource" && b.root != "resource" && len(fromTail) == 0 && len(toTail) == 0 &&
			target.To == toAddress {
			return
		}
		if okA && okB && a.root != "resource" && b.root != "resource" && len(fromTail) == 0 && len(toTail) == 0 &&
			s.isAppliedRename(fromAddress, toAddress) {
			return
		}
	}
	for _, attr := range block.Body.Attributes {
		s.stateAddress(attr.Expr)
	}
}

// isAppliedRename recognizes the moved block a previous migration wrote for a
// resource that has since been renamed. The source no longer exists, so its
// destination is a reference-only route: an address that keeps its name and
// needs no move. That history is valid and must survive repeated runs. A moved
// block that leaves a route's address, or enters it from anything but the
// registry's rename of the same name, is still a conflict.
func (s *referenceScanner) isAppliedRename(fromAddress, toAddress string) bool {
	target, found := s.edit.Routes[toAddress]
	if !found || target.To != toAddress || fromAddress == toAddress {
		return false
	}
	if _, stillRouted := s.edit.Routes[fromAddress]; stillRouted {
		return false
	}
	fromType, fromName, _ := strings.Cut(fromAddress, ".")
	toType, toName, _ := strings.Cut(toAddress, ".")
	if fromName != toName {
		return false
	}
	for _, rule := range s.edit.Registry.RulesForLegacy(fromType) {
		if rule.Kind == compatibility.ConstructResource && rule.Target == toType {
			return true
		}
	}
	return false
}

func (s *referenceScanner) stateAddress(expr hclsyntax.Expression) {
	for _, c := range freeChains(expr) {
		address, _, _ := c.address()
		if _, found := s.edit.Routes[address]; found || s.isLocalTarget(address) {
			s.problem(c.rng, "existing moved, import or removed address conflicts with a planned migration")
			continue
		}
		if c.root != "module" {
			continue
		}
		// Match only actual traversal segments, not literal strings or indexes.
		// Module call resolution for pre-existing cross-module state constructs
		// is not yet proven; conservatively block possible collisions.
		for i := 0; i+1 < len(c.steps); i++ {
			if c.steps[i].name != "" && c.steps[i+1].name != "" &&
				s.edit.AllRoutes[c.steps[i].name+"."+c.steps[i+1].name] {
				s.problem(c.rng, "module-qualified moved, import or removed address may conflict with a planned migration")
				break
			}
		}
	}
}

func (s *referenceScanner) isLocalTarget(address string) bool {
	for _, target := range s.edit.Routes {
		if target.To == address {
			return true
		}
	}
	return false
}
