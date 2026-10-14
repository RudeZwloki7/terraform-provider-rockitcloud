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
	var serviceID, userID, profileID, quotaID, shardID string

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccServicePreCheck(t) },
		ErrorCheck:        acctest.ErrorCheck(t, paas.EndpointsID),
		ProviderFactories: acctest.ProviderFactories,
		CheckDestroy:      testAccCheckServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccClickHouseResourcesConfig(randServiceName, randKeyName, publicKey, testAccClickHouseResourcesConfigOptions{
					profileMaxThreads: 4, quotaQueries: 1000, quotaErrors: 100, shardWeight: 1,
				}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceExists(serviceResourceName),
					testAccCheckClickHouseResourceIDUnchanged(serviceResourceName, &serviceID),
					testAccCheckClickHouseUserIDUnchanged(serviceResourceName, &userID),
					testAccCheckClickHouseProfileExists(profileResourceName),
					testAccCheckClickHouseResourceIDUnchanged(profileResourceName, &profileID),
					resource.TestCheckResourceAttr(profileResourceName, "name", "test_profile"),
					resource.TestCheckResourceAttrPair(profileResourceName, "service_id", serviceResourceName, "id"),
					resource.TestCheckResourceAttr(profileResourceName, "settings.%", "1"),
					resource.TestCheckResourceAttr(profileResourceName, "settings.max_threads", "4"),
					testAccCheckClickHouseQuotaExists(quotaResourceName),
					testAccCheckClickHouseResourceIDUnchanged(quotaResourceName, &quotaID),
					resource.TestCheckResourceAttr(quotaResourceName, "name", "test_quota"),
					resource.TestCheckResourceAttrPair(quotaResourceName, "service_id", serviceResourceName, "id"),
					resource.TestCheckResourceAttr(quotaResourceName, "interval.#", "1"),
					resource.TestCheckResourceAttr(quotaResourceName, "interval.0.duration", "3600"),
					resource.TestCheckResourceAttr(quotaResourceName, "interval.0.queries", "1000"),
					resource.TestCheckResourceAttr(quotaResourceName, "interval.0.errors", "100"),
					testAccCheckClickHouseShardExists(shardResourceName),
					testAccCheckClickHouseResourceIDUnchanged(shardResourceName, &shardID),
					resource.TestCheckResourceAttr(shardResourceName, "index", "0"),
					resource.TestCheckResourceAttr(shardResourceName, "internal_replication", "false"),
					resource.TestCheckResourceAttr(shardResourceName, "replicas", "1"),
					resource.TestCheckResourceAttrPair(shardResourceName, "service_id", serviceResourceName, "id"),
					resource.TestCheckResourceAttr(shardResourceName, "weight", "1"),
				),
			},
			{
				ResourceName:            serviceResourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"delete_interfaces_on_destroy", "clickhouse.0.user.0.password"},
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
				Config: testAccClickHouseResourcesConfig(randServiceName, randKeyName, publicKey, testAccClickHouseResourcesConfigOptions{
					profileMaxThreads: 8, quotaQueries: 2000, quotaErrors: 200, shardWeight: 2,
				}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckClickHouseResourceIDUnchanged(serviceResourceName, &serviceID),
					testAccCheckClickHouseUserIDUnchanged(serviceResourceName, &userID),
					testAccCheckClickHouseResourceIDUnchanged(profileResourceName, &profileID),
					testAccCheckClickHouseResourceIDUnchanged(quotaResourceName, &quotaID),
					testAccCheckClickHouseResourceIDUnchanged(shardResourceName, &shardID),
					testAccCheckClickHouseQuotaExists(quotaResourceName),
					resource.TestCheckResourceAttr(quotaResourceName, "interval.0.queries", "2000"),
					resource.TestCheckResourceAttr(quotaResourceName, "interval.0.errors", "200"),
					resource.TestCheckResourceAttr(profileResourceName, "settings.max_threads", "8"),
					resource.TestCheckResourceAttr(shardResourceName, "weight", "2"),
				),
			},
			{
				// Profiles and quotas refer to their parent service. Assign their
				// literal names only after creation to avoid a Terraform graph cycle.
				Config: testAccClickHouseResourcesConfig(randServiceName, randKeyName, publicKey, testAccClickHouseResourcesConfigOptions{
					profileMaxThreads: 8, quotaQueries: 2000, quotaErrors: 200, shardWeight: 2,
					userProfile: "test_profile", userQuota: "test_quota",
				}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckClickHouseResourceIDUnchanged(serviceResourceName, &serviceID),
					testAccCheckClickHouseUserIDUnchanged(serviceResourceName, &userID),
					resource.TestCheckResourceAttr(serviceResourceName, "clickhouse.0.user.0.profile", "test_profile"),
					resource.TestCheckResourceAttr(serviceResourceName, "clickhouse.0.user.0.quota", "test_quota"),
				),
			},
			{
				Config: testAccClickHouseResourcesConfig(randServiceName, randKeyName, publicKey, testAccClickHouseResourcesConfigOptions{
					quotaQueries: 2000, shardWeight: 2, userProfile: "test_profile", userQuota: "test_quota",
				}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckClickHouseResourceIDUnchanged(profileResourceName, &profileID),
					testAccCheckClickHouseResourceIDUnchanged(quotaResourceName, &quotaID),
					testAccCheckClickHouseUserIDUnchanged(serviceResourceName, &userID),
					resource.TestCheckResourceAttr(profileResourceName, "settings.%", "0"),
					resource.TestCheckResourceAttr(quotaResourceName, "interval.0.queries", "2000"),
					resource.TestCheckResourceAttr(quotaResourceName, "interval.0.errors", "0"),
					resource.TestCheckResourceAttr(serviceResourceName, "clickhouse.0.user.0.profile", "test_profile"),
					resource.TestCheckResourceAttr(serviceResourceName, "clickhouse.0.user.0.quota", "test_quota"),
				),
			},
			{
				Config: testAccClickHouseResourcesConfig(randServiceName, randKeyName, publicKey, testAccClickHouseResourcesConfigOptions{
					quotaQueries: 2000, shardWeight: 2,
				}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckClickHouseUserIDUnchanged(serviceResourceName, &userID),
					resource.TestCheckResourceAttr(serviceResourceName, "clickhouse.0.user.0.profile", "default"),
					resource.TestCheckResourceAttr(serviceResourceName, "clickhouse.0.user.0.quota", "default"),
				),
			},
			{
				Config: testAccClickHouseResourcesConfig(randServiceName, randKeyName, publicKey, testAccClickHouseResourcesConfigOptions{
					omitProfile: true, quotaQueries: 2000, shardWeight: 2,
				}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckClickHouseProfileAbsent(profileResourceName, &profileID),
					testAccCheckClickHouseQuotaExists(quotaResourceName),
					testAccCheckClickHouseShardExists(shardResourceName),
					testAccCheckServiceExists(serviceResourceName),
					testAccCheckClickHouseResourceIDUnchanged(serviceResourceName, &serviceID),
					testAccCheckClickHouseUserIDUnchanged(serviceResourceName, &userID),
					resource.TestCheckResourceAttr(serviceResourceName, "status", tfpaas.ServiceStatusReady),
				),
			},
			{
				Config: testAccClickHouseResourcesConfig(randServiceName, randKeyName, publicKey, testAccClickHouseResourcesConfigOptions{
					omitProfile: true, omitQuota: true, shardWeight: 2,
				}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckClickHouseQuotaAbsent(quotaResourceName, &quotaID),
					testAccCheckClickHouseShardExists(shardResourceName),
					testAccCheckServiceExists(serviceResourceName),
					testAccCheckClickHouseResourceIDUnchanged(serviceResourceName, &serviceID),
					testAccCheckClickHouseUserIDUnchanged(serviceResourceName, &userID),
					resource.TestCheckResourceAttr(serviceResourceName, "status", tfpaas.ServiceStatusReady),
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

func testAccCheckClickHouseProfileAbsent(resourceName string, resourceID *string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		if _, exists := state.RootModule().Resources[resourceName]; exists {
			return fmt.Errorf("PaaS ClickHouse Profile %s remains in state", resourceName)
		}
		serviceID, id, err := tfpaas.ClickHouseProfileParseResourceID(*resourceID)
		if err != nil {
			return err
		}
		conn := acctest.Provider.Meta().(*conns.AWSClient).PaaSConn
		_, err = tfpaas.FindClickHouseProfileByID(conn, serviceID, id)
		if !tfresource.NotFound(err) {
			return fmt.Errorf("PaaS ClickHouse Profile (%s) should be absent from the API, got: %v", *resourceID, err)
		}
		return nil
	}
}

func testAccCheckClickHouseQuotaAbsent(resourceName string, resourceID *string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		if _, exists := state.RootModule().Resources[resourceName]; exists {
			return fmt.Errorf("PaaS ClickHouse Quota %s remains in state", resourceName)
		}
		serviceID, id, err := tfpaas.ClickHouseQuotaParseResourceID(*resourceID)
		if err != nil {
			return err
		}
		conn := acctest.Provider.Meta().(*conns.AWSClient).PaaSConn
		_, err = tfpaas.FindClickHouseQuotaByID(conn, serviceID, id)
		if !tfresource.NotFound(err) {
			return fmt.Errorf("PaaS ClickHouse Quota (%s) should be absent from the API, got: %v", *resourceID, err)
		}
		return nil
	}
}

