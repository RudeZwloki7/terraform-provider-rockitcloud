package paas

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	sdkpaas "github.com/aws/aws-sdk-go/service/paas"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestResourceServiceCreateRejectsMissingServiceID(t *testing.T) {
	t.Parallel()

	for _, body := range []string{
		`{}`,
		`{"service":null}`,
		`{"service":{}}`,
		`{"service":{"id":""}}`,
	} {
		t.Run(body, func(t *testing.T) {
			conn := testPaaSClientWithDebugBodyLogging(&recordingSDKLogger{}, roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if request.Method != http.MethodPost || request.URL.Path != "/services" {
					t.Fatalf("unexpected PaaS request after malformed create response: %s %s", request.Method, request.URL.Path)
				}
				return jsonResponse(request, body), nil
			}))
			resourceData := schema.TestResourceDataRaw(t, ResourceService().Schema, testClickHouseSensitiveLoggingConfig("password", "option"))
			resourceData.MarkNewResource()

			diagnostics := resourceServiceCreate(context.Background(), resourceData, testPaaSClientMeta(conn))
			if !diagnostics.HasError() || !strings.Contains(diagnostics[0].Summary, "missing service ID in response") {
				t.Fatalf("expected missing service ID error, got %#v", diagnostics)
			}
			if resourceData.Id() != "" {
				t.Fatalf("malformed create response must not assign a resource ID, got %q", resourceData.Id())
			}
		})
	}
}

func TestResourceServiceClickHouseMutationsDoNotLogSecrets(t *testing.T) {
	// The provider uses the process-wide standard logger. Keep this test serial
	// while capturing it; parallel tests start after this test returns.
	var providerLogs bytes.Buffer
	originalLogOutput := log.Writer()
	log.SetOutput(&providerLogs)
	defer log.SetOutput(originalLogOutput)

	const (
		password     = "clickhouse-user-password-secret"
		optionSecret = "clickhouse-option-secret"
		serviceID    = "fm-cluster-12345678"
	)

	for _, operation := range []string{"create", "update parameters", "update users"} {
		t.Run(operation, func(t *testing.T) {
			providerLogs.Reset()
			logger := &recordingSDKLogger{}
			mutationRequests := make(map[string]bool)
			conn := testPaaSClientWithDebugBodyLogging(logger, roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if request.Method == http.MethodPost || request.Method == http.MethodPut {
					mutationRequests[request.Method+" "+request.URL.Path] = true
					if request.Method == http.MethodPost && request.URL.Path == "/services" {
						return jsonResponse(request, `{"service":{"id":"`+serviceID+`","serviceType":"clickhouse","serviceClass":"database"}}`), nil
					}
					if request.Method == http.MethodPut && strings.HasPrefix(request.URL.Path, "/services/"+serviceID) {
						return jsonResponse(request, `{}`), nil
					}
				}
				if request.Method == http.MethodGet && request.URL.Path == "/services/"+serviceID {
					return jsonResponse(request, `{
						"service": {
							"id": "`+serviceID+`",
							"name": "tf-ch-log-test",
							"serviceType": "clickhouse",
							"serviceClass": "database",
							"status": "READY",
							"instanceType": "c5.large",
							"rootVolumeType": "gp2",
							"rootVolumeSize": 32,
							"dataVolumeType": "gp2",
							"dataVolumeSize": 32,
							"additionalRoles": ["coordinator"],
							"parameters": {"version":"24.8", "options":{"s3_secret_access_key":"`+optionSecret+`"}},
							"users": [{"name":"application", "parameters":{"password":"`+password+`","passwordType":"double_sha1","profile":"default","quota":"default"}}]
						}
					}`), nil
				}
				t.Fatalf("unexpected PaaS request: %s %s", request.Method, request.URL.Path)
				return nil, nil
			}))

			resource := ResourceService()
			config := testClickHouseSensitiveLoggingConfig(password, optionSecret)
			resourceData := schema.TestResourceDataRaw(t, resource.Schema, config)
			if operation == "create" {
				resourceData.MarkNewResource()
			} else {
				oldPassword, oldOption := password, optionSecret
				if operation == "update users" {
					oldPassword = "previous-password"
				} else {
					oldOption = "previous-option-value"
				}
				oldData := schema.TestResourceDataRaw(t, resource.Schema, testClickHouseSensitiveLoggingConfig(oldPassword, oldOption))
				oldData.SetId(serviceID)
				state := oldData.State()
				diff, err := resource.Diff(context.Background(), state, terraform.NewResourceConfigRaw(config), nil)
				if err != nil {
					t.Fatalf("calculating ClickHouse diff: %s", err)
				}
				resourceData, err = schema.InternalMap(resource.Schema).Data(state, diff)
				if err != nil {
					t.Fatalf("building ClickHouse update data: %s", err)
				}
			}

			mutate := resourceServiceUpdate
			if operation == "create" {
				mutate = resourceServiceCreate
			}
			if diagnostics := mutate(context.Background(), resourceData, testPaaSClientMeta(conn)); diagnostics.HasError() {
				t.Fatalf("%s ClickHouse service: %#v", operation, diagnostics)
			}

			expectedRequest := "PUT /services/" + serviceID + "/parameters"
			if operation == "create" {
				expectedRequest = "POST /services"
			} else if operation == "update users" {
				expectedRequest = "PUT /services/" + serviceID
			}
			if !mutationRequests[expectedRequest] {
				t.Fatalf("expected mutation was not sent: %s", expectedRequest)
			}
			for _, secret := range []string{password, optionSecret} {
				if strings.Contains(providerLogs.String(), secret) {
					t.Fatal("ClickHouse secret was written to provider logs")
				}
				if strings.Contains(logger.String(), secret) {
					t.Fatal("ClickHouse secret was written to AWS SDK debug logs")
				}
			}
		})
	}
}

