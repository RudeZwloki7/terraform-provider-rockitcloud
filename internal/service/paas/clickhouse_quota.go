package paas

import (
	"context"
	"encoding/json"
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

func ResourceClickHouseQuota() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceClickHouseQuotaCreate,
		ReadContext:   resourceClickHouseQuotaRead,
		UpdateContext: resourceClickHouseQuotaUpdate,
		DeleteContext: resourceClickHouseQuotaDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(30 * time.Minute),
			Update: schema.DefaultTimeout(30 * time.Minute),
			Delete: schema.DefaultTimeout(30 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"interval": {
				Type:     schema.TypeList,
				Required: true,
				MinItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"duration": {
							Type:         schema.TypeInt,
							Required:     true,
							ValidateFunc: validation.IntBetween(1, 2147483647),
						},
						"errors": {
							Type:         schema.TypeInt,
							Optional:     true,
							ValidateFunc: validation.IntAtLeast(1),
						},
						"execution_time": {
							Type:         schema.TypeFloat,
							Optional:     true,
							ValidateFunc: validation.FloatAtLeast(1),
						},
						"failed_sequential_authentications": {
							Type:         schema.TypeInt,
							Optional:     true,
							ValidateFunc: validation.IntAtLeast(1),
						},
						"queries": {
							Type:         schema.TypeInt,
							Optional:     true,
							ValidateFunc: validation.IntAtLeast(1),
						},
						"query_inserts": {
							Type:         schema.TypeInt,
							Optional:     true,
							ValidateFunc: validation.IntAtLeast(1),
						},
						"query_selects": {
							Type:         schema.TypeInt,
							Optional:     true,
							ValidateFunc: validation.IntAtLeast(1),
						},
						"read_bytes": {
							Type:         schema.TypeInt,
							Optional:     true,
							ValidateFunc: validation.IntAtLeast(1),
						},
						"read_rows": {
							Type:         schema.TypeInt,
							Optional:     true,
							ValidateFunc: validation.IntAtLeast(1),
						},
						"result_bytes": {
							Type:         schema.TypeInt,
							Optional:     true,
							ValidateFunc: validation.IntAtLeast(1),
						},
						"result_rows": {
							Type:         schema.TypeInt,
							Optional:     true,
							ValidateFunc: validation.IntAtLeast(1),
						},
						"written_bytes": {
							Type:         schema.TypeInt,
							Optional:     true,
							ValidateFunc: validation.IntAtLeast(1),
						},
					},
				},
			},
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.All(
					services.ClickHouseNameValidateFunc,
					validation.StringNotInSlice([]string{"default"}, true),
				),
			},
			"service_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
		},
	}
}

func resourceClickHouseQuotaCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).PaaSConn
	serviceID := d.Get("service_id").(string)
	name := d.Get("name").(string)

	input := &paas.CreateClickhouseQuotaInput{
		Name: aws.String(name),
		Parameters: map[string]interface{}{
			"interval": expandClickHouseQuotaIntervals(d.Get("interval").([]interface{})),
		},
		ServiceId: aws.String(serviceID),
	}

	log.Printf("[DEBUG] Creating PaaS ClickHouse Quota: %s", input)
	outputRaw, err := tfresource.RetryWhenAWSErrCodeEqualsContext(ctx, d.Timeout(schema.TimeoutCreate), func() (interface{}, error) {
		return conn.CreateClickhouseQuota(input)
	}, OperationNotAllowedCode)

	if err != nil {
		return diag.Errorf("error creating PaaS ClickHouse Quota (%s) in Service (%s): %s", name, serviceID, err)
	}

	id := aws.StringValue(outputRaw.(*paas.CreateClickhouseQuotaOutput).Quota.Id)
	d.SetId(ClickHouseQuotaCreateResourceID(serviceID, id))

	_, err = waitServiceUpdated(ctx, conn, serviceID, d.Timeout(schema.TimeoutCreate))

	if err != nil {
		return diag.Errorf("error waiting for PaaS Service (%s) to update: %s", serviceID, err)
	}

	return resourceClickHouseQuotaRead(ctx, d, meta)
}

func resourceClickHouseQuotaRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).PaaSConn

	serviceID, id, err := ClickHouseQuotaParseResourceID(d.Id())

	if err != nil {
		return diag.FromErr(err)
	}

	quota, err := FindClickHouseQuotaByID(conn, serviceID, id)

	if !d.IsNewResource() && tfresource.NotFound(err) {
		log.Printf("[WARN] PaaS ClickHouse Quota (%s) not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}

	if err != nil {
		return diag.Errorf("error reading PaaS ClickHouse Quota (%s): %s", d.Id(), err)
	}

	if v, ok := quota.Parameters["interval"].([]interface{}); ok {
		d.Set("interval", flattenClickHouseQuotaIntervals(v))
	}

	d.Set("name", quota.Name)
	d.Set("service_id", serviceID)

	return nil
}

func resourceClickHouseQuotaUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).PaaSConn

	serviceID, id, err := ClickHouseQuotaParseResourceID(d.Id())

	if err != nil {
		return diag.FromErr(err)
	}

	if d.HasChange("interval") {
		input := &paas.ModifyClickhouseQuotaInput{
			Parameters: map[string]interface{}{
				"interval": expandClickHouseQuotaIntervals(d.Get("interval").([]interface{})),
			},
			QuotaId:   aws.String(id),
			ServiceId: aws.String(serviceID),
		}

		log.Printf("[DEBUG] Modifying PaaS ClickHouse Quota: %s", input)
		_, err := tfresource.RetryWhenAWSErrCodeEqualsContext(ctx, d.Timeout(schema.TimeoutUpdate), func() (interface{}, error) {
			return conn.ModifyClickhouseQuota(input)
		}, OperationNotAllowedCode)

		if err != nil {
			return diag.Errorf("error modifying PaaS ClickHouse Quota (%s): %s", d.Id(), err)
		}

		_, err = waitServiceUpdated(ctx, conn, serviceID, d.Timeout(schema.TimeoutUpdate))

		if err != nil {
			return diag.Errorf("error waiting for PaaS Service (%s) to update: %s", serviceID, err)
		}
	}

	return resourceClickHouseQuotaRead(ctx, d, meta)
}

func resourceClickHouseQuotaDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).PaaSConn

	serviceID, id, err := ClickHouseQuotaParseResourceID(d.Id())

	if err != nil {
		return diag.FromErr(err)
	}

	input := &paas.DeleteClickhouseQuotaInput{
		QuotaId:   aws.String(id),
		ServiceId: aws.String(serviceID),
	}

	log.Printf("[DEBUG] Deleting PaaS ClickHouse Quota: %s", input)
	_, err = tfresource.RetryWhenAWSErrCodeEqualsContext(ctx, d.Timeout(schema.TimeoutDelete), func() (interface{}, error) {
		return conn.DeleteClickhouseQuota(input)
	}, OperationNotAllowedCode)

	if tfawserr.ErrCodeEquals(err, ServiceNotFoundCode, ResourceDoesNotExistCode) {
		log.Printf("[WARN] PaaS ClickHouse Quota (%s) not found, removing from state", d.Id())
		return nil
	}

	if err != nil {
		return diag.Errorf("error deleting PaaS ClickHouse Quota (%s): %s", d.Id(), err)
	}

	_, err = waitServiceUpdated(ctx, conn, serviceID, d.Timeout(schema.TimeoutDelete))

	if err != nil {
		return diag.Errorf("error waiting for PaaS Service (%s) to update: %s", serviceID, err)
	}

	return nil
}

