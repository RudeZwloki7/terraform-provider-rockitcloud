package paas_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/service/paas"
	sdkacctest "github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
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

	var serviceID, userID string
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccServicePreCheck(t) },
		ErrorCheck:        acctest.ErrorCheck(t, paas.EndpointsID),
		ProviderFactories: acctest.ProviderFactories,
		CheckDestroy:      testAccCheckServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccServiceClickHouseConfig(randServiceName, randKeyName, publicKey),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceExists(resourceName),
					testAccCheckClickHouseResourceIDUnchanged(resourceName, &serviceID),
					testAccCheckClickHouseUserIDUnchanged(resourceName, &userID),
					resource.TestCheckResourceAttr(resourceName, "additional_roles.#", "1"),
					resource.TestCheckTypeSetElemAttr(resourceName, "additional_roles.*", "coordinator"),
					resource.TestCheckResourceAttr(resourceName, "clickhouse.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "clickhouse.0.class", "database"),
					resource.TestCheckResourceAttr(resourceName, "clickhouse.0.version", "25.3"),
					resource.TestCheckResourceAttr(resourceName, "clickhouse.0.options.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "clickhouse.0.options.max_connections", "1024"),
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
					resource.TestCheckResourceAttrPair("data.aws_paas_service.test", "id", resourceName, "id"),
					resource.TestCheckResourceAttr("data.aws_paas_service.test", "clickhouse.0.version", "25.3"),
					resource.TestCheckResourceAttr("data.aws_paas_service.test", "clickhouse.0.options.max_connections", "1024"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"delete_interfaces_on_destroy", "clickhouse.0.user.0.password"},
			},
			{
				Config: testAccServiceClickHouseUpdatedConfig(randServiceName, randKeyName, publicKey),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckClickHouseResourceIDUnchanged(resourceName, &serviceID),
					testAccCheckClickHouseUserIDUnchanged(resourceName, &userID),
					resource.TestCheckResourceAttr(resourceName, "clickhouse.0.options.max_connections", "2048"),
					resource.TestCheckResourceAttr(resourceName, "clickhouse.0.user.0.grants.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "clickhouse.0.user.0.networks.0.host_regexp.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "clickhouse.0.user.0.networks.0.ip.#", "0"),
					resource.TestCheckResourceAttr("data.aws_paas_service.test", "clickhouse.0.options.max_connections", "2048"),
				),
			},
			{
				Config: testAccServiceClickHouseResetConfig(randServiceName, randKeyName, publicKey),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckClickHouseResourceIDUnchanged(resourceName, &serviceID),
					testAccCheckClickHouseUserIDUnchanged(resourceName, &userID),
					resource.TestCheckResourceAttr(resourceName, "clickhouse.0.options.%", "0"),
					resource.TestCheckResourceAttr(resourceName, "clickhouse.0.user.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "clickhouse.0.user.0.grants.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "clickhouse.0.user.0.networks.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "clickhouse.0.database.#", "1"),
					resource.TestCheckResourceAttr("data.aws_paas_service.test", "clickhouse.0.options.%", "0"),
				),
			},
			{
				Config: testAccServiceClickHouseEmptyConfig(randServiceName, randKeyName, publicKey),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckClickHouseResourceIDUnchanged(resourceName, &serviceID),
					resource.TestCheckResourceAttr(resourceName, "clickhouse.0.user.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "clickhouse.0.database.#", "0"),
				),
			},
		},
	})
}

func testAccServiceClickHouseConfig(serviceName, keyName, publicKey string) string {
	return testAccServiceClickHouseCustomConfig(serviceName, keyName, publicKey, `additional_roles = ["coordinator"]`, `
    options = {
      max_connections = "1024"
    }
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
`)
}

func testAccServiceClickHouseUpdatedConfig(serviceName, keyName, publicKey string) string {
	return testAccServiceClickHouseCustomConfig(serviceName, keyName, publicKey, `additional_roles = ["coordinator"]`, `
    options = {
      max_connections = "2048"
    }
    database {
      name = "test_db"
    }
    user {
      name     = "test_user"
      password = "Test_password2"
      grants   = ["GRANT SELECT ON test_db.*", "GRANT INSERT ON test_db.*"]
      networks {
        host_regexp = [".*[.]example[.]com"]
      }
    }
`)
}

func testAccServiceClickHouseResetConfig(serviceName, keyName, publicKey string) string {
	return testAccServiceClickHouseCustomConfig(serviceName, keyName, publicKey, `additional_roles = ["coordinator"]`, `
    database {
      name = "test_db"
    }
    user {
      name     = "test_user"
      password = "Test_password2"
    }
`)
}

func testAccServiceClickHouseEmptyConfig(serviceName, keyName, publicKey string) string {
	return testAccServiceClickHouseCustomConfig(serviceName, keyName, publicKey, `additional_roles = ["coordinator"]`, "")
}

