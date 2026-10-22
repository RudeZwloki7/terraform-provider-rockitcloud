package services

import (
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-provider-aws/internal/experimental/nullable"
)

func TestMySQLBinaryLogSchema(t *testing.T) {
	t.Parallel()

	resourceSchema := MySQL.ResourceSchema().Elem.(*schema.Resource).Schema
	flag := resourceSchema["bin_log_enabled"]
	if flag == nil || flag.Type != nullable.TypeNullableBool || !flag.Optional || !flag.Computed || flag.ForceNew || flag.Default != nil {
		t.Fatalf("binary log must preserve absent values and update in place: %#v", flag)
	}
	retention := resourceSchema["binlog_retention_days"]
	if retention == nil || retention.Type != schema.TypeInt || !retention.Optional || !retention.Computed || retention.ForceNew || retention.Default != nil {
		t.Fatalf("retention must preserve the backend default and update in place: %#v", retention)
	}
	for _, days := range []int{1, 7, 365} {
		if _, errs := retention.ValidateFunc(days, "binlog_retention_days"); len(errs) != 0 {
			t.Errorf("valid retention %d rejected: %v", days, errs)
		}
	}
	for _, days := range []int{0, 366} {
		if _, errs := retention.ValidateFunc(days, "binlog_retention_days"); len(errs) == 0 {
			t.Errorf("invalid retention %d accepted", days)
		}
	}
	dataSourceSchema := MySQL.DataSourceSchema().Elem.(*schema.Resource).Schema
	for _, name := range []string{"bin_log_enabled", "binlog_retention_days"} {
		if field := dataSourceSchema[name]; field == nil || !field.Computed || field.Optional {
			t.Errorf("data source must expose computed %s: %#v", name, field)
		}
	}
}

func TestMySQLBinaryLogParameters(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		input map[string]interface{}
		want  ServiceParameters
	}{
		"absent on older environments": {input: map[string]interface{}{}, want: ServiceParameters{}},
		"enabled with backend retention default": {
			input: map[string]interface{}{"bin_log_enabled": "true"},
			want:  ServiceParameters{"bin_log_enabled": true},
		},
		"enabled with retention": {
			input: map[string]interface{}{"bin_log_enabled": "true", "binlog_retention_days": 30},
			want:  ServiceParameters{"bin_log_enabled": true, "binlog_retention_days": int64(30)},
		},
		"disabled with previous retention in state": {
			input: map[string]interface{}{"bin_log_enabled": "false", "binlog_retention_days": 7},
			want:  ServiceParameters{"bin_log_enabled": false},
		},
	} {
		t.Run(name, func(t *testing.T) {
			tc.input["vendor"] = "mysql"
			tc.input["version"] = "8.0.32"
			data := schema.TestResourceDataRaw(t, map[string]*schema.Schema{"mysql": MySQL.ResourceSchema()}, map[string]interface{}{
				"mysql": []interface{}{tc.input},
			})
			parameters := MySQL.ExpandServiceParameters(data.Get("mysql").([]interface{})[0].(map[string]interface{}))
			for _, key := range []string{"bin_log_enabled", "binlog_retention_days"} {
				got, gotOK := parameters[key]
				want, wantOK := tc.want[key]
				if gotOK != wantOK || !reflect.DeepEqual(got, want) {
					t.Errorf("unexpected %s: got %#v (present %t), want %#v (present %t)", key, got, gotOK, want, wantOK)
				}
			}
		})
	}

	for _, enabled := range []bool{true, false} {
		parameters := ServiceParameters{"binLogEnabled": enabled}
		if enabled {
			parameters["binlogRetentionDays"] = int64(30)
		}
		flattened := MySQL.flattenServiceParameters(parameters)
		wantFlag := "false"
		if enabled {
			wantFlag = "true"
			if flattened["binlog_retention_days"] != int64(30) {
				t.Errorf("retention lost on read: %#v", flattened)
			}
		}
		if flattened["bin_log_enabled"] != wantFlag {
			t.Errorf("binary log flag lost on read: %#v", flattened)
		}
	}
	for _, key := range []string{"bin_log_enabled", "binlog_retention_days"} {
		if _, exists := MySQL.flattenServiceParameters(ServiceParameters{})[key]; exists {
			t.Errorf("absent API parameter %s must remain absent", key)
		}
	}
}

func TestMySQLUserGlobalPrivileges(t *testing.T) {
	t.Parallel()

	input := map[string]interface{}{
		"password":          "mysql-test-password",
		"global_privileges": schema.NewSet(schema.HashString, []interface{}{"SELECT", "REPLICATION CLIENT"}),
	}
	parameters := MySQL.expandUserParameters(input)
	privileges, ok := parameters["global_privileges"].([]interface{})
	if !ok || !schema.NewSet(schema.HashString, privileges).Equal(input["global_privileges"]) || parameters["password"] != input["password"] {
		t.Fatalf("user parameters lost when expanding global privileges: %#v", parameters)
	}
	if flattened := MySQL.flattenUserParameters(parameters); !reflect.DeepEqual(flattened, parameters) {
		t.Fatalf("global privileges lost on read: %#v", flattened)
	}
	for _, input := range []map[string]interface{}{nil, {"password": "mysql-test-password"}, {"global_privileges": schema.NewSet(schema.HashString, nil)}} {
		if _, exists := MySQL.expandUserParameters(input)["global_privileges"]; exists {
			t.Error("empty global privileges must be omitted for older environments and revocation")
		}
	}
}

func TestMySQLUserGlobalPrivilegesSchema(t *testing.T) {
	t.Parallel()

	resourceSchema := MySQL.ResourceSchema().Elem.(*schema.Resource).Schema
	privileges := resourceSchema["user"].Elem.(*schema.Resource).Schema["global_privileges"]
	if privileges == nil || privileges.Type != schema.TypeSet || !privileges.Optional || privileges.Computed || privileges.ForceNew {
		t.Fatalf("global privileges must be an optional mutable set: %#v", privileges)
	}
	validator := privileges.Elem.(*schema.Schema).ValidateFunc
	for _, privilege := range []string{"REPLICATION SLAVE", "REPLICATION CLIENT", "SELECT", "RELOAD", "SHOW DATABASES"} {
		if _, errs := validator(privilege, "global_privileges"); len(errs) != 0 {
			t.Errorf("API privilege %q rejected: %v", privilege, errs)
		}
	}
	for _, privilege := range []string{"ALL", "SUPER", "select"} {
		if _, errs := validator(privilege, "global_privileges"); len(errs) == 0 {
			t.Errorf("unsupported privilege %q accepted", privilege)
		}
	}
	for _, serviceSchema := range []*schema.Schema{MySQL.ResourceSchema(), MySQL.DataSourceSchema()} {
		service := serviceSchema.Elem.(*schema.Resource).Schema
		if field := service["user"].Elem.(*schema.Resource).Schema["global_privileges"]; field == nil {
			t.Error("global privileges must be exposed for service users")
		}
		databaseUser := service["database"].Elem.(*schema.Resource).Schema["user"].Elem.(*schema.Resource).Schema
		if _, exists := databaseUser["global_privileges"]; exists {
			t.Error("global privileges belong to service users, not database users")
		}
	}
}
