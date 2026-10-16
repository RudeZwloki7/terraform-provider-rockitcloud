package paas

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/paas"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func validatePostgreSQLServiceConfiguration(d *schema.ResourceDiff) error {
	const userKey = "pgsql.0.user"
	if _, ok := d.GetOk("pgsql"); !ok || !d.NewValueKnown(userKey) || !d.NewValueKnown("pgsql.0.version") {
		return nil
	}
	major, _ := strconv.Atoi(strings.Split(d.Get("pgsql.0.version").(string), ".")[0])
	for index, user := range d.Get(userKey).([]interface{}) {
		roles := user.(map[string]interface{})["roles"].(*schema.Set)
		for role, minimum := range map[string]int{"pg_create_subscription": 16, "pg_maintain": 17} {
			if roles.Contains(role) && major < minimum {
				return fmt.Errorf("%s.%d.roles: %s requires PostgreSQL %d or later", userKey, index, role, minimum)
			}
		}
	}
	return nil
}

// Preserve compatibility with old environments by sending an empty roles array
// only for users whose previously assigned roles are being removed.
func clearPostgreSQLUserRoles(users []*paas.UserCreateRequest, previous []interface{}) {
	assigned := make(map[string]bool)
	for _, user := range previous {
		user := user.(map[string]interface{})
		if roles, ok := user["roles"].(*schema.Set); ok && roles.Len() > 0 {
			assigned[user["name"].(string)] = true
		}
	}
	for _, user := range users {
		if _, hasRoles := user.Parameters["roles"]; !hasRoles && assigned[aws.StringValue(user.Name)] {
			user.Parameters["roles"] = []interface{}{}
		}
	}
}
