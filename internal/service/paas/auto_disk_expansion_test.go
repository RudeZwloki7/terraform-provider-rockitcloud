package paas

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/paas"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestExpandAutoDiskExpansionSettings(t *testing.T) {
	t.Parallel()

	got := expandAutoDiskExpansionSettings(map[string]interface{}{
		"condition_value": 80,
		"enabled":         true,
		"max_size_gb":     64,
		"status":          "active",
	})

	if aws.Int64Value(got.ConditionValue) != 80 {
		t.Errorf("ConditionValue = %d, want 80", aws.Int64Value(got.ConditionValue))
	}
	if !aws.BoolValue(got.Enabled) {
		t.Error("Enabled = false, want true")
	}
	if aws.Int64Value(got.MaxSizeGb) != 64 {
		t.Errorf("MaxSizeGb = %d, want 64", aws.Int64Value(got.MaxSizeGb))
	}

	if expandAutoDiskExpansionSettings(nil) != nil {
		t.Error("expected nil settings for nil map")
	}
}

func TestFlattenAutoDiskExpansionSettings(t *testing.T) {
	t.Parallel()

	if got := flattenAutoDiskExpansionSettings(nil); len(got) != 0 {
		t.Fatalf("expected empty list when ADE was never configured, got %v", got)
	}

	got := flattenAutoDiskExpansionSettings(&paas.AutoDiskExpansionSettings{
		ConditionValue: aws.Int64(75),
		Enabled:        aws.Bool(true),
		MaxSizeGb:      aws.Int64(128),
		Status:         aws.String("active"),
	})

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}

	want := map[string]interface{}{
		"condition_value": int64(75),
		"enabled":         true,
		"max_size_gb":     int64(128),
		"status":          "active",
	}
	for key, value := range want {
		if got[0][key] != value {
			t.Errorf("%s = %v, want %v", key, got[0][key], value)
		}
	}
}

func TestAutoDiskExpansionUpdateInput(t *testing.T) {
	t.Parallel()

	enabledBlock := []interface{}{
		map[string]interface{}{
			"condition_value": 80,
			"enabled":         true,
			"max_size_gb":     64,
			"status":          "active",
		},
	}
	disabledBlock := []interface{}{
		map[string]interface{}{
			"condition_value": 80,
			"enabled":         false,
			"max_size_gb":     64,
			"status":          "active",
		},
	}
	modifiedBlock := []interface{}{
		map[string]interface{}{
			"condition_value": 90,
			"enabled":         true,
			"max_size_gb":     128,
			"status":          "",
		},
	}

	testCases := []struct {
		name        string
		old         []interface{}
		new         []interface{}
		wantNil     bool
		wantEnabled bool
		wantValue   int64
		wantMaxSize int64
	}{
		{
			name:        "block added",
			new:         enabledBlock,
			wantEnabled: true,
			wantValue:   80,
			wantMaxSize: 64,
		},
		{
			name:        "block modified",
			old:         enabledBlock,
			new:         modifiedBlock,
			wantEnabled: true,
			wantValue:   90,
			wantMaxSize: 128,
		},
		{
			name:        "block disabled",
			old:         enabledBlock,
			new:         disabledBlock,
			wantEnabled: false,
			wantValue:   80,
			wantMaxSize: 64,
		},
		{
			name:        "enabled block removed",
			old:         enabledBlock,
			wantEnabled: false,
			wantValue:   80,
			wantMaxSize: 64,
		},
		{
			name:    "disabled block removed",
			old:     disabledBlock,
			wantNil: true,
		},
		{
			name:    "no block",
			wantNil: true,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := autoDiskExpansionUpdateInput("svc-12345678", testCase.old, testCase.new)

			if testCase.wantNil {
				if got != nil {
					t.Fatalf("expected no request, got %s", got)
				}
				return
			}

			if got == nil {
				t.Fatal("expected request, got nil")
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("request is invalid: %s", err)
			}
			if aws.StringValue(got.ServiceId) != "svc-12345678" {
				t.Errorf("ServiceId = %s, want svc-12345678", aws.StringValue(got.ServiceId))
			}
			if aws.BoolValue(got.Enabled) != testCase.wantEnabled {
				t.Errorf("Enabled = %t, want %t", aws.BoolValue(got.Enabled), testCase.wantEnabled)
			}
			if aws.Int64Value(got.ConditionValue) != testCase.wantValue {
				t.Errorf("ConditionValue = %d, want %d", aws.Int64Value(got.ConditionValue), testCase.wantValue)
			}
			if aws.Int64Value(got.MaxSizeGb) != testCase.wantMaxSize {
				t.Errorf("MaxSizeGb = %d, want %d", aws.Int64Value(got.MaxSizeGb), testCase.wantMaxSize)
			}
		})
	}
}

