package paas_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/service/paas"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	sdkacctest "github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	tfpaas "github.com/hashicorp/terraform-provider-aws/internal/service/paas"
	"github.com/zclconf/go-cty/cty"
)

func TestAccPaaSServiceGitLab_basic(t *testing.T) {
	resourceName := "aws_paas_service.test"
	dataSourceName := "data.aws_paas_service.test"
	randServiceName := fmt.Sprintf("terraform-test-%s", sdkacctest.RandString(5))
	randKeyName := fmt.Sprintf("terraform-test-%s", sdkacctest.RandString(5))
	publicKey, _, err := sdkacctest.RandSSHKeyPair(acctest.DefaultEmailAddress)
	if err != nil {
		t.Fatalf("error generating random SSH key: %s", err)
	}

	initialConfig := testAccServiceGitLabConfig(randServiceName, randKeyName, publicKey)
	updatedConfig := testAccServiceGitLabUpdatedConfig(randServiceName, randKeyName, publicKey)
	resetConfig := testAccServiceGitLabResetConfig(randServiceName, randKeyName, publicKey)
	emptyConfig := testAccServiceGitLabCustomConfig(randServiceName, randKeyName, publicKey, "")
	registryConfig := testAccServiceGitLabCustomConfig(randServiceName, randKeyName, publicKey, `
    registry {
      storage = "filesystem"
    }
`)

	var serviceID string
	commonChecks := func() resource.TestCheckFunc {
		return resource.ComposeTestCheckFunc(
			testAccCheckServiceExists(resourceName),
			testAccCheckGitLabServiceIDUnchanged(resourceName, &serviceID),
			resource.TestCheckResourceAttr(resourceName, "gitlab.#", "1"),
			resource.TestCheckResourceAttr(resourceName, "gitlab.0.class", "development"),
			resource.TestCheckResourceAttrSet(resourceName, "gitlab.0.external_url"),
			resource.TestCheckResourceAttr(resourceName, "gitlab.0.password", "Test_password1"),
			resource.TestCheckResourceAttr(resourceName, "high_availability", "false"),
			resource.TestCheckResourceAttr(resourceName, "service_class", "development"),
			resource.TestCheckResourceAttr(resourceName, "service_type", "gitlab"),
			resource.TestCheckResourceAttr(resourceName, "status", tfpaas.ServiceStatusReady),
			resource.TestCheckResourceAttrPair(dataSourceName, "id", resourceName, "id"),
			resource.TestCheckResourceAttrPair(dataSourceName, "gitlab.0.external_url", resourceName, "gitlab.0.external_url"),
			resource.TestCheckResourceAttr(dataSourceName, "service_type", "gitlab"),
		)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:          func() { testAccServicePreCheck(t) },
		ErrorCheck:        acctest.ErrorCheck(t, paas.EndpointsID),
		ProviderFactories: acctest.ProviderFactories,
		CheckDestroy:      testAccCheckServiceDestroy,
		Steps: []resource.TestStep{
			{
				Config: initialConfig,
				Check: resource.ComposeTestCheckFunc(
					commonChecks(),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.ldap.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.oidc.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.registry.0.storage", "filesystem"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.smtp.0.address", "smtp.gitlab.invalid"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.smtp.0.authentication", "plain"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.smtp.0.port", "587"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.smtp.0.password", "SMTP_password1"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.smtp.0.tls", "false"),
					resource.TestCheckResourceAttr(dataSourceName, "gitlab.0.smtp.0.address", "smtp.gitlab.invalid"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"delete_interfaces_on_destroy", "gitlab.0.password", "gitlab.0.smtp.0.password"},
			},
			{
				Config:   initialConfig,
				PlanOnly: true,
			},
			{
				Config: updatedConfig,
				Check: resource.ComposeTestCheckFunc(
					commonChecks(),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.smtp.0.address", "smtp-updated.gitlab.invalid"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.smtp.0.authentication", "login"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.smtp.0.password", "SMTP_password2"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.smtp.0.starttls_auto", "true"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.ldap.0.host", "ldap.gitlab.invalid"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.ldap.0.uid", "uid"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.ldap.0.timeout", "20"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.ldap.0.allow_username_or_email_login", "true"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.ldap.0.retry_empty_result_with_codes.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.oidc.0.issuer", "https://oidc.gitlab.invalid"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.oidc.0.pkce", "true"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.oidc.0.send_scope_to_token_endpoint", "true"),
					resource.TestCheckResourceAttr(dataSourceName, "gitlab.0.ldap.0.host", "ldap.gitlab.invalid"),
					resource.TestCheckResourceAttr(dataSourceName, "gitlab.0.oidc.0.issuer", "https://oidc.gitlab.invalid"),
					resource.TestCheckResourceAttr(dataSourceName, "gitlab.0.smtp.0.address", "smtp-updated.gitlab.invalid"),
				),
			},
			{
				Config:   updatedConfig,
				PlanOnly: true,
			},
			{
				Config: resetConfig,
				Check: resource.ComposeTestCheckFunc(
					commonChecks(),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.smtp.0.authentication", "plain"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.smtp.0.username", ""),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.smtp.0.password", ""),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.smtp.0.domain", ""),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.smtp.0.email_from", ""),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.smtp.0.email_reply_to", ""),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.smtp.0.force_tls", "false"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.smtp.0.starttls_auto", "false"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.ldap.0.bind_dn", ""),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.ldap.0.password", ""),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.ldap.0.user_filter", ""),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.ldap.0.retry_empty_result_with_codes.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.ldap.0.timeout", "0"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.ldap.0.active_directory", "false"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.ldap.0.allow_username_or_email_login", "false"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.ldap.0.lowercase_usernames", "false"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.ldap.0.verify_certificates", "false"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.oidc.0.client_auth_method", ""),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.oidc.0.client_id", ""),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.oidc.0.client_secret", ""),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.oidc.0.pkce", "false"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.oidc.0.send_scope_to_token_endpoint", "false"),
					resource.TestCheckResourceAttr(dataSourceName, "gitlab.0.ldap.0.timeout", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "gitlab.0.oidc.0.send_scope_to_token_endpoint", "false"),
				),
			},
			{
				Config:   resetConfig,
				PlanOnly: true,
			},
			{
				Config: emptyConfig,
				Check: resource.ComposeTestCheckFunc(
					commonChecks(),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.ldap.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.oidc.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.registry.#", "0"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.smtp.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "gitlab.0.ldap.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "gitlab.0.oidc.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "gitlab.0.registry.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "gitlab.0.smtp.#", "0"),
				),
			},
			{
				Config: registryConfig,
				Check: resource.ComposeTestCheckFunc(
					commonChecks(),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.registry.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.registry.0.storage", "filesystem"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.smtp.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "gitlab.0.registry.0.storage", "filesystem"),
				),
			},
			{
				Config:   registryConfig,
				PlanOnly: true,
			},
		},
	})
}

