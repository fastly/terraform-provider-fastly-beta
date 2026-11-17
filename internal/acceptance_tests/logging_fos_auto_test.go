package acceptancetests

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccFastlyServiceCDNAuto_withLoggingFOS(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("fos-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigCDNAutoBasic(serviceName, domainName),
				Check:  resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_fos.#", "0"),
			},
			{
				Config: ConfigCDNAutoWithLoggingFOS(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_fos.#", "1"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_fos.0.name", loggerName),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_fos.0.region", "us-east"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "2"),
				),
			},
			{
				Config:   ConfigCDNAutoWithLoggingFOS(serviceName, domainName, loggerName),
				PlanOnly: true,
			},
			{
				Config: ConfigCDNAutoWithLoggingFOSUpdated(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_fos.0.region", "eu-central"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_fos.0.period", "600"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_fos.0.compression_codec", "zstd"),
					resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "active_version", "3"),
				),
			},
			{
				Config: ConfigCDNAutoBasic(serviceName, domainName),
				Check:  resource.TestCheckResourceAttr("fastly_service_cdn_auto.test", "logging_fos.#", "0"),
			},
		},
	})
}

func TestAccFastlyServiceComputeAuto_withLoggingFOS(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("fos-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_compute_auto"),
		Steps: []resource.TestStep{
			{
				Config: ConfigComputeAutoWithLoggingFOS(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "logging_fos.#", "1"),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "logging_fos.0.name", loggerName),
					resource.TestCheckResourceAttr("fastly_service_compute_auto.test", "logging_fos.0.region", "us-east"),
				),
			},
			{
				Config:   ConfigComputeAutoWithLoggingFOS(serviceName, domainName, loggerName),
				PlanOnly: true,
			},
		},
	})
}
