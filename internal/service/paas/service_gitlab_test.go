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

func TestAccPaaSServiceGitLab_basic(t *testing.T) {
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
				Config: testAccServiceGitLabConfig(randServiceName, randKeyName, publicKey),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckServiceExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "gitlab.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.class", "development"),
					resource.TestCheckResourceAttrSet(resourceName, "gitlab.0.external_url"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.ldap.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.oidc.#", "0"),
					resource.TestCheckResourceAttrSet(resourceName, "gitlab.0.password"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.registry.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.registry.0.storage", "filesystem"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.smtp.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.smtp.0.address", "smtp.example.com"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.smtp.0.authentication", "plain"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.smtp.0.port", "587"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.smtp.0.tls", "false"),
					resource.TestCheckResourceAttr(resourceName, "service_class", "development"),
					resource.TestCheckResourceAttr(resourceName, "service_type", "gitlab"),
					resource.TestCheckResourceAttr(resourceName, "status", tfpaas.ServiceStatusReady),
				),
			},
		},
	})
}

func testAccServiceGitLabConfig(serviceName, keyName, publicKey string) string {
	return acctest.ConfigCompose(testAccServiceBaseConfig(serviceName, keyName, publicKey), fmt.Sprintf(`
resource "aws_paas_service" "test" {
  depends_on = ["aws_internet_gateway.test"]

  name          = %[1]q
  instance_type = "c5.xlarge"

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

  gitlab {
    password = "Test_password1"

    registry {
      storage = "filesystem"
    }

    smtp {
      address        = "smtp.example.com"
      port           = 587
      authentication = "plain"
    }
  }
}
`, serviceName))
}
