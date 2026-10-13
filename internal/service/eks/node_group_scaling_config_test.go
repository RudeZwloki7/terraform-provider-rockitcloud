package eks

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
)

func TestExpandNodegroupScalingConfigSendsZeroMinSize(t *testing.T) {
	apiObject := expandNodegroupScalingConfig(map[string]interface{}{
		"desired_size": 0,
		"max_size":     1,
		"min_size":     0,
	})

	if apiObject.MinSize == nil || aws.Int64Value(apiObject.MinSize) != 0 {
		t.Fatalf("min_size=0 must be sent, got %v", apiObject.MinSize)
	}
	if apiObject.DesiredSize == nil || aws.Int64Value(apiObject.DesiredSize) != 0 {
		t.Fatalf("desired_size=0 must be sent, got %v", apiObject.DesiredSize)
	}
}
