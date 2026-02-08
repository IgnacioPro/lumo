// Package remediation provides image-related remediation actions.
package remediation

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/diagnostics"
)

// =============================================================================
// Image Typo Detection
// =============================================================================

// CommonImages is a list of frequently used container images
// Used for typo detection via Levenshtein distance
var CommonImages = []string{
	// Web servers
	"nginx", "apache", "httpd", "traefik", "caddy", "haproxy",
	// Databases
	"postgres", "postgresql", "mysql", "mariadb", "mongo", "mongodb", "redis", "memcached",
	"elasticsearch", "cassandra", "couchdb", "influxdb", "timescaledb",
	// Messaging
	"rabbitmq", "kafka", "nats", "activemq", "mosquitto",
	// Languages/Runtimes
	"node", "python", "golang", "ruby", "java", "openjdk", "php", "dotnet",
	// Infrastructure
	"alpine", "ubuntu", "debian", "centos", "amazonlinux", "busybox",
	"vault", "consul", "envoy", "istio",
	// Monitoring
	"prometheus", "grafana", "alertmanager", "jaeger", "zipkin",
	// CI/CD
	"jenkins", "gitlab", "sonarqube", "nexus", "artifactory",
	// Other common
	"wordpress", "drupal", "ghost", "minio", "keycloak", "nginx-ingress",
}

// CommonTags is a list of frequently used image tags
var CommonTags = []string{
	"latest", "stable", "alpine", "slim", "buster", "bullseye", "bookworm",
	"lts", "edge", "dev", "prod", "staging",
	"1", "2", "3", "1.0", "2.0", "3.0",
	"v1", "v2", "v3", "v1.0", "v2.0", "v3.0",
}

// ImageTypoDetector analyzes container image references for common typos
type ImageTypoDetector struct {
	knownImages []string
	knownTags   []string
	logger      *logrus.Logger
}

// NewImageTypoDetector creates a new typo detector
func NewImageTypoDetector(logger *logrus.Logger) *ImageTypoDetector {
	return &ImageTypoDetector{
		knownImages: CommonImages,
		knownTags:   CommonTags,
		logger:      logger,
	}
}

// TypoSuggestion represents a suggested correction for an image typo
type TypoSuggestion struct {
	Original   string  // The original (incorrect) image reference
	Suggested  string  // The suggested correction
	Confidence float64 // Confidence score (0.0 - 1.0)
	Reason     string  // Why this suggestion was made
}

// DetectTypo analyzes an image reference and suggests corrections
func (d *ImageTypoDetector) DetectTypo(errorImage string) *TypoSuggestion {
	// Parse image reference
	imageName, tag := parseImageReference(errorImage)

	// Check for tag typos first (most common)
	if suggestion := d.checkTagTypo(imageName, tag); suggestion != nil {
		return suggestion
	}

	// Check for image name typos
	if suggestion := d.checkImageNameTypo(imageName, tag); suggestion != nil {
		return suggestion
	}

	return nil
}

// checkTagTypo checks if the tag has a typo
func (d *ImageTypoDetector) checkTagTypo(imageName, tag string) *TypoSuggestion {
	if tag == "" {
		return nil
	}

	// Check against known tags
	for _, knownTag := range d.knownTags {
		distance := levenshteinDistance(tag, knownTag)
		// If very close (1-2 char difference), suggest correction
		if distance > 0 && distance <= 2 && len(tag) >= 3 {
			confidence := 1.0 - float64(distance)/float64(max(len(tag), len(knownTag)))
			if confidence >= 0.6 {
				return &TypoSuggestion{
					Original:   fmt.Sprintf("%s:%s", imageName, tag),
					Suggested:  fmt.Sprintf("%s:%s", imageName, knownTag),
					Confidence: confidence,
					Reason:     fmt.Sprintf("Tag '%s' looks like a typo for '%s'", tag, knownTag),
				}
			}
		}
	}

	// Common specific typos
	typoMap := map[string]string{
		"latst":    "latest",
		"latet":    "latest",
		"lastest":  "latest",
		"lastes":   "latest",
		"lates":    "latest",
		"laetst":   "latest",
		"altest":   "latest",
		"stabla":   "stable",
		"stabale":  "stable",
		"stabel":   "stable",
		"alpne":    "alpine",
		"apline":   "alpine",
		"alphine":  "alpine",
		"slm":      "slim",
		"silm":     "slim",
		"busetr":   "buster",
		"bulleye":  "bullseye",
		"bullsye":  "bullseye",
		"bookwrom": "bookworm",
	}

	if correct, ok := typoMap[strings.ToLower(tag)]; ok {
		return &TypoSuggestion{
			Original:   fmt.Sprintf("%s:%s", imageName, tag),
			Suggested:  fmt.Sprintf("%s:%s", imageName, correct),
			Confidence: 0.95,
			Reason:     fmt.Sprintf("Tag '%s' is a common typo for '%s'", tag, correct),
		}
	}

	return nil
}

