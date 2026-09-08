package acceptancetests

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/go-fastly/v17/fastly"
)

func TestAccFastlyServiceCDNAuto_withDynamicSnippet(t *testing.T) {
	t.Parallel()

	serviceName := fmt.Sprintf("tf-test-dynamic-snippet-auto-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	backendName := fmt.Sprintf("backend-%s", acctest.RandString(10))
	snippetName := fmt.Sprintf("dynamic_%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCDNAutoWithDynamicSnippet(serviceName, domainName, backendName, snippetName, "recv", 100),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					CheckDynamicSnippetExistsInFastlyAtActiveVersion("fastly_service_cdn_auto.test", snippetName),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "dynamic_snippet.#", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "dynamic_snippet.0.name", snippetName),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "dynamic_snippet.0.type", "recv"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "dynamic_snippet.0.priority", "100"),
					resource.TestCheckResourceAttrSet("fastly_service_cdn_auto.test", "dynamic_snippet.0.snippet_id"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "1"),
				),
			},
		},
	})
}

func TestAccFastlyServiceCDNAuto_withDynamicSnippetMetadataUpdate(t *testing.T) {
	t.Parallel()

	serviceName := fmt.Sprintf("tf-test-dynamic-snippet-auto-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	backendName := fmt.Sprintf("backend-%s", acctest.RandString(10))
	snippetName := fmt.Sprintf("dynamic_%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCDNAutoWithDynamicSnippet(serviceName, domainName, backendName, snippetName, "recv", 100),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "1"),
				),
			},
			{
				Config: ConfigCDNAutoWithDynamicSnippet(serviceName, domainName, backendName, snippetName, "deliver", 50),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					CheckDynamicSnippetExistsInFastlyAtActiveVersion("fastly_service_cdn_auto.test", snippetName),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "dynamic_snippet.0.type", "deliver"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "dynamic_snippet.0.priority", "50"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "2"),
				),
			},
		},
	})
}

func TestAccFastlyServiceDynamicSnippetContent_create(t *testing.T) {
	t.Parallel()

	serviceName := fmt.Sprintf("tf-test-dynamic-snippet-content-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	backendName := fmt.Sprintf("backend-%s", acctest.RandString(10))
	snippetName := fmt.Sprintf("dynamic_%s", acctest.RandString(10))
	content := dynamicSnippetBoilerplate("one")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCDNAutoWithDynamicSnippetContent(serviceName, domainName, backendName, snippetName, "recv", 100, content, false),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					CheckDynamicSnippetContentInFastly("fastly_service_cdn_auto.test", snippetName, content),
					resource.TestCheckResourceAttr("fastly_service_dynamic_snippet_content.test", "content", content),
					resource.TestCheckResourceAttr("fastly_service_dynamic_snippet_content.test", "manage_snippets", "false"),
					resource.TestCheckResourceAttrSet("fastly_service_dynamic_snippet_content.test", "id"),
				),
			},
		},
	})
}

func TestAccFastlyServiceDynamicSnippetContent_updateDoesNotCloneService(t *testing.T) {
	t.Parallel()

	serviceName := fmt.Sprintf("tf-test-dynamic-snippet-content-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	backendName := fmt.Sprintf("backend-%s", acctest.RandString(10))
	snippetName := fmt.Sprintf("dynamic_%s", acctest.RandString(10))
	contentOne := dynamicSnippetBoilerplate("one")
	contentTwo := dynamicSnippetBoilerplate("two")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCDNAutoWithDynamicSnippetContent(serviceName, domainName, backendName, snippetName, "recv", 100, contentOne, false),
				Check: resource.ComposeTestCheckFunc(
					CheckDynamicSnippetContentInFastly("fastly_service_cdn_auto.test", snippetName, contentOne),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "1"),
				),
			},
			{
				Config: ConfigCDNAutoWithDynamicSnippetContent(serviceName, domainName, backendName, snippetName, "recv", 100, contentTwo, false),
				Check: resource.ComposeTestCheckFunc(
					CheckDynamicSnippetContentInFastly("fastly_service_cdn_auto.test", snippetName, contentTwo),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "1"),
				),
			},
		},
	})
}

