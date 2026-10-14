package services

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/paas"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestGitLabSchema(t *testing.T) {
	t.Parallel()

	resourceSchema := GitLab.ResourceSchema().Elem.(*schema.Resource).Schema
	dataSourceSchema := GitLab.DataSourceSchema().Elem.(*schema.Resource).Schema
	var compare func(string, map[string]*schema.Schema, map[string]*schema.Schema)
	compare = func(path string, resourceFields, dataSourceFields map[string]*schema.Schema) {
		t.Helper()
		if len(resourceFields) != len(dataSourceFields) {
			t.Errorf("%s resource/data source field counts differ: %d/%d", path, len(resourceFields), len(dataSourceFields))
		}
		for name, resourceField := range resourceFields {
			fieldPath := path + name
			dataSourceField, exists := dataSourceFields[name]
			if !exists {
				t.Errorf("%s missing from data source", fieldPath)
				continue
			}
			if resourceField.Type != dataSourceField.Type || resourceField.Sensitive != dataSourceField.Sensitive {
				t.Errorf("%s resource/data source type or sensitivity differs", fieldPath)
			}
			if !dataSourceField.Computed || dataSourceField.Optional || dataSourceField.Required {
				t.Errorf("%s data source must be computed only", fieldPath)
			}
			if nested, ok := resourceField.Elem.(*schema.Resource); ok {
				dataSourceNested, ok := dataSourceField.Elem.(*schema.Resource)
				if !ok {
					t.Errorf("%s data source nested schema missing", fieldPath)
					continue
				}
				compare(fieldPath+".", nested.Schema, dataSourceNested.Schema)
			}
		}
	}
	compare("", resourceSchema, dataSourceSchema)
	for _, path := range []string{"password", "ldap.password", "ldap.client_cert", "ldap.client_key", "oidc.client_secret", "smtp.password"} {
		for _, fields := range []map[string]*schema.Schema{resourceSchema, dataSourceSchema} {
			parts := strings.Split(path, ".")
			field := fields[parts[0]]
			if len(parts) == 2 {
				field = field.Elem.(*schema.Resource).Schema[parts[1]]
			}
			if !field.Sensitive {
				t.Errorf("%s must be sensitive in both schemas", path)
			}
		}
	}
	if !resourceSchema["password"].ForceNew || !resourceSchema["password"].Required {
		t.Error("initial root password must be required and immutable")
	}
	for _, name := range []string{"user", "database"} {
		if _, exists := resourceSchema[name]; exists {
			t.Errorf("GitLab must not expose PaaS %s management", name)
		}
	}
	if !GitLab.dataVolumeRequired || GitLab.allowBackup || GitLab.allowArbitrator {
		t.Error("unexpected GitLab volume, backup or arbitrator capabilities")
	}
}

func TestGitLabSMTPRequiredConfiguration(t *testing.T) {
	t.Parallel()

	resource := &schema.Resource{Schema: GitLab.serviceParametersSchema()}
	for _, testCase := range []struct {
		name      string
		smtp      map[string]interface{}
		wantError bool
	}{
		{name: "missing address and authentication", smtp: map[string]interface{}{"port": 25}, wantError: true},
		{name: "missing address", smtp: map[string]interface{}{"port": 25, "authentication": "plain"}, wantError: true},
		{name: "missing authentication", smtp: map[string]interface{}{"address": "smtp.example.invalid", "port": 25}, wantError: true},
		{name: "plain without credentials", smtp: map[string]interface{}{"address": "smtp.example.invalid", "port": 25, "authentication": "plain"}},
		{name: "login without credentials", smtp: map[string]interface{}{"address": "smtp.example.invalid", "port": 25, "authentication": "login"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			config := map[string]interface{}{
				"password": "gitlab-root-test-secret",
				"smtp":     []interface{}{testCase.smtp},
			}
			if diagnostics := resource.Validate(terraform.NewResourceConfigRaw(config)); diagnostics.HasError() != testCase.wantError {
				t.Fatalf("unexpected SMTP configuration validation: %#v", diagnostics)
			}
			if testCase.wantError {
				return
			}
			parameters := GitLab.expandServiceParameters(testGitLabNormalizedParameters(t, config))
			if parameters["enable_smtp"] != true || parameters["address"] != testCase.smtp["address"] || parameters["authentication"] != testCase.smtp["authentication"] || parameters["port_smtp"] != int64(25) || parameters["tls"] != false {
				t.Fatalf("SMTP required fields or defaults not expanded: %#v", parameters)
			}
			for _, key := range []string{"username", "password_smtp", "force_tls", "starttls_auto"} {
				if _, exists := parameters[key]; exists {
					t.Errorf("omitted SMTP field %s must not be sent", key)
				}
			}
		})
	}
}

