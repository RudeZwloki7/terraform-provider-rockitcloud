package services

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

type clickHouseManager struct {
	service
}

var ClickHouse = clickHouseManager{
	service{
		name:               ServiceTypeClickHouse,
		class:              []string{ServiceClassDatabase},
		defaultClass:       ServiceClassDatabase,
		allowArbitrator:    false,
		allowBackup:        false,
		dataVolumeRequired: true,
		usersEnabled:       true,
		databasesEnabled:   true,
		noDatabaseUsers:    true,
		loggingEnabled:     true,
		monitoringEnabled:  true,
	},
}

var clickHouseNameRegexp = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// ClickHouseNameValidateFunc validates names of ClickHouse resources: users, databases, profiles and quotas.
var ClickHouseNameValidateFunc = validation.StringMatch(
	clickHouseNameRegexp,
	"name can only contain Latin letters, numbers and underscores (_)",
)

func (s clickHouseManager) serviceParametersSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"options": {
			Type:     schema.TypeMap,
			Optional: true,
			Elem:     &schema.Schema{Type: schema.TypeString},
		},
		"version": {
			Type:     schema.TypeString,
			Required: true,
			ForceNew: true,
		},
	}
}

func (s clickHouseManager) serviceParametersDataSourceSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"options": {
			Type:     schema.TypeMap,
			Computed: true,
			Elem:     &schema.Schema{Type: schema.TypeString},
		},
		"version": {
			Type:     schema.TypeString,
			Computed: true,
		},
	}
}

func (s clickHouseManager) userParametersSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"grants": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Schema{
				Type:         schema.TypeString,
				ValidateFunc: validation.StringIsNotEmpty,
			},
		},
		"networks": {
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"host": {
						Type:     schema.TypeSet,
						Optional: true,
						Elem:     &schema.Schema{Type: schema.TypeString},
					},
					"host_regexp": {
						Type:     schema.TypeSet,
						Optional: true,
						Elem:     &schema.Schema{Type: schema.TypeString},
					},
					"ip": {
						Type:     schema.TypeSet,
						Optional: true,
						Elem:     &schema.Schema{Type: schema.TypeString},
					},
				},
			},
		},
		"password": {
			Type:         schema.TypeString,
			Optional:     true,
			Sensitive:    true,
			ValidateFunc: validation.StringIsNotEmpty,
		},
		"password_type": {
			Type:         schema.TypeString,
			Optional:     true,
			Default:      "double_sha1",
			ValidateFunc: validation.StringInSlice([]string{"double_sha1"}, false),
		},
		"profile": {
			Type:         schema.TypeString,
			Optional:     true,
			Default:      "default",
			ValidateFunc: ClickHouseNameValidateFunc,
		},
		"quota": {
			Type:         schema.TypeString,
			Optional:     true,
			Default:      "default",
			ValidateFunc: ClickHouseNameValidateFunc,
		},
	}
}

func (s clickHouseManager) userParametersDataSourceSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"grants": {
			Type:     schema.TypeList,
			Computed: true,
			Elem:     &schema.Schema{Type: schema.TypeString},
		},
		"networks": {
			Type:     schema.TypeList,
			Computed: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"host": {
						Type:     schema.TypeSet,
						Computed: true,
						Elem:     &schema.Schema{Type: schema.TypeString},
					},
					"host_regexp": {
						Type:     schema.TypeSet,
						Computed: true,
						Elem:     &schema.Schema{Type: schema.TypeString},
					},
					"ip": {
						Type:     schema.TypeSet,
						Computed: true,
						Elem:     &schema.Schema{Type: schema.TypeString},
					},
				},
			},
		},
		"password": {
			Type:     schema.TypeString,
			Computed: true,
		},
		"password_type": {
			Type:     schema.TypeString,
			Computed: true,
		},
		"profile": {
			Type:     schema.TypeString,
			Computed: true,
		},
		"quota": {
			Type:     schema.TypeString,
			Computed: true,
		},
	}
}