// checkImageNameTypo checks if the image name has a typo
func (d *ImageTypoDetector) checkImageNameTypo(imageName, tag string) *TypoSuggestion {
	// Extract base name (without registry/org prefix)
	baseName := extractBaseName(imageName)

	// Find closest known image
	type match struct {
		image    string
		distance int
	}

	var matches []match
	for _, known := range d.knownImages {
		distance := levenshteinDistance(baseName, known)
		if distance > 0 && distance <= 2 && len(baseName) >= 3 {
			matches = append(matches, match{known, distance})
		}
	}

	if len(matches) == 0 {
		return nil
	}

	// Sort by distance
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].distance < matches[j].distance
	})

	best := matches[0]
	confidence := 1.0 - float64(best.distance)/float64(max(len(baseName), len(best.image)))

	if confidence >= 0.7 {
		// Reconstruct image reference with corrected name
		correctedImage := strings.Replace(imageName, baseName, best.image, 1)
		if tag != "" {
			return &TypoSuggestion{
				Original:   fmt.Sprintf("%s:%s", imageName, tag),
				Suggested:  fmt.Sprintf("%s:%s", correctedImage, tag),
				Confidence: confidence,
				Reason:     fmt.Sprintf("Image name '%s' looks like a typo for '%s'", baseName, best.image),
			}
		}
		return &TypoSuggestion{
			Original:   imageName,
			Suggested:  correctedImage,
			Confidence: confidence,
			Reason:     fmt.Sprintf("Image name '%s' looks like a typo for '%s'", baseName, best.image),
		}
	}

	return nil
}

// parseImageReference splits an image reference into name and tag
func parseImageReference(ref string) (name, tag string) {
	// Handle digest format (image@sha256:...)
	if idx := strings.Index(ref, "@"); idx != -1 {
		return ref[:idx], ref[idx+1:]
	}

	// Handle tag format (image:tag)
	// Be careful with registry ports (e.g., registry:5000/image:tag)
	parts := strings.Split(ref, "/")
	lastPart := parts[len(parts)-1]

	if idx := strings.LastIndex(lastPart, ":"); idx != -1 {
		tag = lastPart[idx+1:]
		parts[len(parts)-1] = lastPart[:idx]
		name = strings.Join(parts, "/")
		return
	}

	return ref, ""
}

// extractBaseName gets the image name without registry/organization
func extractBaseName(imageName string) string {
	parts := strings.Split(imageName, "/")
	return parts[len(parts)-1]
}

// levenshteinDistance calculates the edit distance between two strings
func levenshteinDistance(s1, s2 string) int {
	s1 = strings.ToLower(s1)
	s2 = strings.ToLower(s2)

	if s1 == s2 {
		return 0
	}
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	// Create matrix
	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
		matrix[i][0] = i
	}
	for j := 0; j <= len(s2); j++ {
		matrix[0][j] = j
	}

	// Fill matrix
	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 1
			if s1[i-1] == s2[j-1] {
				cost = 0
			}
			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(s1)][len(s2)]
}

// =============================================================================
// Image Patch Action
// =============================================================================

// ActionPatchImage action type constant
const ActionPatchImage = "kubernetes.patch_image"

// PatchImageAction corrects a container image in a Kubernetes deployment
type PatchImageAction struct {
	*BaseAction
	namespace      string
	deploymentName string
	containerName  string
	newImage       string
	oldImage       string
}

// NewPatchImageAction creates a new action to patch a container image
func NewPatchImageAction(namespace, deploymentName, containerName, newImage, oldImage string, logger *logrus.Logger) *PatchImageAction {
	if namespace == "" {
		namespace = "default"
	}

	actionID := fmt.Sprintf("%s.%s.%s.%s", ActionPatchImage, namespace, deploymentName, containerName)

	return &PatchImageAction{
		BaseAction: NewBaseAction(
			actionID,
			fmt.Sprintf("Fix image for %s/%s/%s", namespace, deploymentName, containerName),
			fmt.Sprintf("Updates container '%s' in deployment '%s' to use image '%s'", containerName, deploymentName, newImage),
			CategoryService,
			RiskSafe, // Image typo fixes are safe
			true,     // Can revert to old image
			fmt.Sprintf("Container image will change from '%s' to '%s'", oldImage, newImage),
			logger,
		),
		namespace:      namespace,
		deploymentName: deploymentName,
		containerName:  containerName,
		newImage:       newImage,
		oldImage:       oldImage,
	}
}

