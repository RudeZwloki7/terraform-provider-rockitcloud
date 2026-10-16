package paas_test

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go/service/paas"
	sdkacctest "github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
)

func TestAccPaaSServicePostgreSQL_roles(t *testing.T) {
	const resourceName = "aws_paas_service.test"
	name := "tf-pgsql-" + sdkacctest.RandString(5)
	publicKey, _, err := sdkacctest.RandSSHKeyPair(acctest.DefaultEmailAddress)
	if err != nil {
		t.Fatal(err)
	}
	var serviceID string
	var steps []resource.TestStep
	for _, roles := range [][]string{
		nil,
		{"pg_monitor"},
		{"pg_monitor", "pg_read_all_settings", "pg_read_all_stats", "pg_stat_scan_tables", "pg_signal_backend", "pg_maintain", "pg_create_subscription"},
		{"pg_signal_backend"},
		{},
		{"pg_read_all_settings"},
		nil,
	} {
		attributes := map[string]string{"name": "application", "roles.#": strconv.Itoa(len(roles))}
		for _, role := range roles {
			attributes["roles."+strconv.Itoa(schema.HashSchema(&schema.Schema{Type: schema.TypeString})(role))] = role
		}
		steps = append(steps, resource.TestStep{
			Config: testAccServicePostgreSQLRolesConfig(name, publicKey, roles),
			Check: resource.ComposeTestCheckFunc(
				testAccCheckServiceExists(resourceName),
				resource.TestCheckResourceAttrWith(resourceName, "id", func(id string) error {
					if serviceID != "" && id != serviceID {
						return fmt.Errorf("PostgreSQL service replaced: %s -> %s", serviceID, id)
					}
					serviceID = id
					return nil
				}),
				resource.TestCheckResourceAttr(resourceName, "pgsql.0.user.0.roles.#", strconv.Itoa(len(roles))),
				resource.TestCheckTypeSetElemNestedAttrs("data.aws_paas_service.test", "pgsql.0.user.*", attributes),
			),
		})
	}
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccServicePreCheck(t) },
		ErrorCheck:        acctest.ErrorCheck(t, paas.EndpointsID),
		ProviderFactories: acctest.ProviderFactories,
		CheckDestroy:      testAccCheckServiceDestroy,
		Steps:             steps,
	})
}

func testAccServicePostgreSQLRolesConfig(name, publicKey string, roles []string) string {
	roleArgument := ""
	if roles != nil {
		encoded, _ := json.Marshal(roles)
		roleArgument = "roles = " + string(encoded)
	}
	return testAccServiceConfigCompose(name, name, publicKey, fmt.Sprintf(`
resource "aws_paas_service" "test" {
  depends_on = [aws_route.test]
  name          = %[1]q
  instance_type = "c5.large"
  arbitrator_required = false
  root_volume {
    type = "gp2"
    size = 32
  }
  data_volume {
    type = "gp2"
    size = 32
  }
  delete_interfaces_on_destroy = true
  security_group_ids = [aws_vpc.test.default_security_group_id]
  subnet_ids = [aws_subnet.test.id]
  ssh_key_name = aws_key_pair.test.key_name
  pgsql {
    version = "18"
    user {
      name = "application"
      password = "Test_pgsql_password1"
      %[2]s
    }
    database {
      name = "appdb"
      owner = "application"
    }
  }
}
data "aws_paas_service" "test" {
  id = aws_paas_service.test.id
}
`, name, roleArgument))
}