func testAccServiceGitLabConfig(serviceName, keyName, publicKey string) string {
	return testAccServiceGitLabCustomConfig(serviceName, keyName, publicKey, `
    registry {
      storage = "filesystem"
    }
    smtp {
      address        = "smtp.gitlab.invalid"
      port           = 587
      authentication = "plain"
      username       = "gitlab"
      password       = "SMTP_password1"
    }
`)
}

// These settings exercise the PaaS control plane without contacting third-party
// mail or identity providers. They do not test mail delivery or external login.
func testAccServiceGitLabUpdatedConfig(serviceName, keyName, publicKey string) string {
	return testAccServiceGitLabCustomConfig(serviceName, keyName, publicKey, `
    registry {
      storage = "filesystem"
    }
    smtp {
      address         = "smtp-updated.gitlab.invalid"
      port            = 587
      authentication  = "login"
      username        = "gitlab-updated"
      password        = "SMTP_password2"
      domain          = "gitlab.invalid"
      email_from      = "gitlab@gitlab.invalid"
      email_reply_to  = "reply@gitlab.invalid"
      force_tls       = true
      starttls_auto   = true
      tls_verify_mode = "peer"
    }
    ldap {
      host                          = "ldap.gitlab.invalid"
      port                          = 389
      encryption                    = "plain"
      label                         = "Acceptance LDAP"
      uid                           = "uid"
      base                          = "ou=people,dc=gitlab,dc=invalid"
      bind_dn                       = "cn=gitlab,dc=gitlab,dc=invalid"
      password                      = "LDAP_password1"
      user_filter                   = "(objectClass=inetOrgPerson)"
      timeout                       = 20
      active_directory              = false
      allow_username_or_email_login = true
      lowercase_usernames           = true
      verify_certificates           = false
      retry_empty_result_with_codes = [80, 81]
    }
    oidc {
      issuer                      = "https://oidc.gitlab.invalid"
      label                       = "Acceptance OIDC"
      client_auth_method          = "basic"
      client_id                   = "gitlab-acceptance"
      client_secret               = "OIDC_secret1"
      uid_field                   = "sub"
      pkce                        = true
      send_scope_to_token_endpoint = true
    }
`)
}

