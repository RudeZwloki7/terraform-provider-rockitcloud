package paas

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
)

func TestClickHouseQuotaPlanRequiresLimitForEveryInterval(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name      string
		intervals []interface{}
		wantError bool
	}{
		{"duration only", []interface{}{map[string]interface{}{"duration": 3600}}, true},
		{"query limit", []interface{}{map[string]interface{}{"duration": 3600, "queries": 1000}}, false},
		{"execution time limit", []interface{}{map[string]interface{}{"duration": 3600, "execution_time": 1.5}}, false},
		{"second interval has no limit", []interface{}{
			map[string]interface{}{"duration": 3600, "queries": 1000},
			map[string]interface{}{"duration": 86400},
		}, true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := ResourceClickHouseQuota().Diff(context.Background(), nil, terraform.NewResourceConfigRaw(map[string]interface{}{
				"name":       "analytics_quota",
				"service_id": "fm-cluster-12345678",
				"interval":   testCase.intervals,
			}), nil)
			if (err != nil) != testCase.wantError {
				t.Fatalf("unexpected plan validation result: %v", err)
			}
		})
	}
}

func TestClickHouseQuotaPlanAllowsUnknownLimit(t *testing.T) {
	t.Parallel()

	resource := ResourceClickHouseQuota()
	config := terraform.NewResourceConfigShimmed(cty.ObjectVal(map[string]cty.Value{
		"name":       cty.StringVal("analytics_quota"),
		"service_id": cty.StringVal("fm-cluster-12345678"),
		"interval": cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{
			"duration": cty.NumberIntVal(3600),
			"queries":  cty.UnknownVal(cty.Number),
		})}),
	}), resource.CoreConfigSchema())
	if _, err := resource.Diff(context.Background(), nil, config, nil); err != nil {
		t.Fatalf("a limit known only during apply must not fail the plan: %s", err)
	}
}

func TestClickHouseShardPlanRejectsDecreasingReplicas(t *testing.T) {
	t.Parallel()

	resource := ResourceClickHouseShard()
	initial := schema.TestResourceDataRaw(t, resource.Schema, map[string]interface{}{
		"service_id": "fm-cluster-12345678",
		"replicas":   3,
	})
	initial.SetId("fm-cluster-12345678/0")
	for _, testCase := range []struct {
		replicas  int
		wantError bool
	}{{2, true}, {3, false}, {4, false}} {
		_, err := resource.Diff(context.Background(), initial.State(), terraform.NewResourceConfigRaw(map[string]interface{}{
			"service_id": "fm-cluster-12345678",
			"replicas":   testCase.replicas,
		}), nil)
		if (err != nil) != testCase.wantError {
			t.Fatalf("replicas=%d: unexpected plan validation result: %v", testCase.replicas, err)
		}
	}
}

func TestClickHouseProfileAndQuotaNames(t *testing.T) {
	t.Parallel()

	for _, resource := range []*schema.Resource{ResourceClickHouseProfile(), ResourceClickHouseQuota()} {
		validate := resource.Schema["name"].ValidateFunc
		for _, name := range []string{"analytics", "Profile_123"} {
			if _, errors := validate(name, "name"); len(errors) > 0 {
				t.Errorf("name %q must be accepted: %v", name, errors)
			}
		}
		for _, name := range []string{"", "_analytics", "123analytics", "analytics-profile", "default"} {
			if _, errors := validate(name, "name"); len(errors) == 0 {
				t.Errorf("name %q must be rejected", name)
			}
		}
	}
}

