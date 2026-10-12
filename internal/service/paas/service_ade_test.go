package paas_test

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/service/paas"
	sdkacctest "github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
)

func TestAccPaaSServiceAutoDiskExpansion_basic(t *testing.T) {
	resourceName := "aws_paas_service.test"

	randServiceName := fmt.Sprintf("tf-test-ade-%s", sdkacctest.RandString(5))

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { acctest.PreCheck(t) },
		ErrorCheck:        acctest.ErrorCheck(t, paas.EndpointsID),
		ProviderFactories: acctest.ProviderFactories,
		CheckDestroy:      testAccCheckServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceAutoDiskExpansionConfig(randServiceName, `
  auto_disk_expansion {
    condition_value = 80
    max_size_gb     = 64
  }
`),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "auto_disk_expansion.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "auto_disk_expansion.0.condition_value", "80"),
					resource.TestCheckResourceAttr(resourceName, "auto_disk_expansion.0.enabled", "true"),
					resource.TestCheckResourceAttr(resourceName, "auto_disk_expansion.0.max_size_gb", "64"),
					resource.TestCheckResourceAttrSet(resourceName, "auto_disk_expansion.0.status"),
					resource.TestCheckTypeSetElemAttr(resourceName, "supported_features.*", "common:auto_disk_expansion"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"delete_interfaces_on_destroy", "redis.0.password"},
			},
			{
				Config: testAccServiceAutoDiskExpansionConfig(randServiceName, `
  auto_disk_expansion {
    condition_value = 90
    max_size_gb     = 128
  }
`),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "auto_disk_expansion.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "auto_disk_expansion.0.condition_value", "90"),
					resource.TestCheckResourceAttr(resourceName, "auto_disk_expansion.0.enabled", "true"),
					resource.TestCheckResourceAttr(resourceName, "auto_disk_expansion.0.max_size_gb", "128"),
				),
			},
			{
				Config: testAccServiceAutoDiskExpansionConfig(randServiceName, ""),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "auto_disk_expansion.#", "0"),
				),
			},
		},
	})
}

func testAccServiceAutoDiskExpansionConfig(serviceName, autoDiskExpansion string) string {
	return fmt.Sprintf(`
resource "aws_vpc" "test" {
  cidr_block = "10.0.0.0/16"

  tags = {
    Name = %[1]q
  }
}

resource "aws_subnet" "test" {
  cidr_block = "10.0.1.0/24"
  vpc_id     = aws_vpc.test.id

  tags = {
    Name = %[1]q
  }
}

resource "aws_paas_service" "test" {
  name          = %[1]q
  instance_type = "c5.large"

  root_volume {
    type = "gp2"
  }

  data_volume {
    type = "gp2"
    size = 32
  }
%[2]s
  delete_interfaces_on_destroy = true
  security_group_ids           = [aws_vpc.test.default_security_group_id]
  subnet_ids                   = [aws_subnet.test.id]

  redis {
    version = "7.2"
  }
}
`, serviceName, autoDiskExpansion)
}