func TestGitLabLDAPUIDConfiguration(t *testing.T) {
	t.Parallel()

	resource := &schema.Resource{Schema: GitLab.serviceParametersSchema()}
	ldapSchema := resource.Schema["ldap"].Elem.(*schema.Resource).Schema
	if !ldapSchema["uid"].Required || ldapSchema["uid"].Optional || ldapSchema["uid"].Default != nil {
		t.Error("LDAP uid must be explicitly configured without an inferred default")
	}
	for _, testCase := range []struct {
		name      string
		uid       interface{}
		wantError bool
	}{
		{name: "omitted", wantError: true},
		{name: "empty", uid: "", wantError: true},
		{name: "OpenLDAP", uid: "uid"},
		{name: "Active Directory", uid: "sAMAccountName"},
		{name: "custom attribute", uid: "employeeIdentifier"},
		{name: "explicit sub is valid", uid: "sub"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			ldap := map[string]interface{}{
				"base": "dc=example,dc=invalid", "encryption": "start_tls", "host": "ldap.example.invalid",
				"label": "Directory", "port": 389,
			}
			if testCase.uid != nil {
				ldap["uid"] = testCase.uid
			}
			config := terraform.NewResourceConfigRaw(map[string]interface{}{
				"password": "gitlab-root-test-secret", "ldap": []interface{}{ldap},
				"oidc": []interface{}{map[string]interface{}{"issuer": "https://oidc.example.invalid"}},
			})
			// Terraform validates configuration before creating the plan.
			if diagnostics := resource.Validate(config); diagnostics.HasError() != testCase.wantError {
				t.Fatalf("unexpected LDAP uid validation: %#v", diagnostics)
			}
			if testCase.wantError {
				return
			}
			if _, err := resource.Diff(context.Background(), nil, config, nil); err != nil {
				t.Fatalf("explicit LDAP uid must allow planning: %v", err)
			}
			parameters := GitLab.expandServiceParameters(testGitLabNormalizedParameters(t, config.Raw))
			if parameters["uid"] != testCase.uid {
				t.Fatalf("explicit LDAP uid was replaced: %#v", parameters["uid"])
			}
			if parameters["uid_field"] != "sub" {
				t.Fatalf("OIDC uid_field default must remain sub: %#v", parameters["uid_field"])
			}
		})
	}
}

func TestGitLabExpandParameters(t *testing.T) {
	t.Parallel()

	input := testGitLabNormalizedParameters(t, testGitLabAllParameters())
	got := GitLab.expandServiceParameters(input)
	want := ServiceParameters{
		"enable_ldap": true, "active_directory": false, "allow_username_or_email_login": false,
		"base": "dc=example,dc=invalid", "bind_dn": "cn=gitlab,dc=example,dc=invalid", "ca_cert": "test-ca",
		"ciphers": "test-cipher", "client_cert": "test-client-cert", "client_key": "test-client-key",
		"encryption": "start_tls", "host": "ldap.example.invalid", "label_ldap": "Directory",
		"lowercase_usernames": false, "password_ldap": "test-ldap-secret", "port_ldap": int64(389),
		"retry_empty_result_with_codes": []interface{}{int64(0), int64(80)}, "ssl_version": "TLSv1_2", "timeout": int64(0),
		"uid": "uid", "user_filter": "(objectClass=person)", "verify_certificates": false,
		"enable_oidc": true, "client_auth_method": "basic", "client_id": "test-client", "client_secret": "test-oidc-secret",
		"issuer": "https://oidc.example.invalid", "label_oidc": "OpenID", "pkce": false,
		"send_scope_to_token_endpoint": false, "uid_field": "sub",
		"enable_registry": true, "bucket": "gitlab-test-bucket", "s3_user": "gitlab-test-user",
		"storage": "s3", "url": "https://s3.example.invalid",
		"enable_smtp": true, "address": "smtp.example.invalid", "authentication": "plain", "domain": "example.invalid",
		"email_from": "gitlab@example.invalid", "email_reply_to": "support@example.invalid", "force_tls": false,
		"password_smtp": "test-smtp-secret", "port_smtp": int64(587), "starttls_auto": false, "tls": false,
		"tls_verify_mode": "peer", "username": "gitlab",
		"password": "gitlab-root-test-secret",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expanded GitLab parameters:\ngot  %#v\nwant %#v", got, want)
	}
}