type testAccClickHouseResourcesConfigOptions struct {
	profileMaxThreads int
	quotaQueries      int
	quotaErrors       int
	shardWeight       int
	userProfile       string
	userQuota         string
	omitProfile       bool
	omitQuota         bool
}

func testAccClickHouseResourcesConfig(serviceName, keyName, publicKey string, options testAccClickHouseResourcesConfigOptions) string {
	profile, quota := options.userProfile, options.userQuota
	if profile == "" {
		profile = "default"
	}
	if quota == "" {
		quota = "default"
	}
	config := testAccServiceClickHouseCustomConfig(serviceName, keyName, publicKey, `additional_roles = ["coordinator"]`, fmt.Sprintf(`
    user {
      name     = "test_user"
      password = "Test_password1"
      profile  = %[1]q
      quota    = %[2]q
    }
`, profile, quota))
	if !options.omitProfile {
		settings := ""
		if options.profileMaxThreads > 0 {
			settings = fmt.Sprintf(`settings = { max_threads = "%d" }`, options.profileMaxThreads)
		}
		config = acctest.ConfigCompose(config, fmt.Sprintf(`
resource "aws_paas_clickhouse_profile" "test" {
  service_id = aws_paas_service.test.id
  name       = "test_profile"

  %s
}
`, settings))
	}
	if !options.omitQuota {
		errors := ""
		if options.quotaErrors > 0 {
			errors = fmt.Sprintf("errors = %d", options.quotaErrors)
		}
		config = acctest.ConfigCompose(config, fmt.Sprintf(`
resource "aws_paas_clickhouse_quota" "test" {
  service_id = aws_paas_service.test.id
  name       = "test_quota"

  interval {
    duration = 3600
    queries  = %[1]d
    %[2]s
  }
}
`, options.quotaQueries, errors))
	}
	return acctest.ConfigCompose(config, fmt.Sprintf(`
resource "aws_paas_clickhouse_shard" "test" {
  service_id = aws_paas_service.test.id
  weight     = %d
}
`, options.shardWeight))
}

