package services

import (
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/hashicorp/terraform-provider-aws/internal/experimental/nullable"
)

type gitLabManager struct {
	service
}

var GitLab = gitLabManager{
	service{
		name:               ServiceTypeGitLab,
		class:              []string{ServiceClassDevelopment},
		defaultClass:       ServiceClassDevelopment,
		allowArbitrator:    false,
		allowBackup:        false,
		dataVolumeRequired: true,
		usersEnabled:       false,
		databasesEnabled:   false,
		loggingEnabled:     true,
		monitoringEnabled:  true,
		deferredParameters: []string{"ldap", "oidc", "smtp"},
	},
}

func (s gitLabManager) serviceParametersSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"external_url": {
			Type:     schema.TypeString,
			Computed: true,
		},
		"ldap": {
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"active_directory": {
						Type:         nullable.TypeNullableBool,
						Optional:     true,
						Computed:     true,
						ValidateFunc: nullable.ValidateTypeStringNullableBool,
					},
					"allow_username_or_email_login": {
						Type:         nullable.TypeNullableBool,
						Optional:     true,
						Computed:     true,
						ValidateFunc: nullable.ValidateTypeStringNullableBool,
					},
					"base": {
						Type:     schema.TypeString,
						Required: true,
					},
					"bind_dn": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"ca_cert": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"ciphers": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"client_cert": {
						Type:      schema.TypeString,
						Optional:  true,
						Sensitive: true,
					},
					"client_key": {
						Type:      schema.TypeString,
						Optional:  true,
						Sensitive: true,
					},
					"encryption": {
						Type:         schema.TypeString,
						Required:     true,
						ValidateFunc: validation.StringInSlice([]string{"start_tls", "simple_tls", "plain"}, false),
					},
					"host": {
						Type:     schema.TypeString,
						Required: true,
					},
					"label": {
						Type:     schema.TypeString,
						Required: true,
					},
					"lowercase_usernames": {
						Type:         nullable.TypeNullableBool,
						Optional:     true,
						Computed:     true,
						ValidateFunc: nullable.ValidateTypeStringNullableBool,
					},
					"password": {
						Type:      schema.TypeString,
						Optional:  true,
						Sensitive: true,
					},
					"port": {
						Type:         schema.TypeInt,
						Required:     true,
						ValidateFunc: validation.IsPortNumber,
					},
					"retry_empty_result_with_codes": {
						Type:     schema.TypeList,
						Optional: true,
						Elem:     &schema.Schema{Type: schema.TypeInt},
					},
					"ssl_version": {
						Type:         schema.TypeString,
						Optional:     true,
						ValidateFunc: validation.StringInSlice([]string{"TLSv1_1", "TLSv1_2", "TLSv1_3"}, false),
					},
					"timeout": {
						Type:         schema.TypeInt,
						Optional:     true,
						Default:      10,
						ValidateFunc: validation.IntBetween(0, 60),
					},
					"uid": {
						Type:         schema.TypeString,
						Required:     true,
						ValidateFunc: validation.StringIsNotEmpty,
					},
					"user_filter": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"verify_certificates": {
						Type:         nullable.TypeNullableBool,
						Optional:     true,
						Computed:     true,
						ValidateFunc: nullable.ValidateTypeStringNullableBool,
					},
				},
			},
		},
		"oidc": {
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"client_auth_method": {
						Type:         schema.TypeString,
						Optional:     true,
						ValidateFunc: validation.StringInSlice([]string{"query", "basic"}, false),
					},
					"client_id": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"client_secret": {
						Type:      schema.TypeString,
						Optional:  true,
						Sensitive: true,
					},
					"issuer": {
						Type:         schema.TypeString,
						Required:     true,
						ValidateFunc: validation.IsURLWithHTTPorHTTPS,
					},
					"label": {
						Type:     schema.TypeString,
						Optional: true,
						Default:  "OpenID Connect",
					},
					"pkce": {
						Type:         nullable.TypeNullableBool,
						Optional:     true,
						Computed:     true,
						ValidateFunc: nullable.ValidateTypeStringNullableBool,
					},
					"send_scope_to_token_endpoint": {
						Type:     schema.TypeBool,
						Optional: true,
						Default:  true,
					},
					"uid_field": {
						Type:     schema.TypeString,
						Optional: true,
						Default:  "sub",
					},
				},
			},
		},
		"password": {
			Type:      schema.TypeString,
			Required:  true,
			Sensitive: true,
			ForceNew:  true,
			ValidateFunc: validation.All(
				validation.StringLenBetween(8, 128),
				validation.StringDoesNotContainAny("`'\"\\"),
			),
		},
		"registry": {
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"bucket": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"s3_user": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"storage": {
						Type:         schema.TypeString,
						Optional:     true,
						Default:      "filesystem",
						ValidateFunc: validation.StringInSlice([]string{"filesystem", "s3"}, false),
					},
					"url": {
						Type:         schema.TypeString,
						Optional:     true,
						Computed:     true,
						ValidateFunc: validation.IsURLWithHTTPorHTTPS,
					},
				},
			},
		},
		"smtp": {
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"address": {
						Type:     schema.TypeString,
						Required: true,
					},
					"authentication": {
						Type:         schema.TypeString,
						Required:     true,
						ValidateFunc: validation.StringInSlice([]string{"plain", "login"}, false),
					},
					"domain": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"email_from": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"email_reply_to": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"force_tls": {
						Type:         nullable.TypeNullableBool,
						Optional:     true,
						Computed:     true,
						ValidateFunc: nullable.ValidateTypeStringNullableBool,
					},
					"password": {
						Type:      schema.TypeString,
						Optional:  true,
						Sensitive: true,
					},
					"port": {
						Type:         schema.TypeInt,
						Required:     true,
						ValidateFunc: validation.IsPortNumber,
					},
					"starttls_auto": {
						Type:         nullable.TypeNullableBool,
						Optional:     true,
						Computed:     true,
						ValidateFunc: nullable.ValidateTypeStringNullableBool,
					},
					"tls": {
						Type:     schema.TypeBool,
						Optional: true,
						Default:  false,
					},
					"tls_verify_mode": {
						Type:         schema.TypeString,
						Optional:     true,
						ValidateFunc: validation.StringInSlice([]string{"none", "peer"}, false),
					},
					"username": {
						Type:     schema.TypeString,
						Optional: true,
					},
				},
			},
		},
	}
}

