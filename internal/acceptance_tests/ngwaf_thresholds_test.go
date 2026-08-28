package acceptancetests

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/go-fastly/v17/fastly"
	ws "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces"
	th "github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/workspaces/thresholds"
)

func TestAccFastlyNGWAFThresholds_lifecycle(t *testing.T) {
	t.Parallel()

	suffix := acctest.RandString(10)
	workspaceName := fmt.Sprintf("tf-test-workspace-%s", suffix)
	thresholdName := fmt.Sprintf("tf-test-threshold-%s", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFThresholdsDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFThresholds("ngwaf_thresholds_block.tf", workspaceName, thresholdName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_thresholds.test", "action", "block"),
					resource.TestCheckResourceAttr("fastly_ngwaf_thresholds.test", "dont_notify", "false"),
					resource.TestCheckResourceAttr("fastly_ngwaf_thresholds.test", "duration", "86400"),
					resource.TestCheckResourceAttr("fastly_ngwaf_thresholds.test", "enabled", "true"),
					resource.TestCheckResourceAttr("fastly_ngwaf_thresholds.test", "interval", "3600"),
					resource.TestCheckResourceAttr("fastly_ngwaf_thresholds.test", "limit", "10"),
					resource.TestCheckResourceAttr("fastly_ngwaf_thresholds.test", "name", thresholdName),
					resource.TestCheckResourceAttr("fastly_ngwaf_thresholds.test", "signal", "SQLI"),
					resource.TestCheckResourceAttrPair("fastly_ngwaf_thresholds.test", "workspace_id", "fastly_ngwaf_workspace.test", "id"),
					resource.TestCheckResourceAttrSet("fastly_ngwaf_thresholds.test", "id"),
					CheckNGWAFThresholdsExists("fastly_ngwaf_thresholds.test"),
				),
			},
			{
				// Omitting duration/interval/limit falls back to the
				// resource's schema defaults, which must match the API's
				// own documented defaults for these fields.
				Config: ConfigNGWAFThresholds("ngwaf_thresholds_minimal.tf", workspaceName, thresholdName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_thresholds.test", "duration", "86400"),
					resource.TestCheckResourceAttr("fastly_ngwaf_thresholds.test", "interval", "3600"),
					resource.TestCheckResourceAttr("fastly_ngwaf_thresholds.test", "limit", "10"),
					CheckNGWAFThresholdsExists("fastly_ngwaf_thresholds.test"),
				),
			},
			{
				Config:   ConfigNGWAFThresholds("ngwaf_thresholds_minimal.tf", workspaceName, thresholdName),
				PlanOnly: true,
			},
			{
				Config: ConfigNGWAFThresholds("ngwaf_thresholds_log.tf", workspaceName, thresholdName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_thresholds.test", "action", "log"),
					resource.TestCheckResourceAttr("fastly_ngwaf_thresholds.test", "dont_notify", "true"),
					resource.TestCheckResourceAttr("fastly_ngwaf_thresholds.test", "duration", "43200"),
					resource.TestCheckResourceAttr("fastly_ngwaf_thresholds.test", "enabled", "false"),
					resource.TestCheckResourceAttr("fastly_ngwaf_thresholds.test", "interval", "600"),
					resource.TestCheckResourceAttr("fastly_ngwaf_thresholds.test", "limit", "50"),
					resource.TestCheckResourceAttr("fastly_ngwaf_thresholds.test", "name", thresholdName),
					resource.TestCheckResourceAttr("fastly_ngwaf_thresholds.test", "signal", "BHH"),
					CheckNGWAFThresholdsExists("fastly_ngwaf_thresholds.test"),
				),
			},
			{
				Config: ConfigNGWAFThresholds("ngwaf_thresholds_block_immediately.tf", workspaceName, thresholdName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_ngwaf_thresholds.test", "action", "block_immediately"),
					resource.TestCheckResourceAttr("fastly_ngwaf_thresholds.test", "dont_notify", "true"),
					resource.TestCheckResourceAttr("fastly_ngwaf_thresholds.test", "duration", "43200"),
					resource.TestCheckResourceAttr("fastly_ngwaf_thresholds.test", "enabled", "false"),
					resource.TestCheckResourceAttr("fastly_ngwaf_thresholds.test", "interval", "600"),
					resource.TestCheckResourceAttr("fastly_ngwaf_thresholds.test", "limit", "50"),
					resource.TestCheckResourceAttr("fastly_ngwaf_thresholds.test", "name", thresholdName),
					resource.TestCheckResourceAttr("fastly_ngwaf_thresholds.test", "signal", "XXE"),
					CheckNGWAFThresholdsExists("fastly_ngwaf_thresholds.test"),
				),
			},
			{
				Config:   ConfigNGWAFThresholds("ngwaf_thresholds_block_immediately.tf", workspaceName, thresholdName),
				PlanOnly: true,
			},
			{
				ResourceName:      "fastly_ngwaf_thresholds.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: ImportStateIDForNGWAFThresholds("fastly_ngwaf_thresholds.test"),
			},
		},
	})
}

