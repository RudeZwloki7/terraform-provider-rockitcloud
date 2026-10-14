package paas

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestResourceServiceGitLabReadPreservesInputOnlySecrets(t *testing.T) {
	t.Parallel()

	resource := ResourceService()
	config := testGitLabInternalConfig(true)
	data := schema.TestResourceDataRaw(t, resource.Schema, config)
	data.SetId(testGitLabInternalServiceID)
	conn := testPaaSClientWithDebugBodyLogging(&recordingSDKLogger{}, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.URL.Path != "/services/"+testGitLabInternalServiceID {
			t.Fatalf("unexpected PaaS request: %s %s", request.Method, request.URL.Path)
		}
		return jsonResponse(request, testGitLabInternalResponse(t, true, nil)), nil
	}))
	if diags := resourceServiceRead(context.Background(), data, testPaaSClientMeta(conn)); diags.HasError() {
		t.Fatalf("reading GitLab service: %#v", diags)
	}
	for path, want := range testGitLabInternalSecrets() {
		if got := data.State().Attributes[path]; got != want {
			t.Errorf("input-only secret %s was lost after refresh", path)
		}
	}
	diff, err := resource.Diff(context.Background(), data.State(), terraform.NewResourceConfigRaw(config), nil)
	if err != nil {
		t.Fatal(err)
	}
	if diff != nil && !diff.Empty() {
		t.Fatalf("refresh with omitted secrets must not create a diff or replacement: %#v", diff)
	}
}

func TestResourceServiceGitLabCreateDefersIntegrations(t *testing.T) {
	t.Parallel()

	for _, integrations := range []bool{false, true} {
		t.Run(map[bool]string{false: "without integrations", true: "with integrations"}[integrations], func(t *testing.T) {
			var events []string
			var createParameters, updateParameters map[string]interface{}
			conn := testPaaSClientWithDebugBodyLogging(&recordingSDKLogger{}, roundTripFunc(func(request *http.Request) (*http.Response, error) {
				events = append(events, request.Method+" "+request.URL.Path)
				switch {
				case request.Method == http.MethodPost && request.URL.Path == "/services":
					createParameters = testGitLabInternalRequestParameters(t, request)
					return jsonResponse(request, `{"service":{"id":"`+testGitLabInternalServiceID+`","serviceType":"gitlab"}}`), nil
				case request.Method == http.MethodPut && request.URL.Path == "/services/"+testGitLabInternalServiceID+"/parameters":
					updateParameters = testGitLabInternalRequestParameters(t, request)
					return jsonResponse(request, `{}`), nil
				case request.Method == http.MethodGet && request.URL.Path == "/services/"+testGitLabInternalServiceID:
					return jsonResponse(request, testGitLabInternalResponse(t, integrations, nil)), nil
				default:
					t.Fatalf("unexpected PaaS request: %s %s", request.Method, request.URL.Path)
					return nil, nil
				}
			}))
			resource := ResourceService()
			data := schema.TestResourceDataRaw(t, resource.Schema, testGitLabInternalConfig(integrations))
			data.MarkNewResource()
			if diags := resourceServiceCreate(context.Background(), data, testPaaSClientMeta(conn)); diags.HasError() {
				t.Fatalf("creating GitLab service: %#v", diags)
			}
			if createParameters["password"] != testGitLabInternalSecrets()["gitlab.0.password"] {
				t.Fatal("initial root password must be sent on create")
			}
			for _, key := range []string{"issuer", "client_secret", "host", "password_ldap", "client_cert", "client_key", "address", "password_smtp"} {
				if _, exists := createParameters[key]; exists {
					t.Errorf("deferred integration field %s must not be sent on create", key)
				}
			}
			if !integrations {
				if updateParameters != nil {
					t.Fatal("create without deferred blocks must not send a parameters update")
				}
				return
			}
			if updateParameters == nil {
				t.Fatal("deferred integration parameters were not applied after create")
			}
			if _, exists := updateParameters["password"]; exists {
				t.Error("initial root password must not be resent by deferred parameters update")
			}
			for _, key := range []string{"enable_ldap", "enable_oidc", "enable_smtp"} {
				if updateParameters[key] != true {
					t.Errorf("deferred update must enable integration %s", key)
				}
			}
			for key, want := range map[string]string{"password_ldap": "gitlab-ldap-test-secret", "password_smtp": "gitlab-smtp-test-secret", "client_secret": "gitlab-oidc-test-secret"} {
				if updateParameters[key] != want {
					t.Errorf("deferred update lost %s", key)
				}
			}
			if len(events) < 3 || events[0] != "POST /services" || events[1] != "GET /services/"+testGitLabInternalServiceID {
				t.Fatalf("deferred parameters must follow the create waiter: %#v", events)
			}
		})
	}
}

