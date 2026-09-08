package acceptancetests

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/go-fastly/v17/fastly/apisecurity/operations"
	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"
)

// Covers create, in-place rename (name is mutable, unlike the operation
// resource's ForceNew fields), description update/clear, and import.
func TestAccFastlyAPISecurityOperationTag_lifecycle(t *testing.T) {
	t.Parallel()

	serviceName := fmt.Sprintf("tf_test_apisectag_%s", acctest.RandString(10))
	tagName1 := fmt.Sprintf("tf-test-tag-%s", acctest.RandString(10))
	tagName2 := fmt.Sprintf("tf-test-tag-updated-%s", acctest.RandString(10))
	desc1 := "example"
	desc2 := "example-updated"

	var tagID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckAPISecurityOperationTagAndServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigAPISecurityOperationTag(serviceName, tagName1, desc1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"fastly_api_security_operation_tag.example", "service_id",
						"fastly_service_cdn.test", "id",
					),
					resource.TestCheckResourceAttr("fastly_api_security_operation_tag.example", "name", tagName1),
					resource.TestCheckResourceAttr("fastly_api_security_operation_tag.example", "description", desc1),
					resource.TestCheckResourceAttrSet("fastly_api_security_operation_tag.example", "tag_id"),
					CaptureAPISecurityOperationTagID("fastly_api_security_operation_tag.example", &tagID),
					CheckAPISecurityOperationTagRemoteState("fastly_api_security_operation_tag.example", tagName1, desc1),
				),
			},
			{
				// name is mutable in place: tag_id must not change.
				Config: ConfigAPISecurityOperationTag(serviceName, tagName2, desc1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_api_security_operation_tag.example", "name", tagName2),
					CheckAPISecurityOperationTagIDUnchanged("fastly_api_security_operation_tag.example", &tagID),
					CheckAPISecurityOperationTagRemoteState("fastly_api_security_operation_tag.example", tagName2, desc1),
				),
			},
			{
				Config: ConfigAPISecurityOperationTag(serviceName, tagName2, desc2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_api_security_operation_tag.example", "description", desc2),
					CheckAPISecurityOperationTagIDUnchanged("fastly_api_security_operation_tag.example", &tagID),
					CheckAPISecurityOperationTagRemoteState("fastly_api_security_operation_tag.example", tagName2, desc2),
				),
			},
			{
				// Omitting description entirely must clear it back to its empty-string default.
				Config: ConfigAPISecurityOperationTag(serviceName, tagName2, ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_api_security_operation_tag.example", "description", ""),
					CheckAPISecurityOperationTagIDUnchanged("fastly_api_security_operation_tag.example", &tagID),
					CheckAPISecurityOperationTagRemoteState("fastly_api_security_operation_tag.example", tagName2, ""),
				),
			},
			{
				ResourceName:      "fastly_api_security_operation_tag.example",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func CaptureAPISecurityOperationTagID(resourceName string, target *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		id := rs.Primary.Attributes["tag_id"]
		if id == "" {
			return fmt.Errorf("%s has no tag_id", resourceName)
		}

		*target = id
		return nil
	}
}

func CheckAPISecurityOperationTagIDUnchanged(resourceName string, expected *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		got := rs.Primary.Attributes["tag_id"]
		if got != *expected {
			return fmt.Errorf("%s tag_id changed unexpectedly: got %q, want %q", resourceName, got, *expected)
		}

		return nil
	}
}

func CheckAPISecurityOperationTagRemoteState(resourceName, expectedName, expectedDescription string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		serviceID := rs.Primary.Attributes["service_id"]
		tagID := rs.Primary.Attributes["tag_id"]

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		tag, err := operations.DescribeTag(context.Background(), client, &operations.DescribeTagInput{
			ServiceID: &serviceID,
			TagID:     &tagID,
		})
		if err != nil {
			return fmt.Errorf("error reading API Security operation tag %s/%s: %w", serviceID, tagID, err)
		}
		if tag.Name != expectedName {
			return fmt.Errorf("unexpected API Security operation tag name: got %q, want %q", tag.Name, expectedName)
		}
		if tag.Description != expectedDescription {
			return fmt.Errorf("unexpected API Security operation tag description: got %q, want %q", tag.Description, expectedDescription)
		}

		return nil
	}
}

func CheckAPISecurityOperationTagAndServiceDestroy(s *terraform.State) error {
	if err := checkAPISecurityOperationTagDestroy(s); err != nil {
		return err
	}
	return CheckServiceDestroy("fastly_service_cdn")(s)
}

func checkAPISecurityOperationTagDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return fmt.Errorf("error creating Fastly client: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "fastly_api_security_operation_tag" {
			continue
		}

		serviceID := rs.Primary.Attributes["service_id"]
		tagID := rs.Primary.Attributes["tag_id"]

		_, err := operations.DescribeTag(context.Background(), client, &operations.DescribeTagInput{
			ServiceID: &serviceID,
			TagID:     &tagID,
		})
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("error checking if API Security operation tag %s/%s was destroyed: %w", serviceID, tagID, err)
		}

		return fmt.Errorf("API Security operation tag %s/%s still exists", serviceID, tagID)
	}

	return nil
}
