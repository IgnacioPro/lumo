package checkers

import (
	"strings"
	"testing"

	"github.com/ignacio/lumo/internal/diagnostics"
)

func TestNewServiceChecker(t *testing.T) {
	services := []string{"nginx", "mysql"}
	checker := NewServiceChecker(services)

	if checker == nil {
		t.Fatal("NewServiceChecker returned nil")
	}

	if len(checker.monitoredServices) != 2 {
		t.Errorf("Expected 2 monitored services, got %d", len(checker.monitoredServices))
	}
}

func TestServiceChecker_Name(t *testing.T) {
	checker := NewServiceChecker(nil)
	if got := checker.Name(); got != "service_check" {
		t.Errorf("Name() = %v, want service_check", got)
	}
}

func TestServiceChecker_Category(t *testing.T) {
	checker := NewServiceChecker(nil)
	if got := checker.Category(); got != diagnostics.CategoryService {
		t.Errorf("Category() = %v, want %v", got, diagnostics.CategoryService)
	}
}

func TestServiceChecker_Description(t *testing.T) {
	checker := NewServiceChecker(nil)
	desc := checker.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}
	if !strings.Contains(strings.ToLower(desc), "service") {
		t.Error("Description should mention service")
	}
}

func TestServiceChecker_RequiresRoot(t *testing.T) {
	checker := NewServiceChecker(nil)
	if checker.RequiresRoot() {
		t.Error("ServiceChecker should not require root")
	}
}

// TestServiceChecker_Run is complex - tested via integration tests

// detectServiceManager is complex - tested via integration tests

func TestServiceChecker_FilterServices(t *testing.T) {
	services := []ServiceInfo{
		{Name: "nginx", State: "running"},
		{Name: "mysql", State: "running"},
		{Name: "redis", State: "failed"},
		{Name: "postgres", State: "inactive"},
	}

	checker := NewServiceChecker(nil)

	tests := []struct {
		name      string
		filter    []string
		wantCount int
	}{
		{
			name:      "filter specific services",
			filter:    []string{"nginx", "mysql"},
			wantCount: 2,
		},
		{
			name:      "filter single service",
			filter:    []string{"redis"},
			wantCount: 1,
		},
		{
			name:      "filter non-existent service",
			filter:    []string{"nonexistent"},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := checker.filterServices(services, tt.filter)
			if len(filtered) != tt.wantCount {
				t.Errorf("filterServices() returned %d services, want %d", len(filtered), tt.wantCount)
			}
		})
	}
}