// NewPatchImageActionFactory returns a factory for PatchImageAction
func NewPatchImageActionFactory() ActionFactory {
	return func(params map[string]interface{}, logger *logrus.Logger) (Action, error) {
		namespace, _ := params["namespace"].(string)
		deploymentName, ok1 := params["deployment_name"].(string)
		containerName, ok2 := params["container_name"].(string)
		newImage, ok3 := params["new_image"].(string)
		oldImage, _ := params["old_image"].(string)

		if !ok1 || deploymentName == "" {
			return nil, fmt.Errorf("deployment_name is required")
		}
		if !ok2 || containerName == "" {
			return nil, fmt.Errorf("container_name is required")
		}
		if !ok3 || newImage == "" {
			return nil, fmt.Errorf("new_image is required")
		}

		return NewPatchImageAction(namespace, deploymentName, containerName, newImage, oldImage, logger), nil
	}
}

// Validate checks if kubectl is available and deployment exists
func (a *PatchImageAction) Validate(ctx context.Context, executor diagnostics.CommandExecutor) error {
	if err := a.BaseAction.Validate(ctx, executor); err != nil {
		return err
	}

	// Check kubectl is available
	if _, _, exitCode, _ := executor.ExecuteWithContext(ctx, "which kubectl"); exitCode != 0 {
		return fmt.Errorf("kubectl not found")
	}

	// Check deployment exists
	cmd := fmt.Sprintf("kubectl get deployment %s -n %s -o name",
		shellQuote(a.deploymentName),
		shellQuote(a.namespace))

	if _, stderr, exitCode, _ := executor.ExecuteWithContext(ctx, cmd); exitCode != 0 {
		return fmt.Errorf("deployment %s not found in namespace %s: %s", a.deploymentName, a.namespace, stderr)
	}

	return nil
}

// Execute patches the container image
func (a *PatchImageAction) Execute(ctx context.Context, executor diagnostics.CommandExecutor) (*ActionResult, error) {
	result := &ActionResult{
		ActionID:  a.ID(),
		StartTime: time.Now(),
		RollbackData: map[string]interface{}{
			"old_image":       a.oldImage,
			"deployment_name": a.deploymentName,
			"container_name":  a.containerName,
			"namespace":       a.namespace,
		},
	}

	cmd := fmt.Sprintf("kubectl set image deployment/%s %s=%s -n %s",
		shellQuote(a.deploymentName),
		shellQuote(a.containerName),
		shellQuote(a.newImage),
		shellQuote(a.namespace))

	stdout, stderr, exitCode, err := executor.ExecuteWithContext(ctx, cmd)
	result.Output = fmt.Sprintf("stdout: %s\nstderr: %s", stdout, stderr)

	if err != nil || exitCode != 0 {
		result.Status = StatusFailed
		result.Message = fmt.Sprintf("Failed to patch image for %s/%s", a.namespace, a.deploymentName)
		result.Error = fmt.Sprintf("exit code %d: %s", exitCode, stderr)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, fmt.Errorf("kubectl set image failed: %v", err)
	}

	result.Status = StatusSuccess
	result.Message = fmt.Sprintf("Successfully updated image to %s", a.newImage)
	result.ChangesApplied = []string{
		fmt.Sprintf("Changed %s/%s container %s image from %s to %s",
			a.namespace, a.deploymentName, a.containerName, a.oldImage, a.newImage),
	}
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result, nil
}

// Rollback reverts to the original image
func (a *PatchImageAction) Rollback(ctx context.Context, executor diagnostics.CommandExecutor, result *ActionResult) error {
	if a.oldImage == "" {
		return fmt.Errorf("no old image to rollback to")
	}

	cmd := fmt.Sprintf("kubectl set image deployment/%s %s=%s -n %s",
		shellQuote(a.deploymentName),
		shellQuote(a.containerName),
		shellQuote(a.oldImage),
		shellQuote(a.namespace))

	if _, stderr, exitCode, err := executor.ExecuteWithContext(ctx, cmd); err != nil || exitCode != 0 {
		return fmt.Errorf("failed to rollback image: %s", stderr)
	}

	return nil
}