func (s gitLabManager) serviceParametersDataSourceSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"external_url": {
			Type:     schema.TypeString,
			Computed: true,
		},
		"ldap": {
			Type:     schema.TypeList,
			Computed: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"active_directory": {
						Type:     nullable.TypeNullableBool,
						Computed: true,
					},
					"allow_username_or_email_login": {
						Type:     nullable.TypeNullableBool,
						Computed: true,
					},
					"base": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"bind_dn": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"ca_cert": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"ciphers": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"client_cert": {
						Type:      schema.TypeString,
						Computed:  true,
						Sensitive: true,
					},
					"client_key": {
						Type:      schema.TypeString,
						Computed:  true,
						Sensitive: true,
					},
					"encryption": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"host": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"label": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"lowercase_usernames": {
						Type:     nullable.TypeNullableBool,
						Computed: true,
					},
					"password": {
						Type:      schema.TypeString,
						Computed:  true,
						Sensitive: true,
					},
					"port": {
						Type:     schema.TypeInt,
						Computed: true,
					},
					"retry_empty_result_with_codes": {
						Type:     schema.TypeList,
						Computed: true,
						Elem:     &schema.Schema{Type: schema.TypeInt},
					},
					"ssl_version": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"timeout": {
						Type:     schema.TypeInt,
						Computed: true,
					},
					"uid": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"user_filter": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"verify_certificates": {
						Type:     nullable.TypeNullableBool,
						Computed: true,
					},
				},
			},
		},
		"oidc": {
			Type:     schema.TypeList,
			Computed: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"client_auth_method": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"client_id": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"client_secret": {
						Type:      schema.TypeString,
						Computed:  true,
						Sensitive: true,
					},
					"issuer": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"label": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"pkce": {
						Type:     nullable.TypeNullableBool,
						Computed: true,
					},
					"send_scope_to_token_endpoint": {
						Type:     schema.TypeBool,
						Computed: true,
					},
					"uid_field": {
						Type:     schema.TypeString,
						Computed: true,
					},
				},
			},
		},
		"password": {
			Type:      schema.TypeString,
			Computed:  true,
			Sensitive: true,
		},
		"registry": {
			Type:     schema.TypeList,
			Computed: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"bucket": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"s3_user": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"storage": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"url": {
						Type:     schema.TypeString,
						Computed: true,
					},
				},
			},
		},
		"smtp": {
			Type:     schema.TypeList,
			Computed: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"address": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"authentication": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"domain": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"email_from": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"email_reply_to": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"force_tls": {
						Type:     nullable.TypeNullableBool,
						Computed: true,
					},
					"password": {
						Type:      schema.TypeString,
						Computed:  true,
						Sensitive: true,
					},
					"port": {
						Type:     schema.TypeInt,
						Computed: true,
					},
					"starttls_auto": {
						Type:     nullable.TypeNullableBool,
						Computed: true,
					},
					"tls": {
						Type:     schema.TypeBool,
						Computed: true,
					},
					"tls_verify_mode": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"username": {
						Type:     schema.TypeString,
						Computed: true,
					},
				},
			},
		},
	}
}

