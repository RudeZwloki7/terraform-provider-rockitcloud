package paas

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/paas"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
)

const (
	autoDiskExpansionMinConditionValue = 70
	autoDiskExpansionMaxConditionValue = 90
	autoDiskExpansionSizeMultipleGb    = 8

	// c2 returns OperationInProgress when it cannot lock the service document in time.
	autoDiskExpansionOperationInProgressErrorCode = "OperationInProgress"
	autoDiskExpansionUpdateTimeout                = 2 * time.Minute
)

func expandAutoDiskExpansionSettings(tfMap map[string]interface{}) *paas.CreateAutoDiskExpansionSettingsRequest {
	if tfMap == nil {
		return nil
	}

	return &paas.CreateAutoDiskExpansionSettingsRequest{
		ConditionValue: aws.Int64(int64(tfMap["condition_value"].(int))),
		Enabled:        aws.Bool(tfMap["enabled"].(bool)),
		MaxSizeGb:      aws.Int64(int64(tfMap["max_size_gb"].(int))),
	}
}

func flattenAutoDiskExpansionSettings(settings *paas.AutoDiskExpansionSettings) []map[string]interface{} {
	if settings == nil {
		return []map[string]interface{}{}
	}

	return []map[string]interface{}{
		{
			"condition_value": aws.Int64Value(settings.ConditionValue),
			"enabled":         aws.BoolValue(settings.Enabled),
			"max_size_gb":     aws.Int64Value(settings.MaxSizeGb),
			"status":          aws.StringValue(settings.Status),
		},
	}
}

// autoDiskExpansionUpdateInput builds the UpdateAutoDiskExpansionSettings request.
// The API requires all settings, so a removed block disables ADE keeping the last known values.
func autoDiskExpansionUpdateInput(id string, oldList, newList []interface{}) *paas.UpdateAutoDiskExpansionSettingsInput {
	input := &paas.UpdateAutoDiskExpansionSettingsInput{
		ServiceId: aws.String(id),
	}

	if len(newList) > 0 && newList[0] != nil {
		settings := expandAutoDiskExpansionSettings(newList[0].(map[string]interface{}))
		input.ConditionValue = settings.ConditionValue
		input.Enabled = settings.Enabled
		input.MaxSizeGb = settings.MaxSizeGb

		return input
	}

	if len(oldList) == 0 || oldList[0] == nil {
		return nil
	}

	settings := expandAutoDiskExpansionSettings(oldList[0].(map[string]interface{}))
	if !aws.BoolValue(settings.Enabled) {
		return nil
	}

	input.ConditionValue = settings.ConditionValue
	input.Enabled = aws.Bool(false)
	input.MaxSizeGb = settings.MaxSizeGb

	return input
}

func updateAutoDiskExpansionSettings(ctx context.Context, conn *paas.PaaS, input *paas.UpdateAutoDiskExpansionSettingsInput) error {
	log.Printf("[DEBUG] Modifying PaaS Service auto disk expansion settings: %s", input)
	_, err := tfresource.RetryWhenAWSErrCodeEqualsContext(
		ctx,
		autoDiskExpansionUpdateTimeout,
		func() (interface{}, error) {
			return conn.UpdateAutoDiskExpansionSettingsWithContext(ctx, input)
		},
		autoDiskExpansionOperationInProgressErrorCode,
	)

	return err
}

// dataVolumeSizeDiffSuppressFunc suppresses the diff when auto disk expansion
// has increased the data volume above the configured size.
func dataVolumeSizeDiffSuppressFunc(_, old, new string, d *schema.ResourceData) bool {
	if !autoDiskExpansionEnabled(d.Get("auto_disk_expansion").([]interface{})) {
		return false
	}

	oldSize, err := strconv.Atoi(old)
	if err != nil {
		return false
	}

	newSize, err := strconv.Atoi(new)
	if err != nil {
		return false
	}

	return oldSize > newSize
}

func autoDiskExpansionEnabled(tfList []interface{}) bool {
	if len(tfList) == 0 || tfList[0] == nil {
		return false
	}

	enabled, ok := tfList[0].(map[string]interface{})["enabled"].(bool)

	return ok && enabled
}

// validateAutoDiskExpansionConfiguration rejects auto disk expansion without a data volume:
// K2 Cloud expands only the data volume and does not validate the settings without it.
func validateAutoDiskExpansionConfiguration(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
	_, hasAutoDiskExpansion := d.GetOk("auto_disk_expansion")
	_, hasDataVolume := d.GetOk("data_volume")

	return validateAutoDiskExpansionConfig(hasAutoDiskExpansion, hasDataVolume)
}

func validateAutoDiskExpansionConfig(hasAutoDiskExpansion, hasDataVolume bool) error {
	if hasAutoDiskExpansion && !hasDataVolume {
		return fmt.Errorf("auto_disk_expansion requires data_volume to be configured")
	}

	return nil
}
