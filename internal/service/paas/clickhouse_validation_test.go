package paas

import (
	"context"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestClickHouseServicePlanUnknownTopology(t *testing.T) {
	t.Parallel()

	coordinator := cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{
		"instance_type":    cty.StringVal("c5.large"),
		"root_volume_size": cty.NumberIntVal(32),
		"root_volume_type": cty.StringVal("gp2"),
	})})
	coordinatorWithUnknownInstanceType := cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{
		"instance_type":    cty.UnknownVal(cty.String),
		"root_volume_size": cty.NumberIntVal(32),
		"root_volume_type": cty.StringVal("gp2"),
	})})
	for _, testCase := range []struct {
		name      string
		topology  map[string]cty.Value
		wantError bool
	}{
		{
			name:     "unknown additional roles list",
			topology: map[string]cty.Value{"additional_roles": cty.UnknownVal(cty.List(cty.String))},
		},
		{
			name:     "unknown additional role item",
			topology: map[string]cty.Value{"additional_roles": cty.ListVal([]cty.Value{cty.UnknownVal(cty.String)})},
		},
		{
			name: "unknown availability with dedicated coordinator",
			topology: map[string]cty.Value{
				"high_availability": cty.UnknownVal(cty.Bool),
				"coordinator":       coordinator,
			},
		},
		{
			name: "unknown coordinator block",
			topology: map[string]cty.Value{
				"high_availability": cty.BoolVal(true),
				"coordinator":       cty.UnknownVal(coordinator.Type()),
			},
		},
		{
			name:      "known missing coordinator remains invalid",
			wantError: true,
		},
		{
			name: "standalone dedicated coordinator remains invalid when instance type is unknown",
			topology: map[string]cty.Value{
				"coordinator": coordinatorWithUnknownInstanceType,
			},
			wantError: true,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			configValues := map[string]cty.Value{
				"name":               cty.StringVal("clickhouse-plan"),
				"instance_type":      cty.StringVal("c5.large"),
				"security_group_ids": cty.SetVal([]cty.Value{cty.StringVal("sg-12345678")}),
				"subnet_ids":         cty.SetVal([]cty.Value{cty.StringVal("subnet-12345678")}),
				"root_volume": cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{
					"type": cty.StringVal("gp2"),
				})}),
				"data_volume": cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{
					"type": cty.StringVal("gp2"),
				})}),
				"clickhouse": cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{
					"version": cty.StringVal("25.3"),
				})}),
			}
			for key, value := range testCase.topology {
				configValues[key] = value
			}
			resource := ResourceService()
			rawConfig := cty.ObjectVal(configValues)
			config := terraform.NewResourceConfigShimmed(rawConfig, resource.CoreConfigSchema())
			_, err := resource.Diff(context.Background(), &terraform.InstanceState{RawConfig: rawConfig}, config, nil)
			if (err != nil) != testCase.wantError {
				t.Fatalf("unexpected topology validation result: %v", err)
			}
		})
	}
}

func TestValidateClickHouseServiceConfig(t *testing.T) {
	testCases := []struct {
		name      string
		config    clickHouseServiceConfig
		expectErr bool
	}{
		{
			name: "standalone allows coordinator role",
			config: clickHouseServiceConfig{
				hasCoordinatorRole: true,
			},
		},
		{
			name:      "standalone requires coordinator role",
			config:    clickHouseServiceConfig{},
			expectErr: true,
		},
		{
			name: "standalone rejects dedicated coordinator block",
			config: clickHouseServiceConfig{
				hasCoordinatorBlock: true,
			},
			expectErr: true,
		},
		{
			name: "ha allows combined nodes",
			config: clickHouseServiceConfig{
				highAvailability:   true,
				hasCoordinatorRole: true,
			},
		},
		{
			name: "ha allows dedicated coordinator block",
			config: clickHouseServiceConfig{
				highAvailability:    true,
				hasCoordinatorBlock: true,
			},
		},
		{
			name: "ha requires coordinator role or block",
			config: clickHouseServiceConfig{
				highAvailability: true,
			},
			expectErr: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := validateClickHouseServiceConfig(testCase.config)

			if testCase.expectErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error, got %s", err)
			}
		})
	}
}