func TestGitLabLDAPRetryCodesSDKRequest(t *testing.T) {
	t.Parallel()

	input := testGitLabNormalizedParameters(t, map[string]interface{}{
		"password": "gitlab-root-test-secret",
		"ldap": []interface{}{map[string]interface{}{
			"base": "dc=example,dc=invalid", "encryption": "start_tls", "host": "ldap.example.invalid",
			"label": "Directory", "port": 389, "uid": "uid", "retry_empty_result_with_codes": []interface{}{int(0), int(80)},
		}},
	})
	parameters := GitLab.expandServiceParameters(input)
	client := paas.New(session.Must(session.NewSession(&aws.Config{
		Region: aws.String("test-region"), Endpoint: aws.String("https://paas.example.invalid"),
		Credentials: credentials.NewStaticCredentials("test-access", "test-secret", ""),
	})))
	request, _ := client.ModifyServiceParametersRequest(&paas.ModifyServiceParametersInput{
		ServiceId: aws.String("fm-cluster-12345678"), Parameters: parameters,
	})
	if err := request.Build(); err != nil {
		t.Fatalf("LDAP retry codes must serialize to a valid SDK request: %v", err)
	}
	codes := parameters["retry_empty_result_with_codes"].([]interface{})
	if !reflect.DeepEqual(codes, []interface{}{int64(0), int64(80)}) {
		t.Fatalf("Terraform integers must be converted to SDK int64 values: %#v", codes)
	}
	var body struct {
		Parameters struct {
			RetryCodes []int64 `json:"retry_empty_result_with_codes"`
		} `json:"parameters"`
	}
	if err := json.NewDecoder(request.HTTPRequest.Body).Decode(&body); err != nil {
		t.Fatalf("SDK request body must be valid JSON: %v", err)
	}
	if !reflect.DeepEqual(body.Parameters.RetryCodes, []int64{0, 80}) {
		t.Fatalf("SDK request lost LDAP retry codes: %#v", body.Parameters.RetryCodes)
	}
}

func TestGitLabCreationDefersIntegrations(t *testing.T) {
	t.Parallel()

	input := testGitLabNormalizedParameters(t, testGitLabAllParameters())
	creation := GitLab.ExpandServiceCreationParameters(input)
	if creation["password"] != input["password"] || creation["enable_registry"] != true || creation["storage"] != "s3" {
		t.Fatalf("initial password and registry must be available on create: %#v", creation)
	}
	for _, key := range []string{"enable_ldap", "enable_oidc", "enable_smtp"} {
		if creation[key] == true {
			t.Errorf("deferred integration must not be enabled at create: %s", key)
		}
	}
	for _, key := range []string{"base", "host", "password_ldap", "client_cert", "client_key", "issuer", "client_id", "client_secret", "address", "password_smtp", "port_smtp"} {
		if _, exists := creation[key]; exists {
			t.Errorf("deferred field %s must not be sent on create", key)
		}
	}
	if !GitLab.HasDeferredParameters(input) {
		t.Fatal("creation must retain deferred settings for the subsequent update")
	}
	for _, block := range []string{"ldap", "oidc", "smtp"} {
		if len(input[block].([]interface{})) != 1 {
			t.Errorf("creation expansion mutated the source %s block", block)
		}
		if !GitLab.HasDeferredParameters(map[string]interface{}{block: input[block]}) {
			t.Errorf("%s alone must cause a deferred update", block)
		}
	}
	for _, input := range []map[string]interface{}{nil, {}, {"registry": []interface{}{map[string]interface{}{"storage": "filesystem"}}}, {"ldap": []interface{}{}, "oidc": []interface{}{}, "smtp": []interface{}{}}} {
		if GitLab.HasDeferredParameters(input) {
			t.Errorf("non-deferred settings unexpectedly require an update: %#v", input)
		}
	}
}