func TestResourceServiceGitLabUpdateDisablesIntegrations(t *testing.T) {
	t.Parallel()

	resource := ResourceService()
	initial := schema.TestResourceDataRaw(t, resource.Schema, testGitLabInternalConfig(true))
	initial.SetId(testGitLabInternalServiceID)
	config := testGitLabInternalConfig(false)
	diff, err := resource.Diff(context.Background(), initial.State(), terraform.NewResourceConfigRaw(config), nil)
	if err != nil || diff == nil || diff.RequiresNew() {
		t.Fatalf("removing GitLab integrations must update in place: %#v, %v", diff, err)
	}
	data, err := schema.InternalMap(resource.Schema).Data(initial.State(), diff)
	if err != nil {
		t.Fatal(err)
	}
	var updateParameters map[string]interface{}
	conn := testPaaSClientWithDebugBodyLogging(&recordingSDKLogger{}, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodPut && request.URL.Path == "/services/"+testGitLabInternalServiceID+"/parameters":
			updateParameters = testGitLabInternalRequestParameters(t, request)
			return jsonResponse(request, `{}`), nil
		case request.Method == http.MethodGet && request.URL.Path == "/services/"+testGitLabInternalServiceID:
			return jsonResponse(request, testGitLabInternalResponse(t, false, nil)), nil
		default:
			t.Fatalf("unexpected PaaS request: %s %s", request.Method, request.URL.Path)
			return nil, nil
		}
	}))
	if diags := resourceServiceUpdate(context.Background(), data, testPaaSClientMeta(conn)); diags.HasError() {
		t.Fatalf("disabling GitLab integrations: %#v", diags)
	}
	for _, key := range []string{"enable_ldap", "enable_oidc", "enable_smtp", "enable_registry"} {
		if updateParameters[key] != false {
			t.Errorf("removing a block must explicitly disable %s", key)
		}
	}
	if _, exists := updateParameters["password"]; exists {
		t.Error("initial root password must not be sent on settings update")
	}
	for _, block := range []string{"ldap", "oidc", "smtp", "registry"} {
		if blocks := data.Get("gitlab.0." + block).([]interface{}); len(blocks) != 0 {
			t.Errorf("disabled %s block reappeared after refresh", block)
		}
	}
	diff, err = resource.Diff(context.Background(), data.State(), terraform.NewResourceConfigRaw(config), nil)
	if err != nil || (diff != nil && !diff.Empty()) {
		t.Fatalf("disabling integrations must converge after refresh: %#v, %v", diff, err)
	}
}

func TestResourceServiceGitLabReadUsesReturnedSecrets(t *testing.T) {
	t.Parallel()

	data := schema.TestResourceDataRaw(t, ResourceService().Schema, testGitLabInternalConfig(true))
	data.SetId(testGitLabInternalServiceID)
	returned := map[string]interface{}{
		"password": "returned-root-test-secret", "passwordLdap": "returned-ldap-test-secret", "clientCert": "returned-cert-test-secret", "clientKey": "returned-key-test-secret",
		"clientSecret": "returned-oidc-test-secret", "passwordSmtp": "returned-smtp-test-secret", "address": "smtp-normalized.example.invalid",
	}
	conn := testPaaSClientWithDebugBodyLogging(&recordingSDKLogger{}, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return jsonResponse(request, testGitLabInternalResponse(t, true, returned)), nil
	}))
	if diags := resourceServiceRead(context.Background(), data, testPaaSClientMeta(conn)); diags.HasError() {
		t.Fatalf("reading GitLab service: %#v", diags)
	}
	for path, want := range map[string]string{
		"gitlab.0.password": "returned-root-test-secret", "gitlab.0.ldap.0.password": "returned-ldap-test-secret", "gitlab.0.ldap.0.client_cert": "returned-cert-test-secret",
		"gitlab.0.ldap.0.client_key": "returned-key-test-secret", "gitlab.0.oidc.0.client_secret": "returned-oidc-test-secret", "gitlab.0.smtp.0.password": "returned-smtp-test-secret",
		"gitlab.0.smtp.0.address": "smtp-normalized.example.invalid",
	} {
		if data.State().Attributes[path] != want {
			t.Errorf("API value at %s was overwritten by previous state", path)
		}
	}
}