// TestAccFastlyNGWAFThresholds_importZeroValues validates the specific
// behavior schema.go's Default* comments attribute to omitted
// duration/interval/limit: the API can return a bare zero for a
// block_immediately threshold that never set them (the spec explicitly
// allows this - limit's own minimum is lowered to 0 for that variant), and
// FlattenToModel resolves that zero back to the documented default instead
// of surfacing a literal 0 that would fail this schema's own validators.
//
// This resource's own create/update path can't exercise this: because
// duration/interval/limit are Optional+Computed with a static schema
// Default, Terraform Core fills in 86400/3600/10 at plan time before the
// request ever reaches BuildCreateInput, so the API never actually
// receives or returns a zero from our own writes. go-fastly's typed
// thresholds.Create also can't produce this state - it requires Limit and
// Interval to be non-nil regardless of action - so the out-of-band
// threshold is created with a raw POST that mimics the spec's
// ThresholdCreateImmediate body (action/enabled/signal only).
func TestAccFastlyNGWAFThresholds_importZeroValues(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	client, err := NewFastlyClient()
	if err != nil {
		t.Fatal(err)
	}

	suffix := acctest.RandString(10)
	workspaceName := fmt.Sprintf("tf-test-workspace-import-zero-%s", suffix)
	thresholdName := fmt.Sprintf("tf-test-threshold-%s", suffix)

	workspace, err := ws.Create(context.Background(), client, &ws.CreateInput{
		Name:        &workspaceName,
		Description: new("created out-of-band, bypassing Terraform, to leave duration/interval/limit unset"),
		Mode:        new("block"),
	})
	if err != nil {
		t.Fatalf("creating out-of-band NGWAF workspace: %s", err)
	}
	workspaceID := workspace.WorkspaceID
	t.Cleanup(func() {
		if err := ws.Delete(context.Background(), client, &ws.DeleteInput{WorkspaceID: &workspaceID}); err != nil {
			t.Logf("cleanup: deleting out-of-band NGWAF workspace %s: %s", workspaceID, err)
		}
	})

	type rawImmediateThreshold struct {
		Action  string `json:"action"`
		Enabled bool   `json:"enabled"`
		Signal  string `json:"signal"`
	}

	resp, err := client.PostJSON(
		context.Background(),
		fastly.ToSafeURL("ngwaf", "v1", "workspaces", workspaceID, "thresholds"),
		rawImmediateThreshold{Action: "block_immediately", Enabled: true, Signal: "XXE"},
		fastly.CreateRequestOptions(),
	)
	if err != nil {
		t.Fatalf("creating out-of-band NGWAF threshold: %s", err)
	}
	var created th.Threshold
	decodeErr := json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	if decodeErr != nil {
		t.Fatalf("decoding out-of-band NGWAF threshold response: %s", decodeErr)
	}
	thresholdID := created.ThresholdID

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:            ConfigNGWAFThresholds("ngwaf_thresholds_block.tf", workspaceName, thresholdName),
				ResourceName:      "fastly_ngwaf_thresholds.test",
				ImportState:       true,
				ImportStateId:     fmt.Sprintf("%s/%s", workspaceID, thresholdID),
				ImportStateVerify: false,
				ImportStateCheck:  checkThresholdZeroValuesResolvedToDefaults,
			},
		},
	})
}

