package paas_test

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/service/paas"
	sdkacctest "github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	tfpaas "github.com/hashicorp/terraform-provider-aws/internal/service/paas"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
)

func TestAccPaaSClickHouseResources_basic(t *testing.T) {
	profileResourceName := "aws_paas_clickhouse_profile.test"
	quotaResourceName := "aws_paas_clickhouse_quota.test"
	shardResourceName := "aws_paas_clickhouse_shard.test"
	serviceResourceName := "aws_paas_service.test"

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
				Config: testAccClickHouseResourcesConfig(randServiceName, randKeyName, publicKey, 1000),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckClickHouseProfileExists(profileResourceName),
					resource.TestCheckResourceAttr(profileResourceName, "name", "test_profile"),
					resource.TestCheckResourceAttrPair(profileResourceName, "service_id", serviceResourceName, "id"),
					resource.TestCheckResourceAttr(profileResourceName, "settings.%", "1"),
					resource.TestCheckResourceAttr(profileResourceName, "settings.max_threads", "4"),
					testAccCheckClickHouseQuotaExists(quotaResourceName),
					resource.TestCheckResourceAttr(quotaResourceName, "name", "test_quota"),
					resource.TestCheckResourceAttrPair(quotaResourceName, "service_id", serviceResourceName, "id"),
					resource.TestCheckResourceAttr(quotaResourceName, "interval.#", "1"),
					resource.TestCheckResourceAttr(quotaResourceName, "interval.0.duration", "3600"),
					resource.TestCheckResourceAttr(quotaResourceName, "interval.0.queries", "1000"),
					testAccCheckClickHouseShardExists(shardResourceName),
					resource.TestCheckResourceAttr(shardResourceName, "index", "0"),
					resource.TestCheckResourceAttr(shardResourceName, "internal_replication", "false"),
					resource.TestCheckResourceAttr(shardResourceName, "replicas", "1"),
					resource.TestCheckResourceAttrPair(shardResourceName, "service_id", serviceResourceName, "id"),
					resource.TestCheckResourceAttr(shardResourceName, "weight", "1"),
				),
			},
			{
				ResourceName:      profileResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      quotaResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      shardResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccClickHouseResourcesConfig(randServiceName, randKeyName, publicKey, 2000),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckClickHouseQuotaExists(quotaResourceName),
					resource.TestCheckResourceAttr(quotaResourceName, "interval.0.queries", "2000"),
				),
			},
		},
	})
}

func testAccCheckClickHouseProfileExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("PaaS ClickHouse Profile %s is not found in state", resourceName)
		}

		serviceID, id, err := tfpaas.ClickHouseProfileParseResourceID(rs.Primary.ID)
		if err != nil {
			return err
		}

		conn := acctest.Provider.Meta().(*conns.AWSClient).PaaSConn
		_, err = tfpaas.FindClickHouseProfileByID(conn, serviceID, id)

		return err
	}
}

func testAccCheckClickHouseQuotaExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("PaaS ClickHouse Quota %s is not found in state", resourceName)
		}

		serviceID, id, err := tfpaas.ClickHouseQuotaParseResourceID(rs.Primary.ID)
		if err != nil {
			return err
		}

		conn := acctest.Provider.Meta().(*conns.AWSClient).PaaSConn
		_, err = tfpaas.FindClickHouseQuotaByID(conn, serviceID, id)

		return err
	}
}

func testAccCheckClickHouseShardExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("PaaS ClickHouse Shard %s is not found in state", resourceName)
		}

		serviceID, index, err := tfpaas.ClickHouseShardParseResourceID(rs.Primary.ID)
		if err != nil {
			return err
		}

		conn := acctest.Provider.Meta().(*conns.AWSClient).PaaSConn
		_, err = tfpaas.FindClickHouseShardByIndex(conn, serviceID, index)

		if tfresource.NotFound(err) {
			return fmt.Errorf("PaaS ClickHouse Shard (%s) not found", rs.Primary.ID)
		}

		return err
	}
}

func testAccClickHouseResourcesConfig(serviceName, keyName, publicKey string, queries int) string {
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
  }
}

resource "aws_paas_clickhouse_profile" "test" {
  service_id = aws_paas_service.test.id
  name       = "test_profile"

  settings = {
    max_threads = "4"
  }
}

resource "aws_paas_clickhouse_quota" "test" {
  service_id = aws_paas_service.test.id
  name       = "test_quota"

  interval {
    duration = 3600
    queries  = %[2]d
  }
}

resource "aws_paas_clickhouse_shard" "test" {
  service_id = aws_paas_service.test.id
}
`, serviceName, queries))
}