func TestGitLabBlockRemoval(t *testing.T) {
	t.Parallel()

	if got := GitLab.expandServiceParameters(nil); got != nil {
		t.Fatalf("nil parameters must remain nil: %#v", got)
	}
	parameters := GitLab.expandServiceParameters(map[string]interface{}{})
	for _, key := range []string{"enable_ldap", "enable_oidc", "enable_registry", "enable_smtp"} {
		if parameters[key] != false {
			t.Errorf("removed block must send %s=false: %#v", key, parameters)
		}
	}
	// A disabled integration can still have stale settings in an API response.
	// Those settings must not resurrect a removed Terraform block.
	response := ServiceParameters{
		"enableLdap": false, "host": "ldap.example.invalid", "passwordLdap": "old-ldap-secret",
		"enableOidc": false, "issuer": "https://oidc.example.invalid", "clientSecret": "old-oidc-secret",
		"enableRegistry": false, "storage": "filesystem",
		"enableSmtp": false, "address": "smtp.example.invalid", "passwordSmtp": "old-smtp-secret",
	}
	if got := GitLab.flattenServiceParameters(response); len(got) != 0 {
		t.Fatalf("disabled integration settings must not appear in state: %#v", got)
	}
	if got := GitLab.flattenServiceParameters(nil); len(got) != 0 {
		t.Fatalf("nil response must flatten to empty settings: %#v", got)
	}
}

func TestGitLabLDAPClientCertificateResponseAliases(t *testing.T) {
	t.Parallel()

	for _, input := range []ServiceParameters{
		{"clientCert": "test-cert", "clientKey": "test-key"},
		{"client_cert": "test-cert", "client_key": "test-key"},
		{"clientCert": "test-cert", "clientKey": "test-key", "client_cert": "legacy-cert", "client_key": "legacy-key"},
	} {
		got := flattenGitLabLDAPParameters(input)
		if got["client_cert"] != "test-cert" || got["client_key"] != "test-key" {
			t.Fatalf("LDAP client credentials were not read compatibly: %#v", got)
		}
	}
}

func TestGitLabFlattenSDKResponse(t *testing.T) {
	t.Parallel()

	// Exercise the SDK free-form parameters decoder as well as every GitLab
	// field mapping. This verifies numeric list values and false/zero values
	// survive HTTP decoding and can be written to Terraform state.
	parameters := ServiceParameters{
		"externalUrl": "https://gitlab.example.invalid", "password": "gitlab-root-test-secret",
		"enableLdap": true, "activeDirectory": false, "allowUsernameOrEmailLogin": false,
		"base": "dc=example,dc=invalid", "bindDn": "cn=gitlab,dc=example,dc=invalid", "caCert": "test-ca",
		"ciphers": "test-cipher", "clientCert": "test-client-cert", "clientKey": "test-client-key",
		"encryption": "start_tls", "host": "ldap.example.invalid", "labelLdap": "Directory",
		"lowercaseUsernames": false, "passwordLdap": "test-ldap-secret", "portLdap": 389,
		"retryEmptyResultWithCodes": []interface{}{0, 80}, "sslVersion": "TLSv1_2", "timeout": 0,
		"uid": "uid", "userFilter": "(objectClass=person)", "verifyCertificates": false,
		"enableOidc": true, "clientAuthMethod": "basic", "clientId": "test-client", "clientSecret": "test-oidc-secret",
		"issuer": "https://oidc.example.invalid", "labelOidc": "OpenID", "pkce": false,
		"sendScopeToTokenEndpoint": false, "uidField": "sub",
		"enableRegistry": true, "bucket": "gitlab-test-bucket", "s3User": "gitlab-test-user",
		"storage": "s3", "url": "https://s3.example.invalid",
		"enableSmtp": true, "address": "smtp.example.invalid", "authentication": "plain", "domain": "example.invalid",
		"emailFrom": "gitlab@example.invalid", "emailReplyTo": "support@example.invalid", "forceTls": false,
		"passwordSmtp": "test-smtp-secret", "portSmtp": 587, "starttlsAuto": false, "tls": false,
		"tlsVerifyMode": "peer", "username": "gitlab",
	}
	body, err := json.Marshal(map[string]interface{}{"service": map[string]interface{}{"id": "fm-cluster-12345678", "serviceType": "gitlab", "parameters": parameters}})
	if err != nil {
		t.Fatal(err)
	}
	client := paas.New(session.Must(session.NewSession(&aws.Config{
		Region: aws.String("test-region"), Endpoint: aws.String("https://paas.example.invalid"),
		Credentials: credentials.NewStaticCredentials("test-access", "test-secret", ""),
		HTTPClient: &http.Client{Transport: gitLabTestTransport(func(request *http.Request) (*http.Response, error) {
			if request.Method != http.MethodGet || request.URL.Path != "/services/fm-cluster-12345678" {
				t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
			}
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(string(body))), Request: request}, nil
		})},
	})))
	output, err := client.DescribeServiceWithContext(context.Background(), &paas.DescribeServiceInput{ServiceId: aws.String("fm-cluster-12345678")})
	if err != nil {
		t.Fatal(err)
	}
	if output.Service == nil {
		t.Fatal("SDK did not decode the service")
	}
	flat := GitLab.flattenServiceParameters(output.Service.Parameters)
	data := schema.TestResourceDataRaw(t, GitLab.serviceParametersSchema(), nil)
	for key, value := range flat {
		if err := data.Set(key, value); err != nil {
			t.Fatalf("writing flattened %s to Terraform state: %v", key, err)
		}
	}
	want := testGitLabNormalizedParameters(t, testGitLabAllParameters())
	for _, key := range []string{"password", "ldap", "oidc", "registry", "smtp"} {
		if got := data.Get(key); !reflect.DeepEqual(got, want[key]) {
			t.Errorf("%s did not survive HTTP decode and state conversion:\ngot  %#v\nwant %#v", key, got, want[key])
		}
	}
	if got := data.Get("external_url"); got != "https://gitlab.example.invalid" {
		t.Errorf("external URL was lost: %#v", got)
	}
}

