package services

import (
	"reflect"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/paas"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestClickHouseNames(t *testing.T) {
	t.Parallel()

	resourceSchema := ClickHouse.ResourceSchema().Elem.(*schema.Resource).Schema
	user := resourceSchema["user"].Elem.(*schema.Resource).Schema
	database := resourceSchema["database"].Elem.(*schema.Resource).Schema
	for _, testCase := range []struct {
		name     string
		schema   *schema.Schema
		accepted []string
		rejected []string
	}{
		{
			name:     "users",
			schema:   user["name"],
			accepted: []string{"analytics", "User_123", strings.Repeat("a", 63)},
			rejected: []string{"", "1user", "_user", "user-name", "пользователь", "default", "cluster_user", strings.Repeat("a", 64)},
		},
		{
			name:     "databases",
			schema:   database["name"],
			accepted: []string{"analytics", "Database_123", strings.Repeat("a", 63)},
			rejected: []string{"", "1db", "_db", "db.name", "default", "readonly", "system", "information_schema", "INFORMATION_SCHEMA", strings.Repeat("a", 64)},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			for _, value := range testCase.accepted {
				if _, errs := testCase.schema.ValidateFunc(value, "name"); len(errs) != 0 {
					t.Errorf("name %q must be accepted: %v", value, errs)
				}
			}
			for _, value := range testCase.rejected {
				if _, errs := testCase.schema.ValidateFunc(value, "name"); len(errs) == 0 {
					t.Errorf("name %q must be rejected", value)
				}
			}
		})
	}
}

func TestClickHouseSchema(t *testing.T) {
	t.Parallel()

	resourceSchema := ClickHouse.ResourceSchema().Elem.(*schema.Resource).Schema
	dataSourceSchema := ClickHouse.DataSourceSchema().Elem.(*schema.Resource).Schema
	for _, serviceSchema := range []map[string]*schema.Schema{resourceSchema, dataSourceSchema} {
		user := serviceSchema["user"].Elem.(*schema.Resource).Schema
		if !user["password"].Sensitive {
			t.Error("ClickHouse passwords must be sensitive in resources and data sources")
		}
		database := serviceSchema["database"].Elem.(*schema.Resource).Schema
		if _, ok := database["user"]; ok {
			t.Error("ClickHouse database users are managed by grants, not a nested database.user block")
		}
		if _, ok := database["backup_enabled"]; ok {
			t.Error("ClickHouse does not support PaaS database backups")
		}
	}
	if !resourceSchema["version"].ForceNew || !resourceSchema["version"].Required {
		t.Error("ClickHouse version must be required and immutable")
	}
	if _, errors := resourceSchema["version"].ValidateFunc("", "version"); len(errors) == 0 {
		t.Error("an empty version must fail validation before CreateService")
	}
}

func TestClickHouseUserParameters(t *testing.T) {
	t.Parallel()

	networks := map[string]interface{}{
		"host":        schema.NewSet(schema.HashString, []interface{}{"db.example.com"}),
		"host_regexp": schema.NewSet(schema.HashString, []interface{}{`.*\.example\.com`}),
		"ip":          schema.NewSet(schema.HashString, []interface{}{"10.0.0.0/24"}),
	}
	input := map[string]interface{}{
		"grants":        []interface{}{"GRANT SELECT ON analytics.*"},
		"networks":      []interface{}{networks},
		"password":      "clickhouse-user-secret",
		"password_type": "double_sha1",
		"profile":       "analytics_profile",
		"quota":         "analytics_quota",
	}
	expected := map[string]interface{}{
		"grants": []interface{}{"GRANT SELECT ON analytics.*"},
		"networks": map[string]interface{}{
			"host":        []interface{}{"db.example.com"},
			"host_regexp": []interface{}{`.*\.example\.com`},
			"ip":          []interface{}{"10.0.0.0/24"},
		},
		"password":      "clickhouse-user-secret",
		"password_type": "double_sha1",
		"profile":       "analytics_profile",
		"quota":         "analytics_quota",
	}
	if got := ClickHouse.expandUserParameters(input); !reflect.DeepEqual(got, expected) {
		t.Fatalf("expanded user parameters: got %#v want %#v", got, expected)
	}

	// Removing every restriction must send explicit empty collections.
	cleared := ClickHouse.expandUserParameters(map[string]interface{}{
		"grants":   []interface{}{},
		"networks": []interface{}{},
		"password": "",
	})
	if got, ok := cleared["grants"].([]interface{}); !ok || len(got) != 0 {
		t.Errorf("empty grants must be sent: %#v", cleared)
	}
	if got, ok := cleared["networks"].(map[string]interface{}); !ok || len(got) != 0 {
		t.Errorf("empty networks must be sent: %#v", cleared)
	}
	if _, ok := cleared["password"]; ok {
		t.Error("an omitted password must not reset an existing password")
	}
}

func TestClickHouseFlattenUser(t *testing.T) {
	t.Parallel()

	got := ClickHouse.FlattenUser(&paas.UserResponse{
		Id:   aws.String("clickhouse-user-12345678"),
		Name: aws.String("analytics_user"),
		Parameters: map[string]interface{}{
			"grants":       []interface{}{"GRANT SELECT ON analytics.*"},
			"networks":     map[string]interface{}{"ip": []interface{}{"10.0.0.0/24"}},
			"password":     "returned-password",
			"passwordType": "double_sha1",
			"profile":      "default",
			"quota":        "default",
		},
	}, false)
	if got["password_type"] != "double_sha1" || got["password"] != "returned-password" {
		t.Fatalf("API password fields were not flattened: %#v", got)
	}
	if got["profile"] != "default" || got["quota"] != "default" {
		t.Fatalf("user profile and quota were not flattened: %#v", got)
	}
	networks := got["networks"].([]map[string]interface{})
	if !reflect.DeepEqual(networks[0]["ip"], []interface{}{"10.0.0.0/24"}) {
		t.Fatalf("unexpected networks: %#v", networks)
	}
	empty := ClickHouse.flattenUserParameters(map[string]interface{}{"networks": map[string]interface{}{}})
	if _, exists := empty["networks"]; exists {
		t.Fatal("API networks={} must preserve an omitted networks block")
	}
}

func TestClickHouseDatabaseHasNoNestedUsers(t *testing.T) {
	t.Parallel()

	got := ClickHouse.FlattenDatabase(&paas.DatabaseResponse{
		Name: aws.String("analytics"),
		Users: []*paas.UserResponse{
			{Name: aws.String("analytics_user")},
		},
	})
	if _, ok := got["user"]; ok {
		t.Fatalf("API users must not populate an unsupported database.user block: %#v", got)
	}
}

func TestClickHouseServiceParameters(t *testing.T) {
	t.Parallel()

	input := map[string]interface{}{
		"version": "25.3",
		"options": map[string]interface{}{"max_connections": "1024"},
	}
	if got := ClickHouse.expandServiceParameters(input); !reflect.DeepEqual(got, input) {
		t.Fatalf("unexpected expanded parameters: %#v", got)
	}
	if got := ClickHouse.flattenServiceParameters(input); !reflect.DeepEqual(got, input) {
		t.Fatalf("unexpected flattened parameters: %#v", got)
	}
	if got := ClickHouse.expandServiceParameters(nil); got != nil {
		t.Fatalf("nil input must remain nil: %#v", got)
	}
	cleared := ClickHouse.expandServiceParameters(map[string]interface{}{"options": map[string]interface{}{}})
	if options, ok := cleared["options"].(map[string]interface{}); !ok || len(options) != 0 {
		t.Fatalf("removing the last server option must send an explicit empty map: %#v", cleared)
	}
}