func TestAutoDiskExpansionSchemaValidation(t *testing.T) {
	t.Parallel()

	autoDiskExpansionSchema := ResourceService().Schema["auto_disk_expansion"].Elem.(*schema.Resource).Schema

	testCases := []struct {
		name      string
		attribute string
		value     int
		expectErr bool
	}{
		{name: "condition value below range", attribute: "condition_value", value: 69, expectErr: true},
		{name: "condition value lower bound", attribute: "condition_value", value: 70},
		{name: "condition value upper bound", attribute: "condition_value", value: 90},
		{name: "condition value above range", attribute: "condition_value", value: 91, expectErr: true},
		{name: "max size below minimum", attribute: "max_size_gb", value: 0, expectErr: true},
		{name: "max size minimum", attribute: "max_size_gb", value: 8},
		{name: "max size not multiple of 8", attribute: "max_size_gb", value: 100, expectErr: true},
		{name: "max size multiple of 8", attribute: "max_size_gb", value: 1024},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, errs := autoDiskExpansionSchema[testCase.attribute].ValidateFunc(testCase.value, testCase.attribute)

			if testCase.expectErr && len(errs) == 0 {
				t.Fatal("expected validation error, got none")
			}
			if !testCase.expectErr && len(errs) > 0 {
				t.Fatalf("expected no validation error, got: %v", errs)
			}
		})
	}
}

func TestValidateAutoDiskExpansionConfig(t *testing.T) {
	t.Parallel()

	if err := validateAutoDiskExpansionConfig(true, true); err != nil {
		t.Fatalf("expected ADE with data volume to be allowed, got: %s", err)
	}
	if err := validateAutoDiskExpansionConfig(false, false); err != nil {
		t.Fatalf("expected service without ADE and data volume to be allowed, got: %s", err)
	}
	if err := validateAutoDiskExpansionConfig(true, false); err == nil {
		t.Fatal("expected ADE without data volume to be rejected")
	}
}

func TestDataVolumeSizeDiffSuppressFunc(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name              string
		autoDiskExpansion []interface{}
		old               string
		new               string
		want              bool
	}{
		{
			name:              "expanded by ADE",
			autoDiskExpansion: []interface{}{map[string]interface{}{"enabled": true, "condition_value": 80, "max_size_gb": 64}},
			old:               "40",
			new:               "32",
			want:              true,
		},
		{
			name:              "configured increase",
			autoDiskExpansion: []interface{}{map[string]interface{}{"enabled": true, "condition_value": 80, "max_size_gb": 64}},
			old:               "32",
			new:               "48",
		},
		{
			name:              "ADE disabled",
			autoDiskExpansion: []interface{}{map[string]interface{}{"enabled": false, "condition_value": 80, "max_size_gb": 64}},
			old:               "40",
			new:               "32",
		},
		{
			name: "ADE not configured",
			old:  "40",
			new:  "32",
		},
		{
			name:              "new resource",
			autoDiskExpansion: []interface{}{map[string]interface{}{"enabled": true, "condition_value": 80, "max_size_gb": 64}},
			old:               "",
			new:               "32",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			raw := map[string]interface{}{}
			if testCase.autoDiskExpansion != nil {
				raw["auto_disk_expansion"] = testCase.autoDiskExpansion
			}
			d := schema.TestResourceDataRaw(t, ResourceService().Schema, raw)

			if got := dataVolumeSizeDiffSuppressFunc("data_volume.0.size", testCase.old, testCase.new, d); got != testCase.want {
				t.Fatalf("dataVolumeSizeDiffSuppressFunc() = %t, want %t", got, testCase.want)
			}
		})
	}
}