type gitLabTestTransport func(*http.Request) (*http.Response, error)

func (f gitLabTestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func testGitLabNormalizedParameters(t *testing.T, input map[string]interface{}) map[string]interface{} {
	t.Helper()
	fields := GitLab.serviceParametersSchema()
	data := schema.TestResourceDataRaw(t, fields, input)
	result := make(map[string]interface{}, len(fields))
	for key := range fields {
		result[key] = data.Get(key)
	}
	return result
}

func testGitLabAllParameters() map[string]interface{} {
	return map[string]interface{}{
		"password": "gitlab-root-test-secret",
		"ldap": []interface{}{map[string]interface{}{
			"active_directory": "false", "allow_username_or_email_login": "false", "base": "dc=example,dc=invalid",
			"bind_dn": "cn=gitlab,dc=example,dc=invalid", "ca_cert": "test-ca", "ciphers": "test-cipher",
			"client_cert": "test-client-cert", "client_key": "test-client-key", "encryption": "start_tls",
			"host": "ldap.example.invalid", "label": "Directory", "lowercase_usernames": "false", "password": "test-ldap-secret",
			"port": 389, "retry_empty_result_with_codes": []interface{}{0, 80}, "ssl_version": "TLSv1_2", "timeout": 0,
			"uid": "uid", "user_filter": "(objectClass=person)", "verify_certificates": "false",
		}},
		"oidc": []interface{}{map[string]interface{}{
			"client_auth_method": "basic", "client_id": "test-client", "client_secret": "test-oidc-secret", "issuer": "https://oidc.example.invalid",
			"label": "OpenID", "pkce": "false", "send_scope_to_token_endpoint": false, "uid_field": "sub",
		}},
		"registry": []interface{}{map[string]interface{}{"bucket": "gitlab-test-bucket", "s3_user": "gitlab-test-user", "storage": "s3", "url": "https://s3.example.invalid"}},
		"smtp": []interface{}{map[string]interface{}{
			"address": "smtp.example.invalid", "authentication": "plain", "domain": "example.invalid", "email_from": "gitlab@example.invalid",
			"email_reply_to": "support@example.invalid", "force_tls": "false", "password": "test-smtp-secret", "port": 587,
			"starttls_auto": "false", "tls": false, "tls_verify_mode": "peer", "username": "gitlab",
		}},
	}
}