func (s clickHouseManager) expandServiceParameters(tfMap map[string]interface{}) ServiceParameters {
	if tfMap == nil {
		return nil
	}

	serviceParameters := ServiceParameters{}

	if v, ok := tfMap["options"].(map[string]interface{}); ok && len(v) > 0 {
		serviceParameters["options"] = v
	}

	if v, ok := tfMap["version"].(string); ok && v != "" {
		serviceParameters["version"] = v
	}

	return serviceParameters
}

func (s clickHouseManager) expandUserParameters(tfMap map[string]interface{}) UserParameters {
	if tfMap == nil {
		return nil
	}

	userParameters := UserParameters{}

	// Grants and networks are always sent to be able to remove them.
	if v, ok := tfMap["grants"].([]interface{}); ok {
		userParameters["grants"] = v
	}

	networks := map[string]interface{}{}
	if v, ok := tfMap["networks"].([]interface{}); ok && len(v) > 0 && v[0] != nil {
		networks = expandClickHouseNetworks(v[0].(map[string]interface{}))
	}
	userParameters["networks"] = networks

	if v, ok := tfMap["password"].(string); ok && v != "" {
		userParameters["password"] = v
	}

	if v, ok := tfMap["password_type"].(string); ok && v != "" {
		userParameters["password_type"] = v
	}

	if v, ok := tfMap["profile"].(string); ok && v != "" {
		userParameters["profile"] = v
	}

	if v, ok := tfMap["quota"].(string); ok && v != "" {
		userParameters["quota"] = v
	}

	return userParameters
}

func expandClickHouseNetworks(tfMap map[string]interface{}) map[string]interface{} {
	networks := map[string]interface{}{}

	if v, ok := tfMap["host"].(*schema.Set); ok && v.Len() > 0 {
		networks["host"] = v.List()
	}

	if v, ok := tfMap["host_regexp"].(*schema.Set); ok && v.Len() > 0 {
		networks["host_regexp"] = v.List()
	}

	if v, ok := tfMap["ip"].(*schema.Set); ok && v.Len() > 0 {
		networks["ip"] = v.List()
	}

	return networks
}

func (s clickHouseManager) flattenServiceParameters(serviceParameters ServiceParameters) map[string]interface{} {
	if serviceParameters == nil {
		return map[string]interface{}{}
	}

	tfMap := map[string]interface{}{}

	if v, ok := serviceParameters["options"].(map[string]interface{}); ok {
		tfMap["options"] = v
	}

	if v, ok := serviceParameters["version"].(string); ok {
		tfMap["version"] = v
	}

	return tfMap
}

func (s clickHouseManager) flattenUserParameters(userParameters UserParameters) map[string]interface{} {
	if userParameters == nil {
		return map[string]interface{}{}
	}

	tfMap := map[string]interface{}{}

	if v, ok := userParameters["grants"].([]interface{}); ok {
		tfMap["grants"] = v
	}

	// ignore when api returns `"networks": {}` (block is omitted in config)
	if v, ok := userParameters["networks"].(map[string]interface{}); ok && len(v) > 0 {
		tfMap["networks"] = []map[string]interface{}{flattenClickHouseNetworks(v)}
	}

	if v, ok := userParameters["password"].(string); ok {
		tfMap["password"] = v
	}

	if v, ok := userParameters["passwordType"].(string); ok {
		tfMap["password_type"] = v
	}

	if v, ok := userParameters["profile"].(string); ok {
		tfMap["profile"] = v
	}

	if v, ok := userParameters["quota"].(string); ok {
		tfMap["quota"] = v
	}

	return tfMap
}

func flattenClickHouseNetworks(networks map[string]interface{}) map[string]interface{} {
	tfMap := map[string]interface{}{}

	if v, ok := networks["host"].([]interface{}); ok {
		tfMap["host"] = v
	}

	if v, ok := networks["host_regexp"].([]interface{}); ok {
		tfMap["host_regexp"] = v
	}

	if v, ok := networks["ip"].([]interface{}); ok {
		tfMap["ip"] = v
	}

	return tfMap
}
