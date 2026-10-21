package paas

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/service/paas/services"
)

func TestResourceServiceMySQLBinaryLogDiff(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		old, new map[string]interface{}
		wantDiff bool
	}{
		"enable": {
			old: map[string]interface{}{}, new: map[string]interface{}{"bin_log_enabled": "true"}, wantDiff: true,
		},
		"disable": {
			old: map[string]interface{}{"bin_log_enabled": "true", "binlog_retention_days": 7},
			new: map[string]interface{}{"bin_log_enabled": "false"}, wantDiff: true,
		},
		"retention": {
			old: map[string]interface{}{"bin_log_enabled": "true", "binlog_retention_days": 7},
			new: map[string]interface{}{"bin_log_enabled": "true", "binlog_retention_days": 30}, wantDiff: true,
		},
		"omission preserves the existing settings": {
			old: map[string]interface{}{"bin_log_enabled": "true", "binlog_retention_days": 7},
			new: map[string]interface{}{},
		},
		"older environment with no binary log parameters": {
			old: map[string]interface{}{}, new: map[string]interface{}{},
		},
	} {
		t.Run(name, func(t *testing.T) {
			resource := ResourceService()
			initial := schema.TestResourceDataRaw(t, resource.Schema, testMySQLServiceConfig(tc.old))
			initial.SetId(testMySQLServiceID)
			diff, err := resource.Diff(context.Background(), initial.State(), terraform.NewResourceConfigRaw(testMySQLServiceConfig(tc.new)), nil)
			if err != nil {
				t.Fatalf("calculating MySQL diff: %s", err)
			}
			if tc.wantDiff {
				if diff == nil || diff.Empty() || diff.RequiresNew() {
					t.Fatalf("binary log must update in place: %#v", diff)
				}
				return
			}
			if diff != nil {
				for key, attrDiff := range diff.Attributes {
					if strings.HasPrefix(key, "mysql.") {
						t.Errorf("unset settings produced a diff: %s: %#v", key, attrDiff)
					}
				}
			}
		})
	}
}

func TestResourceServiceMySQLBinaryLogRetentionValidation(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name               string
		enabled, retention cty.Value
		previousRetention  bool
		withoutMySQL       bool
		wantError          bool
	}{
		{name: "retention with enabled binary log", enabled: cty.StringVal("true"), retention: cty.NumberIntVal(7)},
		{name: "enabled binary log with default retention", enabled: cty.StringVal("true"), retention: cty.NullVal(cty.Number)},
		{name: "retention without binary log flag", enabled: cty.NullVal(cty.String), retention: cty.NumberIntVal(7), wantError: true},
		{name: "retention with disabled binary log", enabled: cty.StringVal("false"), retention: cty.NumberIntVal(7), wantError: true},
		{name: "unknown binary log flag", enabled: cty.UnknownVal(cty.String), retention: cty.NumberIntVal(7)},
		{name: "unknown retention", enabled: cty.StringVal("false"), retention: cty.UnknownVal(cty.Number)},
		{name: "disable with computed retention from state", enabled: cty.StringVal("false"), retention: cty.NullVal(cty.Number), previousRetention: true},
		{name: "other service without MySQL configuration", enabled: cty.NullVal(cty.String), retention: cty.NullVal(cty.Number), withoutMySQL: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			resource := ResourceService()
			rawConfig := cty.ObjectVal(map[string]cty.Value{
				"mysql": cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{
					"vendor":                cty.StringVal("mysql"),
					"version":               cty.StringVal("8.0.32"),
					"bin_log_enabled":       testCase.enabled,
					"binlog_retention_days": testCase.retention,
				})}),
			})
			if testCase.withoutMySQL {
				rawConfig = cty.ObjectVal(map[string]cty.Value{
					"pgsql": cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{"version": cty.StringVal("18")})}),
				})
			}
			previous := &terraform.InstanceState{RawConfig: rawConfig}
			if testCase.previousRetention {
				initial := schema.TestResourceDataRaw(t, resource.Schema, testMySQLServiceConfig(map[string]interface{}{
					"bin_log_enabled": "true", "binlog_retention_days": 7,
				}))
				initial.SetId(testMySQLServiceID)
				previous = initial.State()
				previous.RawConfig = rawConfig
			}
			config := terraform.NewResourceConfigShimmed(rawConfig, resource.CoreConfigSchema())
			_, err := resource.Diff(context.Background(), previous, config, nil)
			if testCase.wantError {
				if err == nil || !strings.Contains(err.Error(), "binlog_retention_days") {
					t.Fatalf("expected binary log retention validation error, got %v", err)
				}
			} else if err != nil {
				t.Fatalf("valid binary log configuration rejected: %s", err)
			}
		})
	}
}

