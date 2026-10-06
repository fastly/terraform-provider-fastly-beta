package acceptancetests

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/go-fastly/v17/fastly"
)

func TestAccFastlyServiceCDNAuto_withHeader(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	headerName := fmt.Sprintf("header-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCDNAutoBasic(serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "header.#", "0"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "1"),
				),
			},
			{
				Config: ConfigCDNAutoWithHeader(serviceName, domainName, headerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "header.#", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "header.0.name", headerName),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "header.0.action", "delete"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "header.0.type", "cache"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "header.0.destination", "http.x-amz-request-id"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "header.0.priority", "100"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "header.0.ignore_if_set", "false"),
					// Adding a header should create and activate version 2
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "2"),
				),
			},
			{
				Config: ConfigCDNAutoBasic(serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "header.#", "0"),
					// Removing the header should create and activate version 3
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "3"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "3"),
				),
			},
		},
	})
}

func TestAccFastlyServiceCDNAuto_withHeaderUpdate(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	headerName := fmt.Sprintf("header-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCDNAutoWithHeader(serviceName, domainName, headerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "header.#", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "header.0.action", "delete"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "header.0.type", "cache"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "1"),
				),
			},
			{
				// Same name, in-place update of action/type/destination/source/priority/
				// ignore_if_set - confirm update, not delete+recreate.
				Config: ConfigCDNAutoWithHeaderUpdated(serviceName, domainName, headerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "header.#", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "header.0.name", headerName),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "header.0.action", "set"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "header.0.type", "request"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "header.0.destination", "http.X-Custom"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "header.0.source", "req.http.Host"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "header.0.priority", "10"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "header.0.ignore_if_set", "true"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "2"),
				),
			},
		},
	})
}

// TestAccFastlyServiceCDNAuto_withHeaderRequestCondition confirms that a header's
// request_condition can reference a real nested REQUEST-type condition block within the same
// apply, verifying conditions are reconciled before header so the reference resolves within the
// same service version (see servicecdnauto's Create/Update, which reconcile header after ACL and
// after condition for this reason).
func TestAccFastlyServiceCDNAuto_withHeaderRequestCondition(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	headerName := fmt.Sprintf("header-%s", acctest.RandString(10))
	conditionName := fmt.Sprintf("condition-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCDNAutoWithHeaderRequestCondition(serviceName, domainName, headerName, conditionName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "condition.0.name", conditionName),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "header.0.name", headerName),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "header.0.request_condition", conditionName),
				),
			},
			{
				// Removing both the header and the condition it references in the same apply
				// must not fail - see the analogous backendWithRequestCondition test's comment.
				Config: ConfigCDNAutoBasic(serviceName, domainName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "condition.#", "0"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "header.#", "0"),
				),
			},
		},
	})
}

func TestAccFastlyServiceCDNAuto_headerInvalidAction(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	headerName := fmt.Sprintf("header-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigCDNAutoWithHeaderInvalidAction(serviceName, domainName, headerName),
				ExpectError: regexp.MustCompile(`Attribute header\[0\]\.action value must be one of`),
			},
		},
	})
}

func TestAccFastlyServiceCDNAuto_importWithHeader(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	headerName := fmt.Sprintf("header-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCDNAutoWithHeader(serviceName, domainName, headerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "header.#", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "header.0.name", headerName),
				),
			},
			{
				ResourceName:            "fastly_service_cdn_auto.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"force_destroy", "reuse"},
			},
		},
	})
}

// --- fastly_service_header (explicit version management) ---

func TestAccFastlyServiceHeader_basic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	headerName := fmt.Sprintf("header-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigHeaderBasic(serviceName, domainName, headerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_header.test", "name", headerName),
					resource.TestCheckResourceAttr("fastly_service_header.test", "action", "delete"),
					resource.TestCheckResourceAttr("fastly_service_header.test", "type", "cache"),
					resource.TestCheckResourceAttr("fastly_service_header.test", "destination", "http.aws-id"),
					resource.TestCheckResourceAttr("fastly_service_header.test", "ignore_if_set", "false"),
					resource.TestCheckResourceAttr("fastly_service_header.test", "priority", "100"),
					resource.TestCheckResourceAttr("fastly_service_header.test", "version", "1"),
					resource.TestCheckResourceAttrSet("fastly_service_header.test", "service_id"),
					resource.TestCheckResourceAttrSet("fastly_service_header.test", "id"),
				),
			},
			{
				Config:   ConfigHeaderBasic(serviceName, domainName, headerName),
				PlanOnly: true,
			},
		},
	})
}

func TestAccFastlyServiceHeader_update(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	headerName := fmt.Sprintf("header-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigHeaderBasic(serviceName, domainName, headerName),
				Check:  resource.TestCheckResourceAttr("fastly_service_header.test", "action", "delete"),
			},
			{
				Config: ConfigHeaderUpdated(serviceName, domainName, headerName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("fastly_service_header.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_header.test", "name", headerName),
					resource.TestCheckResourceAttr("fastly_service_header.test", "action", "set"),
					resource.TestCheckResourceAttr("fastly_service_header.test", "type", "request"),
					resource.TestCheckResourceAttr("fastly_service_header.test", "destination", "http.X-Custom"),
					resource.TestCheckResourceAttr("fastly_service_header.test", "ignore_if_set", "true"),
					resource.TestCheckResourceAttr("fastly_service_header.test", "priority", "10"),
					resource.TestCheckResourceAttr("fastly_service_header.test", "source", "http.server-name"),
				),
			},
			{
				Config:   ConfigHeaderUpdated(serviceName, domainName, headerName),
				PlanOnly: true,
			},
		},
	})
}

