package paas_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/paas"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/hashicorp/aws-sdk-go-base/v2/awsv1shim/v2/tfawserr"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	sdkacctest "github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	tfpaas "github.com/hashicorp/terraform-provider-aws/internal/service/paas"
	tfs3 "github.com/hashicorp/terraform-provider-aws/internal/service/s3"
)

func TestAccPaaSServiceGitLab_registryS3(t *testing.T) {
	const (
		resourceName   = "aws_paas_service.test"
		dataSourceName = "data.aws_paas_service.test"
		bucketResource = "aws_s3_bucket.registry"
	)
	userLogin := strings.TrimSpace(os.Getenv("TF_ACC_PAAS_GITLAB_S3_USER"))
	serviceName := fmt.Sprintf("terraform-test-%s", sdkacctest.RandString(5))
	keyName := fmt.Sprintf("terraform-test-%s", sdkacctest.RandString(5))
	bucketName := fmt.Sprintf("terraform-test-gitlab-%s", sdkacctest.RandString(12))
	publicKey, _, err := sdkacctest.RandSSHKeyPair(acctest.DefaultEmailAddress)
	if err != nil {
		t.Fatalf("error generating random SSH key: %s", err)
	}
	config := func(storage string) string {
		return testAccServiceGitLabRegistryConfig(serviceName, keyName, publicKey, bucketName, userLogin, storage)
	}
	var serviceID string
	commonChecks := func() resource.TestCheckFunc {
		return resource.ComposeTestCheckFunc(
			testAccCheckServiceExists(resourceName),
			testAccCheckGitLabServiceIDUnchanged(resourceName, &serviceID),
			resource.TestCheckResourceAttr(resourceName, "status", tfpaas.ServiceStatusReady),
			resource.TestCheckResourceAttr(resourceName, "service_type", "gitlab"),
			resource.TestCheckResourceAttr(resourceName, "gitlab.0.password", "Test_password1"),
			resource.TestCheckResourceAttr(bucketResource, "bucket", bucketName),
			resource.TestCheckResourceAttrPair(dataSourceName, "id", resourceName, "id"),
		)
	}
	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			if userLogin == "" {
				t.Skip("TF_ACC_PAAS_GITLAB_S3_USER must identify an existing eligible PaaS backup user")
			}
			testAccServicePreCheck(t)
		},
		ErrorCheck:        acctest.ErrorCheck(t, paas.EndpointsID),
		ProviderFactories: acctest.ProviderFactories,
		CheckDestroy:      testAccCheckGitLabRegistryDestroy,
		Steps: []resource.TestStep{
			{
				Config: config("s3"),
				Check: resource.ComposeTestCheckFunc(
					commonChecks(),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.registry.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.registry.0.storage", "s3"),
					resource.TestCheckResourceAttrPair(resourceName, "gitlab.0.registry.0.bucket", bucketResource, "bucket"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.registry.0.s3_user", userLogin),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.registry.0.url", "https://s3.ru-msk.k2.cloud"),
					resource.TestCheckResourceAttr(dataSourceName, "gitlab.0.registry.0.storage", "s3"),
					resource.TestCheckResourceAttrPair(dataSourceName, "gitlab.0.registry.0.bucket", bucketResource, "bucket"),
					resource.TestCheckResourceAttr(dataSourceName, "gitlab.0.registry.0.s3_user", userLogin),
					resource.TestCheckResourceAttrPair(dataSourceName, "gitlab.0.registry.0.url", resourceName, "gitlab.0.registry.0.url"),
				),
			},
			{
				ResourceName:            resourceName,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"delete_interfaces_on_destroy", "gitlab.0.password"},
			},
			{
				Config:   config("s3"),
				PlanOnly: true,
			},
			{
				Config: config("disabled"),
				Check: resource.ComposeTestCheckFunc(
					commonChecks(),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.registry.#", "0"),
					resource.TestCheckResourceAttr(dataSourceName, "gitlab.0.registry.#", "0"),
				),
			},
			{
				Config:   config("disabled"),
				PlanOnly: true,
			},
			{
				Config: config("filesystem"),
				Check: resource.ComposeTestCheckFunc(
					commonChecks(),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.registry.0.storage", "filesystem"),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.registry.0.bucket", ""),
					resource.TestCheckResourceAttr(resourceName, "gitlab.0.registry.0.s3_user", ""),
					resource.TestCheckResourceAttr(dataSourceName, "gitlab.0.registry.0.storage", "filesystem"),
				),
			},
			{
				Config:   config("filesystem"),
				PlanOnly: true,
			},
		},
	})
}

func testAccServiceGitLabRegistryConfig(serviceName, keyName, publicKey, bucketName, userLogin, storage string) string {
	parameters := ""
	if storage == "s3" {
		parameters = fmt.Sprintf(`
    registry {
      storage = "s3"
      bucket  = aws_s3_bucket.registry.bucket
      s3_user = %q
      url     = "https://s3.ru-msk.k2.cloud"
    }
`, userLogin)
	} else if storage == "filesystem" {
		parameters = `
    registry {
      storage = "filesystem"
    }
`
	}
	configuration := testAccServiceGitLabCustomConfig(serviceName, keyName, publicKey, parameters)
	// Keep this dependency after S3 is disabled, so teardown waits for PaaS
	// deletion before removing the owned bucket. Preserve the created-network
	// route dependency; the shared helper already removes it for network reuse.
	withBucketDependency := strings.Replace(configuration, "depends_on = [aws_route.test]", "depends_on = [aws_route.test, aws_s3_bucket.registry]", 1)
	if withBucketDependency == configuration {
		withBucketDependency = strings.Replace(configuration, `resource "aws_paas_service" "test" {`, `resource "aws_paas_service" "test" {
  depends_on = [aws_s3_bucket.registry]`, 1)
	}
	return acctest.ConfigCompose(withBucketDependency, fmt.Sprintf(`
resource "aws_s3_bucket" "registry" {
  bucket        = %q
  force_destroy = true
}
`, bucketName))
}