func testAccServiceGitLabResetConfig(serviceName, keyName, publicKey string) string {
	return testAccServiceGitLabCustomConfig(serviceName, keyName, publicKey, `
    registry {
      storage = "filesystem"
    }
    smtp {
      address        = "smtp.gitlab.invalid"
      port           = 587
      authentication = "plain"
      tls            = false
      force_tls      = false
      starttls_auto  = false
    }
    ldap {
      host                          = "ldap.gitlab.invalid"
      port                          = 389
      encryption                    = "plain"
      label                         = "Acceptance LDAP"
      uid                           = "uid"
      base                          = "ou=people,dc=gitlab,dc=invalid"
      timeout                       = 0
      active_directory              = false
      allow_username_or_email_login = false
      lowercase_usernames           = false
      verify_certificates           = false
    }
    oidc {
      issuer                      = "https://oidc.gitlab.invalid"
      pkce                        = false
      send_scope_to_token_endpoint = false
    }
`)
}

func testAccServiceGitLabCustomConfig(serviceName, keyName, publicKey, parameters string) string {
	return testAccServiceConfigCompose(serviceName, keyName, publicKey, fmt.Sprintf(`
resource "aws_paas_service" "test" {
  depends_on = [aws_route.test]

  name          = %[1]q
  instance_type = "m5.large"

  root_volume {
    type = "gp2"
    size = 32
  }

  data_volume {
    type = "gp2"
    size = 64
  }

  delete_interfaces_on_destroy = true
  security_group_ids           = [aws_vpc.test.default_security_group_id]
  subnet_ids                   = [aws_subnet.test.id]
  ssh_key_name                 = aws_key_pair.test.key_name

  gitlab {
    password = "Test_password1"
    %[2]s
  }
}

data "aws_paas_service" "test" {
  id = aws_paas_service.test.id
}
`, serviceName, parameters))
}

func testAccCheckGitLabServiceIDUnchanged(resourceName string, previousID *string) resource.TestCheckFunc {
	return resource.TestCheckResourceAttrWith(resourceName, "id", func(id string) error {
		if id == "" {
			return fmt.Errorf("GitLab service has no ID in state")
		}
		if *previousID == "" {
			*previousID = id
		} else if id != *previousID {
			return fmt.Errorf("GitLab service was unexpectedly replaced: %s -> %s", *previousID, id)
		}
		return nil
	})
}

func TestPaaSServiceGitLabAcceptanceConfigurations(t *testing.T) {
	publicKey, _, err := sdkacctest.RandSSHKeyPair(acctest.DefaultEmailAddress)
	if err != nil {
		t.Fatalf("error generating random SSH key: %s", err)
	}
	for _, existingNetwork := range []bool{false, true} {
		t.Run(fmt.Sprintf("existing_network_%t", existingNetwork), func(t *testing.T) {
			t.Setenv("TF_ACC_PAAS_VPC_ID", "")
			t.Setenv("TF_ACC_PAAS_SUBNET_ID", "")
			if existingNetwork {
				t.Setenv("TF_ACC_PAAS_VPC_ID", "vpc-12345678")
				t.Setenv("TF_ACC_PAAS_SUBNET_ID", "subnet-12345678")
			}
			var previousState *terraform.InstanceState
			for _, testCase := range []struct {
				name   string
				config string
			}{
				{"create", testAccServiceGitLabConfig("tf-gitlab-check", "gitlab-check", publicKey)},
				{"update", testAccServiceGitLabUpdatedConfig("tf-gitlab-check", "gitlab-check", publicKey)},
				{"reset", testAccServiceGitLabResetConfig("tf-gitlab-check", "gitlab-check", publicKey)},
				{"disabled", testAccServiceGitLabCustomConfig("tf-gitlab-check", "gitlab-check", publicKey, "")},
				{"registry_reenabled", testAccServiceGitLabCustomConfig("tf-gitlab-check", "gitlab-check", publicKey, `registry { storage = "filesystem" }`)},
			} {
				t.Run(testCase.name, func(t *testing.T) {
					config := testGitLabServiceConfigFromHCL(t, testCase.config)
					serviceResource := tfpaas.ResourceService()
					resourceConfig := terraform.NewResourceConfigRaw(config)
					if diagnostics := serviceResource.Validate(resourceConfig); diagnostics.HasError() {
						t.Fatalf("invalid acceptance resource configuration: %v", diagnostics)
					}
					if _, err := serviceResource.Diff(context.Background(), nil, resourceConfig, nil); err != nil {
						t.Fatalf("planning acceptance resource configuration: %s", err)
					}
					if previousState != nil {
						diff, err := serviceResource.Diff(context.Background(), previousState, resourceConfig, nil)
						if err != nil {
							t.Fatalf("planning acceptance resource update: %s", err)
						}
						if diff == nil || diff.Empty() || diff.RequiresNew() {
							t.Fatalf("acceptance update must change the existing service in place: %#v", diff)
						}
					}
					data := schema.TestResourceDataRaw(t, serviceResource.Schema, config)
					data.SetId("fm-cluster-12345678")
					// The fixture has not been refreshed from the API. Give this
					// computed collection a known value before checking its plan.
					if err := data.Set("nodes", []interface{}{}); err != nil {
						t.Fatalf("setting fixture nodes: %s", err)
					}
					diff, err := serviceResource.Diff(context.Background(), data.State(), resourceConfig, nil)
					if err != nil {
						t.Fatalf("replanning unchanged acceptance resource configuration: %s", err)
					}
					if diff != nil && !diff.Empty() {
						t.Fatalf("unchanged acceptance resource configuration produced a diff: %#v", diff.Attributes)
					}
					previousState = data.State()
				})
			}
		})
	}
}