func TestClickHouseQuotaIntervalConversion(t *testing.T) {
	t.Parallel()
	if strconv.IntSize < 64 {
		t.Skip("Terraform TypeInt cannot represent limits above 2^53 on a 32-bit platform")
	}
	largeLimit := int64(9007199254740993)

	terraformInterval := map[string]interface{}{
		"duration":                          3600,
		"errors":                            2,
		"execution_time":                    1.5,
		"failed_sequential_authentications": 3,
		"queries":                           4,
		"query_inserts":                     5,
		"query_selects":                     6,
		"read_bytes":                        int(largeLimit),
		"read_rows":                         8,
		"result_bytes":                      9,
		"result_rows":                       10,
		"written_bytes":                     11,
	}
	expanded := expandClickHouseQuotaIntervals([]interface{}{terraformInterval})
	if len(expanded) != 1 || expanded[0].(map[string]interface{})["read_bytes"] != int64(9007199254740993) {
		t.Fatalf("large limits lost integer precision: %#v", expanded)
	}
	apiInterval := map[string]interface{}{
		"duration":                        json.Number("3600"),
		"errors":                          json.Number("2"),
		"executionTime":                   json.Number("1.5"),
		"failedSequentialAuthentications": json.Number("3"),
		"queries":                         json.Number("4"),
		"queryInserts":                    json.Number("5"),
		"querySelects":                    json.Number("6"),
		"readBytes":                       json.Number("9007199254740993"),
		"readRows":                        json.Number("8"),
		"resultBytes":                     json.Number("9"),
		"resultRows":                      json.Number("10"),
		"writtenBytes":                    json.Number("11"),
	}
	flattened := flattenClickHouseQuotaIntervals([]interface{}{apiInterval})
	if len(flattened) != 1 {
		t.Fatalf("unexpected flattened intervals: %#v", flattened)
	}
	for key, value := range terraformInterval {
		want := value
		if integer, ok := value.(int); ok {
			want = int64(integer)
		}
		if got := flattened[0].(map[string]interface{})[key]; got != want {
			t.Errorf("%s: got %#v want %#v", key, got, want)
		}
	}
	if _, ok := clickHouseQuotaInt(json.Number("1.5")); ok {
		t.Error("a fractional limit must not be silently truncated")
	}
	if got, ok := clickHouseQuotaFloat(json.Number("2")); !ok || got != 2 {
		t.Error("API integer execution time must be accepted as a float")
	}
}

