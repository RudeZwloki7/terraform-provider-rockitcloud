package paas

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestGitLabServicePlanConfiguration(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name            string
		availability    cty.Value
		quantity        cty.Value
		registry        map[string]cty.Value
		unknownRegistry bool
		errorText       string
	}{
		{name: "single node", availability: cty.BoolVal(false)},
		{name: "unsupported HA", availability: cty.BoolVal(true), errorText: "gitlab supports only high_availability = false"},
		{name: "unknown availability", availability: cty.UnknownVal(cty.Bool)},
		{name: "explicit single node", availability: cty.BoolVal(false), quantity: cty.NumberIntVal(1)},
		{name: "unsupported node count", availability: cty.BoolVal(false), quantity: cty.NumberIntVal(2), errorText: "gitlab supports only instance_qty = 1"},
		{name: "unknown node count", availability: cty.BoolVal(false), quantity: cty.UnknownVal(cty.Number)},
		{name: "unknown availability with known invalid count", availability: cty.UnknownVal(cty.Bool), quantity: cty.NumberIntVal(2), errorText: "gitlab supports only instance_qty = 1"},
		{name: "S3 missing bucket", availability: cty.BoolVal(false), registry: map[string]cty.Value{"storage": cty.StringVal("s3"), "s3_user": cty.StringVal("registry-test")}, errorText: "gitlab.0.registry.0.bucket is required"},
		{name: "S3 missing user", availability: cty.BoolVal(false), registry: map[string]cty.Value{"storage": cty.StringVal("s3"), "bucket": cty.StringVal("registry-test")}, errorText: "gitlab.0.registry.0.s3_user is required"},
		{name: "S3 complete", availability: cty.BoolVal(false), registry: map[string]cty.Value{"storage": cty.StringVal("s3"), "bucket": cty.StringVal("registry-test"), "s3_user": cty.StringVal("registry-test")}},
		{name: "S3 unknown bucket", availability: cty.BoolVal(false), registry: map[string]cty.Value{"storage": cty.StringVal("s3"), "bucket": cty.UnknownVal(cty.String), "s3_user": cty.StringVal("registry-test")}},
		{name: "S3 unknown user", availability: cty.BoolVal(false), registry: map[string]cty.Value{"storage": cty.StringVal("s3"), "bucket": cty.StringVal("registry-test"), "s3_user": cty.UnknownVal(cty.String)}},
		{name: "S3 unknown bucket with missing user", availability: cty.BoolVal(false), registry: map[string]cty.Value{"storage": cty.StringVal("s3"), "bucket": cty.UnknownVal(cty.String)}, errorText: "gitlab.0.registry.0.s3_user is required"},
		{name: "unknown storage", availability: cty.BoolVal(false), registry: map[string]cty.Value{"storage": cty.UnknownVal(cty.String)}},
		{name: "unknown registry block", availability: cty.BoolVal(false), unknownRegistry: true},
		{name: "filesystem without S3 configuration", availability: cty.BoolVal(false), registry: map[string]cty.Value{"storage": cty.StringVal("filesystem")}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			resource := ResourceService()
			gitlab := map[string]cty.Value{"password": cty.StringVal("GitlabPlan_password1")}
			if testCase.registry != nil {
				gitlab["registry"] = cty.ListVal([]cty.Value{cty.ObjectVal(testCase.registry)})
			}
			if testCase.unknownRegistry {
				gitlab["registry"] = cty.UnknownVal(cty.List(cty.Object(map[string]cty.Type{
					"storage": cty.String, "bucket": cty.String, "s3_user": cty.String,
				})))
			}
			values := map[string]cty.Value{
				"name":               cty.StringVal("gitlab-plan"),
				"instance_type":      cty.StringVal("m5.large"),
				"high_availability":  testCase.availability,
				"security_group_ids": cty.SetVal([]cty.Value{cty.StringVal("sg-12345678")}),
				"subnet_ids":         cty.SetVal([]cty.Value{cty.StringVal("subnet-12345678")}),
				"root_volume": cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{
					"type": cty.StringVal("gp2"),
				})}),
				"data_volume": cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{
					"type": cty.StringVal("gp2"),
				})}),
				"gitlab": cty.ListVal([]cty.Value{cty.ObjectVal(gitlab)}),
			}
			if testCase.quantity != cty.NilVal {
				values["instance_qty"] = testCase.quantity
			}
			rawConfig := cty.ObjectVal(values)
			config := terraform.NewResourceConfigShimmed(rawConfig, resource.CoreConfigSchema())
			_, err := resource.Diff(context.Background(), &terraform.InstanceState{RawConfig: rawConfig}, config, nil)
			if (err != nil) != (testCase.errorText != "") {
				t.Fatalf("unexpected GitLab availability validation result: %v", err)
			}
			if err != nil && !strings.Contains(err.Error(), testCase.errorText) {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestGitLabRegistryStorageTransitions(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name, before, after string
		wantError           bool
	}{
		{name: "S3 to filesystem while enabled", before: "s3", after: "filesystem", wantError: true},
		{name: "filesystem to S3 while enabled", before: "filesystem", after: "s3", wantError: true},
		{name: "disable S3", before: "s3"},
		{name: "enable filesystem after disabling", after: "filesystem"},
		{name: "enable S3 after disabling", after: "s3"},
		{name: "unchanged S3", before: "s3", after: "s3"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			resource := ResourceService()
			configuration := func(storage string) map[string]interface{} {
				gitlab := map[string]interface{}{"password": "GitlabPlan_password1"}
				if storage != "" {
					registry := map[string]interface{}{"storage": storage}
					if storage == "s3" {
						registry["bucket"] = "registry-test"
						registry["s3_user"] = "registry-test@c2dev"
					}
					gitlab["registry"] = []interface{}{registry}
				}
				return map[string]interface{}{
					"name": "gitlab-plan", "instance_type": "m5.large",
					"security_group_ids": []interface{}{"sg-12345678"},
					"subnet_ids":         []interface{}{"subnet-12345678"},
					"root_volume":        []interface{}{map[string]interface{}{"type": "gp2"}},
					"data_volume":        []interface{}{map[string]interface{}{"type": "gp2"}},
					"gitlab":             []interface{}{gitlab},
				}
			}
			previous := schema.TestResourceDataRaw(t, resource.Schema, configuration(testCase.before))
			previous.SetId("fm-cluster-12345678")
			_, err := resource.Diff(context.Background(), previous.State(), terraform.NewResourceConfigRaw(configuration(testCase.after)), nil)
			if (err != nil) != testCase.wantError {
				t.Fatalf("unexpected storage transition validation result: %v", err)
			}
			if err != nil && !strings.Contains(err.Error(), "remove the registry block and apply") {
				t.Fatalf("unexpected validation error: %s", err)
			}
		})
	}
}