func TestAccPaaSClickHouseResources_highAvailabilityShard(t *testing.T) {
	serviceResourceName := "aws_paas_service.test"
	shardResourceName := "aws_paas_clickhouse_shard.test"
	serviceName := fmt.Sprintf("terraform-test-%s", sdkacctest.RandString(5))
	keyName := fmt.Sprintf("terraform-test-%s", sdkacctest.RandString(5))
	publicKey, _, err := sdkacctest.RandSSHKeyPair(acctest.DefaultEmailAddress)
	if err != nil {
		t.Fatalf("error generating random SSH key: %s", err)
	}
	var serviceID, shardID string
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccServicePreCheck(t) },
		ErrorCheck:        acctest.ErrorCheck(t, paas.EndpointsID),
		ProviderFactories: acctest.ProviderFactories,
		CheckDestroy:      testAccCheckServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccClickHouseHAShardConfig(serviceName, keyName, publicKey, 1, false),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceExists(serviceResourceName),
					testAccCheckClickHouseResourceIDUnchanged(serviceResourceName, &serviceID),
					testAccCheckClickHouseShardExists(shardResourceName),
					testAccCheckClickHouseResourceIDUnchanged(shardResourceName, &shardID),
					resource.TestCheckResourceAttr(serviceResourceName, "high_availability", "true"),
					resource.TestCheckResourceAttr(serviceResourceName, "instance_qty", "3"),
					resource.TestCheckResourceAttr(shardResourceName, "replicas", "1"),
					resource.TestCheckResourceAttr(shardResourceName, "internal_replication", "false"),
				),
			},
			{
				ResourceName:      shardResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccClickHouseHAShardConfig(serviceName, keyName, publicKey, 2, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckClickHouseResourceIDUnchanged(serviceResourceName, &serviceID),
					testAccCheckClickHouseResourceIDUnchanged(shardResourceName, &shardID),
					testAccCheckClickHouseShardExists(shardResourceName),
					resource.TestCheckResourceAttr(shardResourceName, "replicas", "2"),
					resource.TestCheckResourceAttr(shardResourceName, "internal_replication", "true"),
				),
			},
			{
				Config: testAccClickHouseHAShardConfig(serviceName, keyName, publicKey, 2, false),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckClickHouseResourceIDUnchanged(serviceResourceName, &serviceID),
					testAccCheckClickHouseResourceIDUnchanged(shardResourceName, &shardID),
					testAccCheckClickHouseShardExists(shardResourceName),
					resource.TestCheckResourceAttr(shardResourceName, "replicas", "2"),
					resource.TestCheckResourceAttr(shardResourceName, "internal_replication", "false"),
				),
			},
		},
	})
}

func testAccClickHouseHAShardConfig(serviceName, keyName, publicKey string, replicas int, internalReplication bool) string {
	config := testAccServiceClickHouseCustomConfig(serviceName, keyName, publicKey, `
  additional_roles  = ["coordinator"]
  high_availability = true
  instance_qty      = 3
`, "")
	return acctest.ConfigCompose(config, fmt.Sprintf(`
resource "aws_paas_clickhouse_shard" "test" {
  service_id           = aws_paas_service.test.id
  replicas             = %d
  internal_replication = %t
}
`, replicas, internalReplication))
}