// Evaluate fixture references against their declared resource/data addresses, so
// reusing a network cannot leave references to resources that were removed.
func testGitLabServiceConfigFromHCL(t *testing.T, configuration string) map[string]interface{} {
	t.Helper()
	file, diagnostics := hclsyntax.ParseConfig([]byte(configuration), "gitlab.tf", hcl.InitialPos)
	if diagnostics.HasErrors() {
		t.Fatalf("invalid acceptance HCL: %s", diagnostics.Error())
	}
	body := file.Body.(*hclsyntax.Body)
	variables := map[string]cty.Value{}
	resources := map[string]map[string]cty.Value{}
	dataSources := map[string]map[string]cty.Value{}
	for _, block := range body.Blocks {
		if len(block.Labels) != 2 {
			t.Fatalf("unexpected fixture block: %s %v", block.Type, block.Labels)
		}
		addresses := resources
		if block.Type == "data" {
			addresses = dataSources
		}
		if addresses[block.Labels[0]] == nil {
			addresses[block.Labels[0]] = map[string]cty.Value{}
		}
		addresses[block.Labels[0]][block.Labels[1]] = cty.ObjectVal(map[string]cty.Value{
			"id":                        cty.StringVal("fixture-id"),
			"bucket":                    cty.StringVal("fixture-id"),
			"default_security_group_id": cty.StringVal("sg-12345678"),
			"main_route_table_id":       cty.StringVal("rtb-12345678"),
			"key_name":                  cty.StringVal("gitlab-check"),
		})
	}
	for resourceType, instances := range resources {
		variables[resourceType] = cty.ObjectVal(instances)
	}
	dataTypes := map[string]cty.Value{}
	for resourceType, instances := range dataSources {
		dataTypes[resourceType] = cty.ObjectVal(instances)
	}
	variables["data"] = cty.ObjectVal(dataTypes)
	evaluationContext := &hcl.EvalContext{Variables: variables}
	for _, block := range body.Blocks {
		if block.Type == "data" && strings.Join(block.Labels, ".") == "aws_paas_service.test" {
			config := terraform.NewResourceConfigRaw(testGitLabHCLBody(t, block.Body, evaluationContext))
			if diagnostics := tfpaas.DataSourceService().Validate(config); diagnostics.HasError() {
				t.Fatalf("invalid acceptance data source configuration: %v", diagnostics)
			}
		}
	}
	for _, block := range body.Blocks {
		if block.Type == "resource" && strings.Join(block.Labels, ".") == "aws_paas_service.test" {
			return testGitLabHCLBody(t, block.Body, evaluationContext)
		}
	}
	t.Fatal("GitLab acceptance fixture has no service resource")
	return nil
}

func testGitLabHCLBody(t *testing.T, body *hclsyntax.Body, evaluationContext *hcl.EvalContext) map[string]interface{} {
	t.Helper()
	configuration := map[string]interface{}{}
	for name, attribute := range body.Attributes {
		value, diagnostics := attribute.Expr.Value(evaluationContext)
		if diagnostics.HasErrors() {
			t.Fatalf("invalid fixture attribute %s: %s", name, diagnostics.Error())
		}
		if name != "depends_on" {
			configuration[name] = testGitLabHCLValue(value)
		}
	}
	for _, block := range body.Blocks {
		blocks, _ := configuration[block.Type].([]interface{})
		configuration[block.Type] = append(blocks, testGitLabHCLBody(t, block.Body, evaluationContext))
	}
	return configuration
}

func testGitLabHCLValue(value cty.Value) interface{} {
	if value.IsNull() {
		return nil
	}
	switch value.Type() {
	case cty.String:
		return value.AsString()
	case cty.Bool:
		return value.True()
	case cty.Number:
		integer, _ := value.AsBigFloat().Int64()
		return int(integer)
	default:
		values := []interface{}{}
		for iterator := value.ElementIterator(); iterator.Next(); {
			_, element := iterator.Element()
			values = append(values, testGitLabHCLValue(element))
		}
		return values
	}
}