// expandServiceParameters converts terraform representation of GitLab parameters to api representation.
//
// Parameter blocks `registry`, `smtp`, `oidc` and `ldap` are represented in api as plain parameters,
// and block presence is represented by `enable_registry`, `enable_smtp`, `enable_oidc`
// and `enable_ldap` parameters accordingly.
func (s gitLabManager) expandServiceParameters(tfMap map[string]interface{}) ServiceParameters {
	if tfMap == nil {
		return nil
	}

	serviceParameters := ServiceParameters{}

	if v, ok := tfMap["ldap"].([]interface{}); ok && len(v) > 0 && v[0] != nil {
		serviceParameters["enable_ldap"] = true

		for key, value := range expandGitLabLDAPParameters(v[0].(map[string]interface{})) {
			serviceParameters[key] = value
		}
	} else {
		serviceParameters["enable_ldap"] = false
	}

	if v, ok := tfMap["oidc"].([]interface{}); ok && len(v) > 0 && v[0] != nil {
		serviceParameters["enable_oidc"] = true

		for key, value := range expandGitLabOIDCParameters(v[0].(map[string]interface{})) {
			serviceParameters[key] = value
		}
	} else {
		serviceParameters["enable_oidc"] = false
	}

	if v, ok := tfMap["password"].(string); ok && v != "" {
		serviceParameters["password"] = v
	}

	if v, ok := tfMap["registry"].([]interface{}); ok && len(v) > 0 && v[0] != nil {
		serviceParameters["enable_registry"] = true

		for key, value := range expandGitLabRegistryParameters(v[0].(map[string]interface{})) {
			serviceParameters[key] = value
		}
	} else {
		serviceParameters["enable_registry"] = false
	}

	if v, ok := tfMap["smtp"].([]interface{}); ok && len(v) > 0 && v[0] != nil {
		serviceParameters["enable_smtp"] = true

		for key, value := range expandGitLabSMTPParameters(v[0].(map[string]interface{})) {
			serviceParameters[key] = value
		}
	} else {
		serviceParameters["enable_smtp"] = false
	}

	return serviceParameters
}

