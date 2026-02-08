package remediation

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ignacio/lumo/internal/notifications"
)

func TestImageTypoDetector_TagTypos(t *testing.T) {
	detector := NewImageTypoDetector(logrus.New())

	tests := []struct {
		name          string
		errorImage    string
		expectedImage string
		shouldDetect  bool
		minConfidence float64
	}{
		{
			name:          "latst -> latest",
			errorImage:    "nginx:latst",
			expectedImage: "nginx:latest",
			shouldDetect:  true,
			minConfidence: 0.8,
		},
		{
			name:          "lates -> latest",
			errorImage:    "nginx:lates",
			expectedImage: "nginx:latest",
			shouldDetect:  true,
			minConfidence: 0.8,
		},
		{
			name:          "lastest -> latest",
			errorImage:    "nginx:lastest",
			expectedImage: "nginx:latest",
			shouldDetect:  true,
			minConfidence: 0.8,
		},
		{
			name:          "alpne -> alpine",
			errorImage:    "node:alpne",
			expectedImage: "node:alpine",
			shouldDetect:  true,
			minConfidence: 0.7,
		},
		{
			name:          "stabla -> stable",
			errorImage:    "debian:stabla",
			expectedImage: "debian:stable",
			shouldDetect:  true,
			minConfidence: 0.8,
		},
		{
			name:          "correct tag - no detection",
			errorImage:    "nginx:latest",
			expectedImage: "",
			shouldDetect:  false,
		},
		{
			name:          "custom version tag - no detection",
			errorImage:    "myapp:v1.2.3",
			expectedImage: "",
			shouldDetect:  false,
		},
		{
			name:          "registry with port",
			errorImage:    "registry:5000/nginx:latst",
			expectedImage: "registry:5000/nginx:latest",
			shouldDetect:  true,
			minConfidence: 0.8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestion := detector.DetectTypo(tt.errorImage)

			if tt.shouldDetect {
				require.NotNil(t, suggestion, "Expected typo detection for %s", tt.errorImage)
				assert.Equal(t, tt.errorImage, suggestion.Original)
				assert.Equal(t, tt.expectedImage, suggestion.Suggested)
				assert.GreaterOrEqual(t, suggestion.Confidence, tt.minConfidence)
				assert.NotEmpty(t, suggestion.Reason)
			} else {
				assert.Nil(t, suggestion, "Expected no typo detection for %s", tt.errorImage)
			}
		})
	}
}

func TestImageTypoDetector_ImageNameTypos(t *testing.T) {
	detector := NewImageTypoDetector(logrus.New())

	tests := []struct {
		name          string
		errorImage    string
		expectedImage string
		shouldDetect  bool
	}{
		{
			name:          "ngix -> nginx",
			errorImage:    "ngix:latest",
			expectedImage: "nginx:latest",
			shouldDetect:  true,
		},
		{
			name:          "postgress -> postgres",
			errorImage:    "postgress:13",
			expectedImage: "postgres:13",
			shouldDetect:  true,
		},
		{
			name:          "correct name - no detection",
			errorImage:    "nginx:latest",
			expectedImage: "",
			shouldDetect:  false,
		},
		{
			name:          "unknown custom image - no detection",
			errorImage:    "mycompany/myapp:v1",
			expectedImage: "",
			shouldDetect:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestion := detector.DetectTypo(tt.errorImage)

			if tt.shouldDetect {
				require.NotNil(t, suggestion, "Expected typo detection for %s", tt.errorImage)
				assert.Equal(t, tt.expectedImage, suggestion.Suggested)
			} else {
				assert.Nil(t, suggestion, "Expected no typo detection for %s", tt.errorImage)
			}
		})
	}
}

func TestParseImageReference(t *testing.T) {
	tests := []struct {
		input        string
		expectedName string
		expectedTag  string
	}{
		{"nginx", "nginx", ""},
		{"nginx:latest", "nginx", "latest"},
		{"nginx:1.21", "nginx", "1.21"},
		{"myregistry.com/nginx:v1", "myregistry.com/nginx", "v1"},
		{"registry:5000/myimage:tag", "registry:5000/myimage", "tag"},
		{"gcr.io/project/image:sha256abc", "gcr.io/project/image", "sha256abc"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			name, tag := parseImageReference(tt.input)
			assert.Equal(t, tt.expectedName, name)
			assert.Equal(t, tt.expectedTag, tag)
		})
	}
}

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		s1       string
		s2       string
		expected int
	}{
		{"", "", 0},
		{"a", "", 1},
		{"", "b", 1},
		{"abc", "abc", 0},
		{"abc", "ab", 1},
		{"abc", "abcd", 1},
		{"abc", "adc", 1},
		{"latst", "latest", 1},
		{"ngix", "nginx", 1},
		{"postgress", "postgres", 1},
		{"hello", "world", 4},
	}

	for _, tt := range tests {
		t.Run(tt.s1+"->"+tt.s2, func(t *testing.T) {
			assert.Equal(t, tt.expected, levenshteinDistance(tt.s1, tt.s2))
		})
	}
}

