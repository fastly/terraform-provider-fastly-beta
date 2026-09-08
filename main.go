package main

import (
	"context"
	"log"

	"github.com/fastly/terraform-provider-fastly-beta/internal/provider"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

func main() {
	err := providerserver.Serve(context.Background(), provider.New, providerserver.ServeOpts{
		Address: "registry.terraform.io/fastly/fastly-beta",
	})
	if err != nil {
		log.Fatal(err)
	}
}
