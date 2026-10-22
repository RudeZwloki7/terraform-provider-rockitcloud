package paas_test

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go/service/paas"
	sdkacctest "github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
)

func TestAccPaaSServiceMySQL_binlogAndGlobalPrivileges(t *testing.T) {
	const resourceName = "aws_paas_service.test"
	name := "tf-mysql-" + sdkacctest.RandString(5)
	publicKey, _, err := sdkacctest.RandSSHKeyPair(acctest.DefaultEmailAddress)
	if err != nil {
		t.Fatal(err)
	}
	var serviceID string
	var steps []resource.TestStep
	for _, testCase := range []struct {
		binlog     string
		days       int
		privileges []string
	}{
		{},
		{binlog: "true", days: 7, privileges: []string{"REPLICATION SLAVE", "REPLICATION CLIENT", "SELECT", "RELOAD", "SHOW DATABASES"}},
		{binlog: "true", days: 3, privileges: []string{"REPLICATION CLIENT"}},
		{binlog: "false", privileges: []string{}},
		{binlog: "true", days: 1, privileges: []string{"SELECT"}},
		{binlog: "true", days: 1},
	} {
		attributes := map[string]string{"name": "application", "global_privileges.#": strconv.Itoa(len(testCase.privileges))}
		checks := []resource.TestCheckFunc{
			testAccCheckServiceExists(resourceName),
			resource.TestCheckResourceAttrWith(resourceName, "id", func(id string) error {
				if serviceID != "" && id != serviceID {
					return fmt.Errorf("MySQL service replaced: %s -> %s", serviceID, id)
				}
				serviceID = id
				return nil
			}),
			resource.TestCheckResourceAttr(resourceName, "mysql.0.user.0.global_privileges.#", strconv.Itoa(len(testCase.privileges))),
			resource.TestCheckTypeSetElemNestedAttrs("data.aws_paas_service.test", "mysql.0.user.*", attributes),
		}
		for _, privilege := range testCase.privileges {
			checks = append(checks, resource.TestCheckTypeSetElemAttr("data.aws_paas_service.test", "mysql.0.user.*.global_privileges.*", privilege))
		}
		if testCase.binlog != "" {
			checks = append(checks,
				resource.TestCheckResourceAttr(resourceName, "mysql.0.bin_log_enabled", testCase.binlog),
				resource.TestCheckResourceAttr("data.aws_paas_service.test", "mysql.0.bin_log_enabled", testCase.binlog),
			)
		}
		if testCase.days != 0 {
			checks = append(checks,
				resource.TestCheckResourceAttr(resourceName, "mysql.0.binlog_retention_days", strconv.Itoa(testCase.days)),
				resource.TestCheckResourceAttr("data.aws_paas_service.test", "mysql.0.binlog_retention_days", strconv.Itoa(testCase.days)),
			)
		}
		steps = append(steps, resource.TestStep{
			Config: testAccServiceMySQLConfig(name, publicKey, testCase.binlog, testCase.days, testCase.privileges),
			Check:  resource.ComposeTestCheckFunc(checks...),
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

func testAccServiceMySQLConfig(name, publicKey, binlog string, days int, privileges []string) string {
	arguments := ""
	if binlog != "" {
		arguments += "bin_log_enabled = " + binlog + "\n"
	}
	// Exercise the API default retention when the binary log is first enabled.
	if days != 0 && days != 7 {
		arguments += fmt.Sprintf("binlog_retention_days = %d\n", days)
	}
	privilegeArgument := ""
	if privileges != nil {
		encoded, _ := json.Marshal(privileges)
		privilegeArgument = "global_privileges = " + string(encoded)
	}
	return testAccServiceConfigCompose(name, name, publicKey, fmt.Sprintf(`
resource "aws_paas_service" "test" {
  depends_on = [aws_route.test]
  name                         = %[1]q
  instance_type                = "c5.large"
  arbitrator_required          = false
  delete_interfaces_on_destroy = true
  security_group_ids           = [aws_vpc.test.default_security_group_id]
  subnet_ids                   = [aws_subnet.test.id]
  ssh_key_name                 = aws_key_pair.test.key_name
  root_volume {
    type = "gp2"
    size = 32
  }
  data_volume {
    type = "gp2"
    size = 32
  }
  mysql {
    vendor  = "mysql"
    version = "8.0"
    %[2]s
    user {
      name     = "application"
      password = "Test_mysql_password1"
      %[3]s
    }
    database {
      name = "appdb"
      user {
        name       = "application"
        privileges = ["SELECT"]
      }
    }
  }
}
data "aws_paas_service" "test" {
  id = aws_paas_service.test.id
}
`, name, arguments, privilegeArgument))
}
