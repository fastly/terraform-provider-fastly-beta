package acceptancetests

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/scope"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/signals"
)

func TestAccFastlyNGWAFWorkspaceSignal_lifecycle(t *testing.T) {
	t.Parallel()

	suffix := acctest.RandString(10)
	workspaceName := fmt.Sprintf("tf-test-workspace-signal-%s", suffix)
	signalName := fmt.Sprintf("Signal Test %s", suffix)
	signalDescription := fmt.Sprintf("Terraform Signal Test %s", suffix)
	updatedSignalDescription := signalDescription + " updated"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFWorkspaceSignalAndWorkspaceDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFWorkspaceSignal(workspaceName, signalName, signalDescription),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "name", workspaceName),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_signal.test", "name", signalName),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_signal.test", "description", signalDescription),
					resource.TestCheckResourceAttrSet("fastly_ngwaf_workspace_signal.test", "id"),
					resource.TestCheckResourceAttrSet("fastly_ngwaf_workspace_signal.test", "reference_id"),
					CheckNGWAFWorkspaceSignalExists("fastly_ngwaf_workspace_signal.test", signalName, signalDescription),
				),
			},
			{
				Config: ConfigNGWAFWorkspaceSignalUpdated(workspaceName, signalName, updatedSignalDescription),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_signal.test", "name", signalName),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_signal.test", "description", updatedSignalDescription),
					CheckNGWAFWorkspaceSignalExists("fastly_ngwaf_workspace_signal.test", signalName, updatedSignalDescription),
				),
			},
			{
				Config:   ConfigNGWAFWorkspaceSignalUpdated(workspaceName, signalName, updatedSignalDescription),
				PlanOnly: true,
			},
			{
				ResourceName:      "fastly_ngwaf_workspace_signal.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					signal := s.RootModule().Resources["fastly_ngwaf_workspace_signal.test"]
					workspace := s.RootModule().Resources["fastly_ngwaf_workspace.test"]
					return fmt.Sprintf("%s/%s", workspace.Primary.ID, signal.Primary.ID), nil
				},
			},
		},
	})
}

func TestAccFastlyDataSourceNGWAFWorkspaceSignals(t *testing.T) {
	t.Parallel()

	suffix := acctest.RandString(10)
	workspaceName := fmt.Sprintf("tf-test-workspace-signal-ds-%s", suffix)
	signalName1 := fmt.Sprintf("Signal One %s", suffix)
	signalName2 := fmt.Sprintf("Signal Two %s", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFWorkspaceSignalAndWorkspaceDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFWorkspaceSignalsDataSource(workspaceName, signalName1, signalName2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.fastly_ngwaf_workspace_signals.test", "workspace_id",
						"fastly_ngwaf_workspace.test", "id",
					),
					resource.TestCheckResourceAttr("data.fastly_ngwaf_workspace_signals.test", "signals.#", "2"),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["data.fastly_ngwaf_workspace_signals.test"]
						if !ok {
							return fmt.Errorf("not found: data.fastly_ngwaf_workspace_signals.test")
						}

						want := []string{signalName1, signalName2}

						var found int
						var got []string
						for k, v := range rs.Primary.Attributes {
							if strings.HasSuffix(k, ".name") {
								got = append(got, v)
								if slices.Contains(want, v) {
									found++
								}
							}
							if strings.HasSuffix(k, ".reference_id") && v == "" {
								return fmt.Errorf("expected data source signal %s to have reference_id", k)
							}
							if strings.HasSuffix(k, ".description") && v == "" {
								return fmt.Errorf("expected data source signal %s to have description", k)
							}
						}

						if found != len(want) {
							return fmt.Errorf("want: %v, got: %v", want, got)
						}

						return nil
					},
				),
			},
		},
	})
}

func CheckNGWAFWorkspaceSignalExists(resourceName, expectedName, expectedDescription string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		workspaceID := rs.Primary.Attributes["workspace_id"]
		if workspaceID == "" {
			return fmt.Errorf("workspace_id is not set for %s", resourceName)
		}

		client, err := NewFastlyClient()
		if err != nil {
			return err
		}

		signal, err := signals.Get(context.Background(), client, &signals.GetInput{
			SignalID: &rs.Primary.ID,
			Scope: &scope.Scope{
				Type:      scope.ScopeTypeWorkspace,
				AppliesTo: []string{workspaceID},
			},
		})
		if err != nil {
			return fmt.Errorf("error reading NGWAF workspace signal %s: %w", rs.Primary.ID, err)
		}

		if signal.Name != expectedName {
			return fmt.Errorf("NGWAF workspace signal name = %q, want %q", signal.Name, expectedName)
		}
		if signal.Description != expectedDescription {
			return fmt.Errorf("NGWAF workspace signal description = %q, want %q", signal.Description, expectedDescription)
		}
		if signal.ReferenceID == "" {
			return fmt.Errorf("NGWAF workspace signal %s has empty reference_id", rs.Primary.ID)
		}

		return nil
	}
}

func CheckNGWAFWorkspaceSignalDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return err
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "fastly_ngwaf_workspace_signal" {
			continue
		}

		workspaceID := rs.Primary.Attributes["workspace_id"]
		if workspaceID == "" {
			continue
		}

		_, err := signals.Get(context.Background(), client, &signals.GetInput{
			SignalID: &rs.Primary.ID,
			Scope: &scope.Scope{
				Type:      scope.ScopeTypeWorkspace,
				AppliesTo: []string{workspaceID},
			},
		})
		if err == nil {
			return fmt.Errorf("NGWAF workspace signal %s still exists after destroy", rs.Primary.ID)
		}
	}

	return nil
}

func CheckNGWAFWorkspaceSignalAndWorkspaceDestroy(s *terraform.State) error {
	if err := CheckNGWAFWorkspaceSignalDestroy(s); err != nil {
		return err
	}
	return CheckNGWAFWorkspaceDestroy(s)
}
