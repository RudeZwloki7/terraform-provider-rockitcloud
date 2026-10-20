package paas

import (
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-provider-aws/internal/service/paas/services"
)

func TestServiceMongoDBSlowmsParameters(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		rawValue    cty.Value
		missing     bool
		stateValue  int
		wantPresent bool
	}{
		{name: "omitted", missing: true},
		{name: "null", rawValue: cty.NullVal(cty.Number)},
		{name: "explicit_zero", rawValue: cty.NumberIntVal(0), wantPresent: true},
		{name: "explicit_threshold", rawValue: cty.NumberIntVal(100), stateValue: 100, wantPresent: true},
		{name: "omitted_with_existing_zero", rawValue: cty.NullVal(cty.Number)},
		{name: "omitted_with_existing_threshold", rawValue: cty.NullVal(cty.Number), stateValue: 100},
		{name: "unknown_threshold", rawValue: cty.UnknownVal(cty.Number), wantPresent: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resource := ResourceService()
			data := schema.TestResourceDataRaw(t, resource.Schema, map[string]interface{}{
				"mongodb": []interface{}{map[string]interface{}{
					"version": "7.0",
					"slowms":  tc.stateValue,
				}},
			})
			data.SetId("mongodb-slowms-test")
			rawParameters := map[string]cty.Value{"version": cty.UnknownVal(cty.String)}
			if !tc.missing {
				rawParameters["slowms"] = tc.rawValue
			}
			state := data.State()
			state.RawConfig = cty.ObjectVal(map[string]cty.Value{
				"mongodb": cty.ListVal([]cty.Value{cty.ObjectVal(rawParameters)}),
			})
			data = resource.Data(state)

			for name, expand := range map[string]func(map[string]interface{}) services.ServiceParameters{
				"create": services.MongoDB.ExpandServiceCreationParameters,
				"update": services.MongoDB.ExpandServiceParameters,
			} {
				t.Run(name, func(t *testing.T) {
					parameters := serviceParametersFromResourceData(data, services.ServiceTypeMongoDB)
					got, present := expand(parameters)["slowms"]
					if present != tc.wantPresent || (present && got != int64(tc.stateValue)) {
						t.Fatalf("slowms in API parameters: got (%#v, present=%t), want (%d, present=%t)", got, present, tc.stateValue, tc.wantPresent)
					}
					if got := parameters["version"]; got != "7.0" {
						t.Fatalf("unrelated version parameter changed: %#v", got)
					}
				})
			}
			original := data.Get("mongodb").([]interface{})[0].(map[string]interface{})
			if got := original["slowms"]; got != tc.stateValue {
				t.Fatalf("resource state was changed: got %#v, want %d", got, tc.stateValue)
			}
		})
	}
}