func TestAccFastlyServiceDynamicSnippetContent_import(t *testing.T) {
	t.Parallel()

	serviceName := fmt.Sprintf("tf-test-dynamic-snippet-content-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	backendName := fmt.Sprintf("backend-%s", acctest.RandString(10))
	snippetName := fmt.Sprintf("dynamic_%s", acctest.RandString(10))
	content := dynamicSnippetBoilerplate("import")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCDNAutoWithDynamicSnippetContent(serviceName, domainName, backendName, snippetName, "recv", 100, content, false),
				Check: resource.ComposeTestCheckFunc(
					CheckDynamicSnippetContentInFastly("fastly_service_cdn_auto.test", snippetName, content),
				),
			},
			{
				ResourceName:      "fastly_service_dynamic_snippet_content.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: importDynamicSnippetContentID("fastly_service_dynamic_snippet_content.test"),
			},
		},
	})
}

func TestAccFastlyServiceDynamicSnippetContent_deleteManageSnippetsTrue(t *testing.T) {
	t.Parallel()

	serviceName := fmt.Sprintf("tf-test-dynamic-snippet-content-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	backendName := fmt.Sprintf("backend-%s", acctest.RandString(10))
	snippetName := fmt.Sprintf("dynamic_%s", acctest.RandString(10))
	content := dynamicSnippetBoilerplate("delete")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCDNAutoWithDynamicSnippetContent(serviceName, domainName, backendName, snippetName, "recv", 100, content, true),
				Check:  CheckDynamicSnippetContentInFastly("fastly_service_cdn_auto.test", snippetName, content),
			},
			{
				Config: ConfigCDNAutoWithDynamicSnippet(serviceName, domainName, backendName, snippetName, "recv", 100),
				Check:  CheckDynamicSnippetContentInFastly("fastly_service_cdn_auto.test", snippetName, ""),
			},
		},
	})
}

func TestAccFastlyServiceDynamicSnippetContent_deleteManageSnippetsFalse(t *testing.T) {
	t.Parallel()

	serviceName := fmt.Sprintf("tf-test-dynamic-snippet-content-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	backendName := fmt.Sprintf("backend-%s", acctest.RandString(10))
	snippetName := fmt.Sprintf("dynamic_%s", acctest.RandString(10))
	content := dynamicSnippetBoilerplate("delete")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCDNAutoWithDynamicSnippetContent(serviceName, domainName, backendName, snippetName, "recv", 100, content, false),
				Check:  CheckDynamicSnippetContentInFastly("fastly_service_cdn_auto.test", snippetName, content),
			},
			{
				Config: ConfigCDNAutoWithDynamicSnippet(serviceName, domainName, backendName, snippetName, "recv", 100),
				Check:  CheckDynamicSnippetContentInFastly("fastly_service_cdn_auto.test", snippetName, content),
			},
		},
	})
}

func TestAccFastlyServiceCDNAuto_createsWithDynamicSnippetIncludedByMainVCL(t *testing.T) {
	t.Parallel()

	serviceName := fmt.Sprintf("tf-test-dynamic-snippet-seed-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	backendName := fmt.Sprintf("backend-%s", acctest.RandString(10))
	snippetName := fmt.Sprintf("dynamic_%s", acctest.RandString(10))
	vclName := fmt.Sprintf("main_%s", acctest.RandString(10))
	subName := fmt.Sprintf("sub_%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				// Without a seeded content, this VCL fails to validate on creation because
				// the snippet it includes - and calls a subroutine from - is empty.
				Config: ConfigCDNAutoWithDynamicSnippetSeededContent(serviceName, domainName, backendName, snippetName, vclName, subName, "seed-one"),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn_auto.test"),
					CheckDynamicSnippetExistsInFastlyAtActiveVersion("fastly_service_cdn_auto.test", snippetName),
					CheckDynamicSnippetContentInFastly("fastly_service_cdn_auto.test", snippetName, dynamicSnippetSeedContent(subName, "seed-one")),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "dynamic_snippet.0.name", snippetName),
				),
			},
			{
				// The block's own content attribute keeps enforcing its configured value on
				// every subsequent apply too, not just at creation (see its schema
				// description) - verify a changed value actually propagates to the API.
				Config: ConfigCDNAutoWithDynamicSnippetSeededContent(serviceName, domainName, backendName, snippetName, vclName, subName, "seed-two"),
				Check: resource.ComposeTestCheckFunc(
					CheckDynamicSnippetContentInFastly("fastly_service_cdn_auto.test", snippetName, dynamicSnippetSeedContent(subName, "seed-two")),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "2"),
				),
			},
		},
	})
}

