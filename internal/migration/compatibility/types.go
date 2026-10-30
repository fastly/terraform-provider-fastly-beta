package compatibility

// ConstructKind identifies the Terraform surface represented by a compatibility rule.
type ConstructKind string

const (
	// ConstructResource is a top-level Terraform resource.
	ConstructResource ConstructKind = "resource"
	// ConstructServiceBlock is a nested block inside a legacy aggregate service resource.
	ConstructServiceBlock ConstructKind = "service_block"
	// ConstructProduct is a legacy product_enablement field that becomes a standalone resource.
	ConstructProduct ConstructKind = "product"
	// ConstructAttribute is a field-level compatibility exception or reshape.
	ConstructAttribute ConstructKind = "attribute"
)

// ServiceFamily identifies the service family to which a rule applies.
type ServiceFamily string

const (
	// FamilyGlobal applies outside a specific service family.
	FamilyGlobal ServiceFamily = "global"
	// FamilyCDN applies to legacy VCL/CDN services.
	FamilyCDN ServiceFamily = "cdn"
	// FamilyCompute applies to Compute services.
	FamilyCompute ServiceFamily = "compute"
)

// SupportStatus describes whether a migration construct can be automated safely.
type SupportStatus string

const (
	// SupportAutomatic means the construct has a deterministic migration mapping.
	SupportAutomatic SupportStatus = "automatic"
	// SupportManual means the construct needs explicit practitioner acknowledgement or handling.
	SupportManual SupportStatus = "manual"
	// SupportBlocked means the mapping is known but a provider-side prerequisite is not complete.
	SupportBlocked SupportStatus = "blocked"
	// SupportUnsupported means the target model cannot safely represent the legacy construct.
	SupportUnsupported SupportStatus = "unsupported"
)

// HCLStrategy describes how configuration must change.
type HCLStrategy string

const (
	// HCLUnchanged preserves the HCL shape.
	HCLUnchanged HCLStrategy = "unchanged"
	// HCLRename changes a resource, block, or attribute name without changing its semantic shape.
	HCLRename HCLStrategy = "rename"
	// HCLReshape changes nesting or groups fields into a different HCL shape.
	HCLReshape HCLStrategy = "reshape"
	// HCLSplit expands one legacy construct into multiple target constructs.
	HCLSplit HCLStrategy = "split"
	// HCLRemove removes a legacy-only field after its semantics are proven unnecessary or acknowledged.
	HCLRemove HCLStrategy = "remove"
	// HCLReject prevents migration of a construct that cannot be represented safely.
	HCLReject HCLStrategy = "reject"
)

// StateStrategy describes the Terraform-native state transition required by a rule.
type StateStrategy string

const (
	// StateUnchanged keeps the same Terraform resource address and compatible state shape.
	StateUnchanged StateStrategy = "unchanged"
	// StateMove uses Terraform moved blocks with provider ResourceWithMoveState support.
	StateMove StateStrategy = "move_state"
	// StateUpgrade uses provider ResourceWithUpgradeState support for a same-type schema change.
	StateUpgrade StateStrategy = "upgrade_state"
	// StateImport adopts an existing remote object through a Terraform import block.
	StateImport StateStrategy = "import"
	// StateNone means the rule does not itself perform a state transition.
	StateNone StateStrategy = "none"
)

// ImportIdentity describes the identifier used by a generated Terraform import block.
type ImportIdentity string

const (
	// ImportNone means the rule does not use Terraform import.
	ImportNone ImportIdentity = "none"
	// ImportServiceID imports a service-scoped object by Fastly service ID.
	ImportServiceID ImportIdentity = "service_id"
	// ImportLegacyID reuses the legacy resource ID as the target import identity.
	ImportLegacyID ImportIdentity = "legacy_id"
	// ImportProviderSpecific requires a resource-specific import identity.
	ImportProviderSpecific ImportIdentity = "provider_specific"
)

// Rule is the typed source of truth for one legacy-to-_auto compatibility decision.
//
// Selector narrows a rule when a single legacy construct can map to multiple targets,
// such as NGWAF resources selected by their legacy type attribute.
type Rule struct {
	ID                   string
	Kind                 ConstructKind
	Family               ServiceFamily
	Legacy               string
	Target               string
	Selector             string
	Support              SupportStatus
	HCL                  HCLStrategy
	State                StateStrategy
	ImportIdentity       ImportIdentity
	SourceSchemaVersions []int64
	ProviderPrerequisite string
	Notes                string
}
