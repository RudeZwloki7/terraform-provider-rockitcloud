package paas

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/paas"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestPostgreSQLRoleVersionValidation(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name, previousVersion, errorText string
		version, role                    cty.Value
	}{
		{name: "monitor on 15", version: cty.StringVal("15"), role: cty.StringVal("pg_monitor")},
		{name: "subscription on 16", version: cty.StringVal("16"), role: cty.StringVal("pg_create_subscription")},
		{name: "maintenance on 17", version: cty.StringVal("17"), role: cty.StringVal("pg_maintain")},
		{name: "subscription on 15", version: cty.StringVal("15"), role: cty.StringVal("pg_create_subscription"), errorText: "pg_create_subscription requires PostgreSQL 16"},
		{name: "maintenance on 16", version: cty.StringVal("16"), role: cty.StringVal("pg_maintain"), errorText: "pg_maintain requires PostgreSQL 17"},
		{name: "unknown version", version: cty.UnknownVal(cty.String), role: cty.StringVal("pg_maintain")},
		{name: "unknown roles", version: cty.StringVal("15"), role: cty.UnknownVal(cty.String)},
		{name: "legacy replacement", previousVersion: "15", version: cty.StringVal("18"), role: cty.StringVal("pg_monitor")},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			resource := ResourceService()
			config := terraform.NewResourceConfigShimmed(cty.ObjectVal(map[string]cty.Value{
				"pgsql": cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{
					"version": testCase.version,
					"user": cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{
						"name": cty.StringVal("application"), "password": cty.StringVal("pgsql-test-password"),
						"roles": cty.SetVal([]cty.Value{testCase.role}),
					})}),
				})}),
			}), resource.CoreConfigSchema())
			var previous *terraform.InstanceState
			if testCase.previousVersion != "" {
				initial := schema.TestResourceDataRaw(t, resource.Schema, map[string]interface{}{
					"pgsql": []interface{}{map[string]interface{}{"version": testCase.previousVersion}},
				})
				initial.SetId("fm-cluster-12345678")
				if err := initial.Set("environment_version", "paas_v4_2"); err != nil {
					t.Fatal(err)
				}
				previous = initial.State()
			}
			diff, err := resource.Diff(context.Background(), previous, config, nil)
			if testCase.errorText != "" {
				if err == nil || !strings.Contains(err.Error(), testCase.errorText) {
					t.Fatalf("expected %q, got %v", testCase.errorText, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if testCase.previousVersion != "" && (diff == nil || !diff.RequiresNew()) {
				t.Fatal("upgrading a legacy service must allow replacement in the current environment")
			}
		})
	}
}

func TestClearPostgreSQLUserRoles(t *testing.T) {
	t.Parallel()
	previous := []interface{}{
		map[string]interface{}{"name": "remove", "roles": schema.NewSet(schema.HashString, []interface{}{"pg_monitor"})},
		map[string]interface{}{"name": "keep", "roles": schema.NewSet(schema.HashString, []interface{}{"pg_monitor"})},
		map[string]interface{}{"name": "legacy", "roles": schema.NewSet(schema.HashString, nil)},
	}
	// Match users by name even when their order changes.
	users := []*paas.UserCreateRequest{
		{Name: aws.String("keep"), Parameters: map[string]interface{}{"roles": []interface{}{"pg_signal_backend"}}},
		{Name: aws.String("legacy"), Parameters: map[string]interface{}{}},
		{Name: aws.String("remove"), Parameters: map[string]interface{}{}},
		{Name: aws.String("new"), Parameters: map[string]interface{}{}},
	}
	clearPostgreSQLUserRoles(users, previous)
	if roles := users[0].Parameters["roles"].([]interface{}); len(roles) != 1 || roles[0] != "pg_signal_backend" {
		t.Fatal("configured roles must be preserved")
	}
	if roles, ok := users[2].Parameters["roles"].([]interface{}); !ok || len(roles) != 0 {
		t.Fatal("removing assigned roles must send an empty JSON array")
	}
	for _, index := range []int{1, 3} {
		if _, ok := users[index].Parameters["roles"]; ok {
			t.Fatal("users without previously assigned roles must omit roles")
		}
	}
}

func TestPostgreSQLRoleRevocationRequest(t *testing.T) {
	t.Parallel()
	const serviceID = "fm-cluster-12345678"
	resource := ResourceService()
	config := func(roles []interface{}) map[string]interface{} {
		return map[string]interface{}{
			"name": "tf-pgsql-roles", "instance_type": "c5.large", "arbitrator_required": false,
			"root_volume": []interface{}{map[string]interface{}{"type": "gp2", "size": 32}},
			"data_volume": []interface{}{map[string]interface{}{"type": "gp2", "size": 32}},
			"pgsql": []interface{}{map[string]interface{}{
				"version": "18", "user": []interface{}{map[string]interface{}{
					"name": "application", "password": "pgsql-test-secret", "roles": roles,
				}},
			}},
		}
	}
	initial := schema.TestResourceDataRaw(t, resource.Schema, config([]interface{}{"pg_monitor"}))
	initial.SetId(serviceID)
	diff, err := resource.Diff(context.Background(), initial.State(), terraform.NewResourceConfigRaw(config(nil)), nil)
	if err != nil || diff == nil || diff.RequiresNew() {
		t.Fatalf("roles must update without replacing the service: %v", err)
	}
	modified := false
	conn := testPaaSClientWithDebugBodyLogging(&recordingSDKLogger{}, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodPut && request.URL.Path == "/services/"+serviceID+"/parameters":
			return jsonResponse(request, `{}`), nil
		case request.Method == http.MethodPut && request.URL.Path == "/services/"+serviceID:
			var body struct {
				Users []struct {
					Parameters map[string]interface{} `json:"parameters"`
				} `json:"users"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if len(body.Users) != 1 {
				t.Fatalf("missing user: %#v", body)
			}
			roles, ok := body.Users[0].Parameters["roles"].([]interface{})
			if !ok || len(roles) != 0 {
				t.Fatalf("revocation must send roles:[]: %#v", body.Users[0].Parameters)
			}
			modified = true
			return jsonResponse(request, `{}`), nil
		case request.Method == http.MethodGet && request.URL.Path == "/services/"+serviceID:
			return jsonResponse(request, `{"service":{"id":"`+serviceID+`","serviceType":"pgsql","serviceClass":"database","status":"READY","parameters":{"version":"18"},"users":[{"name":"application","parameters":{"password":"pgsql-test-secret","roles":[]}}]}}`), nil
		default:
			t.Fatalf("unexpected PaaS request: %s %s", request.Method, request.URL.Path)
			return nil, nil
		}
	}))
	state, diagnostics := resource.Apply(context.Background(), initial.State(), diff, testPaaSClientMeta(conn))
	if diagnostics.HasError() || !modified || state.ID != serviceID {
		t.Fatalf("role revocation failed: %#v", diagnostics)
	}
}
