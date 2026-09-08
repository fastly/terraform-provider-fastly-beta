package acceptancetests

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/scope"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/signals"
)

func TestAccFastlyNGWAFSignal_lifecycle(t *testing.T) {
	t.Parallel()

	suffix := acctest.RandString(10)
	signalName := fmt.Sprintf("Signal Test %s", suffix)
	description := "Terraform account signal lifecycle"
	updatedDescription := "Terraform account signal lifecycle updated"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFSignalDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFSignal("ngwaf_signal_basic.tf", signalName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_signal.test", "name", signalName),
					resource.TestCheckResourceAttr("fastly_ngwaf_signal.test", "description", description),
					resource.TestCheckResourceAttr("fastly_ngwaf_signal.test", "applies_to.#", "1"),
					resource.TestCheckTypeSetElemAttr("fastly_ngwaf_signal.test", "applies_to.*", "*"),
					resource.TestCheckResourceAttrSet("fastly_ngwaf_signal.test", "id"),
					resource.TestCheckResourceAttrSet("fastly_ngwaf_signal.test", "reference_id"),
					CheckNGWAFSignalExists("fastly_ngwaf_signal.test", signalName, description),
				),
			},
			{
				Config: ConfigNGWAFSignal("ngwaf_signal_updated.tf", signalName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("fastly_ngwaf_signal.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_signal.test", "name", signalName),
					resource.TestCheckResourceAttr("fastly_ngwaf_signal.test", "description", updatedDescription),
					resource.TestCheckResourceAttr("fastly_ngwaf_signal.test", "applies_to.#", "1"),
					resource.TestCheckTypeSetElemAttr("fastly_ngwaf_signal.test", "applies_to.*", "*"),
					CheckNGWAFSignalExists("fastly_ngwaf_signal.test", signalName, updatedDescription),
				),
			},
			{
				Config:   ConfigNGWAFSignal("ngwaf_signal_updated.tf", signalName),
				PlanOnly: true,
			},
			{
				ResourceName:      "fastly_ngwaf_signal.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccFastlyNGWAFSignals_dataSource(t *testing.T) {
	t.Parallel()

	suffix := acctest.RandString(10)
	signalName1 := fmt.Sprintf("Signal One %s", suffix)
	signalName2 := fmt.Sprintf("Signal Two %s", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFSignalDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFSignalsDataSource(signalName1, signalName2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.fastly_ngwaf_signals.test", "id"),
					checkNGWAFSignalsDataSourceHasSignal("data.fastly_ngwaf_signals.test", "fastly_ngwaf_signal.signal_1"),
					checkNGWAFSignalsDataSourceHasSignal("data.fastly_ngwaf_signals.test", "fastly_ngwaf_signal.signal_2"),
				),
			},
		},
	})
}

func CheckNGWAFSignalExists(resourceName, expectedName, expectedDescription string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		client, err := NewFastlyClient()
		if err != nil {
			return err
		}

		signal, err := signals.Get(context.Background(), client, &signals.GetInput{
			SignalID: &rs.Primary.ID,
			Scope:    &scope.Scope{Type: scope.ScopeTypeAccount},
		})
		if err != nil {
			return fmt.Errorf("reading NGWAF account signal %s: %w", rs.Primary.ID, err)
		}

		if signal.Name != expectedName {
			return fmt.Errorf("NGWAF account signal name = %q, want %q", signal.Name, expectedName)
		}
		if signal.Description != expectedDescription {
			return fmt.Errorf("NGWAF account signal description = %q, want %q", signal.Description, expectedDescription)
		}
		if signal.ReferenceID == "" {
			return fmt.Errorf("NGWAF account signal %s has empty reference_id", rs.Primary.ID)
		}
		if scope.Type(signal.Scope.Type) != scope.ScopeTypeAccount {
			return fmt.Errorf("NGWAF signal scope type = %q, want %q", signal.Scope.Type, scope.ScopeTypeAccount)
		}
		if len(signal.Scope.AppliesTo) != 1 || signal.Scope.AppliesTo[0] != "*" {
			return fmt.Errorf("NGWAF account signal applies_to = %v, want [*]", signal.Scope.AppliesTo)
		}

		return nil
	}
}

func CheckNGWAFSignalDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return err
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "fastly_ngwaf_signal" {
			continue
		}

		_, err := signals.Get(context.Background(), client, &signals.GetInput{
			SignalID: &rs.Primary.ID,
			Scope:    &scope.Scope{Type: scope.ScopeTypeAccount},
		})
		if err == nil {
			return fmt.Errorf("NGWAF account signal %s still exists after destroy", rs.Primary.ID)
		}
	}

	return nil
}

func checkNGWAFSignalsDataSourceHasSignal(dataSourceName, resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		signalResource, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		ds, ok := s.RootModule().Resources[dataSourceName]
		if !ok {
			return fmt.Errorf("data source not found: %s", dataSourceName)
		}

		count, err := strconv.Atoi(ds.Primary.Attributes["signals.#"])
		if err != nil {
			return fmt.Errorf("reading %s signals.#: %w", dataSourceName, err)
		}

		for i := range count {
			prefix := fmt.Sprintf("signals.%d.", i)
			if ds.Primary.Attributes[prefix+"id"] != signalResource.Primary.ID {
				continue
			}

			if got, want := ds.Primary.Attributes[prefix+"name"], signalResource.Primary.Attributes["name"]; got != want {
				return fmt.Errorf("signal %s reported name %q, want %q", signalResource.Primary.ID, got, want)
			}
			if got, want := ds.Primary.Attributes[prefix+"description"], signalResource.Primary.Attributes["description"]; got != want {
				return fmt.Errorf("signal %s reported description %q, want %q", signalResource.Primary.ID, got, want)
			}
			if got := ds.Primary.Attributes[prefix+"reference_id"]; got == "" {
				return fmt.Errorf("signal %s reported an empty reference_id", signalResource.Primary.ID)
			}
			if got := ds.Primary.Attributes[prefix+"applies_to.#"]; got != "1" {
				return fmt.Errorf("signal %s reported applies_to count %q, want 1", signalResource.Primary.ID, got)
			}
			return nil
		}

		return fmt.Errorf("signal %s not reported by %s (%d signals listed)", signalResource.Primary.ID, dataSourceName, count)
	}
}

func ConfigNGWAFSignal(blockFile, signalName string) string {
	return RenderBlock("internal/acceptance_tests/blocks/"+blockFile, map[string]string{
		"SIGNAL_NAME": signalName,
	})
}

func ConfigNGWAFSignalsDataSource(signalName1, signalName2 string) string {
	return RenderBlock("internal/acceptance_tests/blocks/ngwaf_signals_data_source.tf", map[string]string{
		"SIGNAL_NAME_1": signalName1,
		"SIGNAL_NAME_2": signalName2,
	})
}
