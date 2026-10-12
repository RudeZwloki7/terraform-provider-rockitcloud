package paas

import "testing"

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