func TestResourceServiceClickHouseRemovesLastUserAndDatabase(t *testing.T) {
	t.Parallel()

	const serviceID = "fm-cluster-12345678"
	var modifyBody map[string]interface{}
	conn := testPaaSClientWithDebugBodyLogging(&recordingSDKLogger{}, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodPut {
			if request.URL.Path == "/services/"+serviceID {
				if err := json.NewDecoder(request.Body).Decode(&modifyBody); err != nil {
					t.Fatalf("decoding ModifyService request: %s", err)
				}
			}
			return jsonResponse(request, `{}`), nil
		}
		if request.Method == http.MethodGet && request.URL.Path == "/services/"+serviceID {
			return jsonResponse(request, `{"service":{"id":"`+serviceID+`","serviceType":"clickhouse","serviceClass":"database","status":"READY","parameters":{"version":"24.8"}}}`), nil
		}
		t.Fatalf("unexpected PaaS request: %s %s", request.Method, request.URL.Path)
		return nil, nil
	}))

	resource := ResourceService()
	oldConfig := testClickHouseSensitiveLoggingConfig("password", "option")
	oldConfig["clickhouse"].([]interface{})[0].(map[string]interface{})["database"] = []interface{}{map[string]interface{}{"name": "application"}}
	oldData := schema.TestResourceDataRaw(t, resource.Schema, oldConfig)
	oldData.SetId(serviceID)
	newConfig := testClickHouseSensitiveLoggingConfig("password", "option")
	delete(newConfig["clickhouse"].([]interface{})[0].(map[string]interface{}), "user")
	state := oldData.State()
	diff, err := resource.Diff(context.Background(), state, terraform.NewResourceConfigRaw(newConfig), nil)
	if err != nil {
		t.Fatalf("calculating ClickHouse removal diff: %s", err)
	}
	resourceData, err := schema.InternalMap(resource.Schema).Data(state, diff)
	if err != nil {
		t.Fatalf("building ClickHouse removal data: %s", err)
	}
	if diagnostics := resourceServiceUpdate(context.Background(), resourceData, testPaaSClientMeta(conn)); diagnostics.HasError() {
		t.Fatalf("removing ClickHouse users and databases: %#v", diagnostics)
	}
	for _, key := range []string{"users", "databases"} {
		values, ok := modifyBody[key].([]interface{})
		if !ok || len(values) != 0 {
			t.Fatalf("%s must be sent as an empty JSON array, got %#v", key, modifyBody[key])
		}
	}
}

func testClickHouseSensitiveLoggingConfig(password, optionSecret string) map[string]interface{} {
	return map[string]interface{}{
		"name":               "tf-ch-log-test",
		"instance_type":      "c5.large",
		"additional_roles":   []interface{}{"coordinator"},
		"subnet_ids":         []interface{}{"subnet-12345678"},
		"security_group_ids": []interface{}{"sg-12345678"},
		"root_volume":        []interface{}{map[string]interface{}{"type": "gp2", "size": 32}},
		"data_volume":        []interface{}{map[string]interface{}{"type": "gp2", "size": 32}},
		"clickhouse": []interface{}{map[string]interface{}{
			"class":   "database",
			"version": "24.8",
			"options": map[string]interface{}{"s3_secret_access_key": optionSecret},
			"user":    []interface{}{map[string]interface{}{"name": "application", "password": password}},
		}},
	}
}

