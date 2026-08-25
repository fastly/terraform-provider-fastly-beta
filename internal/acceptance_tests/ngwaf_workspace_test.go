package acceptancetests

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	ws "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces"
)

func TestAccFastlyNGWAFWorkspace_lifecycle(t *testing.T) {
	t.Parallel()

	suffix := acctest.RandString(10)
	name := fmt.Sprintf("tf-test-workspace-%s", suffix)
	updatedName := fmt.Sprintf("tf-test-workspace-updated-%s", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFWorkspaceDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFWorkspace("ngwaf_workspace_basic.tf", name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "name", name),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "description", "Test NGWAF Workspace"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "mode", "block"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "ip_anonymization", "hashed"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "client_ip_headers.0", "X-Forwarded-For"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "client_ip_headers.1", "X-Real-IP"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "default_blocking_response_code", "429"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "attack_signal_thresholds.0.one_minute", "100"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "attack_signal_thresholds.0.ten_minutes", "500"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "attack_signal_thresholds.0.one_hour", "1000"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "attack_signal_thresholds.0.immediate", "true"),
					resource.TestCheckResourceAttrSet("fastly_ngwaf_workspace.test", "id"),
				),
			},
			{
				Config: ConfigNGWAFWorkspace("ngwaf_workspace_updated.tf", updatedName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "name", updatedName),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "description", "Test NGWAF Workspace Updated"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "mode", "log"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "client_ip_headers.0", "True-Client-IP"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "default_blocking_response_code", "301"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "default_redirect_url", "https://example.com"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "attack_signal_thresholds.0.one_minute", "200"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "attack_signal_thresholds.0.ten_minutes", "1000"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "attack_signal_thresholds.0.one_hour", "2000"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "attack_signal_thresholds.0.immediate", "false"),
				),
			},
			{
				// Omitting most optional fields falls back to the workspace's
				// schema defaults. client_ip_headers is the exception: once
				// set, it cannot be cleared through this resource - the API
				// has no way to distinguish "leave this alone" from an
				// explicit empty list (both an explicit JSON null and an
				// explicit empty array were tried against the live API and
				// either ignored or left unconfirmed), so removing it from
				// configuration leaves the prior step's value
				// ("True-Client-IP") in place. client_ip_headers is Computed
				// for exactly this reason.
				Config: ConfigNGWAFWorkspace("ngwaf_workspace_minimal.tf", updatedName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "mode", "off"),
					resource.TestCheckNoResourceAttr("fastly_ngwaf_workspace.test", "ip_anonymization"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "client_ip_headers.#", "1"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "client_ip_headers.0", "True-Client-IP"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "default_blocking_response_code", "406"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "attack_signal_thresholds.0.one_minute", "50"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "attack_signal_thresholds.0.ten_minutes", "350"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "attack_signal_thresholds.0.one_hour", "1800"),
					resource.TestCheckResourceAttr("fastly_ngwaf_workspace.test", "attack_signal_thresholds.0.immediate", "false"),
				),
			},
			{
				Config:   ConfigNGWAFWorkspace("ngwaf_workspace_minimal.tf", updatedName),
				PlanOnly: true,
			},
			{
				ResourceName:      "fastly_ngwaf_workspace.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccFastlyNGWAFWorkspace_importZeroThresholds validates the specific
// behavior schema.go's DefaultAttackSignal* comment attributes to the legacy
// provider rather than confirming firsthand: that the API returns a bare
// zero value for an attack signal threshold that was never explicitly set,
// and that flattenAttackSignalThresholds (flatten.go) resolves that zero
// back to the documented default instead of surfacing a literal 0.
//
// The resource's own create/update path can't exercise this: because each
// threshold field is Optional+Computed with a static schema Default,
// Terraform Core fills in 50/350/1800/false at plan time before the request
// ever reaches BuildCreateInput/BuildUpdateInput, so the API never actually
// receives or returns a zero from our own writes. The only way a threshold
// is genuinely unset from the API's point of view is a workspace that was
// never touched by this provider — created directly against the API, then
// imported. That matches the legacy provider's own comment, which ties this
// zero-value behavior specifically to "conflicts with imports".
func TestAccFastlyNGWAFWorkspace_importZeroThresholds(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	client, err := NewFastlyClient()
	if err != nil {
		t.Fatal(err)
	}

	name := fmt.Sprintf("tf-test-workspace-import-zero-%s", acctest.RandString(10))

	created, err := ws.Create(context.Background(), client, &ws.CreateInput{
		Name:        &name,
		Description: new("created out-of-band, bypassing Terraform, to leave attack_signal_thresholds unset"),
		Mode:        new("off"),
	})
	if err != nil {
		t.Fatalf("creating out-of-band NGWAF workspace: %s", err)
	}
	workspaceID := created.WorkspaceID
	t.Cleanup(func() {
		if err := ws.Delete(context.Background(), client, &ws.DeleteInput{WorkspaceID: &workspaceID}); err != nil {
			t.Logf("cleanup: deleting out-of-band NGWAF workspace %s: %s", workspaceID, err)
		}
	})

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:            ConfigNGWAFWorkspace("ngwaf_workspace_minimal.tf", name),
				ResourceName:      "fastly_ngwaf_workspace.test",
				ImportState:       true,
				ImportStateId:     workspaceID,
				ImportStateVerify: false,
				ImportStateCheck:  checkThresholdsResolvedToDefaults,
			},
		},
	})
}

