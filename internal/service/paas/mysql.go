package paas

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-provider-aws/internal/experimental/nullable"
)

func validateMySQLServiceConfiguration(d *schema.ResourceDiff) error {
	config := d.GetRawConfig()
	if config.IsNull() || !config.IsKnown() || !config.Type().HasAttribute("mysql") {
		return nil
	}
	mysql := config.GetAttr("mysql")
	if mysql.IsNull() || !mysql.IsKnown() || mysql.LengthInt() == 0 {
		return nil
	}
	parameters := mysql.AsValueSlice()[0]
	if parameters.IsNull() || !parameters.IsKnown() || !parameters.Type().HasAttribute("binlog_retention_days") {
		return nil
	}
	// Check only configured retention; a computed value from state must not block disabling binlog.
	retention := parameters.GetAttr("binlog_retention_days")
	if retention.IsNull() || !retention.IsKnown() {
		return nil
	}
	if parameters.Type().HasAttribute("bin_log_enabled") {
		enabled := parameters.GetAttr("bin_log_enabled")
		if !enabled.IsKnown() {
			return nil
		}
		if !enabled.IsNull() {
			if value, _, err := nullable.Bool(enabled.AsString()).Value(); err == nil && value {
				return nil
			}
		}
	}
	return fmt.Errorf("mysql.0.binlog_retention_days requires bin_log_enabled = true")
}