func TestResourceServiceCreateDoesNotLogELKPassword(t *testing.T) {
	t.Parallel()

	const password = "elk-password-secret"

	logger := &recordingSDKLogger{}
	conn := testPaaSClientWithDebugBodyLogging(logger, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/services":
			return jsonResponse(request, `{
				"service": {
					"id": "fm-cluster-12345678",
					"serviceType": "elk",
					"serviceClass": "logging"
				}
			}`), nil
		case request.Method == http.MethodGet && request.URL.Path == "/services/fm-cluster-12345678":
			return jsonResponse(request, `{
				"service": {
					"id": "fm-cluster-12345678",
					"name": "tf-elk-test",
					"serviceType": "elk",
					"serviceClass": "logging",
					"status": "READY",
					"instanceType": "c5.large",
					"rootVolumeType": "gp2",
					"rootVolumeSize": 32,
					"dataVolumeType": "gp2",
					"dataVolumeSize": 32,
					"parameters": {
						"password": "`+password+`",
						"version": "8.17"
					}
				}
			}`), nil
		case request.Method == http.MethodPut &&
			strings.HasPrefix(request.URL.Path, "/services/fm-cluster-12345678/"):
			return jsonResponse(request, `{}`), nil
		default:
			t.Fatalf("unexpected PaaS request: %s %s", request.Method, request.URL.Path)
			return nil, nil
		}
	}))

	resourceData := schema.TestResourceDataRaw(t, ResourceService().Schema, map[string]interface{}{
		"name":          "tf-elk-test",
		"instance_type": "c5.large",
		"root_volume": []interface{}{
			map[string]interface{}{
				"type": "gp2",
				"size": 32,
			},
		},
		"data_volume": []interface{}{
			map[string]interface{}{
				"type": "gp2",
				"size": 32,
			},
		},
		"nodes": []interface{}{
			map[string]interface{}{
				"main": []interface{}{
					map[string]interface{}{
						"role": "node",
					},
				},
			},
		},
		"subnet_ids": []interface{}{"subnet-12345678"},
		"elk": []interface{}{
			map[string]interface{}{
				"class":    "logging",
				"password": password,
				"version":  "8.17",
			},
		},
	})

	diagnostics := resourceServiceCreate(
		context.Background(),
		resourceData,
		testPaaSClientMeta(conn),
	)
	if diagnostics.HasError() {
		t.Fatalf("creating ELK service: %#v", diagnostics)
	}
	if got := resourceData.Id(); got != "fm-cluster-12345678" {
		t.Fatalf("unexpected service ID: %q", got)
	}
	if strings.Contains(logger.String(), password) {
		t.Fatal("ELK password was written to AWS SDK debug logs")
	}
}

func TestFindServiceByIDDoesNotLogELKPassword(t *testing.T) {
	t.Parallel()

	const password = "elk-password-secret"

	logger := &recordingSDKLogger{}
	conn := testPaaSClientWithDebugBodyLogging(logger, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return jsonResponse(request, `{
			"service": {
				"id": "fm-cluster-12345678",
				"serviceType": "elk",
				"serviceClass": "logging",
				"parameters": {
					"password": "`+password+`",
					"version": "8.17"
				}
			}
		}`), nil
	}))

	service, err := FindServiceByID(conn, "fm-cluster-12345678")
	if err != nil {
		t.Fatalf("reading ELK service: %s", err)
	}
	if got := aws.StringValue(service.ServiceType); got != "elk" {
		t.Fatalf("unexpected service type: %q", got)
	}
	if strings.Contains(logger.String(), password) {
		t.Fatal("ELK password was written to AWS SDK debug logs")
	}
}

