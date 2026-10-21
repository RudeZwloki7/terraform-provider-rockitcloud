package elbv2

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	tftags "github.com/hashicorp/terraform-provider-aws/internal/tags"
)

func TestValidTargetGroupHealthCheckPort(t *testing.T) {
	t.Parallel()

	healthCheck := ResourceTargetGroup().Schema["health_check"].Elem.(*schema.Resource)
	validate := healthCheck.Schema["port"].ValidateFunc
	if validate == nil {
		t.Fatal("health_check.port has no ValidateFunc")
	}

	validCases := []string{"traffic-port", "1", "80", "8081", "65535"}
	for _, v := range validCases {
		_, errs := validate(v, "health_check.0.port")
		if len(errs) != 0 {
			t.Errorf("%q should be a valid health check port, got errors: %v", v, errs)
		}
	}

	invalidCases := []string{"", "0", "-1", "65536", "abc", "80.5", "Traffic-Port", "TRAFFIC-PORT"}
	for _, v := range invalidCases {
		_, errs := validate(v, "health_check.0.port")
		if len(errs) != 1 {
			t.Errorf("%q should be an invalid health check port with exactly one error, got: %v", v, errs)
		}
	}
}

func TestCustomizeDiffTargetGroupHealthCheckPort(t *testing.T) {
	t.Parallel()

	r := ResourceTargetGroup()
	meta := &conns.AWSClient{
		DefaultTagsConfig: &tftags.DefaultConfig{},
		IgnoreTagsConfig:  &tftags.IgnoreConfig{},
	}

	testCases := []struct {
		name        string
		protocol    string
		healthCheck []interface{}
		wantErr     string
	}{
		{
			name:        "HTTP with a port number",
			protocol:    "HTTP",
			healthCheck: []interface{}{map[string]interface{}{"port": "8081"}},
		},
		{
			name:        "lower case HTTP with a port number",
			protocol:    "http",
			healthCheck: []interface{}{map[string]interface{}{"port": "8081"}},
		},
		{
			name:        "HTTP with traffic-port",
			protocol:    "HTTP",
			healthCheck: []interface{}{map[string]interface{}{"port": "traffic-port"}},
		},
		{
			name:        "TCP with traffic-port",
			protocol:    "TCP",
			healthCheck: []interface{}{map[string]interface{}{"port": "traffic-port"}},
		},
		{
			name:     "TCP without the health_check block",
			protocol: "TCP",
		},
		{
			name:        "TCP with a port number",
			protocol:    "TCP",
			healthCheck: []interface{}{map[string]interface{}{"port": "8081"}},
			wantErr:     `health_check.0.port: only "traffic-port" is supported for target groups with protocol "TCP"`,
		},
		{
			name:        "UDP with a port number",
			protocol:    "UDP",
			healthCheck: []interface{}{map[string]interface{}{"port": "8081"}},
			wantErr:     `health_check.0.port: only "traffic-port" is supported for target groups with protocol "UDP"`,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			raw := map[string]interface{}{
				"name":     "tf-acc-test",
				"port":     80,
				"protocol": testCase.protocol,
			}
			if testCase.healthCheck != nil {
				raw["health_check"] = testCase.healthCheck
			}

			_, err := r.Diff(context.Background(), nil, terraform.NewResourceConfigRaw(raw), meta)

			switch {
			case testCase.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %s", err)
			case testCase.wantErr != "" && err == nil:
				t.Fatalf("expected the error %q, got none", testCase.wantErr)
			case testCase.wantErr != "" && err.Error() != testCase.wantErr:
				t.Fatalf("expected the error %q, got %q", testCase.wantErr, err)
			}
		})
	}
}