func TestAccFastlyServiceCDNAuto_unrelatedUpdateDoesNotResetSeparatelyManagedDynamicSnippetContent(t *testing.T) {
	t.Parallel()

	serviceName := fmt.Sprintf("tf-test-dynamic-snippet-content-%s", acctest.RandString(10))
	domainNameOne := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	domainNameTwo := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	backendName := fmt.Sprintf("backend-%s", acctest.RandString(10))
	snippetName := fmt.Sprintf("dynamic_%s", acctest.RandString(10))
	content := dynamicSnippetBoilerplate("unrelated-update")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCDNAutoWithDynamicSnippetContent(serviceName, domainNameOne, backendName, snippetName, "recv", 100, content, false),
				Check: resource.ComposeTestCheckFunc(
					CheckDynamicSnippetContentInFastly("fastly_service_cdn_auto.test", snippetName, content),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "1"),
				),
			},
			{
				// Renaming the domain is unrelated to the dynamic snippet, but it makes the
				// domain block's own Equal check differ, which forces a new service version -
				// and therefore a fresh dynamicsnippet.Reconcile pass against every dynamic
				// snippet, not just the one that changed (Update clones a new version, and
				// runs every nested resource type's reconcile against it, whenever ANY nested
				// resource type differs - see servicecdnauto's Update). Regression test for
				// the dynamic_snippet block's own (here unset) content attribute never
				// overwriting content that fastly_service_dynamic_snippet_content separately
				// manages, just because some other part of the service changed.
				Config: ConfigCDNAutoWithDynamicSnippetContent(serviceName, domainNameTwo, backendName, snippetName, "recv", 100, content, false),
				Check: resource.ComposeTestCheckFunc(
					CheckDynamicSnippetContentInFastly("fastly_service_cdn_auto.test", snippetName, content),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "2"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "managed_version", "2"),
				),
			},
		},
	})
}

func TestAccFastlyServiceDynamicVCLSnippet_seededContent(t *testing.T) {
	t.Parallel()

	serviceName := fmt.Sprintf("tf-test-dynamic-snippet-explicit-%s", acctest.RandString(10))
	snippetName := fmt.Sprintf("dynamic_%s", acctest.RandString(10))
	content := dynamicSnippetBoilerplate("explicit-seed")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				// fastly_service_dynamic_vcl_snippet shares its schema with the dynamic_snippet
				// block on fastly_service_cdn_auto, so it inherited the same optional content
				// attribute - verify it seeds correctly through this resource family too.
				Config: ConfigServiceDynamicVCLSnippetSeededContent(serviceName, snippetName, "recv", 100, content),
				Check: resource.ComposeTestCheckFunc(
					CheckExplicitDynamicSnippetContentInFastly("fastly_service_dynamic_vcl_snippet.test", content),
					resource.TestCheckResourceAttr("fastly_service_dynamic_vcl_snippet.test", "name", snippetName),
					resource.TestCheckResourceAttrSet("fastly_service_dynamic_vcl_snippet.test", "snippet_id"),
				),
			},
		},
	})
}

func TestAccFastlyServiceCDNAuto_withRegularAndDynamicSnippetNameConflict(t *testing.T) {
	t.Parallel()

	serviceName := fmt.Sprintf("tf-test-dynamic-snippet-conflict-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	backendName := fmt.Sprintf("backend-%s", acctest.RandString(10))
	snippetName := fmt.Sprintf("shared_%s", acctest.RandString(10))
	filePath := writeSnippetFile(t, "dynamic-conflict.vcl", snippetBoilerplate("conflict"))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config:      ConfigCDNAutoWithRegularAndDynamicSnippetConflict(serviceName, domainName, backendName, snippetName, filePath),
				ExpectError: regexp.MustCompile(`used by both regular and dynamic\s+snippets`),
			},
		},
	})
}

func CheckDynamicSnippetExistsInFastlyAtActiveVersion(resourceName, snippetName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		version, err := strconv.Atoi(rs.Primary.Attributes["active_version"])
		if err != nil {
			return fmt.Errorf("error parsing active_version for %s: %w", resourceName, err)
		}

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		snippet, err := client.GetSnippet(context.Background(), &fastly.GetSnippetInput{
			ServiceID:      rs.Primary.ID,
			ServiceVersion: version,
			Name:           snippetName,
		})
		if err != nil {
			return fmt.Errorf("error fetching dynamic VCL snippet %q in service %s version %d: %w", snippetName, rs.Primary.ID, version, err)
		}

		if fastly.ToValue(snippet.Dynamic) != 1 {
			return fmt.Errorf("snippet %q is regular; expected dynamic snippet", snippetName)
		}

		if fastly.ToValue(snippet.SnippetID) == "" {
			return fmt.Errorf("dynamic snippet %q has empty snippet_id", snippetName)
		}

		return nil
	}
}

