package ngwafmovestate

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type fixture struct {
	Version int64        `json:"version"`
	Block   fixtureBlock `json:"block"`
}

type fixtureBlock struct {
	Attributes map[string]json.RawMessage `json:"attributes"`
	BlockTypes map[string]fixtureNested   `json:"block_types"`
}

type fixtureNested struct {
	NestingMode string       `json:"nesting_mode"`
	Block       fixtureBlock `json:"block"`
}

// fixtureFile is the 10.3.0 schema export for source, relative to this package.
func fixtureFile(source Source) string {
	return filepath.Join("testdata", "legacy_"+strings.TrimPrefix(source.TypeName, "fastly_ngwaf_")+"_schema_v0_10_3_0_type.json")
}

func TestLegacySchemasMatchFixtures(t *testing.T) {
	t.Parallel()

	sources := Sources()
	if len(sources) != 16 {
		t.Fatalf("Sources() returned %d legacy types, want 16", len(sources))
	}
	for _, source := range sources {
		t.Run(source.TypeName, func(t *testing.T) {
			t.Parallel()

			raw, err := os.ReadFile(fixtureFile(source)) // #nosec G304 -- repository test fixtures
			if err != nil {
				t.Fatal(err)
			}
			var f fixture
			if err := json.Unmarshal(raw, &f); err != nil {
				t.Fatal(err)
			}
			if f.Version != legacySchemaVersion {
				t.Fatalf("fixture schema version = %d, want %d", f.Version, legacySchemaVersion)
			}

			got := source.LegacySchema().Type().TerraformType(context.Background())
			if want := blockType(t, f.Block); !reflect.DeepEqual(got, want) {
				t.Fatalf("decode schema does not match the published legacy type\n got: %s\nwant: %s", got, want)
			}
		})
	}
}

func TestSourcesTargetTypesAreUnique(t *testing.T) {
	t.Parallel()

	seen := make(map[string]string)
	for _, source := range Sources() {
		if (source.Target == "") == (source.Targets == nil) {
			t.Errorf("%s must set exactly one of Target or Targets", source.TypeName)
		}
		for _, target := range source.TargetTypes() {
			if previous, ok := seen[target]; ok {
				t.Errorf("%s is a target of both %s and %s", target, previous, source.TypeName)
			}
			seen[target] = source.TypeName
		}
	}
	if len(seen) != 28 {
		t.Fatalf("got %d target types, want 28", len(seen))
	}
}

func blockType(t *testing.T, block fixtureBlock) tftypes.Type {
	t.Helper()

	attributes := make(map[string]tftypes.Type, len(block.Attributes)+len(block.BlockTypes))
	for name, raw := range block.Attributes {
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			t.Fatal(err)
		}
		attributes[name] = valueType(t, value)
	}
	for name, nested := range block.BlockTypes {
		element := blockType(t, nested.Block)
		switch nested.NestingMode {
		case "list":
			attributes[name] = tftypes.List{ElementType: element}
		case "set":
			attributes[name] = tftypes.Set{ElementType: element}
		default:
			t.Fatalf("unsupported nesting mode %q for %s", nested.NestingMode, name)
		}
	}
	return tftypes.Object{AttributeTypes: attributes}
}

func valueType(t *testing.T, value any) tftypes.Type {
	t.Helper()

	switch v := value.(type) {
	case string:
		switch v {
		case "string":
			return tftypes.String
		case "bool":
			return tftypes.Bool
		case "number":
			return tftypes.Number
		}
	case []any:
		if len(v) == 2 {
			element := valueType(t, v[1])
			switch v[0] {
			case "list":
				return tftypes.List{ElementType: element}
			case "set":
				return tftypes.Set{ElementType: element}
			case "map":
				return tftypes.Map{ElementType: element}
			}
		}
	}
	t.Fatalf("unsupported fixture type %#v", value)
	return nil
}