func checkThresholdZeroValuesResolvedToDefaults(states []*terraform.InstanceState) error {
	if len(states) != 1 {
		return fmt.Errorf("expected exactly one imported state, got %d", len(states))
	}

	attrs := states[0].Attributes
	want := map[string]string{
		"duration": "86400",
		"interval": "3600",
		"limit":    "10",
	}
	for attr, expected := range want {
		if got := attrs[attr]; got != expected {
			return fmt.Errorf("imported state attribute %s = %q, want %q (the API's zero value for an unset field was not resolved to its documented default)", attr, got, expected)
		}
	}
	return nil
}

func TestAccFastlyDataSourceNGWAFThresholds(t *testing.T) {
	t.Parallel()

	suffix := acctest.RandString(10)
	workspaceName := fmt.Sprintf("tf-test-workspace-%s", suffix)
	thresholdName := fmt.Sprintf("tf-test-threshold-%s", suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckNGWAFThresholdsDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigNGWAFThresholds("ngwaf_thresholds_datasource.tf", workspaceName, thresholdName),
				Check: resource.ComposeTestCheckFunc(
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["data.fastly_ngwaf_thresholds.test"]
						if !ok {
							return fmt.Errorf("not found: data.fastly_ngwaf_thresholds.test")
						}

						want := []string{thresholdName + "_1", thresholdName + "_2"}

						var found int
						for k, v := range rs.Primary.Attributes {
							if strings.HasSuffix(k, ".name") {
								for _, w := range want {
									if v == w {
										found++
									}
								}
							}
						}

						if found != len(want) {
							return fmt.Errorf("want threshold names %v to appear in data source, got attributes %v", want, rs.Primary.Attributes)
						}

						return nil
					},
				),
			},
		},
	})
}

func ImportStateIDForNGWAFThresholds(n string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return "", fmt.Errorf("not found: %s", n)
		}
		return fmt.Sprintf("%s/%s", rs.Primary.Attributes["workspace_id"], rs.Primary.ID), nil
	}
}

func CheckNGWAFThresholdsExists(n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("not found: %s", n)
		}

		client, err := NewFastlyClient()
		if err != nil {
			return err
		}

		workspaceID := rs.Primary.Attributes["workspace_id"]
		threshold, err := th.Get(context.Background(), client, &th.GetInput{
			WorkspaceID: &workspaceID,
			ThresholdID: &rs.Primary.ID,
		})
		if err != nil {
			return fmt.Errorf("unable to retrieve NGWAF threshold %s: %w", rs.Primary.ID, err)
		}
		if threshold == nil {
			return fmt.Errorf("NGWAF threshold %s not found in API", rs.Primary.ID)
		}

		return nil
	}
}

func CheckNGWAFThresholdsDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return err
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "fastly_ngwaf_thresholds" {
			continue
		}

		workspaceID := rs.Primary.Attributes["workspace_id"]
		if _, err := th.Get(context.Background(), client, &th.GetInput{WorkspaceID: &workspaceID, ThresholdID: &rs.Primary.ID}); err == nil {
			return fmt.Errorf("NGWAF threshold %s still exists after destroy", rs.Primary.ID)
		}
	}
	return nil
}

func ConfigNGWAFThresholds(blockFile, workspaceName, thresholdName string) string {
	raw, err := os.ReadFile("blocks/" + blockFile)
	if err != nil {
		panic(err)
	}
	replaced := strings.ReplaceAll(string(raw), "{{.WORKSPACE_NAME}}", workspaceName)
	return strings.ReplaceAll(replaced, "{{.THRESHOLD_NAME}}", thresholdName)
}