func TestResourceServiceMySQLUpdateRequests(t *testing.T) {
	t.Parallel()

	user := func(privileges ...interface{}) []interface{} {
		parameters := map[string]interface{}{"name": "application", "password": "mysql-test-password"}
		if len(privileges) > 0 {
			parameters["global_privileges"] = privileges
		}
		return []interface{}{parameters}
	}
	servicePath := "/services/" + testMySQLServiceID
	for name, tc := range map[string]struct {
		old, new map[string]interface{}
		disabled bool
		revoked  bool
	}{
		"disable sends false and omits stale retention": {
			old: map[string]interface{}{"bin_log_enabled": "true", "binlog_retention_days": 7},
			new: map[string]interface{}{"bin_log_enabled": "false"}, disabled: true,
		},
		"removing global privileges replaces user parameters": {
			old: map[string]interface{}{"bin_log_enabled": "true", "binlog_retention_days": 7, "user": user("SELECT", "REPLICATION CLIENT")},
			new: map[string]interface{}{"bin_log_enabled": "true", "binlog_retention_days": 7, "user": user()}, revoked: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			resource := ResourceService()
			initial := schema.TestResourceDataRaw(t, resource.Schema, testMySQLServiceConfig(tc.old))
			initial.SetId(testMySQLServiceID)
			diff, err := resource.Diff(context.Background(), initial.State(), terraform.NewResourceConfigRaw(testMySQLServiceConfig(tc.new)), nil)
			if err != nil || diff == nil || diff.Empty() || diff.RequiresNew() {
				t.Fatalf("MySQL settings must update in place: %#v, %v", diff, err)
			}
			data, err := schema.InternalMap(resource.Schema).Data(initial.State(), diff)
			if err != nil {
				t.Fatalf("building MySQL resource data: %s", err)
			}

			parametersUpdated, userUpdated := false, false
			conn := testPaaSClientWithDebugBodyLogging(&recordingSDKLogger{}, roundTripFunc(func(request *http.Request) (*http.Response, error) {
				switch {
				case request.Method == http.MethodPut && request.URL.Path == servicePath+"/parameters":
					parameters := testGitLabInternalRequestParameters(t, request)
					if flag, ok := parameters["bin_log_enabled"].(bool); !ok || flag != !tc.disabled {
						t.Errorf("binary log update must contain a JSON boolean: %#v", parameters)
					}
					if tc.disabled {
						if _, exists := parameters["binlog_retention_days"]; exists {
							t.Errorf("disabled binary log must omit retention: %#v", parameters)
						}
					} else if parameters["binlog_retention_days"] != float64(7) {
						t.Errorf("full parameters PUT lost binary log retention: %#v", parameters)
					}
					parametersUpdated = true
					return jsonResponse(request, `{}`), nil
				case request.Method == http.MethodPut && request.URL.Path == servicePath:
					var body struct {
						Users []struct {
							Name       string                 `json:"name"`
							Parameters map[string]interface{} `json:"parameters"`
						} `json:"users"`
					}
					if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
						t.Fatal(err)
					}
					if len(body.Users) != 1 || body.Users[0].Name != "application" || body.Users[0].Parameters["password"] != "mysql-test-password" {
						t.Fatalf("full user PUT must preserve the user and password: %#v", body)
					}
					if _, exists := body.Users[0].Parameters["global_privileges"]; exists {
						t.Errorf("revocation must omit global_privileges from replacement user parameters: %#v", body)
					}
					userUpdated = true
					return jsonResponse(request, `{}`), nil
				case request.Method == http.MethodGet && request.URL.Path == servicePath:
					return jsonResponse(request, testMySQLServiceResponse(t, !tc.disabled, tc.revoked)), nil
				default:
					t.Fatalf("unexpected PaaS request: %s %s", request.Method, request.URL.Path)
					return nil, nil
				}
			}))
			if diags := resourceServiceUpdate(context.Background(), data, testPaaSClientMeta(conn)); diags.HasError() {
				t.Fatalf("updating MySQL service: %#v", diags)
			}
			if !parametersUpdated || userUpdated != tc.revoked {
				t.Errorf("expected parameters update and optional user replacement: parameters=%t users=%t", parametersUpdated, userUpdated)
			}
		})
	}
}

func testMySQLServiceConfig(parameters map[string]interface{}) map[string]interface{} {
	settings := map[string]interface{}{"vendor": "mysql", "version": "8.0.32"}
	for key, value := range parameters {
		settings[key] = value
	}
	return map[string]interface{}{
		"name":                "tf-mysql-test",
		"instance_type":       "c5.large",
		"arbitrator_required": false,
		"root_volume":         []interface{}{map[string]interface{}{"type": "gp2", "size": 32}},
		"data_volume":         []interface{}{map[string]interface{}{"type": "gp2", "size": 32}},
		"subnet_ids":          []interface{}{"subnet-12345678"},
		"mysql":               []interface{}{settings},
	}
}

func testMySQLServiceResponse(t *testing.T, enabled, withUser bool) string {
	t.Helper()
	parameters := map[string]interface{}{"vendor": "mysql", "version": "8.0.32", "binLogEnabled": enabled}
	if enabled {
		parameters["binlogRetentionDays"] = 7
	}
	service := map[string]interface{}{
		"id": testMySQLServiceID, "name": "tf-mysql-test", "serviceType": services.ServiceTypeMySQL, "serviceClass": "database", "status": "READY",
		"instanceType": "c5.large", "rootVolumeType": "gp2", "rootVolumeSize": 32, "dataVolumeType": "gp2", "dataVolumeSize": 32,
		"subnetIds": []string{"subnet-12345678"}, "parameters": parameters,
	}
	if withUser {
		service["users"] = []interface{}{map[string]interface{}{"name": "application", "parameters": map[string]interface{}{"password": "mysql-test-password"}}}
	}
	body, err := json.Marshal(map[string]interface{}{"service": service})
	if err != nil {
		t.Fatalf("encoding MySQL HTTP response fixture: %s", err)
	}
	return string(body)
}

const testMySQLServiceID = "fm-cluster-12345678"
