// Command cleanup-test-services deletes any Fastly service whose name matches
// a prefix (default "tf-test-"), deactivating its active version first if one
// exists. It exists to sweep up services left behind by interrupted or failed
// acceptance test runs before they run into the account's service limit.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/fastly/go-fastly/v17/fastly"
)

func main() {
	prefix := flag.String("prefix", "tf-test-", "only delete services whose name has this prefix")
	dryRun := flag.Bool("dry-run", false, "list matching services without deleting them")
	flag.Parse()

	apiToken := os.Getenv("FASTLY_API_TOKEN")
	if apiToken == "" {
		log.Fatal("FASTLY_API_TOKEN environment variable must be set")
	}

	client, err := fastly.NewClient(apiToken)
	if err != nil {
		log.Fatalf("creating Fastly client: %s", err)
	}

	ctx := context.Background()
	services, err := client.ListServices(ctx, &fastly.ListServicesInput{})
	if err != nil {
		log.Fatalf("listing services: %s", err)
	}

	var matched, failures int
	for _, svc := range services {
		name := fastly.ToValue(svc.Name)
		if !strings.HasPrefix(name, *prefix) || svc.DeletedAt != nil {
			continue
		}
		matched++

		serviceID := fastly.ToValue(svc.ServiceID)
		if *dryRun {
			fmt.Printf("[dry-run] would delete service %q (%s)\n", name, serviceID)
			continue
		}

		if activeVersion := fastly.ToValue(svc.ActiveVersion); activeVersion != 0 {
			fmt.Printf("deactivating version %d of service %q (%s)\n", activeVersion, name, serviceID)
			if _, err := client.DeactivateVersion(ctx, &fastly.DeactivateVersionInput{
				ServiceID:      serviceID,
				ServiceVersion: activeVersion,
			}); err != nil {
				fmt.Printf("failed to deactivate version %d of service %q (%s): %s\n", activeVersion, name, serviceID, err)
				failures++
				continue
			}
		}

		fmt.Printf("deleting service %q (%s)\n", name, serviceID)
		if err := client.DeleteService(ctx, &fastly.DeleteServiceInput{ServiceID: serviceID}); err != nil {
			fmt.Printf("failed to delete service %q (%s): %s\n", name, serviceID, err)
			failures++
		}
	}

	fmt.Printf("matched %d service(s) with prefix %q\n", matched, *prefix)
	if failures > 0 {
		log.Fatalf("%d service(s) failed to clean up", failures)
	}
}