func TestResourceLogstashPipelineCreateDoesNotLogConfiguration(t *testing.T) {
	t.Parallel()

	const configuration = "input { stdin {} } # pipeline-secret"

	logger := &recordingSDKLogger{}
	conn := testPaaSClientWithDebugBodyLogging(logger, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodPost &&
			request.URL.Path == "/services/fm-cluster-12345678/logstash-pipelines":
			return jsonResponse(request, `{
				"logstashPipeline": {
					"id": "logstash-pipeline-87654321",
					"name": "terraform-qa",
					"configuration": "`+configuration+`"
				}
			}`), nil
		case request.Method == http.MethodGet &&
			request.URL.Path == "/services/fm-cluster-12345678":
			return jsonResponse(request, `{
				"service": {
					"id": "fm-cluster-12345678",
					"serviceType": "elk",
					"serviceClass": "logging",
					"status": "READY",
					"parameters": {
						"version": "8.17"
					}
				}
			}`), nil
		case request.Method == http.MethodGet &&
			request.URL.Path == "/services/fm-cluster-12345678/logstash-pipelines":
			return jsonResponse(request, `{
				"logstashPipelines": [
					{
						"id": "logstash-pipeline-87654321",
						"name": "terraform-qa",
						"configuration": "`+configuration+`"
					}
				]
			}`), nil
		default:
			t.Fatalf("unexpected PaaS request: %s %s", request.Method, request.URL.Path)
			return nil, nil
		}
	}))

	resourceData := schema.TestResourceDataRaw(t, ResourceLogstashPipeline().Schema, map[string]interface{}{
		"service_id":    "fm-cluster-12345678",
		"name":          "terraform-qa",
		"configuration": configuration,
	})

	diagnostics := resourceLogstashPipelineCreate(
		context.Background(),
		resourceData,
		testPaaSClientMeta(conn),
	)
	if diagnostics.HasError() {
		t.Fatalf("creating Logstash pipeline: %#v", diagnostics)
	}
	if got := resourceData.Id(); got != "logstash-pipeline-87654321" {
		t.Fatalf("unexpected pipeline ID: %q", got)
	}
	if strings.Contains(logger.String(), "pipeline-secret") {
		t.Fatal("Logstash configuration was written to AWS SDK debug logs")
	}
}

func TestFindLogstashPipelineByIDDoesNotLogConfiguration(t *testing.T) {
	t.Parallel()

	const configuration = "input { http { password => pipeline-secret } }"

	logger := &recordingSDKLogger{}
	conn := testPaaSClientWithDebugBodyLogging(logger, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return jsonResponse(request, `{
			"logstashPipelines": [
				{
					"id": "logstash-pipeline-87654321",
					"name": "terraform-qa",
					"configuration": "`+configuration+`"
				}
			]
		}`), nil
	}))

	pipeline, err := FindLogstashPipelineByID(
		context.Background(),
		conn,
		"fm-cluster-12345678",
		"logstash-pipeline-87654321",
	)
	if err != nil {
		t.Fatalf("reading Logstash pipeline: %s", err)
	}
	if got := aws.StringValue(pipeline.Name); got != "terraform-qa" {
		t.Fatalf("unexpected pipeline name: %q", got)
	}
	if strings.Contains(logger.String(), configuration) {
		t.Fatal("Logstash configuration was written to AWS SDK debug logs")
	}
}

func TestResourceLogstashPipelineUpdateDoesNotLogConfiguration(t *testing.T) {
	t.Parallel()

	const configuration = "input { http { password => pipeline-update-secret } }"

	logger := &recordingSDKLogger{}
	modifyCalled := false
	conn := testPaaSClientWithDebugBodyLogging(logger, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodPut &&
			request.URL.Path == "/services/fm-cluster-12345678/logstash-pipelines/logstash-pipeline-87654321":
			modifyCalled = true
			return jsonResponse(request, `{}`), nil
		case request.Method == http.MethodGet &&
			request.URL.Path == "/services/fm-cluster-12345678":
			return jsonResponse(request, `{
				"service": {
					"id": "fm-cluster-12345678",
					"serviceType": "elk",
					"serviceClass": "logging",
					"status": "READY",
					"parameters": {
						"version": "8.17"
					}
				}
			}`), nil
		case request.Method == http.MethodGet &&
			request.URL.Path == "/services/fm-cluster-12345678/logstash-pipelines":
			return jsonResponse(request, `{
				"logstashPipelines": [
					{
						"id": "logstash-pipeline-87654321",
						"name": "terraform-qa",
						"configuration": "`+configuration+`"
					}
				]
			}`), nil
		default:
			t.Fatalf("unexpected PaaS request: %s %s", request.Method, request.URL.Path)
			return nil, nil
		}
	}))

	resourceData := schema.TestResourceDataRaw(t, ResourceLogstashPipeline().Schema, map[string]interface{}{
		"service_id":    "fm-cluster-12345678",
		"name":          "terraform-qa",
		"configuration": configuration,
	})
	resourceData.SetId("logstash-pipeline-87654321")

	diagnostics := resourceLogstashPipelineUpdate(
		context.Background(),
		resourceData,
		testPaaSClientMeta(conn),
	)
	if diagnostics.HasError() {
		t.Fatalf("updating Logstash pipeline: %#v", diagnostics)
	}
	if !modifyCalled {
		t.Fatal("Logstash pipeline update request was not sent")
	}
	if strings.Contains(logger.String(), "pipeline-update-secret") {
		t.Fatal("updated Logstash configuration was written to AWS SDK debug logs")
	}
}

