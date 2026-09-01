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

	vp "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/virtualpatches"
)

const testVirtualPatchID = "CVE-2017-5638"

func TestAccFastlyNGWAFWorkspaceVirtualPatch_lifecycle(t *testing.T) {
	t.Parallel()

	suffix := acctest.RandString(10)
	workspaceName := fmt.Sprintf("tf-test-workspace-vp-%s", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFWorkspaceVirtualPatchAndWorkspaceDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFWorkspaceVirtualPatch("ngwaf_workspace_virtual_patch_basic.tf", workspaceName, testVirtualPatchID),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_virtual_patch.test", "mode", "block"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_virtual_patch.test", "enabled", "true"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_virtual_patch.test", "virtual_patch_id", testVirtualPatchID),
					resource.TestCheckResourceAttrPair("fastly_ngwaf_workspace_virtual_patch.test", "workspace_id", "fastly_ngwaf_workspace.test", "id"),
					resource.TestCheckResourceAttrSet("fastly_ngwaf_workspace_virtual_patch.test", "id"),
					resource.TestCheckResourceAttrSet("fastly_ngwaf_workspace_virtual_patch.test", "description"),
					CheckNGWAFWorkspaceVirtualPatchExists("fastly_ngwaf_workspace_virtual_patch.test", "block", true),
				),
			},
			{
				Config: ConfigNGWAFWorkspaceVirtualPatch("ngwaf_workspace_virtual_patch_updated.tf", workspaceName, testVirtualPatchID),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_virtual_patch.test", "mode", "log"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_virtual_patch.test", "enabled", "false"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace_virtual_patch.test", "virtual_patch_id", testVirtualPatchID),
					CheckNGWAFWorkspaceVirtualPatchExists("fastly_ngwaf_workspace_virtual_patch.test", "log", false),
				),
			},
			{
				Config:   ConfigNGWAFWorkspaceVirtualPatch("ngwaf_workspace_virtual_patch_updated.tf", workspaceName, testVirtualPatchID),
				PlanOnly: true,
			},
			{
				ResourceName:      "fastly_ngwaf_workspace_virtual_patch.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: ImportStateIDForNGWAFWorkspaceVirtualPatch("fastly_ngwaf_workspace_virtual_patch.test"),
			},
		},
	})
}

func TestAccFastlyDataSourceNGWAFWorkspaceVirtualPatches(t *testing.T) {
	t.Parallel()

	suffix := acctest.RandString(10)
	workspaceName := fmt.Sprintf("tf-test-workspace-vps-ds-%s", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFWorkspaceDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFWorkspaceVirtualPatch("ngwaf_workspace_virtual_patches_datasource.tf", workspaceName, testVirtualPatchID),
				Check: resource.ComposeTestCheckFunc(
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["data.fastly_ngwaf_workspace_virtual_patches.test"]
						if !ok {
							return fmt.Errorf("not found: data.fastly_ngwaf_workspace_virtual_patches.test")
						}

						want := []string{"CVE-2017-5638", "CVE-2019-0193", "CVE-2021-44228"}

						var found int
						var got []string
						for k, v := range rs.Primary.Attributes {
							if strings.HasSuffix(k, ".id") {
								got = append(got, v)
								if slices.Contains(want, v) {
									found++
								}
							}
						}

						if found != len(want) {
							return fmt.Errorf("want virtual patch IDs %v to appear in data source, got %v", want, got)
						}

						return nil
					},
				),
			},
		},
	})
}

func ImportStateIDForNGWAFWorkspaceVirtualPatch(n string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return "", fmt.Errorf("not found: %s", n)
		}
		return fmt.Sprintf("%s/%s", rs.Primary.Attributes["workspace_id"], rs.Primary.Attributes["virtual_patch_id"]), nil
	}
}

func CheckNGWAFWorkspaceVirtualPatchExists(resourceName, expectedMode string, expectedEnabled bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		client, err := NewFastlyClient()
		if err != nil {
			return err
		}

		workspaceID := rs.Primary.Attributes["workspace_id"]
		virtualPatchID := rs.Primary.Attributes["virtual_patch_id"]

		virtualPatch, err := vp.Get(context.Background(), client, &vp.GetInput{
			WorkspaceID:    &workspaceID,
			VirtualPatchID: &virtualPatchID,
		})
		if err != nil {
			return fmt.Errorf("unable to retrieve NGWAF virtual patch %s: %w", virtualPatchID, err)
		}
		if virtualPatch == nil {
			return fmt.Errorf("NGWAF virtual patch %s not found in API", virtualPatchID)
		}
		if virtualPatch.Mode != expectedMode {
			return fmt.Errorf("NGWAF virtual patch mode = %q, want %q", virtualPatch.Mode, expectedMode)
		}
		if virtualPatch.Enabled != expectedEnabled {
			return fmt.Errorf("NGWAF virtual patch enabled = %t, want %t", virtualPatch.Enabled, expectedEnabled)
		}

		return nil
	}
}

func CheckNGWAFWorkspaceVirtualPatchDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return err
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "fastly_ngwaf_workspace_virtual_patch" {
			continue
		}

		workspaceID := rs.Primary.Attributes["workspace_id"]
		virtualPatchID := rs.Primary.Attributes["virtual_patch_id"]
		if virtualPatchID == "" {
			virtualPatchID = rs.Primary.ID
		}

		virtualPatch, err := vp.Get(context.Background(), client, &vp.GetInput{
			WorkspaceID:    &workspaceID,
			VirtualPatchID: &virtualPatchID,
		})
		if err != nil {
			continue
		}
		if virtualPatch != nil && virtualPatch.Enabled {
			return fmt.Errorf("NGWAF virtual patch %s is still enabled after destroy", virtualPatchID)
		}
	}

	return nil
}

func CheckNGWAFWorkspaceVirtualPatchAndWorkspaceDestroy(s *terraform.State) error {
	if err := CheckNGWAFWorkspaceVirtualPatchDestroy(s); err != nil {
		return err
	}
	return CheckNGWAFWorkspaceDestroy(s)
}

func ConfigNGWAFWorkspaceVirtualPatch(blockFile, workspaceName, virtualPatchID string) string {
	return RenderBlock("internal/acceptance_tests/blocks/"+blockFile, map[string]string{
		"WORKSPACE_NAME":   workspaceName,
		"VIRTUAL_PATCH_ID": virtualPatchID,
	})
}