func expandClickHouseQuotaIntervals(tfList []interface{}) []interface{} {
	if len(tfList) == 0 {
		return nil
	}

	var intervals []interface{}

	for _, tfMapRaw := range tfList {
		tfMap, ok := tfMapRaw.(map[string]interface{})
		if !ok {
			continue
		}

		interval := map[string]interface{}{}

		if v, ok := tfMap["duration"].(int); ok && v != 0 {
			interval["duration"] = int64(v)
		}

		if v, ok := tfMap["errors"].(int); ok && v != 0 {
			interval["errors"] = int64(v)
		}

		if v, ok := tfMap["execution_time"].(float64); ok && v != 0 {
			interval["execution_time"] = v
		}

		if v, ok := tfMap["failed_sequential_authentications"].(int); ok && v != 0 {
			interval["failed_sequential_authentications"] = int64(v)
		}

		if v, ok := tfMap["queries"].(int); ok && v != 0 {
			interval["queries"] = int64(v)
		}

		if v, ok := tfMap["query_inserts"].(int); ok && v != 0 {
			interval["query_inserts"] = int64(v)
		}

		if v, ok := tfMap["query_selects"].(int); ok && v != 0 {
			interval["query_selects"] = int64(v)
		}

		if v, ok := tfMap["read_bytes"].(int); ok && v != 0 {
			interval["read_bytes"] = int64(v)
		}

		if v, ok := tfMap["read_rows"].(int); ok && v != 0 {
			interval["read_rows"] = int64(v)
		}

		if v, ok := tfMap["result_bytes"].(int); ok && v != 0 {
			interval["result_bytes"] = int64(v)
		}

		if v, ok := tfMap["result_rows"].(int); ok && v != 0 {
			interval["result_rows"] = int64(v)
		}

		if v, ok := tfMap["written_bytes"].(int); ok && v != 0 {
			interval["written_bytes"] = int64(v)
		}

		intervals = append(intervals, interval)
	}

	return intervals
}

func flattenClickHouseQuotaIntervals(intervals []interface{}) []interface{} {
	if len(intervals) == 0 {
		return nil
	}

	var tfList []interface{}

	for _, intervalRaw := range intervals {
		interval, ok := intervalRaw.(map[string]interface{})
		if !ok {
			continue
		}

		tfMap := map[string]interface{}{}

		if v, ok := clickHouseQuotaInt(interval["duration"]); ok {
			tfMap["duration"] = v
		}

		if v, ok := clickHouseQuotaInt(interval["errors"]); ok {
			tfMap["errors"] = v
		}

		if v, ok := clickHouseQuotaFloat(interval["executionTime"]); ok {
			tfMap["execution_time"] = v
		}

		if v, ok := clickHouseQuotaInt(interval["failedSequentialAuthentications"]); ok {
			tfMap["failed_sequential_authentications"] = v
		}

		if v, ok := clickHouseQuotaInt(interval["queries"]); ok {
			tfMap["queries"] = v
		}

		if v, ok := clickHouseQuotaInt(interval["queryInserts"]); ok {
			tfMap["query_inserts"] = v
		}

		if v, ok := clickHouseQuotaInt(interval["querySelects"]); ok {
			tfMap["query_selects"] = v
		}

		if v, ok := clickHouseQuotaInt(interval["readBytes"]); ok {
			tfMap["read_bytes"] = v
		}

		if v, ok := clickHouseQuotaInt(interval["readRows"]); ok {
			tfMap["read_rows"] = v
		}

		if v, ok := clickHouseQuotaInt(interval["resultBytes"]); ok {
			tfMap["result_bytes"] = v
		}

		if v, ok := clickHouseQuotaInt(interval["resultRows"]); ok {
			tfMap["result_rows"] = v
		}

		if v, ok := clickHouseQuotaInt(interval["writtenBytes"]); ok {
			tfMap["written_bytes"] = v
		}

		tfList = append(tfList, tfMap)
	}

	return tfList
}

// clickHouseQuotaInt converts interval limit to integer.
// Sdk decodes numbers nested in free-form json as json.Number.
func clickHouseQuotaInt(value interface{}) (int64, bool) {
	switch v := value.(type) {
	case int64:
		return v, true
	case json.Number:
		i, err := v.Int64()
		return i, err == nil
	}

	return 0, false
}

// clickHouseQuotaFloat converts interval limit to float.
// Api can return integer value for float parameter.
func clickHouseQuotaFloat(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int64:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	}

	return 0, false
}

const clickHouseQuotaResourceIDSeparator = "/"

func ClickHouseQuotaCreateResourceID(serviceID, quotaID string) string {
	parts := []string{serviceID, quotaID}
	id := strings.Join(parts, clickHouseQuotaResourceIDSeparator)

	return id
}

func ClickHouseQuotaParseResourceID(id string) (string, string, error) {
	parts := strings.Split(id, clickHouseQuotaResourceIDSeparator)

	if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		return parts[0], parts[1], nil
	}

	return "", "", fmt.Errorf("unexpected format for ID (%[1]s), expected service-id%[2]squota-id", id, clickHouseQuotaResourceIDSeparator)
}
