package paas

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/paas"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
)

func ResourceClickHouseShard() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceClickHouseShardCreate,
		ReadContext:   resourceClickHouseShardRead,
		UpdateContext: resourceClickHouseShardUpdate,
		DeleteContext: resourceClickHouseShardDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(30 * time.Minute),
			Update: schema.DefaultTimeout(30 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"index": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"internal_replication": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"replicas": {
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      1,
				ValidateFunc: validation.IntAtLeast(1),
			},
			"service_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"weight": {
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      1,
				ValidateFunc: validation.IntBetween(1, 2147483647),
			},
		},
	}
}

func resourceClickHouseShardCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).PaaSConn
	serviceID := d.Get("service_id").(string)

	input := &paas.AddClickhouseShardInput{
		Parameters: expandClickHouseShardParameters(d),
		ServiceId:  aws.String(serviceID),
	}

	log.Printf("[DEBUG] Adding PaaS ClickHouse Shard: %s", input)
	outputRaw, err := tfresource.RetryWhenAWSErrCodeEqualsContext(ctx, d.Timeout(schema.TimeoutCreate), func() (interface{}, error) {
		return conn.AddClickhouseShard(input)
	}, OperationNotAllowedCode)

	if err != nil {
		return diag.Errorf("error adding PaaS ClickHouse Shard to Service (%s): %s", serviceID, err)
	}

	index := aws.Int64Value(outputRaw.(*paas.AddClickhouseShardOutput).Shard.Index)
	d.SetId(ClickHouseShardCreateResourceID(serviceID, index))

	_, err = waitServiceUpdated(ctx, conn, serviceID, d.Timeout(schema.TimeoutCreate))

	if err != nil {
		return diag.Errorf("error waiting for PaaS Service (%s) to update: %s", serviceID, err)
	}

	return resourceClickHouseShardRead(ctx, d, meta)
}

func resourceClickHouseShardRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).PaaSConn

	serviceID, index, err := ClickHouseShardParseResourceID(d.Id())

	if err != nil {
		return diag.FromErr(err)
	}

	shard, err := FindClickHouseShardByIndex(conn, serviceID, index)

	if !d.IsNewResource() && tfresource.NotFound(err) {
		log.Printf("[WARN] PaaS ClickHouse Shard (%s) not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}

	if err != nil {
		return diag.Errorf("error reading PaaS ClickHouse Shard (%s): %s", d.Id(), err)
	}

	d.Set("index", shard.Index)

	if v := shard.Parameters; v != nil {
		d.Set("internal_replication", v.InternalReplication)
		d.Set("replicas", v.Replicas)
		d.Set("weight", v.Weight)
	}

	d.Set("service_id", serviceID)

	return nil
}

func resourceClickHouseShardUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*conns.AWSClient).PaaSConn

	serviceID, index, err := ClickHouseShardParseResourceID(d.Id())

	if err != nil {
		return diag.FromErr(err)
	}

	if d.HasChanges("internal_replication", "replicas", "weight") {
		input := &paas.ModifyClickhouseShardInput{
			Parameters: expandClickHouseShardParameters(d),
			ServiceId:  aws.String(serviceID),
			ShardIndex: aws.Int64(index),
		}

		log.Printf("[DEBUG] Modifying PaaS ClickHouse Shard: %s", input)
		_, err := tfresource.RetryWhenAWSErrCodeEqualsContext(ctx, d.Timeout(schema.TimeoutUpdate), func() (interface{}, error) {
			return conn.ModifyClickhouseShard(input)
		}, OperationNotAllowedCode)

		if err != nil {
			return diag.Errorf("error modifying PaaS ClickHouse Shard (%s): %s", d.Id(), err)
		}

		_, err = waitServiceUpdated(ctx, conn, serviceID, d.Timeout(schema.TimeoutUpdate))

		if err != nil {
			return diag.Errorf("error waiting for PaaS Service (%s) to update: %s", serviceID, err)
		}
	}

	return resourceClickHouseShardRead(ctx, d, meta)
}

func resourceClickHouseShardDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// Api doesn't support removing ClickHouse shards.
	log.Printf("[WARN] PaaS ClickHouse Shard (%s) was not deleted, removing from state", d.Id())

	return nil
}

func expandClickHouseShardParameters(d *schema.ResourceData) *paas.ClickHouseShardParameters {
	return &paas.ClickHouseShardParameters{
		InternalReplication: aws.Bool(d.Get("internal_replication").(bool)),
		Replicas:            aws.Int64(int64(d.Get("replicas").(int))),
		Weight:              aws.Int64(int64(d.Get("weight").(int))),
	}
}

const clickHouseShardResourceIDSeparator = "/"

func ClickHouseShardCreateResourceID(serviceID string, index int64) string {
	parts := []string{serviceID, strconv.FormatInt(index, 10)}
	id := strings.Join(parts, clickHouseShardResourceIDSeparator)

	return id
}

func ClickHouseShardParseResourceID(id string) (string, int64, error) {
	parts := strings.Split(id, clickHouseShardResourceIDSeparator)

	if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		index, err := strconv.ParseInt(parts[1], 10, 64)

		if err == nil {
			return parts[0], index, nil
		}
	}

	return "", 0, fmt.Errorf("unexpected format for ID (%[1]s), expected service-id%[2]sshard-index", id, clickHouseShardResourceIDSeparator)
}
