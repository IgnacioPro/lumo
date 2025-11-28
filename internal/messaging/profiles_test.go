package messaging

import (
	"testing"
)

func TestProfileConfig(t *testing.T) {
	tests := []struct {
		name           string
		profile        DeploymentProfile
		wantProvider   string
		wantBrokers    int
		wantMaxInFligt int
		wantErr        bool
	}{
		{
			name:           "xs profile",
			profile:        ProfileXS,
			wantProvider:   "redis",
			wantBrokers:    1,
			wantMaxInFligt: 5,
			wantErr:        false,
		},
		{
			name:           "s profile",
			profile:        ProfileS,
			wantProvider:   "nats",
			wantBrokers:    1,
			wantMaxInFligt: 10,
			wantErr:        false,
		},
		{
			name:           "m profile",
			profile:        ProfileM,
			wantProvider:   "nats",
			wantBrokers:    3,
			wantMaxInFligt: 50,
			wantErr:        false,
		},
		{
			name:           "xl profile",
			profile:        ProfileXL,
			wantProvider:   "kafka",
			wantBrokers:    3,
			wantMaxInFligt: 100,
			wantErr:        false,
		},
		{
			name:    "unknown profile",
			profile: "unknown",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ProfileConfig(tt.profile)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ProfileConfig() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ProfileConfig() unexpected error: %v", err)
				return
			}

			if cfg.Provider != tt.wantProvider {
				t.Errorf("ProfileConfig().Provider = %v, want %v", cfg.Provider, tt.wantProvider)
			}

			if len(cfg.Brokers) != tt.wantBrokers {
				t.Errorf("ProfileConfig().Brokers count = %d, want %d", len(cfg.Brokers), tt.wantBrokers)
			}

			if cfg.MaxInFlight != tt.wantMaxInFligt {
				t.Errorf("ProfileConfig().MaxInFlight = %v, want %v", cfg.MaxInFlight, tt.wantMaxInFligt)
			}
		})
	}
}

func TestProfileRecommendation(t *testing.T) {
	tests := []struct {
		name       string
		agentCount int
		want       DeploymentProfile
	}{
		{"1 agent", 1, ProfileXS},
		{"10 agents", 10, ProfileXS},
		{"11 agents", 11, ProfileS},
		{"50 agents", 50, ProfileS},
		{"51 agents", 51, ProfileM},
		{"500 agents", 500, ProfileM},
		{"501 agents", 501, ProfileXL},
		{"10000 agents", 10000, ProfileXL},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ProfileRecommendation(tt.agentCount)
			if got != tt.want {
				t.Errorf("ProfileRecommendation(%d) = %v, want %v", tt.agentCount, got, tt.want)
			}
		})
	}
}

func TestProfileDescription(t *testing.T) {
	profiles := []DeploymentProfile{
		ProfileXS,
		ProfileS,
		ProfileM,
		ProfileXL,
	}

	for _, profile := range profiles {
		desc := ProfileDescription(profile)
		if desc == "" || desc == "Unknown profile" {
			t.Errorf("ProfileDescription(%v) returned empty or unknown", profile)
		}
	}
}

func TestProfileResources(t *testing.T) {
	tests := []struct {
		name            string
		profile         DeploymentProfile
		wantMinMemoryMB int
		wantMinNodes    int
	}{
		{"xs", ProfileXS, 0, 0},
		{"s", ProfileS, 64, 1},
		{"m", ProfileM, 512, 3},
		{"xl", ProfileXL, 2048, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := ProfileResources(tt.profile)
			if res.MinMemoryMB != tt.wantMinMemoryMB {
				t.Errorf("ProfileResources(%v).MinMemoryMB = %d, want %d", tt.profile, res.MinMemoryMB, tt.wantMinMemoryMB)
			}
			if res.MinNodes != tt.wantMinNodes {
				t.Errorf("ProfileResources(%v).MinNodes = %d, want %d", tt.profile, res.MinNodes, tt.wantMinNodes)
			}
		})
	}
}
