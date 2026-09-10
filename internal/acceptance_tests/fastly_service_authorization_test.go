package acceptancetests

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/fastly/terraform-provider-fastly-beta/internal/errors"

	"github.com/fastly/go-fastly/v17/fastly"
)

// createTestUser provisions a throwaway engineer-role user to use as the grantee in service
// authorization tests, and registers its cleanup. The API rejects granting a service
// authorization to a superuser ("User with 'superuser' role is not allowed"), which is what the
// acceptance-testing account's own token identifies as, so the current user can't be used
// directly; there is also no fastly_user resource yet to create one via HCL.
func createTestUser(t *testing.T) string {
	t.Helper()

	client, err := NewFastlyClient()
	if err != nil {
		t.Fatalf("error creating Fastly client: %s", err)
	}

	login := fmt.Sprintf("tf-test-%s@fastly-test.example.com", acctest.RandString(10))
	name := "tf-test"
	role := "engineer"

	user, err := client.CreateUser(context.Background(), &fastly.CreateUserInput{
		Login: &login,
		Name:  &name,
		Role:  &role,
	})
	if err != nil {
		t.Fatalf("error creating test user: %s", err)
	}

	t.Cleanup(func() {
		if err := client.DeleteUser(context.Background(), &fastly.DeleteUserInput{UserID: *user.UserID}); err != nil && !errors.IsNotFound(err) {
			t.Errorf("error deleting test user %s: %s", *user.UserID, err)
		}
	})

	return *user.UserID
}

func TestAccFastlyServiceAuthorization_basic(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	userID := createTestUser(t)
	resourceName := "fastly_service_authorization.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceAuthorizationDestroy,
		Steps: []resource.TestStep{
			{
				Config: ConfigServiceAuthorization(serviceName, userID, "purge_select"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceAuthorizationExists(),
					resource.TestCheckResourceAttr(resourceName, "permission", "purge_select"),
					resource.TestCheckResourceAttr(resourceName, "user_id", userID),
					resource.TestCheckResourceAttrPair(resourceName, "service_id", "fastly_service_cdn.test", "id"),
				),
			},
			{
				// permission is updatable in place; service_id and user_id stay fixed.
				Config: ConfigServiceAuthorization(serviceName, userID, "purge_all"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceAuthorizationExists(),
					resource.TestCheckResourceAttr(resourceName, "permission", "purge_all"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccFastlyServiceAuthorization_invalidPermission(t *testing.T) {
	t.Parallel()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' is set")
	}

	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	userID := createTestUser(t)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigServiceAuthorization(serviceName, userID, "not-a-real-permission"),
				ExpectError: regexp.MustCompile(`Attribute permission value must be one of`),
			},
		},
	})
}

func testAccCheckServiceAuthorizationExists() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client, err := NewFastlyClient()
		if err != nil {
			return err
		}

		resourceName := "fastly_service_authorization.test"
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("not found: %s", resourceName)
		}

		_, err = client.GetServiceAuthorization(context.Background(), &fastly.GetServiceAuthorizationInput{ID: rs.Primary.ID})
		return err
	}
}

func CheckServiceAuthorizationDestroy(s *terraform.State) error {
	client, err := NewFastlyClient()
	if err != nil {
		return fmt.Errorf("error creating Fastly client: %w", err)
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "fastly_service_authorization" {
			continue
		}

		_, err := client.GetServiceAuthorization(context.Background(), &fastly.GetServiceAuthorizationInput{ID: rs.Primary.ID})
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("error checking if Service Authorization was destroyed: %w", err)
		}

		return fmt.Errorf("Service Authorization %s still exists", rs.Primary.ID)
	}

	return nil
}