func TestClickHouseChildLifecycle(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name          string
		resource      *schema.Resource
		collection    string
		childID       string
		createJSON    string
		updateJSON    string
		createReply   string
		initialList   string
		updatedList   string
		initialConfig map[string]interface{}
		updatedConfig map[string]interface{}
		updatedState  map[string]string
		apiDelete     bool
	}{
		{
			name: "profile", resource: ResourceClickHouseProfile(), collection: "clickhouse-profiles", childID: "profile-12345678", apiDelete: true,
			createJSON:    `{"name":"analytics_profile","parameters":{"settings":{"max_threads":"4","readonly":"true"}}}`,
			updateJSON:    `{"parameters":{"settings":{}}}`,
			createReply:   `{"profile":{"id":"profile-12345678"}}`,
			initialList:   `{"profiles":[null,{"id":"profile-12345678","name":"analytics_profile","parameters":{"settings":{"max_threads":4,"readonly":true}}}]}`,
			updatedList:   `{"profiles":[{"id":"profile-12345678","name":"analytics_profile"}]}`,
			initialConfig: map[string]interface{}{"name": "analytics_profile", "settings": map[string]interface{}{"max_threads": "4", "readonly": "true"}},
			updatedConfig: map[string]interface{}{"name": "analytics_profile"},
			updatedState:  map[string]string{"settings.%": "0"},
		},
		{
			name: "quota", resource: ResourceClickHouseQuota(), collection: "clickhouse-quotas", childID: "quota-12345678", apiDelete: true,
			createJSON:    `{"name":"analytics_quota","parameters":{"interval":[{"duration":3600,"queries":1000}]}}`,
			updateJSON:    `{"parameters":{"interval":[{"duration":3600,"queries":2000}]}}`,
			createReply:   `{"quota":{"id":"quota-12345678"}}`,
			initialList:   `{"quotas":[null,{"id":"quota-12345678","name":"analytics_quota","parameters":{"interval":[{"duration":3600,"queries":1000}]}}]}`,
			updatedList:   `{"quotas":[{"id":"quota-12345678","name":"analytics_quota","parameters":{"interval":[{"duration":3600,"queries":2000}]}}]}`,
			initialConfig: map[string]interface{}{"name": "analytics_quota", "interval": []interface{}{map[string]interface{}{"duration": 3600, "queries": 1000}}},
			updatedConfig: map[string]interface{}{"name": "analytics_quota", "interval": []interface{}{map[string]interface{}{"duration": 3600, "queries": 2000}}},
			updatedState:  map[string]string{"interval.0.queries": "2000"},
		},
		{
			name: "shard", resource: ResourceClickHouseShard(), collection: "clickhouse-shards", childID: "0",
			createJSON:    `{"parameters":{"internalReplication":false,"replicas":1,"weight":1}}`,
			updateJSON:    `{"parameters":{"internalReplication":true,"replicas":2,"weight":3}}`,
			createReply:   `{"shard":{"index":0}}`,
			initialList:   `{"shards":[null,{"index":0,"parameters":{"internalReplication":false,"replicas":1,"weight":1}}]}`,
			updatedList:   `{"shards":[{"index":0,"parameters":{"internalReplication":true,"replicas":2,"weight":3}}]}`,
			initialConfig: map[string]interface{}{},
			updatedConfig: map[string]interface{}{"internal_replication": true, "replicas": 2, "weight": 3},
			updatedState:  map[string]string{"replicas": "2", "weight": "3", "internal_replication": "true"},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			collectionPath := "/services/fm-cluster-12345678/" + testCase.collection
			created, modified, deleted := false, false, false
			conn := testPaaSClientWithDebugBodyLogging(&recordingSDKLogger{}, roundTripFunc(func(request *http.Request) (*http.Response, error) {
				switch {
				case request.Method == http.MethodPost && request.URL.Path == collectionPath:
					assertClickHouseRequestJSON(t, request, testCase.createJSON)
					created = true
					return jsonResponse(request, testCase.createReply), nil
				case request.Method == http.MethodPut && request.URL.Path == collectionPath+"/"+testCase.childID:
					assertClickHouseRequestJSON(t, request, testCase.updateJSON)
					modified = true
					return jsonResponse(request, `{}`), nil
				case request.Method == http.MethodDelete && request.URL.Path == collectionPath+"/"+testCase.childID:
					deleted = true
					return jsonResponse(request, `{}`), nil
				case request.Method == http.MethodGet && request.URL.Path == "/services/fm-cluster-12345678":
					return jsonResponse(request, `{"service":{"status":"READY"}}`), nil
				case request.Method == http.MethodGet && request.URL.Path == collectionPath:
					if modified {
						return jsonResponse(request, testCase.updatedList), nil
					}
					return jsonResponse(request, testCase.initialList), nil
				default:
					t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
					return nil, nil
				}
			}))
			testCase.initialConfig["service_id"] = "fm-cluster-12345678"
			testCase.updatedConfig["service_id"] = "fm-cluster-12345678"
			data := schema.TestResourceDataRaw(t, testCase.resource.Schema, testCase.initialConfig)
			meta := testPaaSClientMeta(conn)
			if diagnostics := testCase.resource.CreateContext(context.Background(), data, meta); diagnostics.HasError() {
				t.Fatalf("create: %#v", diagnostics)
			}
			if !created || data.Id() != "fm-cluster-12345678/"+testCase.childID {
				t.Fatalf("create did not record the API identity: %q", data.Id())
			}
			if testCase.name == "profile" && !reflect.DeepEqual(data.Get("settings"), map[string]interface{}{"max_threads": "4", "readonly": "true"}) {
				t.Fatalf("numeric and boolean profile settings were not normalized: %#v", data.Get("settings"))
			}
			diff, err := testCase.resource.Diff(context.Background(), data.State(), terraform.NewResourceConfigRaw(testCase.updatedConfig), nil)
			if err != nil {
				t.Fatal(err)
			}
			state, diagnostics := testCase.resource.Apply(context.Background(), data.State(), diff, meta)
			if diagnostics.HasError() || !modified {
				t.Fatalf("update: %#v modified=%t", diagnostics, modified)
			}
			for key, want := range testCase.updatedState {
				if got := state.Attributes[key]; got != want {
					t.Errorf("%s: got %q want %q", key, got, want)
				}
			}
			if diagnostics := testCase.resource.DeleteContext(context.Background(), data, meta); diagnostics.HasError() {
				t.Fatalf("delete: %#v", diagnostics)
			}
			if deleted != testCase.apiDelete {
				t.Fatalf("unexpected API delete: got %t want %t", deleted, testCase.apiDelete)
			}
		})
	}
}

func TestClickHouseCreateRejectsMissingResponseIdentity(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name     string
		resource *schema.Resource
		config   map[string]interface{}
	}{
		{"profile", ResourceClickHouseProfile(), map[string]interface{}{"name": "profile"}},
		{"quota", ResourceClickHouseQuota(), map[string]interface{}{"name": "quota", "interval": []interface{}{map[string]interface{}{"duration": 60, "queries": 1}}}},
		{"shard", ResourceClickHouseShard(), map[string]interface{}{}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			conn := testPaaSClientWithDebugBodyLogging(&recordingSDKLogger{}, roundTripFunc(func(request *http.Request) (*http.Response, error) {
				return jsonResponse(request, `{}`), nil
			}))
			testCase.config["service_id"] = "fm-cluster-12345678"
			data := schema.TestResourceDataRaw(t, testCase.resource.Schema, testCase.config)
			if diagnostics := testCase.resource.CreateContext(context.Background(), data, testPaaSClientMeta(conn)); !diagnostics.HasError() {
				t.Fatal("a malformed create response must return diagnostics")
			}
			if data.Id() != "" {
				t.Fatalf("a malformed API identity must not be stored: %q", data.Id())
			}
		})
	}
}