func TestResourceServiceDiffDataVolumeExpandedByAutoDiskExpansion(t *testing.T) {
	t.Parallel()

	state := &terraform.InstanceState{
		ID: "svc-12345678",
		Attributes: map[string]string{
			"id":                                    "svc-12345678",
			"arbitrator_required":                   "false",
			"auto_disk_expansion.#":                 "1",
			"auto_disk_expansion.0.condition_value": "80",
			"auto_disk_expansion.0.enabled":         "true",
			"auto_disk_expansion.0.max_size_gb":     "64",
			"auto_disk_expansion.0.status":          "active",
			"data_volume.#":                         "1",
			"data_volume.0.iops":                    "0",
			"data_volume.0.size":                    "40",
			"data_volume.0.type":                    "gp2",
			"delete_interfaces_on_destroy":          "false",
			"high_availability":                     "false",
			"subnet_ids.#":                          "1",
			"subnet_ids.0":                          "subnet-12345678",
		},
	}

	testCases := []struct {
		name              string
		autoDiskExpansion interface{}
		size              int
		wantSizeDiff      bool
	}{
		{
			name: "ADE enabled and disk expanded",
			autoDiskExpansion: []interface{}{
				map[string]interface{}{"condition_value": 80, "max_size_gb": 64},
			},
			size: 32,
		},
		{
			name: "ADE enabled and size increased above actual",
			autoDiskExpansion: []interface{}{
				map[string]interface{}{"condition_value": 80, "max_size_gb": 64},
			},
			size:         48,
			wantSizeDiff: true,
		},
		{
			name: "ADE disabled and disk expanded",
			autoDiskExpansion: []interface{}{
				map[string]interface{}{"condition_value": 80, "enabled": false, "max_size_gb": 64},
			},
			size:         32,
			wantSizeDiff: true,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			config := terraform.NewResourceConfigRaw(map[string]interface{}{
				"auto_disk_expansion": testCase.autoDiskExpansion,
				"data_volume": []interface{}{
					map[string]interface{}{"size": testCase.size, "type": "gp2"},
				},
			})

			diff, err := ResourceService().Diff(context.Background(), state, config, nil)
			if err != nil {
				t.Fatalf("unexpected diff error: %s", err)
			}

			_, gotSizeDiff := diff.Attributes["data_volume.0.size"]
			if gotSizeDiff != testCase.wantSizeDiff {
				t.Fatalf("data_volume.0.size diff = %t, want %t", gotSizeDiff, testCase.wantSizeDiff)
			}
		})
	}
}

func TestDataSourceServiceAutoDiskExpansionSchema(t *testing.T) {
	t.Parallel()

	resourceSchema := ResourceService().Schema["auto_disk_expansion"].Elem.(*schema.Resource).Schema
	dataSourceAttribute, ok := DataSourceService().Schema["auto_disk_expansion"]
	if !ok {
		t.Fatal("data source must define auto_disk_expansion because it shares resource read")
	}

	dataSourceSchema := dataSourceAttribute.Elem.(*schema.Resource).Schema
	for key := range resourceSchema {
		attribute, ok := dataSourceSchema[key]
		if !ok {
			t.Errorf("data source auto_disk_expansion is missing %q", key)
			continue
		}
		if !attribute.Computed {
			t.Errorf("data source auto_disk_expansion.%s must be computed", key)
		}
	}
}