func TestRemediationMatrix_DefaultStrategies(t *testing.T) {
	matrix := NewRemediationMatrix(logrus.New())

	// Verify all default strategies are registered
	expectedTypes := []IssueType{
		IssueImagePullTypo,
		IssueImagePullAuth,
		IssueCrashLoopOOM,
		IssueOOMKilled,
		IssueCrashLoopStartup,
		IssueNodeNotReady,
		IssueDeploymentStuck,
		IssuePVCProvisionFailed,
	}

	for _, issueType := range expectedTypes {
		strategy, ok := matrix.GetStrategy(issueType)
		require.True(t, ok, "Strategy %s should be registered", issueType)
		assert.NotEmpty(t, strategy.DisplayName)
		assert.NotEmpty(t, strategy.DetectionRules)
	}
}

func TestRemediationMatrix_DetectIssue(t *testing.T) {
	matrix := NewRemediationMatrix(logrus.New())

	tests := []struct {
		name          string
		eventType     string
		reason        string
		message       string
		exitCode      *int32
		expectedIssue IssueType
	}{
		{
			name:          "ImagePullBackOff",
			eventType:     "Warning",
			reason:        "Failed",
			message:       "Error: ImagePullBackOff",
			expectedIssue: IssueImagePullTypo,
		},
		{
			name:          "Auth failure",
			eventType:     "Warning",
			reason:        "Failed",
			message:       "unauthorized: authentication required",
			expectedIssue: IssueImagePullAuth,
		},
		{
			name:          "OOMKilled",
			eventType:     "Warning",
			reason:        "OOMKilled",
			message:       "Container killed due to OOM",
			exitCode:      ptrInt32(137),
			expectedIssue: IssueOOMKilled,
		},
		{
			name:          "Deployment stuck",
			eventType:     "Warning",
			reason:        "ProgressDeadlineExceeded",
			message:       "Deployment exceeded its progress deadline",
			expectedIssue: IssueDeploymentStuck,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategies := matrix.DetectIssue(tt.eventType, tt.reason, tt.message, tt.exitCode)
			require.NotEmpty(t, strategies, "Should detect at least one strategy")

			found := false
			for _, s := range strategies {
				if s.IssueType == tt.expectedIssue {
					found = true
					break
				}
			}
			assert.True(t, found, "Expected to detect %s", tt.expectedIssue)
		})
	}
}

func TestRemediationMatrix_CreateProposal(t *testing.T) {
	matrix := NewRemediationMatrix(logrus.New())

	strategy, ok := matrix.GetStrategy(IssueImagePullTypo)
	require.True(t, ok)

	params := map[string]string{
		"DeploymentName": "my-app",
		"ContainerName":  "web",
		"CorrectImage":   "nginx:latest",
		"ErrorImage":     "nginx:latst",
	}

	proposal := matrix.CreateProposal(strategy, "incident-123", "default", "my-app", "Deployment", params)

	assert.NotEmpty(t, proposal.Hash)
	assert.Equal(t, "incident-123", proposal.IncidentID)
	assert.Equal(t, "default", proposal.Namespace)
	assert.Contains(t, proposal.Command, "my-app")
	assert.Contains(t, proposal.Command, "nginx:latest")

	// Test conversion to notifications.ProposedFix
	fix := proposal.ToProposedFix()
	assert.Equal(t, strategy.DisplayName, fix.Title)
	assert.Equal(t, strategy.Risk, fix.Risk)
	assert.Equal(t, strategy.AutoRemediate, fix.AutoApprove)
	assert.Equal(t, proposal.Hash, fix.Hash)
}

func TestRemediationMatrix_AutoRemediable(t *testing.T) {
	matrix := NewRemediationMatrix(logrus.New())

	autoRemediable := matrix.ListAutoRemediable()

	// At least some strategies should be auto-remediable
	assert.NotEmpty(t, autoRemediable)

	for _, s := range autoRemediable {
		assert.True(t, s.AutoRemediate, "Strategy %s should be auto-remediable", s.IssueType)
		assert.Equal(t, s.Risk, notifications.RiskLevel("safe"), "Auto-remediable strategies should be safe risk, got %s for %s", s.Risk, s.IssueType)
	}
}

// Helper to create int32 pointer
func ptrInt32(i int32) *int32 {
	return &i
}