func checkThresholdsResolvedToDefaults(states []*terraform.InstanceState) error {
	if len(states) != 1 {
		return fmt.Errorf("expected exactly one imported state, got %d", len(states))
	}

	attrs := states[0].Attributes
	want := map[string]string{
		"attack_signal_thresholds.#":             "1",
		"attack_signal_thresholds.0.one_minute":  "50",
		"attack_signal_thresholds.0.ten_minutes": "350",
		"attack_signal_thresholds.0.one_hour":    "1800",
		"attack_signal_thresholds.0.immediate":   "false",
	}
	for attr, expected := range want {
		if got := attrs[attr]; got != expected {
			return fmt.Errorf("imported state attribute %s = %q, want %q (the API's zero value for an unset threshold was not resolved to its documented default)", attr, got, expected)
		}
	}
	return nil
}

func ConfigNGWAFWorkspace(blockFile, name string) string {
	raw, err := os.ReadFile("blocks/" + blockFile)
	if err != nil {
		panic(err)
	}
	return strings.ReplaceAll(string(raw), "{{.WORKSPACE_NAME}}", name)
}

func TestAccFastlyDataSourceNGWAFWorkspaces(t *testing.T) {
	t.Parallel()
	h := acctest.RandString(10)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFWorkspaceDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFWorkspacesDataSource(h),
				Check: resource.ComposeTestCheckFunc(
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["data.fastly_ngwaf_workspaces.example"]
						if !ok {
							return fmt.Errorf("not found: data.fastly_ngwaf_workspaces.example")
						}

						want := []string{
							fmt.Sprintf("tf_%s_1", h),
							fmt.Sprintf("tf_%s_2", h),
							fmt.Sprintf("tf_%s_3", h),
						}

						var found int
						var got []string
						for k, v := range rs.Primary.Attributes {
							if strings.HasSuffix(k, ".name") {
								got = append(got, v)
								if slices.Contains(want, v) {
									found++
								}
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

func CheckNGWAFWorkspaceDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return err
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "fastly_ngwaf_workspace" {
			continue
		}

		if _, err := ws.Get(context.Background(), client, &ws.GetInput{WorkspaceID: &rs.Primary.ID}); err == nil {
			return fmt.Errorf("NGWAF workspace %s still exists after destroy", rs.Primary.ID)
		}
	}
	return nil
}
