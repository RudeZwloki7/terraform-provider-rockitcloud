package paas_test

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/service/paas"
	sdkacctest "github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	tfpaas "github.com/hashicorp/terraform-provider-aws/internal/service/paas"
)

func TestAccPaaSServiceClickHouse_basic(t *testing.T) {
	resourceName := "aws_paas_service.test"

	randServiceName := fmt.Sprintf("terraform-test-%s", sdkacctest.RandString(5))
	randKeyName := fmt.Sprintf("terraform-test-%s", sdkacctest.RandString(5))
	publicKey, _, err := sdkacctest.RandSSHKeyPair(acctest.DefaultEmailAddress)
	if err != nil {
		t.Fatalf("error generating random SSH key: %s", err)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { acctest.PreCheck(t) },
		ErrorCheck:        acctest.ErrorCheck(t, paas.EndpointsID),
		ProviderFactories: acctest.ProviderFactories,
		CheckDestroy:      testAccCheckServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceClickHouseConfig(randServiceName, randKeyName, publicKey),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "additional_roles.#", "1"),
					resource.TestCheckTypeSetElemAttr(resourceName, "additional_roles.*", "coordinator"),
					resource.TestCheckResourceAttr(resourceName, "clickhouse.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "clickhouse.0.class", "database"),
					resource.TestCheckResourceAttr(resourceName, "clickhouse.0.version", "25.3"),
					resource.TestCheckResourceAttr(resourceName, "clickhouse.0.database.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "clickhouse.0.database.0.name", "test_db"),
					resource.TestCheckResourceAttr(resourceName, "clickhouse.0.user.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "clickhouse.0.user.0.name", "test_user"),
					resource.TestCheckResourceAttr(resourceName, "clickhouse.0.user.0.grants.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "clickhouse.0.user.0.grants.0", "GRANT SELECT ON test_db.*"),
					resource.TestCheckResourceAttr(resourceName, "clickhouse.0.user.0.networks.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "clickhouse.0.user.0.networks.0.ip.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "clickhouse.0.user.0.password_type", "double_sha1"),
					resource.TestCheckResourceAttr(resourceName, "clickhouse.0.user.0.profile", "default"),
					resource.TestCheckResourceAttr(resourceName, "clickhouse.0.user.0.quota", "default"),
					resource.TestCheckResourceAttr(resourceName, "high_availability", "false"),
					resource.TestCheckResourceAttr(resourceName, "service_class", "database"),
					resource.TestCheckResourceAttr(resourceName, "service_type", "clickhouse"),
					resource.TestCheckResourceAttr(resourceName, "status", tfpaas.ServiceStatusReady),
				),
			},
		},
	})
}

func testAccServiceClickHouseConfig(serviceName, keyName, publicKey string) string {
	return acctest.ConfigCompose(testAccServiceBaseConfig(serviceName, keyName, publicKey), fmt.Sprintf(`
resource "aws_paas_service" "test" {
  depends_on = ["aws_internet_gateway.test"]

  name          = %[1]q
  instance_type = "c5.large"

  additional_roles = ["coordinator"]

  root_volume {
    type = "gp2"
  }

  data_volume {
    type = "gp2"
  }

  delete_interfaces_on_destroy = true
  security_group_ids           = [aws_vpc.test.default_security_group_id]
  subnet_ids                   = [aws_subnet.test.id]

  ssh_key_name = aws_key_pair.test.key_name

  clickhouse {
    version = "25.3"

    database {
      name = "test_db"
    }

    user {
      name     = "test_user"
      password = "Test_password1"
      grants   = ["GRANT SELECT ON test_db.*"]

      networks {
        ip = ["10.0.0.0/16"]
      }
    }
  }
}
`, serviceName))
}

// testAccServiceBaseConfig returns network and SSH key configuration for ClickHouse and GitLab service tests.
func testAccServiceBaseConfig(serviceName, keyName, publicKey string) string {
	return fmt.Sprintf(`
resource "aws_vpc" "test" {
  cidr_block = "10.0.0.0/16"

  tags = {
    Name = %[1]q
  }
}

resource "aws_internet_gateway" "test" {
  vpc_id = aws_vpc.test.id

  tags = {
    Name = %[1]q
  }
}

resource "aws_route" "test" {
  route_table_id         = aws_vpc.test.main_route_table_id
  destination_cidr_block = "0.0.0.0/0"
  gateway_id             = aws_internet_gateway.test.id
}

resource "aws_subnet" "test" {
  cidr_block = "10.0.1.0/24"
  vpc_id     = aws_vpc.test.id

  tags = {
    Name = %[1]q
  }
}

resource "aws_key_pair" "test" {
  key_name   = %[2]q
  public_key = %[3]q
}
`, serviceName, keyName, publicKey)
}
