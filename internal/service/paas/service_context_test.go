package paas

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws/request"
	sdkpaas "github.com/aws/aws-sdk-go/service/paas"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestServiceReadsHonorHTTPContextCancellation(t *testing.T) {
	t.Parallel()

	const serviceID = "fm-cluster-12345678"
	for _, testCase := range []struct {
		name string
		read func(context.Context, *sdkpaas.PaaS) error
	}{
		{
			name: "finder",
			read: func(ctx context.Context, conn *sdkpaas.PaaS) error {
				_, err := findServiceByID(ctx, conn, serviceID)
				return err
			},
		},
		{
			name: "status refresh",
			read: func(ctx context.Context, conn *sdkpaas.PaaS) error {
				_, _, err := statusService(ctx, conn, serviceID)()
				return err
			},
		},
		{
			name: "resource read",
			read: func(ctx context.Context, conn *sdkpaas.PaaS) error {
				data := schema.TestResourceDataRaw(t, ResourceService().Schema, nil)
				data.SetId(serviceID)
				diagnostics := resourceServiceRead(ctx, data, testPaaSClientMeta(conn))
				if data.Id() != serviceID {
					t.Fatal("a canceled read must not remove the service from state")
				}
				if diagnostics.HasError() {
					return fmt.Errorf("%s", diagnostics[0].Summary)
				}
				return nil
			},
		},
		{
			name: "data source read",
			read: func(ctx context.Context, conn *sdkpaas.PaaS) error {
				data := schema.TestResourceDataRaw(t, DataSourceService().Schema, map[string]interface{}{"id": serviceID})
				diagnostics := dataSourceServiceRead(ctx, data, testPaaSClientMeta(conn))
				if diagnostics.HasError() {
					return fmt.Errorf("%s", diagnostics[0].Summary)
				}
				return nil
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			transportCalled := false
			conn := testPaaSClientWithDebugBodyLogging(&recordingSDKLogger{}, roundTripFunc(func(httpRequest *http.Request) (*http.Response, error) {
				transportCalled = true
				cancel()
				if err := httpRequest.Context().Err(); err != context.Canceled {
					t.Errorf("in-flight HTTP request did not inherit caller cancellation: %v", err)
				}
				return nil, context.Canceled
			}))

			err := testCase.read(ctx, conn)
			if !transportCalled {
				t.Fatal("expected an in-flight HTTP request before cancellation")
			}
			if err == nil || !strings.Contains(err.Error(), request.CanceledErrorCode) {
				t.Fatalf("expected SDK request cancellation error, got %v", err)
			}
		})
	}
}

func TestServiceMutationsHonorHTTPContextCancellation(t *testing.T) {
	t.Parallel()

	const serviceID = "fm-cluster-12345678"
	for _, testCase := range []struct {
		name   string
		path   string
		method string
		change func(map[string]interface{}, map[string]interface{})
	}{
		{
			name: "instance type", path: "/instance-type", method: http.MethodPut,
			change: func(_, updated map[string]interface{}) { updated["instance_type"] = "c5.xlarge" },
		},
		{
			name: "volume size", path: "/instance-volume-size", method: http.MethodPut,
			change: func(_, updated map[string]interface{}) {
				updated["data_volume"].([]interface{})[0].(map[string]interface{})["size"] = 64
			},
		},
		{
			name: "volume IOPS", path: "/instance-volume-iops", method: http.MethodPut,
			change: func(initial, updated map[string]interface{}) {
				for _, config := range []map[string]interface{}{initial, updated} {
					volume := config["data_volume"].([]interface{})[0].(map[string]interface{})
					volume["type"], volume["iops"] = "io2", 3000
				}
				updated["data_volume"].([]interface{})[0].(map[string]interface{})["iops"] = 4000
			},
		},
		{name: "delete", method: http.MethodDelete},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			transportCalled := false
			conn := testPaaSClientWithDebugBodyLogging(&recordingSDKLogger{}, roundTripFunc(func(httpRequest *http.Request) (*http.Response, error) {
				if httpRequest.Method != testCase.method || httpRequest.URL.Path != "/services/"+serviceID+testCase.path {
					t.Fatalf("unexpected mutation: %s %s", httpRequest.Method, httpRequest.URL.Path)
				}
				transportCalled = true
				cancel()
				if err := httpRequest.Context().Err(); err != context.Canceled {
					t.Errorf("in-flight HTTP mutation did not inherit caller cancellation: %v", err)
				}
				return nil, context.Canceled
			}))

			resource := ResourceService()
			initialConfig := testClickHouseSensitiveLoggingConfig("password", "option")
			updatedConfig := testClickHouseSensitiveLoggingConfig("password", "option")
			if testCase.change != nil {
				testCase.change(initialConfig, updatedConfig)
			}
			data := schema.TestResourceDataRaw(t, resource.Schema, initialConfig)
			data.SetId(serviceID)
			if err := data.Set("nodes", []interface{}{map[string]interface{}{
				"main": []interface{}{map[string]interface{}{"role": "node,coordinator"}},
			}}); err != nil {
				t.Fatal(err)
			}
			mutate := resourceServiceDelete
			if testCase.method == http.MethodPut {
				state := data.State()
				diff, err := resource.Diff(context.Background(), state, terraform.NewResourceConfigRaw(updatedConfig), nil)
				if err != nil || diff == nil || diff.RequiresNew() {
					t.Fatalf("expected in-place mutation diff: diff=%#v err=%v", diff, err)
				}
				data, err = schema.InternalMap(resource.Schema).Data(state, diff)
				if err != nil {
					t.Fatal(err)
				}
				mutate = resourceServiceUpdate
			}

			diagnostics := mutate(ctx, data, testPaaSClientMeta(conn))
			if !transportCalled {
				t.Fatal("expected an in-flight HTTP mutation before cancellation")
			}
			if !diagnostics.HasError() || !strings.Contains(diagnostics[0].Summary, request.CanceledErrorCode) {
				t.Fatalf("expected SDK request cancellation diagnostic, got %#v", diagnostics)
			}
			if data.Id() != serviceID {
				t.Fatal("a canceled mutation must retain the service ID for retry")
			}
		})
	}
}