// TestAccFastlyServiceHeader_regexAction covers the regex/substitution fields, which only apply
// to the regex and regex_repeat actions - distinct from ignore_if_set, which only applies to set
// (see TestAccFastlyServiceHeader_update).
func TestAccFastlyServiceHeader_regexAction(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	headerName := fmt.Sprintf("header-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigHeaderRegex(serviceName, domainName, headerName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_header.test", "name", headerName),
					resource.TestCheckResourceAttr("fastly_service_header.test", "action", "regex"),
					resource.TestCheckResourceAttr("fastly_service_header.test", "type", "request"),
					resource.TestCheckResourceAttr("fastly_service_header.test", "destination", "http.X-Custom"),
					resource.TestCheckResourceAttr("fastly_service_header.test", "priority", "10"),
					resource.TestCheckResourceAttr("fastly_service_header.test", "regex", "^foo"),
					resource.TestCheckResourceAttr("fastly_service_header.test", "substitution", "bar"),
				),
			},
		},
	})
}

func TestAccFastlyServiceHeader_withCacheCondition(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	headerName := fmt.Sprintf("header-%s", acctest.RandString(10))
	conditionName := fmt.Sprintf("condition-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigHeaderWithCacheCondition(serviceName, domainName, headerName, conditionName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_header.test", "cache_condition", conditionName),
					resource.TestCheckResourceAttr("fastly_service_condition.cache", "type", "CACHE"),
				),
			},
		},
	})
}

// TestAccFastlyServiceHeader_computeServiceRejected verifies that fastly_service_header, a
// VCL-only resource, is rejected when targeting a Compute service.
func TestAccFastlyServiceHeader_computeServiceRejected(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	headerName := fmt.Sprintf("header-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_compute"),
		Steps: []resource.TestStep{
			{
				Config:      ConfigHeaderOnComputeService(serviceName, headerName),
				ExpectError: regexp.MustCompile(`(?s)fastly_service_header does not support Fastly service.*of type "Compute"`),
			},
		},
	})
}

// TestAccFastlyServiceHeader_lockedVersion verifies that the provider refuses to write a header
// to an activated (locked) service version. Version 1, which holds the service and domain the
// test cleans up afterward, is never activated - only a cloned version 2 is - so the
// locked-version write attempt itself never lands in state and cleanup of version 1 is
// unaffected.
func TestAccFastlyServiceHeader_lockedVersion(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	headerName := fmt.Sprintf("header-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigServiceCDNWithDomain(serviceName, domainName, 1),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_cdn.test"]
						if !ok {
							return fmt.Errorf("service resource not found")
						}

						client, err := NewFastlyClient()
						if err != nil {
							return fmt.Errorf("error creating Fastly client: %w", err)
						}

						ctx := context.Background()
						cloned, err := client.CloneVersion(ctx, &fastly.CloneVersionInput{
							ServiceID:      rs.Primary.ID,
							ServiceVersion: 1,
						})
						if err != nil {
							return fmt.Errorf("error cloning version: %w", err)
						}

						_, err = client.ActivateVersion(ctx, &fastly.ActivateVersionInput{
							ServiceID:      rs.Primary.ID,
							ServiceVersion: fastly.ToValue(cloned.Number),
						})
						if err != nil {
							return fmt.Errorf("error activating cloned version: %w", err)
						}

						return nil
					},
				),
			},
			{
				Config:      ConfigHeaderOnLockedVersion(serviceName, domainName, headerName),
				ExpectError: regexp.MustCompile(`(?s)is locked and cannot be modified`),
			},
		},
	})
}

func TestAccFastlyServiceHeader_importBasic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	headerName := fmt.Sprintf("header-%s", acctest.RandString(10))

	var serviceID string
	var versionNumber string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigHeaderForImport(serviceName, domainName, headerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_header.test", "name", headerName),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["fastly_service_header.test"]
						if !ok {
							return fmt.Errorf("header resource not found")
						}
						serviceID = rs.Primary.Attributes["service_id"]
						versionNumber = rs.Primary.Attributes["version"]
						return nil
					},
				),
			},
			{
				ResourceName: "fastly_service_header.test",
				ImportStateIdFunc: func(_ *terraform.State) (string, error) {
					return fmt.Sprintf("%s/%s/%s", serviceID, versionNumber, headerName), nil
				},
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccFastlyServiceHeader_nameForcesReplace verifies changing name forces destroy/create
// rather than an in-place update.
func TestAccFastlyServiceHeader_nameForcesReplace(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	headerName1 := fmt.Sprintf("header-%s", acctest.RandString(10))
	headerName2 := fmt.Sprintf("header-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigHeaderBasic(serviceName, domainName, headerName1),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_header.test", "name", headerName1),
				),
			},
			{
				Config: ConfigHeaderBasic(serviceName, domainName, headerName2),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("fastly_service_header.test", plancheck.ResourceActionReplace),
					},
				},
				Check: resource.TestCheckResourceAttr("fastly_service_header.test", "name", headerName2),
			},
		},
	})
}
