package paas

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-provider-aws/internal/service/paas/services"
)

// K2 Cloud supports GitLab only as a single master without high availability:
// https://docs.k2.cloud/ru/services/paas/development/index.html
func validateGitLabServiceConfiguration(d *schema.ResourceDiff) error {
	if _, ok := d.GetOk(services.GitLab.ServiceType()); !ok {
		return nil
	}
	if d.NewValueKnown("high_availability") && d.Get("high_availability").(bool) {
		return fmt.Errorf("gitlab supports only high_availability = false in K2 Cloud")
	}
	if d.NewValueKnown("instance_qty") {
		if v, ok := d.GetOk("instance_qty"); ok && v.(int) != 1 {
			return fmt.Errorf("gitlab supports only instance_qty = 1 in K2 Cloud")
		}
	}
	if v, ok := d.GetOk("gitlab.0.registry"); ok && d.NewValueKnown("gitlab.0.registry") {
		registry := v.([]interface{})[0].(map[string]interface{})
		if previous, _ := d.GetChange("gitlab.0.registry"); previous != nil && len(previous.([]interface{})) > 0 && d.NewValueKnown("gitlab.0.registry.0.storage") && d.HasChange("gitlab.0.registry.0.storage") {
			return fmt.Errorf("gitlab.registry.storage cannot be changed while Container Registry is enabled; remove the registry block and apply before enabling it with a different storage")
		}
		if d.NewValueKnown("gitlab.0.registry.0.storage") && registry["storage"].(string) == "s3" {
			for _, key := range []string{"bucket", "s3_user"} {
				path := "gitlab.0.registry.0." + key
				if d.NewValueKnown(path) && strings.TrimSpace(registry[key].(string)) == "" {
					return fmt.Errorf("%s is required when gitlab.registry.storage = s3", path)
				}
			}
		}
	}
	return nil
}
