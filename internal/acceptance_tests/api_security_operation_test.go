package acceptancetests

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/go-fastly/v17/fastly/apisecurity/operations"
	"github.com/fastly/terraform-provider-fastly/internal/errors"
)

// TestAccFastlyAPISecurityOperation_lifecycle covers create, in-place description update,
// clearing an optional description back to its empty-string default, ForceNew replacement
// on a method change, and import. tag_ids round-tripping against a real, Terraform-managed
// tag is covered by CDTOOL-1544 (fastly_api_security_operation_tag), which does not exist yet.
func TestAccFastlyAPISecurityOperation_lifecycle(t *testing.T) {
	t.Parallel()

	serviceName := fmt.Sprintf("tf_test_apisec_%s", acctest.RandString(10))
	domainName := fmt.Sprintf("tf-test-%s.example.com", acctest.RandString(10))
	path := "/v1/things"
	desc1 := "example"
	desc2 := "example-updated"

	var operationID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckAPISecurityOperationAndServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigAPISecurityOperation(serviceName, "GET", domainName, path, desc1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"fastly_api_security_operation.example", "service_id",
						"fastly_service_cdn.test", "id",
					),
					resource.TestCheckResourceAttr("fastly_api_security_operation.example", "method", "GET"),
					resource.TestCheckResourceAttr("fastly_api_security_operation.example", "domain", domainName),
					resource.TestCheckResourceAttr("fastly_api_security_operation.example", "path", path),
					resource.TestCheckResourceAttr("fastly_api_security_operation.example", "description", desc1),
					resource.TestCheckResourceAttrSet("fastly_api_security_operation.example", "operation_id"),
					CaptureAPISecurityOperationID("fastly_api_security_operation.example", &operationID),
					CheckAPISecurityOperationRemoteState("fastly_api_security_operation.example", desc1),
				),
			},
			{
				// Description is mutable in place: operation_id must not change.
				Config: ConfigAPISecurityOperation(serviceName, "GET", domainName, path, desc2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_api_security_operation.example", "description", desc2),
					CheckAPISecurityOperationIDUnchanged("fastly_api_security_operation.example", &operationID),
					CheckAPISecurityOperationRemoteState("fastly_api_security_operation.example", desc2),
				),
			},
			{
				// Omitting description entirely must clear it back to its empty-string default.
				Config: ConfigAPISecurityOperation(serviceName, "GET", domainName, path, ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_api_security_operation.example", "description", ""),
					CheckAPISecurityOperationIDUnchanged("fastly_api_security_operation.example", &operationID),
					CheckAPISecurityOperationRemoteState("fastly_api_security_operation.example", ""),
				),
			},
			{
				// method is ForceNew: changing it must replace the resource (new operation_id).
				Config: ConfigAPISecurityOperation(serviceName, "POST", domainName, path, desc1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_api_security_operation.example", "method", "POST"),
					CheckAPISecurityOperationIDChanged("fastly_api_security_operation.example", &operationID),
				),
			},
			{
				ResourceName:      "fastly_api_security_operation.example",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func CaptureAPISecurityOperationID(resourceName string, target *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		id := rs.Primary.Attributes["operation_id"]
		if id == "" {
			return fmt.Errorf("%s has no operation_id", resourceName)
		}

		*target = id
		return nil
	}
}

func CheckAPISecurityOperationIDUnchanged(resourceName string, expected *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		got := rs.Primary.Attributes["operation_id"]
		if got != *expected {
			return fmt.Errorf("%s operation_id changed unexpectedly: got %q, want %q", resourceName, got, *expected)
		}

		return nil
	}
}

func CheckAPISecurityOperationIDChanged(resourceName string, previous *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		got := rs.Primary.Attributes["operation_id"]
		if got == *previous {
			return fmt.Errorf("%s operation_id did not change after a ForceNew field changed", resourceName)
		}

		*previous = got
		return nil
	}
}

func CheckAPISecurityOperationRemoteState(resourceName, expectedDescription string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		serviceID := rs.Primary.Attributes["service_id"]
		operationID := rs.Primary.Attributes["operation_id"]

		client, err := NewFastlyClient()
		if err != nil {
			return fmt.Errorf("error creating Fastly client: %w", err)
		}

		op, err := operations.Describe(context.Background(), client, &operations.DescribeInput{
			ServiceID:   &serviceID,
			OperationID: &operationID,
		})
		if err != nil {
			return fmt.Errorf("error reading API Security operation %s/%s: %w", serviceID, operationID, err)
		}
		if op.Description != expectedDescription {
			return fmt.Errorf("unexpected API Security operation description: got %q, want %q", op.Description, expectedDescription)
		}

		return nil
	}
}

func CheckAPISecurityOperationAndServiceDestroy(s *terraform.State) error {
	if err := checkAPISecurityOperationDestroy(s); err != nil {
		return err
	}
	return CheckServiceDestroy("fastly_service_cdn")(s)
}

func checkAPISecurityOperationDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return fmt.Errorf("error creating Fastly client: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "fastly_api_security_operation" {
			continue
		}

		serviceID := rs.Primary.Attributes["service_id"]
		operationID := rs.Primary.Attributes["operation_id"]

		_, err := operations.Describe(context.Background(), client, &operations.DescribeInput{
			ServiceID:   &serviceID,
			OperationID: &operationID,
		})
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("error checking if API Security operation %s/%s was destroyed: %w", serviceID, operationID, err)
		}

		return fmt.Errorf("API Security operation %s/%s still exists", serviceID, operationID)
	}

	return nil
}