func TestResourceServiceGitLabReadDoesNotRestoreDisabledBlocks(t *testing.T) {
	t.Parallel()

	data := schema.TestResourceDataRaw(t, ResourceService().Schema, testGitLabInternalConfig(true))
	data.SetId(testGitLabInternalServiceID)
	conn := testPaaSClientWithDebugBodyLogging(&recordingSDKLogger{}, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return jsonResponse(request, testGitLabInternalResponse(t, false, nil)), nil
	}))
	if diags := resourceServiceRead(context.Background(), data, testPaaSClientMeta(conn)); diags.HasError() {
		t.Fatalf("reading GitLab service: %#v", diags)
	}
	for _, block := range []string{"ldap", "oidc", "smtp", "registry"} {
		if data.State().Attributes["gitlab.0."+block+".#"] != "0" {
			t.Errorf("remote disabled %s was restored from previous state", block)
		}
	}
}

func TestResourceServiceGitLabUpdateDoesNotRestoreRemovedSecrets(t *testing.T) {
	t.Parallel()

	resource := ResourceService()
	initial := schema.TestResourceDataRaw(t, resource.Schema, testGitLabInternalConfig(true))
	initial.SetId(testGitLabInternalServiceID)
	config := testGitLabInternalConfig(true)
	parameters := config["gitlab"].([]interface{})[0].(map[string]interface{})
	for block, keys := range map[string][]string{"ldap": {"password", "client_cert", "client_key"}, "oidc": {"client_secret"}, "smtp": {"password"}} {
		for _, key := range keys {
			delete(parameters[block].([]interface{})[0].(map[string]interface{}), key)
		}
	}
	diff, err := resource.Diff(context.Background(), initial.State(), terraform.NewResourceConfigRaw(config), nil)
	if err != nil || diff == nil || diff.RequiresNew() {
		t.Fatalf("removing integration credentials must update in place: %v", err)
	}
	data, err := schema.InternalMap(resource.Schema).Data(initial.State(), diff)
	if err != nil {
		t.Fatal(err)
	}
	conn := testPaaSClientWithDebugBodyLogging(&recordingSDKLogger{}, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodPut {
			parameters := testGitLabInternalRequestParameters(t, request)
			for _, key := range []string{"password", "password_ldap", "password_smtp", "client_secret", "client_cert", "client_key"} {
				if _, exists := parameters[key]; exists {
					t.Errorf("removed or initial-only %s was sent on update", key)
				}
			}
			return jsonResponse(request, `{}`), nil
		}
		return jsonResponse(request, testGitLabInternalResponse(t, true, nil)), nil
	}))
	if diags := resourceServiceUpdate(context.Background(), data, testPaaSClientMeta(conn)); diags.HasError() {
		t.Fatalf("removing integration credentials: %#v", diags)
	}
	for path := range testGitLabInternalSecrets() {
		if path != "gitlab.0.password" && data.State().Attributes[path] != "" {
			t.Errorf("removed secret %s was restored", path)
		}
	}
}

func TestDataSourceServiceGitLabReadDoesNotRetainElidedSecrets(t *testing.T) {
	t.Parallel()

	data := schema.TestResourceDataRaw(t, DataSourceService().Schema, map[string]interface{}{"id": testGitLabInternalServiceID})
	if err := data.Set("gitlab", testGitLabInternalConfig(true)["gitlab"]); err != nil {
		t.Fatal(err)
	}
	conn := testPaaSClientWithDebugBodyLogging(&recordingSDKLogger{}, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return jsonResponse(request, testGitLabInternalResponse(t, true, nil)), nil
	}))
	if diags := dataSourceServiceRead(context.Background(), data, testPaaSClientMeta(conn)); diags.HasError() {
		t.Fatalf("reading GitLab data source: %#v", diags)
	}
	for path := range testGitLabInternalSecrets() {
		if data.State().Attributes[path] != "" {
			t.Errorf("data source kept an elided secret from previous state at %s", path)
		}
	}
}

