package paas

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type clickHouseServiceConfig struct {
	highAvailability    bool
	hasCoordinatorBlock bool
	hasCoordinatorRole  bool
}

// validateClickHouseServiceConfiguration enforces ClickHouse node configurations from K2 Cloud:
//
// - standalone (high_availability = false): additional_roles = ["coordinator"];
// - cluster of combined nodes (high_availability = true): additional_roles = ["coordinator"];
// - cluster with dedicated keepers (high_availability = true): coordinator block.
func validateClickHouseServiceConfiguration(d *schema.ResourceDiff) error {
	// Unknown topology values are resolved during apply. GetOk otherwise exposes
	// them as zero values and can reject a valid dependency during planning.
	for _, key := range []string{"high_availability", "additional_roles", "coordinator"} {
		if !d.NewValueKnown(key) {
			return nil
		}
	}

	config := clickHouseServiceConfig{}

	if v, ok := d.GetOk("high_availability"); ok {
		config.highAvailability = v.(bool)
	}

	if v, ok := d.GetOk("coordinator"); ok {
		if list, ok := v.([]interface{}); ok && len(list) > 0 && list[0] != nil {
			config.hasCoordinatorBlock = true
		}
	}

	if v, ok := d.GetOk("additional_roles"); ok {
		for index, r := range v.([]interface{}) {
			if !d.NewValueKnown(fmt.Sprintf("additional_roles.%d", index)) {
				return nil
			}
			if s, ok := r.(string); ok && s == "coordinator" {
				config.hasCoordinatorRole = true
				break
			}
		}
	}

	return validateClickHouseServiceConfig(config)
}

func validateClickHouseServiceConfig(config clickHouseServiceConfig) error {
	if config.highAvailability {
		if config.hasCoordinatorRole || config.hasCoordinatorBlock {
			return nil
		}
		return fmt.Errorf(
			"clickhouse with high_availability = true requires a \"coordinator\" block or additional_roles = [\"coordinator\"]",
		)
	}

	if config.hasCoordinatorBlock {
		return fmt.Errorf(
			"clickhouse with high_availability = false requires additional_roles = [\"coordinator\"]; " +
				"a dedicated \"coordinator\" block is supported only for clickhouse clusters",
		)
	}
	if !config.hasCoordinatorRole {
		return fmt.Errorf("clickhouse with high_availability = false requires additional_roles = [\"coordinator\"]")
	}
	return nil
}
