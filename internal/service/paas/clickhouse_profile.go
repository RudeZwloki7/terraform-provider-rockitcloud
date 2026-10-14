package paas

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/paas"
	"github.com/hashicorp/aws-sdk-go-base/v2/awsv1shim/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/service/paas/services"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
)

func ResourceClickHouseProfile() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceClickHouseProfileCreate,
		ReadContext:   resourceClickHouseProfileRead,
		UpdateContext: resourceClickHouseProfileUpdate,
		DeleteContext: resourceClickHouseProfileDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(30 * time.Minute),
			Update: schema.DefaultTimeout(30 * time.Minute),
			Delete: schema.DefaultTimeout(30 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.All(
					services.ClickHouseNameValidateFunc,
					validation.StringNotInSlice([]string{"default", "readonly"}, true),
				),
			},
			"service_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"settings": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func resourceClickHouseProfileCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).PaaSConn
	serviceID := d.Get("service_id").(string)
	name := d.Get("name").(string)

	input := &paas.CreateClickhouseProfileInput{
		Name: aws.String(name),
		Parameters: &paas.ClickHouseProfileParameters{
			Settings: d.Get("settings").(map[string]interface{}),
		},
		ServiceId: aws.String(serviceID),
	}

	log.Printf("[DEBUG] Creating PaaS ClickHouse Profile: %s", input)
	outputRaw, err := tfresource.RetryWhenAWSErrCodeEqualsContext(ctx, d.Timeout(schema.TimeoutCreate), func() (interface{}, error) {
		return conn.CreateClickhouseProfileWithContext(ctx, input)
	}, OperationNotAllowedCode)

	if err != nil {
		return diag.Errorf("error creating PaaS ClickHouse Profile (%s) in Service (%s): %s", name, serviceID, err)
	}

	output := outputRaw.(*paas.CreateClickhouseProfileOutput)
	if output == nil || output.Profile == nil || aws.StringValue(output.Profile.Id) == "" {
		return diag.Errorf("error creating PaaS ClickHouse Profile (%s) in Service (%s): API returned no profile ID", name, serviceID)
	}
	id := aws.StringValue(output.Profile.Id)
	d.SetId(ClickHouseProfileCreateResourceID(serviceID, id))

	_, err = waitServiceUpdated(ctx, conn, serviceID, d.Timeout(schema.TimeoutCreate))

	if err != nil {
		return diag.Errorf("error waiting for PaaS Service (%s) to update: %s", serviceID, err)
	}

	return resourceClickHouseProfileRead(ctx, d, meta)
}

func resourceClickHouseProfileRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).PaaSConn

	serviceID, id, err := ClickHouseProfileParseResourceID(d.Id())

	if err != nil {
		return diag.FromErr(err)
	}

	profile, err := findClickHouseProfileByID(ctx, conn, serviceID, id)

	if !d.IsNewResource() && tfresource.NotFound(err) {
		log.Printf("[WARN] PaaS ClickHouse Profile (%s) not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}

	if err != nil {
		return diag.Errorf("error reading PaaS ClickHouse Profile (%s): %s", d.Id(), err)
	}

	d.Set("name", profile.Name)
	d.Set("service_id", serviceID)

	var settings map[string]interface{}
	if profile.Parameters != nil {
		settings = profile.Parameters.Settings
	}
	if err := d.Set("settings", flattenClickHouseProfileSettings(settings)); err != nil {
		return diag.Errorf("error setting PaaS ClickHouse Profile (%s) settings: %s", d.Id(), err)
	}

	return nil
}

func resourceClickHouseProfileUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).PaaSConn

	serviceID, id, err := ClickHouseProfileParseResourceID(d.Id())

	if err != nil {
		return diag.FromErr(err)
	}

	if d.HasChange("settings") {
		input := &paas.ModifyClickhouseProfileInput{
			Parameters: &paas.ClickHouseProfileParameters{
				Settings: d.Get("settings").(map[string]interface{}),
			},
			ProfileId: aws.String(id),
			ServiceId: aws.String(serviceID),
		}

		log.Printf("[DEBUG] Modifying PaaS ClickHouse Profile: %s", input)
		_, err := tfresource.RetryWhenAWSErrCodeEqualsContext(ctx, d.Timeout(schema.TimeoutUpdate), func() (interface{}, error) {
			return conn.ModifyClickhouseProfileWithContext(ctx, input)
		}, OperationNotAllowedCode)

		if err != nil {
			return diag.Errorf("error modifying PaaS ClickHouse Profile (%s): %s", d.Id(), err)
		}

		_, err = waitServiceUpdated(ctx, conn, serviceID, d.Timeout(schema.TimeoutUpdate))

		if err != nil {
			return diag.Errorf("error waiting for PaaS Service (%s) to update: %s", serviceID, err)
		}
	}

	return resourceClickHouseProfileRead(ctx, d, meta)
}

func resourceClickHouseProfileDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).PaaSConn

	serviceID, id, err := ClickHouseProfileParseResourceID(d.Id())

	if err != nil {
		return diag.FromErr(err)
	}

	input := &paas.DeleteClickhouseProfileInput{
		ProfileId: aws.String(id),
		ServiceId: aws.String(serviceID),
	}

	log.Printf("[DEBUG] Deleting PaaS ClickHouse Profile: %s", input)
	_, err = tfresource.RetryWhenAWSErrCodeEqualsContext(ctx, d.Timeout(schema.TimeoutDelete), func() (interface{}, error) {
		return conn.DeleteClickhouseProfileWithContext(ctx, input)
	}, OperationNotAllowedCode)

	if tfawserr.ErrCodeEquals(err, ServiceNotFoundCode, ResourceDoesNotExistCode) {
		log.Printf("[WARN] PaaS ClickHouse Profile (%s) not found, removing from state", d.Id())
		return nil
	}

	if err != nil {
		return diag.Errorf("error deleting PaaS ClickHouse Profile (%s): %s", d.Id(), err)
	}

	_, err = waitServiceUpdated(ctx, conn, serviceID, d.Timeout(schema.TimeoutDelete))

	if err != nil {
		return diag.Errorf("error waiting for PaaS Service (%s) to update: %s", serviceID, err)
	}

	return nil
}

// flattenClickHouseProfileSettings converts setting values to strings,
// because api can return them as numbers or booleans.
func flattenClickHouseProfileSettings(settings map[string]interface{}) map[string]interface{} {
	if settings == nil {
		return map[string]interface{}{}
	}

	tfMap := map[string]interface{}{}

	for k, v := range settings {
		tfMap[k] = fmt.Sprint(v)
	}

	return tfMap
}

const clickHouseProfileResourceIDSeparator = "/"

func ClickHouseProfileCreateResourceID(serviceID, profileID string) string {
	parts := []string{serviceID, profileID}
	id := strings.Join(parts, clickHouseProfileResourceIDSeparator)

	return id
}

func ClickHouseProfileParseResourceID(id string) (string, string, error) {
	parts := strings.Split(id, clickHouseProfileResourceIDSeparator)

	if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		return parts[0], parts[1], nil
	}

	return "", "", fmt.Errorf("unexpected format for ID (%[1]s), expected service-id%[2]sprofile-id", id, clickHouseProfileResourceIDSeparator)
}