func testAccServiceClickHouseCustomConfig(serviceName, keyName, publicKey, topology, parameters string) string {
	return testAccServiceClickHouseCustomConfigWithOptions(serviceName, keyName, publicKey, topology, parameters, testAccServiceClickHouseConfigOptions{})
}

type testAccServiceClickHouseConfigOptions struct {
	instanceType   string
	dataVolumeSize int
}

func testAccServiceClickHouseCustomConfigWithOptions(serviceName, keyName, publicKey, topology, parameters string, options testAccServiceClickHouseConfigOptions) string {
	if options.instanceType == "" {
		options.instanceType = "c5.large"
	}
	if options.dataVolumeSize == 0 {
		options.dataVolumeSize = 32
	}
	return testAccServiceConfigCompose(serviceName, keyName, publicKey, fmt.Sprintf(`
resource "aws_paas_service" "test" {
  depends_on = [aws_route.test]

  name          = %[1]q
  instance_type = %[4]q

  %[2]s

  root_volume {
    type = "gp2"
  }

  data_volume {
    type = "gp2"
    size = %[5]d
  }

  delete_interfaces_on_destroy = true
  security_group_ids           = [aws_vpc.test.default_security_group_id]
  subnet_ids                   = [aws_subnet.test.id]
  ssh_key_name                 = aws_key_pair.test.key_name

  clickhouse {
    version = "25.3"
    %[3]s
  }
}

data "aws_paas_service" "test" {
  id = aws_paas_service.test.id
}
`, serviceName, topology, parameters, options.instanceType, options.dataVolumeSize))
}

func testAccCheckClickHouseResourceIDUnchanged(resourceName string, previousID *string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		resourceState, ok := state.RootModule().Resources[resourceName]
		if !ok || resourceState.Primary == nil || resourceState.Primary.ID == "" {
			return fmt.Errorf("PaaS resource %s has no ID in state", resourceName)
		}
		if *previousID == "" {
			*previousID = resourceState.Primary.ID
		} else if resourceState.Primary.ID != *previousID {
			return fmt.Errorf("PaaS resource %s was unexpectedly replaced: %s -> %s", resourceName, *previousID, resourceState.Primary.ID)
		}
		return nil
	}
}

func testAccCheckClickHouseUserIDUnchanged(resourceName string, previousID *string) resource.TestCheckFunc {
	return resource.TestCheckResourceAttrWith(resourceName, "clickhouse.0.user.0.id", func(id string) error {
		if id == "" {
			return fmt.Errorf("ClickHouse user has no ID in state")
		}
		if *previousID == "" {
			*previousID = id
		} else if id != *previousID {
			return fmt.Errorf("ClickHouse user was unexpectedly replaced: %s -> %s", *previousID, id)
		}
		return nil
	})
}

func testAccServicePreCheck(t *testing.T) {
	t.Helper()
	if (os.Getenv("TF_ACC_PAAS_VPC_ID") == "") != (os.Getenv("TF_ACC_PAAS_SUBNET_ID") == "") {
		t.Fatal("TF_ACC_PAAS_VPC_ID and TF_ACC_PAAS_SUBNET_ID must both be set to reuse an existing network")
	}
	acctest.PreCheck(t)
}

func testAccServiceConfigCompose(serviceName, keyName, publicKey, serviceConfig string) string {
	if os.Getenv("TF_ACC_PAAS_VPC_ID") != "" && os.Getenv("TF_ACC_PAAS_SUBNET_ID") != "" {
		serviceConfig = strings.ReplaceAll(serviceConfig, "depends_on = [aws_route.test]", "")
		serviceConfig = strings.ReplaceAll(serviceConfig, "aws_vpc.test.default_security_group_id", "data.aws_security_group.test.id")
		serviceConfig = strings.ReplaceAll(serviceConfig, "aws_subnet.test.id", "data.aws_subnet.test.id")
	}
	return acctest.ConfigCompose(testAccServiceBaseConfig(serviceName, keyName, publicKey), serviceConfig)
}