func expandGitLabLDAPParameters(tfMap map[string]interface{}) ServiceParameters {
	serviceParameters := ServiceParameters{}

	if v, null, _ := nullable.Bool(tfMap["active_directory"].(string)).Value(); !null {
		serviceParameters["active_directory"] = v
	}

	if v, null, _ := nullable.Bool(tfMap["allow_username_or_email_login"].(string)).Value(); !null {
		serviceParameters["allow_username_or_email_login"] = v
	}

	if v, ok := tfMap["base"].(string); ok && v != "" {
		serviceParameters["base"] = v
	}

	if v, ok := tfMap["bind_dn"].(string); ok && v != "" {
		serviceParameters["bind_dn"] = v
	}

	if v, ok := tfMap["ca_cert"].(string); ok && v != "" {
		serviceParameters["ca_cert"] = v
	}

	if v, ok := tfMap["ciphers"].(string); ok && v != "" {
		serviceParameters["ciphers"] = v
	}

	if v, ok := tfMap["client_cert"].(string); ok && v != "" {
		serviceParameters["client_cert"] = v
	}

	if v, ok := tfMap["client_key"].(string); ok && v != "" {
		serviceParameters["client_key"] = v
	}

	if v, ok := tfMap["encryption"].(string); ok && v != "" {
		serviceParameters["encryption"] = v
	}

	if v, ok := tfMap["host"].(string); ok && v != "" {
		serviceParameters["host"] = v
	}

	if v, ok := tfMap["label"].(string); ok && v != "" {
		serviceParameters["label_ldap"] = v
	}

	if v, null, _ := nullable.Bool(tfMap["lowercase_usernames"].(string)).Value(); !null {
		serviceParameters["lowercase_usernames"] = v
	}

	if v, ok := tfMap["password"].(string); ok && v != "" {
		serviceParameters["password_ldap"] = v
	}

	if v, ok := tfMap["port"].(int); ok && v != 0 {
		serviceParameters["port_ldap"] = int64(v)
	}

	if v, ok := tfMap["retry_empty_result_with_codes"].([]interface{}); ok && len(v) > 0 {
		codes := make([]interface{}, len(v))
		for i, code := range v {
			codes[i] = int64(code.(int))
		}
		serviceParameters["retry_empty_result_with_codes"] = codes
	}

	if v, ok := tfMap["ssl_version"].(string); ok && v != "" {
		serviceParameters["ssl_version"] = v
	}

	if v, ok := tfMap["timeout"].(int); ok {
		serviceParameters["timeout"] = int64(v)
	}

	if v, ok := tfMap["uid"].(string); ok && v != "" {
		serviceParameters["uid"] = v
	}

	if v, ok := tfMap["user_filter"].(string); ok && v != "" {
		serviceParameters["user_filter"] = v
	}

	if v, null, _ := nullable.Bool(tfMap["verify_certificates"].(string)).Value(); !null {
		serviceParameters["verify_certificates"] = v
	}

	return serviceParameters
}

func expandGitLabOIDCParameters(tfMap map[string]interface{}) ServiceParameters {
	serviceParameters := ServiceParameters{}

	if v, ok := tfMap["client_auth_method"].(string); ok && v != "" {
		serviceParameters["client_auth_method"] = v
	}

	if v, ok := tfMap["client_id"].(string); ok && v != "" {
		serviceParameters["client_id"] = v
	}

	if v, ok := tfMap["client_secret"].(string); ok && v != "" {
		serviceParameters["client_secret"] = v
	}

	if v, ok := tfMap["issuer"].(string); ok && v != "" {
		serviceParameters["issuer"] = v
	}

	if v, ok := tfMap["label"].(string); ok && v != "" {
		serviceParameters["label_oidc"] = v
	}

	if v, null, _ := nullable.Bool(tfMap["pkce"].(string)).Value(); !null {
		serviceParameters["pkce"] = v
	}

	if v, ok := tfMap["send_scope_to_token_endpoint"].(bool); ok {
		serviceParameters["send_scope_to_token_endpoint"] = v
	}

	if v, ok := tfMap["uid_field"].(string); ok && v != "" {
		serviceParameters["uid_field"] = v
	}

	return serviceParameters
}

func expandGitLabRegistryParameters(tfMap map[string]interface{}) ServiceParameters {
	serviceParameters := ServiceParameters{}

	if v, ok := tfMap["bucket"].(string); ok && v != "" {
		serviceParameters["bucket"] = v
	}

	if v, ok := tfMap["s3_user"].(string); ok && v != "" {
		serviceParameters["s3_user"] = v
	}

	if v, ok := tfMap["storage"].(string); ok && v != "" {
		serviceParameters["storage"] = v
	}

	if v, ok := tfMap["url"].(string); ok && v != "" {
		serviceParameters["url"] = v
	}

	return serviceParameters
}