func testPaaSClientWithDebugBodyLogging(logger aws.Logger, transport http.RoundTripper) *sdkpaas.PaaS {
	return sdkpaas.New(session.Must(session.NewSession(&aws.Config{
		Credentials: credentials.NewStaticCredentials("access-key", "secret-key", ""),
		Endpoint:    aws.String("http://paas.test"),
		HTTPClient:  &http.Client{Transport: transport},
		Logger:      logger,
		LogLevel:    aws.LogLevel(aws.LogDebugWithHTTPBody),
		Region:      aws.String("ru-msk"),
	})))
}

func jsonResponse(request *http.Request, body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body:    io.NopCloser(strings.NewReader(body)),
		Request: request,
	}
}

type recordingSDKLogger struct {
	mu      sync.Mutex
	builder strings.Builder
}

func (l *recordingSDKLogger) Log(values ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	fmt.Fprint(&l.builder, values...)
}

func (l *recordingSDKLogger) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.builder.String()
}

func TestResourceServiceGitLabMutationsDoNotLogSecrets(t *testing.T) {
	// Capturing the process-wide provider logger requires a serial test.
	var providerLogs bytes.Buffer
	originalLogOutput := log.Writer()
	log.SetOutput(&providerLogs)
	defer log.SetOutput(originalLogOutput)

	logger := &recordingSDKLogger{}
	seenSecrets := map[string]bool{}
	mutations := map[string]bool{}
	conn := testPaaSClientWithDebugBodyLogging(logger, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodPost || request.Method == http.MethodPut {
			mutations[request.Method+" "+request.URL.Path] = true
			parameters := testGitLabInternalRequestParameters(t, request)
			body, err := json.Marshal(parameters)
			if err != nil {
				t.Fatal(err)
			}
			for _, secret := range testGitLabInternalSecrets() {
				if strings.Contains(string(body), secret) {
					seenSecrets[secret] = true
				}
			}
			if request.Method == http.MethodPost {
				return jsonResponse(request, `{"service":{"id":"`+testGitLabInternalServiceID+`","serviceType":"gitlab"}}`), nil
			}
			return jsonResponse(request, `{}`), nil
		}
		return jsonResponse(request, testGitLabInternalResponse(t, true, map[string]interface{}{
			"password": "gitlab-root-test-secret", "passwordLdap": "gitlab-ldap-test-secret", "passwordSmtp": "gitlab-smtp-test-secret", "clientSecret": "gitlab-oidc-test-secret",
			"clientCert": "gitlab-client-cert-test-secret", "clientKey": "gitlab-client-key-test-secret",
		})), nil
	}))
	data := schema.TestResourceDataRaw(t, ResourceService().Schema, testGitLabInternalConfig(true))
	data.MarkNewResource()
	if diags := resourceServiceCreate(context.Background(), data, testPaaSClientMeta(conn)); diags.HasError() {
		t.Fatalf("creating GitLab with deferred integrations: %#v", diags)
	}
	if !mutations["POST /services"] || !mutations["PUT /services/"+testGitLabInternalServiceID+"/parameters"] {
		t.Fatal("both create and deferred parameters update must be exercised")
	}
	for path, secret := range testGitLabInternalSecrets() {
		if !seenSecrets[secret] {
			t.Errorf("sensitive input %s was not exercised in an SDK mutation", path)
		}
		if strings.Contains(providerLogs.String(), secret) || strings.Contains(logger.String(), secret) {
			t.Errorf("GitLab secret %s was written to provider or SDK HTTP debug logs", path)
		}
	}
}