// testAccServiceBaseConfig returns network and SSH key configuration for ClickHouse and GitLab service tests.
func testAccServiceBaseConfig(serviceName, keyName, publicKey string) string {
	if vpcID, subnetID := os.Getenv("TF_ACC_PAAS_VPC_ID"), os.Getenv("TF_ACC_PAAS_SUBNET_ID"); vpcID != "" && subnetID != "" {
		return fmt.Sprintf(`
data "aws_vpc" "test" {
  id = %[1]q
}

data "aws_subnet" "test" {
  id     = %[2]q
  vpc_id = data.aws_vpc.test.id
}

data "aws_security_group" "test" {
  name   = "default"
  vpc_id = data.aws_vpc.test.id
}

resource "aws_key_pair" "test" {
  key_name   = %[3]q
  public_key = %[4]q
}
`, vpcID, subnetID, keyName, publicKey)
	}
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

resource "aws_nat_gateway" "test" {
  depends_on = [aws_internet_gateway.test]

  vpc_id = aws_vpc.test.id

  tags = {
    Name = %[1]q
  }
}

resource "aws_route" "test" {
  route_table_id         = aws_vpc.test.main_route_table_id
  destination_cidr_block = "0.0.0.0/0"
  nat_gateway_id         = aws_nat_gateway.test.id
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

func TestAccPaaSServiceClickHouse_highAvailability(t *testing.T) {
	for _, testCase := range []struct {
		name                    string
		placement               string
		instances               string
		hasDedicatedCoordinator bool
	}{
		{
			name:      "combined",
			placement: "additional_roles = [\"coordinator\"]\n  high_availability = true\n  instance_qty = 3",
			instances: "3",
		},
		{
			name: "dedicated",
			placement: `high_availability = true
  instance_qty = 6
  coordinator {
    instance_type = "c5.large"
    root_volume_type = "gp2"
    root_volume_size = 32
  }`,
			instances:               "6",
			hasDedicatedCoordinator: true,
		},
	} {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			serviceName := fmt.Sprintf("terraform-test-%s", sdkacctest.RandString(5))
			keyName := fmt.Sprintf("terraform-test-%s", sdkacctest.RandString(5))
			publicKey, _, err := sdkacctest.RandSSHKeyPair(acctest.DefaultEmailAddress)
			if err != nil {
				t.Fatalf("error generating random SSH key: %s", err)
			}
			var serviceID string
			config := testAccServiceClickHouseCustomConfig(serviceName, keyName, publicKey, testCase.placement, "")
			steps := []resource.TestStep{
				{
					Config: config,
					Check: resource.ComposeTestCheckFunc(
						testAccCheckServiceExists("aws_paas_service.test"),
						testAccCheckClickHouseResourceIDUnchanged("aws_paas_service.test", &serviceID),
						resource.TestCheckResourceAttr("aws_paas_service.test", "high_availability", "true"),
						resource.TestCheckResourceAttr("aws_paas_service.test", "instance_qty", testCase.instances),
						resource.TestCheckResourceAttr("aws_paas_service.test", "instance_type", "c5.large"),
						resource.TestCheckResourceAttr("aws_paas_service.test", "data_volume.0.size", "32"),
						resource.TestCheckResourceAttr("aws_paas_service.test", "status", tfpaas.ServiceStatusReady),
					),
				},
				{
					ResourceName:            "aws_paas_service.test",
					ImportState:             true,
					ImportStateVerify:       true,
					ImportStateVerifyIgnore: []string{"delete_interfaces_on_destroy"},
				},
			}
			if testCase.hasDedicatedCoordinator {
				steps = append(steps,
					resource.TestStep{
						Config: testAccServiceClickHouseCustomConfigWithOptions(serviceName, keyName, publicKey, testCase.placement, "", testAccServiceClickHouseConfigOptions{
							instanceType: "c5.xlarge", dataVolumeSize: 32,
						}),
						Check: resource.ComposeTestCheckFunc(
							testAccCheckServiceExists("aws_paas_service.test"),
							testAccCheckClickHouseResourceIDUnchanged("aws_paas_service.test", &serviceID),
							resource.TestCheckResourceAttr("aws_paas_service.test", "instance_type", "c5.xlarge"),
							resource.TestCheckResourceAttr("aws_paas_service.test", "coordinator.0.instance_type", "c5.large"),
							resource.TestCheckResourceAttr("aws_paas_service.test", "data_volume.0.size", "32"),
							resource.TestCheckResourceAttr("aws_paas_service.test", "status", tfpaas.ServiceStatusReady),
						),
					},
					resource.TestStep{
						Config: testAccServiceClickHouseCustomConfigWithOptions(serviceName, keyName, publicKey, testCase.placement, "", testAccServiceClickHouseConfigOptions{
							instanceType: "c5.xlarge", dataVolumeSize: 40,
						}),
						Check: resource.ComposeTestCheckFunc(
							testAccCheckServiceExists("aws_paas_service.test"),
							testAccCheckClickHouseResourceIDUnchanged("aws_paas_service.test", &serviceID),
							resource.TestCheckResourceAttr("aws_paas_service.test", "instance_type", "c5.xlarge"),
							resource.TestCheckResourceAttr("aws_paas_service.test", "coordinator.0.instance_type", "c5.large"),
							resource.TestCheckResourceAttr("aws_paas_service.test", "data_volume.0.size", "40"),
							resource.TestCheckResourceAttr("aws_paas_service.test", "status", tfpaas.ServiceStatusReady),
						),
					},
				)
			}
			resource.ParallelTest(t, resource.TestCase{
				PreCheck:          func() { testAccServicePreCheck(t) },
				ErrorCheck:        acctest.ErrorCheck(t, paas.EndpointsID),
				ProviderFactories: acctest.ProviderFactories,
				CheckDestroy:      testAccCheckServiceDestroy,
				Steps:             steps,
			})
		})
	}
}