func expandGitLabSMTPParameters(tfMap map[string]interface{}) ServiceParameters {
	serviceParameters := ServiceParameters{}

	if v, ok := tfMap["address"].(string); ok && v != "" {
		serviceParameters["address"] = v
	}

	if v, ok := tfMap["authentication"].(string); ok && v != "" {
		serviceParameters["authentication"] = v
	}

	if v, ok := tfMap["domain"].(string); ok && v != "" {
		serviceParameters["domain"] = v
	}

	if v, ok := tfMap["email_from"].(string); ok && v != "" {
		serviceParameters["email_from"] = v
	}

	if v, ok := tfMap["email_reply_to"].(string); ok && v != "" {
		serviceParameters["email_reply_to"] = v
	}

	if v, null, _ := nullable.Bool(tfMap["force_tls"].(string)).Value(); !null {
		serviceParameters["force_tls"] = v
	}

	if v, ok := tfMap["password"].(string); ok && v != "" {
		serviceParameters["password_smtp"] = v
	}

	if v, ok := tfMap["port"].(int); ok && v != 0 {
		serviceParameters["port_smtp"] = int64(v)
	}

	if v, null, _ := nullable.Bool(tfMap["starttls_auto"].(string)).Value(); !null {
		serviceParameters["starttls_auto"] = v
	}

	if v, ok := tfMap["tls"].(bool); ok {
		serviceParameters["tls"] = v
	}

	if v, ok := tfMap["tls_verify_mode"].(string); ok && v != "" {
		serviceParameters["tls_verify_mode"] = v
	}

	if v, ok := tfMap["username"].(string); ok && v != "" {
		serviceParameters["username"] = v
	}

	return serviceParameters
}

func (s gitLabManager) flattenServiceParameters(serviceParameters ServiceParameters) map[string]interface{} {
	if serviceParameters == nil {
		return map[string]interface{}{}
	}

	tfMap := map[string]interface{}{}

	if v, ok := serviceParameters["externalUrl"].(string); ok {
		tfMap["external_url"] = v
	}

	if v, ok := serviceParameters["enableLdap"].(bool); ok && v {
		tfMap["ldap"] = []map[string]interface{}{flattenGitLabLDAPParameters(serviceParameters)}
	}

	if v, ok := serviceParameters["enableOidc"].(bool); ok && v {
		tfMap["oidc"] = []map[string]interface{}{flattenGitLabOIDCParameters(serviceParameters)}
	}

	if v, ok := serviceParameters["password"].(string); ok {
		tfMap["password"] = v
	}

	if v, ok := serviceParameters["enableRegistry"].(bool); ok && v {
		tfMap["registry"] = []map[string]interface{}{flattenGitLabRegistryParameters(serviceParameters)}
	}

	if v, ok := serviceParameters["enableSmtp"].(bool); ok && v {
		tfMap["smtp"] = []map[string]interface{}{flattenGitLabSMTPParameters(serviceParameters)}
	}

	return tfMap
}

func flattenGitLabLDAPParameters(serviceParameters ServiceParameters) map[string]interface{} {
	tfMap := map[string]interface{}{}

	if v, ok := serviceParameters["activeDirectory"].(bool); ok {
		tfMap["active_directory"] = strconv.FormatBool(v)
	}

	if v, ok := serviceParameters["allowUsernameOrEmailLogin"].(bool); ok {
		tfMap["allow_username_or_email_login"] = strconv.FormatBool(v)
	}

	if v, ok := serviceParameters["base"].(string); ok {
		tfMap["base"] = v
	}

	if v, ok := serviceParameters["bindDn"].(string); ok {
		tfMap["bind_dn"] = v
	}

	if v, ok := serviceParameters["caCert"].(string); ok {
		tfMap["ca_cert"] = v
	}

	if v, ok := serviceParameters["ciphers"].(string); ok {
		tfMap["ciphers"] = v
	}

	if v, ok := serviceParameters["clientCert"].(string); ok {
		tfMap["client_cert"] = v
	} else if v, ok := serviceParameters["client_cert"].(string); ok {
		tfMap["client_cert"] = v
	}

	if v, ok := serviceParameters["clientKey"].(string); ok {
		tfMap["client_key"] = v
	} else if v, ok := serviceParameters["client_key"].(string); ok {
		tfMap["client_key"] = v
	}

	if v, ok := serviceParameters["encryption"].(string); ok {
		tfMap["encryption"] = v
	}

	if v, ok := serviceParameters["host"].(string); ok {
		tfMap["host"] = v
	}

	if v, ok := serviceParameters["labelLdap"].(string); ok {
		tfMap["label"] = v
	}

	if v, ok := serviceParameters["lowercaseUsernames"].(bool); ok {
		tfMap["lowercase_usernames"] = strconv.FormatBool(v)
	}

	if v, ok := serviceParameters["passwordLdap"].(string); ok {
		tfMap["password"] = v
	}

	if v, ok := serviceParameters["portLdap"].(int64); ok {
		tfMap["port"] = v
	}

	if v, ok := serviceParameters["retryEmptyResultWithCodes"].([]interface{}); ok {
		tfMap["retry_empty_result_with_codes"] = v
	}

	if v, ok := serviceParameters["sslVersion"].(string); ok {
		tfMap["ssl_version"] = v
	}

	if v, ok := serviceParameters["timeout"].(int64); ok {
		tfMap["timeout"] = v
	}

	if v, ok := serviceParameters["uid"].(string); ok {
		tfMap["uid"] = v
	}

	if v, ok := serviceParameters["userFilter"].(string); ok {
		tfMap["user_filter"] = v
	}

	if v, ok := serviceParameters["verifyCertificates"].(bool); ok {
		tfMap["verify_certificates"] = strconv.FormatBool(v)
	}

	return tfMap
}

