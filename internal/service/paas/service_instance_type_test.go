package paas

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestServiceInstanceTypeUpdatePreservesDedicatedCoordinator(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name        string
		serviceType string
		mainRole    string
		dedicated   bool
		instanceQty int
	}{
		{name: "clickhouse combined nodes", serviceType: "clickhouse", mainRole: "node,coordinator", instanceQty: 3},
		{name: "clickhouse dedicated coordinator", serviceType: "clickhouse", mainRole: "node", dedicated: true, instanceQty: 6},
		{name: "kafka combined nodes", serviceType: "kafka", mainRole: "node,coordinator", instanceQty: 3},
		{name: "kafka dedicated coordinator", serviceType: "kafka", mainRole: "node", dedicated: true, instanceQty: 6},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			const serviceID = "fm-cluster-12345678"
			version, serviceClass, dataVolumeSize := "25.3", "database", 32
			if testCase.serviceType == "kafka" {
				version, serviceClass, dataVolumeSize = "3.6.1", "message_broker", 64
			}
			mainInstanceType, coordinatorInstanceType := "c5.large", "c4.large"
			var modifiedRoles []string
			conn := testPaaSClientWithDebugBodyLogging(&recordingSDKLogger{}, roundTripFunc(func(request *http.Request) (*http.Response, error) {
				switch {
				case request.Method == http.MethodPut && request.URL.Path == "/services/"+serviceID+"/instance-type":
					var body struct {
						InstanceType string `json:"instanceType"`
						NodeRole     string `json:"nodeRole"`
					}
					if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
						t.Fatalf("decoding instance type update: %s", err)
					}
					modifiedRoles = append(modifiedRoles, body.NodeRole)
					switch body.NodeRole {
					case testCase.mainRole:
						mainInstanceType = body.InstanceType
					case "coordinator":
						coordinatorInstanceType = body.InstanceType
					default:
						t.Fatalf("unexpected modified node role: %q", body.NodeRole)
					}
					return jsonResponse(request, `{}`), nil
				case request.Method == http.MethodGet && request.URL.Path == "/services/"+serviceID:
					nodes := map[string]interface{}{"main": map[string]interface{}{"role": testCase.mainRole}}
					if testCase.dedicated {
						nodes["coordinator"] = map[string]interface{}{
							"role": "coordinator", "instanceType": coordinatorInstanceType,
							"rootVolumeType": "gp2", "rootVolumeSize": 32,
						}
						if testCase.serviceType == "kafka" {
							coordinator := nodes["coordinator"].(map[string]interface{})
							coordinator["dataVolumeType"] = "gp2"
							coordinator["dataVolumeSize"] = 64
						}
					}
					body, err := json.Marshal(map[string]interface{}{"service": map[string]interface{}{
						"id": serviceID, "name": "tf-ch-resize-test", "serviceType": testCase.serviceType, "serviceClass": serviceClass,
						"status": "READY", "highAvailability": true, "instanceQty": testCase.instanceQty,
						"instanceType": mainInstanceType, "nodes": nodes,
						"rootVolumeType": "gp2", "rootVolumeSize": 32, "dataVolumeType": "gp2", "dataVolumeSize": dataVolumeSize,
						"subnetIds": []string{"subnet-12345678"}, "sshKeyName": "tf-ch-resize-key",
						"securityGroups": []map[string]interface{}{{"id": "sg-12345678", "createdAutomatically": false}},
						"parameters":     map[string]interface{}{"version": version},
					}})
					if err != nil {
						t.Fatalf("encoding service response: %s", err)
					}
					return jsonResponse(request, string(body)), nil
				default:
					t.Fatalf("unexpected PaaS request: %s %s", request.Method, request.URL.Path)
					return nil, nil
				}
			}))

			config := func(instanceType string) map[string]interface{} {
				configuration := map[string]interface{}{
					"name": "tf-ch-resize-test", "instance_type": instanceType, "high_availability": true, "instance_qty": testCase.instanceQty,
					"root_volume": []interface{}{map[string]interface{}{"type": "gp2", "size": 32}},
					"data_volume": []interface{}{map[string]interface{}{"type": "gp2", "size": dataVolumeSize}},
					"subnet_ids":  []interface{}{"subnet-12345678"}, "security_group_ids": []interface{}{"sg-12345678"},
					"ssh_key_name": "tf-ch-resize-key", testCase.serviceType: []interface{}{map[string]interface{}{"version": version}},
				}
				if testCase.dedicated {
					configuration["coordinator"] = []interface{}{map[string]interface{}{
						"instance_type": "c4.large", "root_volume_type": "gp2", "root_volume_size": 32,
					}}
					if testCase.serviceType == "kafka" {
						coordinator := configuration["coordinator"].([]interface{})[0].(map[string]interface{})
						coordinator["data_volume_type"] = "gp2"
						coordinator["data_volume_size"] = 64
					}
				} else {
					configuration["additional_roles"] = []interface{}{"coordinator"}
				}
				return configuration
			}
			resource := ResourceService()
			initialData := schema.TestResourceDataRaw(t, resource.Schema, config(mainInstanceType))
			initialData.SetId(serviceID)
			nodes := map[string]interface{}{"main": []interface{}{map[string]interface{}{"role": testCase.mainRole}}}
			if testCase.dedicated {
				nodes["coordinator"] = []interface{}{map[string]interface{}{"role": "coordinator"}}
			}
			if err := initialData.Set("nodes", []interface{}{nodes}); err != nil {
				t.Fatal(err)
			}
			desiredConfig := terraform.NewResourceConfigRaw(config("c5.xlarge"))
			diff, err := resource.Diff(context.Background(), initialData.State(), desiredConfig, nil)
			if err != nil || diff == nil || diff.RequiresNew() {
				t.Fatalf("main instance type must update in-place: diff=%#v err=%v", diff, err)
			}
			state, diagnostics := resource.Apply(context.Background(), initialData.State(), diff, testPaaSClientMeta(conn))
			if diagnostics.HasError() {
				t.Fatalf("updating instance type: %#v", diagnostics)
			}
			if !reflect.DeepEqual(modifiedRoles, []string{testCase.mainRole}) {
				t.Errorf("main instance type update changed other node roles: %#v", modifiedRoles)
			}
			if coordinatorInstanceType != "c4.large" {
				t.Errorf("dedicated coordinator instance type changed to %q", coordinatorInstanceType)
			}
			postUpdateDiff, err := resource.Diff(context.Background(), state, desiredConfig, nil)
			if err != nil || postUpdateDiff != nil && !postUpdateDiff.Empty() {
				t.Fatalf("instance type update left configuration drift: diff=%#v err=%v", postUpdateDiff, err)
			}
		})
	}
}