const testGitLabInternalServiceID = "fm-cluster-12345678"

func testGitLabInternalSecrets() map[string]string {
	return map[string]string{
		"gitlab.0.password":             "gitlab-root-test-secret",
		"gitlab.0.ldap.0.password":      "gitlab-ldap-test-secret",
		"gitlab.0.ldap.0.client_cert":   "gitlab-client-cert-test-secret",
		"gitlab.0.ldap.0.client_key":    "gitlab-client-key-test-secret",
		"gitlab.0.oidc.0.client_secret": "gitlab-oidc-test-secret",
		"gitlab.0.smtp.0.password":      "gitlab-smtp-test-secret",
	}
}

func testGitLabInternalConfig(integrations bool) map[string]interface{} {
	parameters := map[string]interface{}{"password": testGitLabInternalSecrets()["gitlab.0.password"]}
	if integrations {
		parameters["ldap"] = []interface{}{map[string]interface{}{
			"base": "dc=example,dc=invalid", "host": "ldap.example.invalid", "label": "Directory", "port": 389, "encryption": "start_tls", "uid": "uid",
			"password": "gitlab-ldap-test-secret", "client_cert": "gitlab-client-cert-test-secret", "client_key": "gitlab-client-key-test-secret",
		}}
		parameters["oidc"] = []interface{}{map[string]interface{}{"issuer": "https://oidc.example.invalid", "client_secret": "gitlab-oidc-test-secret"}}
		parameters["smtp"] = []interface{}{map[string]interface{}{"address": "smtp.example.invalid", "port": 587, "authentication": "plain", "password": "gitlab-smtp-test-secret"}}
		parameters["registry"] = []interface{}{map[string]interface{}{"storage": "filesystem"}}
	}
	return map[string]interface{}{
		"name": "tf-gitlab-test", "instance_type": "c5.large", "subnet_ids": []interface{}{"subnet-12345678"}, "security_group_ids": []interface{}{"sg-12345678"},
		"root_volume": []interface{}{map[string]interface{}{"type": "gp2", "size": 32}}, "data_volume": []interface{}{map[string]interface{}{"type": "gp2", "size": 32}},
		"gitlab": []interface{}{parameters},
	}
}

func testGitLabInternalResponse(t *testing.T, integrations bool, overrides map[string]interface{}) string {
	t.Helper()
	parameters := map[string]interface{}{"externalUrl": "http://gitlab.example.invalid", "enableLdap": integrations, "enableOidc": integrations, "enableSmtp": integrations, "enableRegistry": integrations}
	if integrations {
		for key, value := range map[string]interface{}{
			"base": "dc=example,dc=invalid", "host": "ldap.example.invalid", "labelLdap": "Directory", "portLdap": 389, "encryption": "start_tls", "timeout": 10, "uid": "uid",
			"issuer": "https://oidc.example.invalid", "labelOidc": "OpenID Connect", "uidField": "sub", "sendScopeToTokenEndpoint": true,
			"address": "smtp.example.invalid", "authentication": "plain", "portSmtp": 587, "tls": false, "storage": "filesystem",
		} {
			parameters[key] = value
		}
	}
	for key, value := range overrides {
		parameters[key] = value
	}
	body, err := json.Marshal(map[string]interface{}{"service": map[string]interface{}{
		"id": testGitLabInternalServiceID, "name": "tf-gitlab-test", "serviceType": "gitlab", "serviceClass": "development", "status": "READY", "instanceType": "c5.large", "instanceQty": 1,
		"rootVolumeType": "gp2", "rootVolumeSize": 32, "dataVolumeType": "gp2", "dataVolumeSize": 32, "subnetIds": []string{"subnet-12345678"},
		"securityGroups": []map[string]interface{}{{"id": "sg-12345678", "createdAutomatically": false}}, "parameters": parameters,
	}})
	if err != nil {
		t.Fatalf("encoding GitLab HTTP response fixture: %s", err)
	}
	return string(body)
}

func testGitLabInternalRequestParameters(t *testing.T, request *http.Request) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	parameters, ok := body["parameters"].(map[string]interface{})
	if !ok {
		t.Fatalf("request has no parameters object: %s", strings.Join([]string{request.Method, request.URL.Path}, " "))
	}
	return parameters
}