func flattenGitLabOIDCParameters(serviceParameters ServiceParameters) map[string]interface{} {
	tfMap := map[string]interface{}{}

	if v, ok := serviceParameters["clientAuthMethod"].(string); ok {
		tfMap["client_auth_method"] = v
	}

	if v, ok := serviceParameters["clientId"].(string); ok {
		tfMap["client_id"] = v
	}

	if v, ok := serviceParameters["clientSecret"].(string); ok {
		tfMap["client_secret"] = v
	}

	if v, ok := serviceParameters["issuer"].(string); ok {
		tfMap["issuer"] = v
	}

	if v, ok := serviceParameters["labelOidc"].(string); ok {
		tfMap["label"] = v
	}

	if v, ok := serviceParameters["pkce"].(bool); ok {
		tfMap["pkce"] = strconv.FormatBool(v)
	}

	if v, ok := serviceParameters["sendScopeToTokenEndpoint"].(bool); ok {
		tfMap["send_scope_to_token_endpoint"] = v
	}

	if v, ok := serviceParameters["uidField"].(string); ok {
		tfMap["uid_field"] = v
	}

	return tfMap
}

func flattenGitLabRegistryParameters(serviceParameters ServiceParameters) map[string]interface{} {
	tfMap := map[string]interface{}{}

	if v, ok := serviceParameters["bucket"].(string); ok {
		tfMap["bucket"] = v
	}

	if v, ok := serviceParameters["s3User"].(string); ok {
		tfMap["s3_user"] = v
	}

	if v, ok := serviceParameters["storage"].(string); ok {
		tfMap["storage"] = v
	}

	if v, ok := serviceParameters["url"].(string); ok {
		tfMap["url"] = v
	}

	return tfMap
}

func flattenGitLabSMTPParameters(serviceParameters ServiceParameters) map[string]interface{} {
	tfMap := map[string]interface{}{}

	if v, ok := serviceParameters["address"].(string); ok {
		tfMap["address"] = v
	}

	if v, ok := serviceParameters["authentication"].(string); ok {
		tfMap["authentication"] = v
	}

	if v, ok := serviceParameters["domain"].(string); ok {
		tfMap["domain"] = v
	}

	if v, ok := serviceParameters["emailFrom"].(string); ok {
		tfMap["email_from"] = v
	}

	if v, ok := serviceParameters["emailReplyTo"].(string); ok {
		tfMap["email_reply_to"] = v
	}

	if v, ok := serviceParameters["forceTls"].(bool); ok {
		tfMap["force_tls"] = strconv.FormatBool(v)
	}

	if v, ok := serviceParameters["passwordSmtp"].(string); ok {
		tfMap["password"] = v
	}

	if v, ok := serviceParameters["portSmtp"].(int64); ok {
		tfMap["port"] = v
	}

	if v, ok := serviceParameters["starttlsAuto"].(bool); ok {
		tfMap["starttls_auto"] = strconv.FormatBool(v)
	}

	if v, ok := serviceParameters["tls"].(bool); ok {
		tfMap["tls"] = v
	}

	if v, ok := serviceParameters["tlsVerifyMode"].(string); ok {
		tfMap["tls_verify_mode"] = v
	}

	if v, ok := serviceParameters["username"].(string); ok {
		tfMap["username"] = v
	}

	return tfMap
}
