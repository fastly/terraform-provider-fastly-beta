package acceptancetests

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"

	"github.com/fastly/go-fastly/v19/fastly"
	"github.com/fastly/go-fastly/v19/fastly/products"
	"github.com/fastly/go-fastly/v19/fastly/products/aiaccelerator"
	"github.com/fastly/go-fastly/v19/fastly/products/airuntimecontrol"
	"github.com/fastly/go-fastly/v19/fastly/products/domainresearch"
	"github.com/fastly/go-fastly/v19/fastly/products/objectstorage"
)

type accountProductTestCase struct {
	name      string
	productID string
	enable    func(context.Context, *fastly.Client) (products.EnableOutput, error)
}

var accountProductTestCases = []accountProductTestCase{
	{"ai_accelerator", aiaccelerator.ProductID, aiaccelerator.Enable},
	{"ai_runtime_control", airuntimecontrol.ProductID, airuntimecontrol.Enable},
	{"domain_research", domainresearch.ProductID, domainresearch.Enable},
	{"object_storage", objectstorage.ProductID, objectstorage.Enable},
}

// accountProductTestsRan limits TestMain's re-enable to runs that could have disabled products.
var accountProductTestsRan atomic.Bool

// TestMain re-enables the products after the package run, covering a test that dies before
// its own cleanup runs and would leave the shared account's products disabled.
func TestMain(m *testing.M) {
	code := m.Run()
	if os.Getenv("TF_ACC") != "" && accountProductTestsRan.Load() {
		if client, err := NewFastlyClient(); err != nil {
			fmt.Fprintf(os.Stderr, "skipping final re-enable of account products: %v\n", err)
		} else {
			for _, tc := range accountProductTestCases {
				if _, err := tc.enable(context.Background(), client); err != nil {
					fmt.Fprintf(os.Stderr, "final re-enable of account product %s: %v\n", tc.productID, err)
				}
			}
		}
	}
	os.Exit(code)
}

func ConfigAccountProduct(product string, enabled *bool) string {
	data := map[string]string{"PRODUCT": product}
	if enabled != nil {
		data["ENABLED"] = strconv.FormatBool(*enabled)
	}
	return RenderBlock("internal/acceptance_tests/blocks/account_product.tf", data)
}

// registerAccountProductReEnable re-enables the product after destroy, which disables it
// account-wide on the shared test account.
func registerAccountProductReEnable(t *testing.T, tc accountProductTestCase) {
	t.Helper()
	t.Cleanup(func() {
		if os.Getenv("TF_ACC") == "" {
			return
		}
		client, err := NewFastlyClient()
		if err != nil {
			t.Logf("skipping re-enable of %s: %v", tc.productID, err)
			return
		}
		if _, err := tc.enable(context.Background(), client); err != nil {
			t.Logf("re-enabling account product %s: %v", tc.productID, err)
		}
	})
}

func TestAccFastlyAccountProductEnablement(t *testing.T) {
	for _, tc := range accountProductTestCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			accountProductTestsRan.Store(true)
			registerAccountProductReEnable(t, tc)

			resourceName := fmt.Sprintf("fastly_product_%s.test", tc.name)
			disabled := false
			enabled := true

			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { PreCheck(t) },
				ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
				Steps: []resource.TestStep{
					{
						Config: ConfigAccountProduct(tc.name, nil),
						Check: resource.ComposeTestCheckFunc(
							resource.TestCheckResourceAttr(resourceName, "id", tc.productID),
							resource.TestCheckResourceAttr(resourceName, "enabled", "true"),
						),
					},
					{
						ResourceName:      resourceName,
						ImportState:       true,
						ImportStateId:     tc.productID,
						ImportStateVerify: true,
					},
					{
						Config: ConfigAccountProduct(tc.name, &disabled),
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PreApply: []plancheck.PlanCheck{
								plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
							},
						},
						Check: resource.TestCheckResourceAttr(resourceName, "enabled", "false"),
					},
					{
						Config: ConfigAccountProduct(tc.name, &enabled),
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PreApply: []plancheck.PlanCheck{
								plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
							},
						},
						Check: resource.TestCheckResourceAttr(resourceName, "enabled", "true"),
					},
				},
			})
		})
	}
}