func testAccCheckGitLabRegistryDestroy(state *terraform.State) error {
	if err := testAccCheckServiceDestroy(state); err != nil {
		return err
	}
	bucket, exists := state.RootModule().Resources["aws_s3_bucket.registry"]
	if !exists || bucket.Primary == nil || bucket.Primary.ID == "" {
		return nil
	}
	conn := acctest.Provider.Meta().(*conns.AWSClient).S3Conn
	_, err := conn.HeadBucketWithContext(context.Background(), &s3.HeadBucketInput{Bucket: aws.String(bucket.Primary.ID)})
	if tfawserr.ErrStatusCodeEquals(err, http.StatusNotFound) || tfawserr.ErrCodeEquals(err, s3.ErrCodeNoSuchBucket, "NotFound") {
		return nil
	}
	if err != nil {
		return fmt.Errorf("checking destruction of GitLab registry bucket (%s): %w", bucket.Primary.ID, err)
	}
	return fmt.Errorf("GitLab registry bucket (%s) still exists", bucket.Primary.ID)
}

func TestPaaSServiceGitLabRegistryS3AcceptanceConfigurations(t *testing.T) {
	publicKey, _, err := sdkacctest.RandSSHKeyPair(acctest.DefaultEmailAddress)
	if err != nil {
		t.Fatalf("error generating random SSH key: %s", err)
	}
	for _, existingNetwork := range []bool{false, true} {
		t.Run(fmt.Sprintf("existing_network_%t", existingNetwork), func(t *testing.T) {
			vpcID, subnetID := "", ""
			if existingNetwork {
				vpcID, subnetID = "vpc-12345678", "subnet-12345678"
			}
			t.Setenv("TF_ACC_PAAS_VPC_ID", vpcID)
			t.Setenv("TF_ACC_PAAS_SUBNET_ID", subnetID)
			service := tfpaas.ResourceService()
			var previous *terraform.InstanceState
			for _, storage := range []string{"s3", "disabled", "filesystem"} {
				t.Run(storage, func(t *testing.T) {
					configuration := testAccServiceGitLabRegistryConfig("terraform-test-s3qa", "terraform-test-s3key", publicKey, "terraform-test-gitlab-registry", "eligible-registry@c2dev", storage)
					config := terraform.NewResourceConfigRaw(testGitLabServiceConfigFromHCL(t, configuration))
					if diagnostics := service.Validate(config); diagnostics.HasError() {
						t.Fatalf("invalid GitLab S3 acceptance schema: %v", diagnostics)
					}
					if diff, err := service.Diff(context.Background(), previous, config, nil); err != nil || diff == nil || previous != nil && diff.RequiresNew() {
						t.Fatalf("GitLab registry transition must apply without replacement: %v", err)
					}
					data := schema.TestResourceDataRaw(t, service.Schema, config.Raw)
					data.SetId("fm-cluster-12345678")
					previous = data.State()
					testGitLabRegistryBucketDependency(t, configuration, existingNetwork)
				})
			}
		})
	}
}

func testGitLabRegistryBucketDependency(t *testing.T, configuration string, existingNetwork bool) {
	t.Helper()
	file, diagnostics := hclsyntax.ParseConfig([]byte(configuration), "gitlab-registry.tf", hcl.InitialPos)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	foundBucket := false
	for _, block := range file.Body.(*hclsyntax.Body).Blocks {
		if block.Type != "resource" {
			continue
		}
		if strings.HasPrefix(block.Labels[0], "aws_iam_") {
			t.Fatal("registry fixture must not manage the existing user's IAM resources")
		}
		if block.Labels[0] == "aws_s3_bucket" {
			foundBucket = true
			raw := testGitLabHCLBody(t, block.Body, nil)
			if raw["force_destroy"] != true {
				t.Fatal("owned registry bucket must be cleaned up on teardown")
			}
			if diagnostics := tfs3.ResourceBucket().Validate(terraform.NewResourceConfigRaw(raw)); diagnostics.HasError() {
				t.Fatalf("invalid owned bucket schema: %v", diagnostics)
			}
		}
		if block.Labels[0] == "aws_paas_service" {
			dependency := block.Body.Attributes["depends_on"]
			if dependency == nil {
				t.Fatal("PaaS service must retain an explicit bucket dependency in all stages")
			}
			addresses := map[string]bool{}
			for _, traversal := range dependency.Expr.Variables() {
				if len(traversal) >= 2 {
					if attribute, ok := traversal[1].(hcl.TraverseAttr); ok {
						addresses[traversal.RootName()+"."+attribute.Name] = true
					}
				}
			}
			if !addresses["aws_s3_bucket.registry"] || addresses["aws_route.test"] == existingNetwork {
				t.Fatalf("incorrect bucket/route cleanup dependencies: %v", addresses)
			}
		}
	}
	if !foundBucket {
		t.Fatal("registry fixture must own a dedicated bucket")
	}
}
