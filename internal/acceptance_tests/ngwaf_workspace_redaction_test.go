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

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/redactions"
)

func TestAccFastlyNGWAFWorkspaceRedaction_lifecycle(t *testing.T) {
	t.Parallel()

	suffix := acctest.RandString(10)
	workspaceName := fmt.Sprintf("tf-test-workspace-redaction-%s", suffix)
	redactionField := fmt.Sprintf("tf-test-redaction-field-%s", suffix)
	updatedRedactionField := redactionField + "-updated"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFWorkspaceRedactionAndWorkspaceDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFWorkspaceRedaction(workspaceName, redactionField),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "name", workspaceName),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_redaction.test", "field", redactionField),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_redaction.test", "type", "request_parameter"),
					resource.TestCheckResourceAttrSet("fastly_ngwaf_workspace_redaction.test", "id"),
					CheckNGWAFWorkspaceRedactionExists("fastly_ngwaf_workspace_redaction.test", redactionField, "request_parameter"),
				),
			},
			{
				Config: ConfigNGWAFWorkspaceRedactionUpdated(workspaceName, updatedRedactionField),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_redaction.test", "field", updatedRedactionField),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_redaction.test", "type", "request_header"),
					CheckNGWAFWorkspaceRedactionExists("fastly_ngwaf_workspace_redaction.test", updatedRedactionField, "request_header"),
				),
			},
			{
				Config:   ConfigNGWAFWorkspaceRedactionUpdated(workspaceName, updatedRedactionField),
				PlanOnly: true,
			},
			{
				ResourceName:      "fastly_ngwaf_workspace_redaction.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					redaction := s.RootModule().Resources["fastly_ngwaf_workspace_redaction.test"]
					workspace := s.RootModule().Resources["fastly_ngwaf_workspace.test"]
					return fmt.Sprintf("%s/%s", workspace.Primary.ID, redaction.Primary.ID), nil
				},
			},
		},
	})
}

func TestAccFastlyDataSourceNGWAFWorkspaceRedactions(t *testing.T) {
	t.Parallel()

	suffix := acctest.RandString(10)
	workspaceName := fmt.Sprintf("tf-test-workspace-redaction-ds-%s", suffix)
	redactionField1 := fmt.Sprintf("tf-test-redaction-field-one-%s", suffix)
	redactionField2 := fmt.Sprintf("tf-test-redaction-field-two-%s", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFWorkspaceRedactionAndWorkspaceDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFWorkspaceRedactionsDataSource(workspaceName, redactionField1, redactionField2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"data.fastly_ngwaf_workspace_redactions.test", "workspace_id",
						"fastly_ngwaf_workspace.test", "id",
					),
					resource.TestCheckResourceAttr("data.fastly_ngwaf_workspace_redactions.test", "redactions.#", "2"),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["data.fastly_ngwaf_workspace_redactions.test"]
						if !ok {
							return fmt.Errorf("not found: data.fastly_ngwaf_workspace_redactions.test")
						}

						want := []string{redactionField1, redactionField2}

						var found int
						var got []string
						for k, v := range rs.Primary.Attributes {
							if strings.HasSuffix(k, ".field") {
								got = append(got, v)
								if slices.Contains(want, v) {
									found++
								}
							}
							if strings.HasSuffix(k, ".type") && v == "" {
								return fmt.Errorf("expected data source redaction %s to have type", k)
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

func CheckNGWAFWorkspaceRedactionExists(resourceName, expectedField, expectedType string) resource.TestCheckFunc {
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

		redaction, err := redactions.Get(context.Background(), client, &redactions.GetInput{
			RedactionID: &rs.Primary.ID,
			WorkspaceID: &workspaceID,
		})
		if err != nil {
			return fmt.Errorf("error reading NGWAF workspace redaction %s: %w", rs.Primary.ID, err)
		}

		if redaction.Field != expectedField {
			return fmt.Errorf("NGWAF workspace redaction field = %q, want %q", redaction.Field, expectedField)
		}
		if redaction.Type != expectedType {
			return fmt.Errorf("NGWAF workspace redaction type = %q, want %q", redaction.Type, expectedType)
		}

		return nil
	}
}

func CheckNGWAFWorkspaceRedactionDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return err
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "fastly_ngwaf_workspace_redaction" {
			continue
		}

		workspaceID := rs.Primary.Attributes["workspace_id"]
		if workspaceID == "" {
			continue
		}

		_, err := redactions.Get(context.Background(), client, &redactions.GetInput{
			RedactionID: &rs.Primary.ID,
			WorkspaceID: &workspaceID,
		})
		if err == nil {
			return fmt.Errorf("NGWAF workspace redaction %s still exists after destroy", rs.Primary.ID)
		}
	}

	return nil
}

func CheckNGWAFWorkspaceRedactionAndWorkspaceDestroy(s *terraform.State) error {
	if err := CheckNGWAFWorkspaceRedactionDestroy(s); err != nil {
		return err
	}
	return CheckNGWAFWorkspaceDestroy(s)
}