func TestClickHouseFindMissingChildrenAndCanceledContext(t *testing.T) {
	t.Parallel()

	conn := testPaaSClientWithDebugBodyLogging(&recordingSDKLogger{}, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return jsonResponse(request, `{"profiles":[null],"quotas":[null],"shards":[null,{}]}`), nil
	}))
	if _, err := FindClickHouseProfileByID(conn, "fm-cluster-12345678", "missing"); !tfresource.NotFound(err) {
		t.Fatalf("missing profile: %v", err)
	}
	if _, err := FindClickHouseQuotaByID(conn, "fm-cluster-12345678", "missing"); !tfresource.NotFound(err) {
		t.Fatalf("missing quota: %v", err)
	}
	if _, err := FindClickHouseShardByIndex(conn, "fm-cluster-12345678", 0); !tfresource.NotFound(err) {
		t.Fatalf("a malformed shard with no index must not match index zero: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := findClickHouseQuotaByID(ctx, conn, "fm-cluster-12345678", "missing"); err == nil || tfresource.NotFound(err) {
		t.Fatalf("canceled context must stop the SDK request: %v", err)
	}
}

func TestClickHouseQuotaReadClearsMissingIntervals(t *testing.T) {
	t.Parallel()

	for _, parameters := range []string{"", `,"parameters":{}`, `,"parameters":{"interval":null}`} {
		conn := testPaaSClientWithDebugBodyLogging(&recordingSDKLogger{}, roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return jsonResponse(request, `{"quotas":[{"id":"quota-12345678","name":"analytics_quota"`+parameters+`}]}`), nil
		}))
		data := schema.TestResourceDataRaw(t, ResourceClickHouseQuota().Schema, map[string]interface{}{
			"service_id": "fm-cluster-12345678",
			"name":       "analytics_quota",
			"interval":   []interface{}{map[string]interface{}{"duration": 3600, "queries": 1000}},
		})
		data.SetId("fm-cluster-12345678/quota-12345678")
		if diagnostics := resourceClickHouseQuotaRead(context.Background(), data, testPaaSClientMeta(conn)); diagnostics.HasError() {
			t.Fatalf("read: %#v", diagnostics)
		}
		if intervals := data.Get("interval").([]interface{}); len(intervals) != 0 {
			t.Fatalf("missing API intervals must not preserve the previous quota limits: %#v", intervals)
		}
	}
}

func TestClickHouseShardReadRejectsMissingParameters(t *testing.T) {
	t.Parallel()

	conn := testPaaSClientWithDebugBodyLogging(&recordingSDKLogger{}, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return jsonResponse(request, `{"shards":[{"index":0}]}`), nil
	}))
	data := schema.TestResourceDataRaw(t, ResourceClickHouseShard().Schema, map[string]interface{}{
		"service_id":           "fm-cluster-12345678",
		"replicas":             3,
		"weight":               2,
		"internal_replication": true,
	})
	data.SetId("fm-cluster-12345678/0")
	diagnostics := resourceClickHouseShardRead(context.Background(), data, testPaaSClientMeta(conn))
	if !diagnostics.HasError() || !strings.Contains(diagnostics[0].Summary, "API returned no shard parameters") {
		t.Fatalf("a shard response without parameters must report an error instead of confirming stale state: %#v", diagnostics)
	}
}

func assertClickHouseRequestJSON(t *testing.T, request *http.Request, expected string) {
	t.Helper()

	var got, want interface{}
	decoder := json.NewDecoder(request.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&got); err != nil {
		t.Fatalf("decoding request body: %s", err)
	}
	decoder = json.NewDecoder(strings.NewReader(expected))
	decoder.UseNumber()
	if err := decoder.Decode(&want); err != nil {
		t.Fatalf("decoding expected body: %s", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected request body: got %#v want %#v", got, want)
	}
}