func CheckDynamicSnippetContentInFastly(resourceName, snippetName string, expectedContent string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		serviceID, snippetID, err := dynamicSnippetIdentityFromState(s, resourceName, snippetName)
		if err != nil {
			return err
		}

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		dynamicSnippet, err := client.GetDynamicSnippet(context.Background(), &fastly.GetDynamicSnippetInput{
			ServiceID: serviceID,
			SnippetID: snippetID,
		})
		if err != nil {
			return fmt.Errorf("error fetching dynamic VCL snippet content %q in service %s: %w", snippetID, serviceID, err)
		}

		if fastly.ToValue(dynamicSnippet.Content) != expectedContent {
			return fmt.Errorf("dynamic snippet content mismatch for snippet %q in service %s:\nexpected: %q\nactual:   %q", snippetID, serviceID, expectedContent, fastly.ToValue(dynamicSnippet.Content))
		}

		return nil
	}
}

// CheckExplicitDynamicSnippetContentInFastly verifies a dynamic snippet's live content directly
// against a resource whose service_id/snippet_id are top-level attributes, such as
// fastly_service_dynamic_vcl_snippet - unlike CheckDynamicSnippetContentInFastly, which reads
// those from a fastly_service_cdn_auto's nested dynamic_snippet.N.* attributes.
func CheckExplicitDynamicSnippetContentInFastly(resourceName, expectedContent string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		serviceID := rs.Primary.Attributes["service_id"]
		snippetID := rs.Primary.Attributes["snippet_id"]
		if serviceID == "" || snippetID == "" {
			return fmt.Errorf("missing service_id/snippet_id for %s: service_id=%q snippet_id=%q", resourceName, serviceID, snippetID)
		}

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		dynamicSnippet, err := client.GetDynamicSnippet(context.Background(), &fastly.GetDynamicSnippetInput{
			ServiceID: serviceID,
			SnippetID: snippetID,
		})
		if err != nil {
			return fmt.Errorf("error fetching dynamic VCL snippet content %q in service %s: %w", snippetID, serviceID, err)
		}

		if fastly.ToValue(dynamicSnippet.Content) != expectedContent {
			return fmt.Errorf("dynamic snippet content mismatch for snippet %q in service %s:\nexpected: %q\nactual:   %q", snippetID, serviceID, expectedContent, fastly.ToValue(dynamicSnippet.Content))
		}

		return nil
	}
}

func importDynamicSnippetContentID(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return "", fmt.Errorf("not found: %s", resourceName)
		}

		serviceID := rs.Primary.Attributes["service_id"]
		snippetID := rs.Primary.Attributes["snippet_id"]

		if serviceID == "" || snippetID == "" {
			return "", fmt.Errorf("missing import identity fields for %s: service_id=%q snippet_id=%q", resourceName, serviceID, snippetID)
		}

		return fmt.Sprintf("%s/%s", serviceID, snippetID), nil
	}
}

func dynamicSnippetIdentityFromState(s *terraform.State, resourceName, snippetName string) (string, string, error) {
	rs, ok := s.RootModule().Resources[resourceName]
	if !ok {
		return "", "", fmt.Errorf("not found: %s", resourceName)
	}

	for i := 0; ; i++ {
		prefix := fmt.Sprintf("dynamic_snippet.%d.", i)
		name, ok := rs.Primary.Attributes[prefix+"name"]
		if !ok {
			break
		}
		if name != snippetName {
			continue
		}

		snippetID := rs.Primary.Attributes[prefix+"snippet_id"]
		if snippetID == "" {
			return "", "", fmt.Errorf("dynamic snippet %q has empty snippet_id", snippetName)
		}

		return rs.Primary.ID, snippetID, nil
	}

	return "", "", fmt.Errorf("dynamic snippet %q not found in state for %s", snippetName, resourceName)
}

func dynamicSnippetBoilerplate(label string) string {
	return fmt.Sprintf(`set req.http.X-Terraform-Dynamic-Snippet-Test = %q;`, label)
}
