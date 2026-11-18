package acceptancetests

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccFastlyServiceLoggingFOS_basic(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("fos-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingFOSBasic(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_cdn.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_fos.test", "name", loggerName),
					resource.TestCheckResourceAttr("fastly_service_logging_fos.test", "bucket_name", "test-fos-bucket"),
					resource.TestCheckResourceAttr("fastly_service_logging_fos.test", "region", "us-east"),
					resource.TestCheckResourceAttr("fastly_service_logging_fos.test", "message_type", "classic"),
					resource.TestCheckResourceAttr("fastly_service_logging_fos.test", "period", "3600"),
					resource.TestCheckResourceAttr("fastly_service_logging_fos.test", "format_version", "2"),
					resource.TestCheckResourceAttrSet("fastly_service_logging_fos.test", "format"),
					resource.TestCheckResourceAttrSet("fastly_service_logging_fos.test", "id"),
					resource.TestCheckNoResourceAttr("fastly_service_logging_fos.test", "authentication.access_key"),
				),
			},
			{
				Config:   ConfigLoggingFOSBasic(serviceName, domainName, loggerName),
				PlanOnly: true,
			},
		},
	})
}

func TestAccFastlyServiceLoggingFOS_update(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("fos-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_cdn"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingFOSBasic(serviceName, domainName, loggerName),
				Check:  resource.TestCheckResourceAttr("fastly_service_logging_fos.test", "region", "us-east"),
			},
			{
				Config: ConfigLoggingFOSUpdated(serviceName, domainName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("fastly_service_logging_fos.test", "region", "eu-central"),
					resource.TestCheckResourceAttr("fastly_service_logging_fos.test", "path", "logs/"),
					resource.TestCheckResourceAttr("fastly_service_logging_fos.test", "period", "600"),
					resource.TestCheckResourceAttr("fastly_service_logging_fos.test", "compression_codec", "zstd"),
					resource.TestCheckResourceAttr("fastly_service_logging_fos.test", "file_max_bytes", "2097152"),
					resource.TestCheckResourceAttr("fastly_service_logging_fos.test", "placement", "none"),
				),
			},
			{
				Config:   ConfigLoggingFOSUpdated(serviceName, domainName, loggerName),
				PlanOnly: true,
			},
		},
	})
}

func TestAccFastlyServiceLoggingFOS_compute(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	loggerName := fmt.Sprintf("fos-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		CheckDestroy:             CheckServiceDestroy("fastly_service_compute"),
		Steps: []resource.TestStep{
			{
				Config: ConfigLoggingFOSCompute(serviceName, loggerName),
				Check: resource.ComposeTestCheckFunc(
					CheckServiceExists("fastly_service_compute.test"),
					resource.TestCheckResourceAttr("fastly_service_logging_fos.test", "name", loggerName),
					resource.TestCheckResourceAttr("fastly_service_logging_fos.test", "region", "us-east"),
				),
			},
			{
				Config:   ConfigLoggingFOSCompute(serviceName, loggerName),
				PlanOnly: true,
			},
		},
	})
}

func TestAccFastlyServiceLoggingFOS_codecConflict(t *testing.T) {
	t.Parallel()
	serviceName := fmt.Sprintf("tf-test-%s", acctest.RandString(10))
	domainName := fmt.Sprintf("%s.example.com", acctest.RandString(10))
	loggerName := fmt.Sprintf("fos-logger-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { PreCheck(t) },
		ProtoV6ProviderFactories: ProtoV6ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      ConfigLoggingFOSCodecConflict(serviceName, domainName, loggerName),
				ExpectError: regexp.MustCompile("Conflicting `gzip_level` and `compression_codec`"),
			},
		},
	})
}
