package paas

import (
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-provider-aws/internal/service/paas/services"
)

func serviceParametersFromResourceData(d *schema.ResourceData, serviceType string) map[string]interface{} {
	parameters := d.Get(serviceType).([]interface{})[0].(map[string]interface{})
	if serviceType != services.ServiceTypeMongoDB {
		return parameters
	}

	// SDK TypeInt normalizes both an omitted slowms and an explicit zero to 0.
	// Use the original Terraform configuration to preserve that distinction.
	rawConfig := d.GetRawConfig()
	if rawConfig.IsNull() || !rawConfig.IsKnown() || !rawConfig.Type().HasAttribute(serviceType) {
		return parameters
	}
	rawService := rawConfig.GetAttr(serviceType)
	if rawService.IsNull() || !rawService.IsKnown() || rawService.LengthInt() == 0 {
		return parameters
	}
	rawParameters := rawService.Index(cty.NumberIntVal(0))
	if rawParameters.IsNull() || !rawParameters.IsKnown() {
		return parameters
	}
	if rawParameters.Type().HasAttribute("slowms") && !rawParameters.GetAttr("slowms").IsNull() {
		return parameters
	}

	filtered := make(map[string]interface{}, len(parameters))
	for key, value := range parameters {
		if key != "slowms" {
			filtered[key] = value
		}
	}
	return filtered
}
