package services

import (
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestPostgreSQLUserRoles(t *testing.T) {
	t.Parallel()
	input := map[string]interface{}{
		"password": "pgsql-test-password",
		"roles":    schema.NewSet(schema.HashString, []interface{}{"pg_monitor", "pg_signal_backend"}),
	}
	parameters := PostgreSQL.expandUserParameters(input)
	if parameters["password"] != "pgsql-test-password" {
		t.Fatal("password lost when expanding roles")
	}
	roles, ok := parameters["roles"].([]interface{})
	if !ok || !schema.NewSet(schema.HashString, roles).Equal(input["roles"]) {
		t.Fatalf("roles lost when expanding user: %#v", parameters)
	}
	flattened := PostgreSQL.flattenUserParameters(parameters)
	if !reflect.DeepEqual(flattened, parameters) {
		t.Fatalf("roles lost when reading user: %#v", flattened)
	}
	for _, input := range []map[string]interface{}{nil, {"password": "pgsql-test-password"}, {"roles": schema.NewSet(schema.HashString, nil)}} {
		if _, ok := PostgreSQL.expandUserParameters(input)["roles"]; ok {
			t.Fatal("roles must be omitted for users without roles on old PaaS environments")
		}
	}
}

func TestPostgreSQLUserRolesSchema(t *testing.T) {
	t.Parallel()
	resourceSchema := PostgreSQL.ResourceSchema().Elem.(*schema.Resource).Schema
	user := resourceSchema["user"].Elem.(*schema.Resource).Schema
	for _, role := range []string{
		"pg_monitor", "pg_read_all_settings", "pg_read_all_stats", "pg_stat_scan_tables",
		"pg_maintain", "pg_signal_backend", "pg_create_subscription",
	} {
		if _, errs := user["roles"].Elem.(*schema.Schema).ValidateFunc(role, "roles"); len(errs) > 0 {
			t.Errorf("API role %q rejected: %v", role, errs)
		}
	}
	if _, errs := user["roles"].Elem.(*schema.Schema).ValidateFunc("superuser", "roles"); len(errs) == 0 {
		t.Error("unsupported roles must be rejected")
	}
	for _, serviceSchema := range []*schema.Schema{PostgreSQL.ResourceSchema(), PostgreSQL.DataSourceSchema()} {
		user := serviceSchema.Elem.(*schema.Resource).Schema["user"].Elem.(*schema.Resource).Schema
		if !user["password"].Sensitive {
			t.Error("PostgreSQL passwords must be sensitive in resources and data sources")
		}
		databaseUser := serviceSchema.Elem.(*schema.Resource).Schema["database"].Elem.(*schema.Resource).Schema["user"].Elem.(*schema.Resource).Schema
		if _, ok := databaseUser["roles"]; ok {
			t.Error("PostgreSQL roles belong to service users, not database users")
		}
	}
}
