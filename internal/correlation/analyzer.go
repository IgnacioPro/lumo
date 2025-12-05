package correlation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/ai"
)

// AIIncidentAnalyzer implements AIAnalyzer using the AI provider
type AIIncidentAnalyzer struct {
	provider ai.Provider
	logger   *logrus.Entry
}

// NewAIIncidentAnalyzer creates a new AI incident analyzer
func NewAIIncidentAnalyzer(provider ai.Provider, logger *logrus.Logger) *AIIncidentAnalyzer {
	return &AIIncidentAnalyzer{
		provider: provider,
		logger:   logger.WithField("component", "incident-analyzer"),
	}
}

// Analyze generates AI-powered analysis for an incident
func (a *AIIncidentAnalyzer) Analyze(ctx context.Context, incident *Incident) (*AIAnalysisResult, error) {
	startTime := time.Now()

	a.logger.WithFields(logrus.Fields{
		"incident_id": incident.ID,
		"category":    incident.Category,
		"event_count": len(incident.Events),
	}).Info("Starting AI analysis of incident")

	// Build the prompt
	prompt := a.buildAnalysisPrompt(incident)
	systemPrompt := a.getSystemPrompt()

	// Call AI provider
	response, tokenUsage, err := a.provider.Ask(ctx, systemPrompt, prompt)
	if err != nil {
		return nil, fmt.Errorf("AI analysis failed: %w", err)
	}

	// Extract token count
	tokensUsed := 0
	if tokenUsage != nil {
		tokensUsed = tokenUsage.TotalTokens
	}

	// Parse the response into structured format
	result := a.parseAnalysisResponse(response, tokensUsed)
	result.AnalysisDuration = time.Since(startTime)

	a.logger.WithFields(logrus.Fields{
		"incident_id": incident.ID,
		"duration":    result.AnalysisDuration,
		"tokens_used": result.TokensUsed,
		"confidence":  result.Confidence,
	}).Info("AI analysis completed")

	return result, nil
}

// getSystemPrompt returns the system prompt for incident analysis
func (a *AIIncidentAnalyzer) getSystemPrompt() string {
	return `You are an expert Kubernetes SRE with deep knowledge of container orchestration, distributed systems, and incident response. You are analyzing a correlated incident - multiple related events that together tell the story of what went wrong.

CRITICAL: You MUST determine a root cause. Never say "could not be determined" if ANY events are provided.

Your analysis approach:
1. Look at the event messages - they often contain the EXACT reason for failure
2. For scheduling-failed events: Parse the message to identify specific constraints (taints, resources, node selectors)
3. For image-pull-backoff: Extract the image name and identify if it's a typo, auth issue, or missing image
4. For OOMKilled: Note the memory limits and suggest specific values
5. For crash-loop: Identify the pattern and likely cause (config, deps, startup failure)

Your analysis should:
1. Identify the ROOT CAUSE from the event data - be specific, cite the event messages
2. Explain the CHAIN OF EVENTS - how one failure led to others
3. Provide ACTIONABLE remediation with specific kubectl commands that can be copy-pasted
4. Consider the BUSINESS IMPACT
5. Suggest preventive measures for the future

IMPORTANT FORMATTING:
- Use plain text, not Slack markdown (no *asterisks* for bold)
- Use "**text**" for bold (Markdown)
- Keep commands in code blocks with triple backticks
- Be concise - each section should be 2-4 sentences max
- Always provide at least one kubectl command in "Immediate Actions"

Example root cause for scheduling failure:
"The pod cannot be scheduled because node 'worker-1' has taint 'node-role.kubernetes.io/control-plane:NoSchedule' and the pod lacks a matching toleration. This is blocking all deployments to this single-node cluster."

Example root cause for image pull:
"Image 'nginxx:latest' (note the typo - double 'x') does not exist in the registry. The image name should be 'nginx:latest'."`
}

// buildAnalysisPrompt builds a comprehensive prompt for incident analysis
func (a *AIIncidentAnalyzer) buildAnalysisPrompt(incident *Incident) string {
	var prompt strings.Builder

	// Incident Overview
	prompt.WriteString("# Incident Analysis Request\n\n")
	prompt.WriteString("## Incident Overview\n")
	prompt.WriteString(fmt.Sprintf("- **Category**: %s\n", incident.Category))
	prompt.WriteString(fmt.Sprintf("- **Severity**: %s\n", incident.Severity))
	prompt.WriteString(fmt.Sprintf("- **Duration**: %s\n", incident.Duration().Round(time.Second)))
	prompt.WriteString(fmt.Sprintf("- **Event Count**: %d\n", len(incident.Events)))
	prompt.WriteString(fmt.Sprintf("- **Affected Resources**: %d\n", len(incident.AffectedResources)))
	if incident.Namespace != nil {
		prompt.WriteString(fmt.Sprintf("- **Namespace**: %s\n", *incident.Namespace))
	}
	prompt.WriteString("\n")

	// Timeline of Events
	prompt.WriteString("## Event Timeline\n\n")
	prompt.WriteString("| Time | Type | Resource | Severity | Message |\n")
	prompt.WriteString("|------|------|----------|----------|--------|\n")
	for _, entry := range incident.Timeline {
		// Truncate message for table
		msg := entry.Message
		if len(msg) > 80 {
			msg = msg[:77] + "..."
		}
		prompt.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
			entry.Timestamp.Format("15:04:05"),
			entry.EventType,
			entry.Resource,
			entry.Severity,
			msg))
	}
	prompt.WriteString("\n")

	// Affected Resources
	prompt.WriteString("## Affected Resources\n\n")
	for _, resource := range incident.AffectedResources {
		prompt.WriteString(fmt.Sprintf("- **%s/%s**", resource.Kind, resource.Name))
		if resource.Namespace != "" {
			prompt.WriteString(fmt.Sprintf(" (namespace: %s)", resource.Namespace))
		}
		prompt.WriteString("\n")
		if len(resource.OwnerChain) > 0 {
			prompt.WriteString("  - Owner chain: ")
			for i, owner := range resource.OwnerChain {
				if i > 0 {
					prompt.WriteString(" → ")
				}
				prompt.WriteString(fmt.Sprintf("%s/%s", owner.Kind, owner.Name))
			}
			prompt.WriteString("\n")
		}
	}
	prompt.WriteString("\n")

	// Context Information
	if incident.Context != nil {
		a.addContextToPrompt(&prompt, incident.Context)
	}

	// Specific event details
	prompt.WriteString("## Event Details\n\n")
	for i, event := range incident.Events {
		if i >= 10 { // Limit to first 10 events for token efficiency
			prompt.WriteString(fmt.Sprintf("\n*... and %d more events*\n", len(incident.Events)-10))
			break
		}
		prompt.WriteString(fmt.Sprintf("### Event %d: %s\n", i+1, event.EventType))
		prompt.WriteString(fmt.Sprintf("- **Message**: %s\n", event.Message))
		prompt.WriteString(fmt.Sprintf("- **Resource**: %s/%s\n", event.ResourceKind, event.ResourceName))
		prompt.WriteString(fmt.Sprintf("- **Severity**: %s\n", event.Severity))

		// Add relevant metadata
		if event.Metadata != nil {
			if restarts, ok := event.Metadata["restart_count"].(float64); ok {
				prompt.WriteString(fmt.Sprintf("- **Restart Count**: %.0f\n", restarts))
			}
			if exitCode, ok := event.Metadata["exit_code"].(float64); ok {
				prompt.WriteString(fmt.Sprintf("- **Exit Code**: %.0f\n", exitCode))
			}
			if image, ok := event.Metadata["image"].(string); ok {
				prompt.WriteString(fmt.Sprintf("- **Image**: %s\n", image))
			}
			if nodeName, ok := event.Metadata["node_name"].(string); ok {
				prompt.WriteString(fmt.Sprintf("- **Node**: %s\n", nodeName))
			}
			if reason, ok := event.Metadata["reason"].(string); ok {
				prompt.WriteString(fmt.Sprintf("- **Reason**: %s\n", reason))
			}
		}
		prompt.WriteString("\n")
	}

	// Analysis Request
	prompt.WriteString("## Analysis Required\n\n")
	prompt.WriteString("Please provide a structured analysis with the following sections:\n\n")
	prompt.WriteString("### 1. Root Cause\n")
	prompt.WriteString("Identify the underlying root cause of this incident. Explain what triggered the chain of events.\n\n")
	prompt.WriteString("### 2. Impact Assessment\n")
	prompt.WriteString("Evaluate the impact on users, services, and data. Is there data loss? Service degradation? Complete outage?\n\n")
	prompt.WriteString("### 3. Immediate Actions\n")
	prompt.WriteString("Provide 3-5 immediate remediation steps with specific kubectl commands or configuration changes. Priority order.\n\n")
	prompt.WriteString("### 4. Long-term Prevention\n")
	prompt.WriteString("Suggest architectural or configuration changes to prevent recurrence.\n\n")
	prompt.WriteString("### 5. Monitoring Recommendations\n")
	prompt.WriteString("What alerts or metrics should be added to catch this earlier?\n\n")

	return prompt.String()
}

// addContextToPrompt adds gathered context to the prompt
func (a *AIIncidentAnalyzer) addContextToPrompt(prompt *strings.Builder, ctx *IncidentContext) {
	// Error Patterns (most valuable for root cause)
	if len(ctx.ErrorPatterns) > 0 {
		prompt.WriteString("## Detected Error Patterns\n\n")
		for _, pattern := range ctx.ErrorPatterns {
			fmt.Fprintf(prompt, "### %s (found %d times in %d containers)\n",
				pattern.Pattern, pattern.Occurrences, len(pattern.Containers))
			prompt.WriteString("Sample errors:\n")
			for i, sample := range pattern.Samples {
				if i >= 2 { // Limit samples
					break
				}
				// Truncate long samples
				if len(sample) > 200 {
					sample = sample[:197] + "..."
				}
				fmt.Fprintf(prompt, "- `%s`\n", sample)
			}
			prompt.WriteString("\n")
		}
	}

	// Related Kubernetes Events
	if len(ctx.RelatedK8sEvents) > 0 {
		prompt.WriteString("## Related Kubernetes Events\n\n")
		for i, event := range ctx.RelatedK8sEvents {
			if i >= 10 { // Limit events
				fmt.Fprintf(prompt, "*... and %d more events*\n", len(ctx.RelatedK8sEvents)-10)
				break
			}
			fmt.Fprintf(prompt, "- **%s** on %s: %s (count: %d)\n",
				event.Reason, event.Resource, event.Message, event.Count)
		}
		prompt.WriteString("\n")
	}

	// Node Conditions
	if len(ctx.NodeConditions) > 0 {
		prompt.WriteString("## Node Conditions\n\n")
		for nodeName, conditions := range ctx.NodeConditions {
			fmt.Fprintf(prompt, "### Node: %s\n", nodeName)
			for _, cond := range conditions {
				status := "✅"
				if cond.Status != "True" && cond.Type == "Ready" {
					status = "❌"
				} else if cond.Status == "True" && cond.Type != "Ready" {
					status = "⚠️"
				}
				fmt.Fprintf(prompt, "- %s %s: %s\n", status, cond.Type, cond.Status)
				if cond.Message != "" {
					fmt.Fprintf(prompt, "  - %s\n", cond.Message)
				}
			}
			prompt.WriteString("\n")
		}
	}

	// Resource Metrics
	if ctx.Metrics != nil {
		if len(ctx.Metrics.NodeMetrics) > 0 || len(ctx.Metrics.PodMetrics) > 0 {
			prompt.WriteString("## Resource Metrics\n\n")

			if len(ctx.Metrics.NodeMetrics) > 0 {
				prompt.WriteString("### Node Metrics\n")
				for nodeName, metrics := range ctx.Metrics.NodeMetrics {
					fmt.Fprintf(prompt, "- **%s**: CPU %.1f%%, Memory %s\n",
						nodeName,
						metrics.CPUUsagePercent,
						formatBytes(metrics.MemoryUsageBytes))
				}
				prompt.WriteString("\n")
			}

			if len(ctx.Metrics.PodMetrics) > 0 {
				prompt.WriteString("### Pod Metrics\n")
				for podName, metrics := range ctx.Metrics.PodMetrics {
					fmt.Fprintf(prompt, "- **%s**: CPU %.3f cores, Memory %s\n",
						podName,
						metrics.CPUUsageCores,
						formatBytes(metrics.MemoryUsageBytes))
				}
				prompt.WriteString("\n")
			}
		}
	}

	// Recent log excerpts (limited to save tokens)
	if len(ctx.PodLogs) > 0 {
		prompt.WriteString("## Recent Error Logs (excerpts)\n\n")
		logCount := 0
		for podKey, logs := range ctx.PodLogs {
			if logCount >= 20 { // Limit total log lines
				break
			}
			// Only include error logs
			errorLogs := make([]LogEntry, 0)
			for _, log := range logs {
				if log.Level == "error" || log.Level == "warn" {
					errorLogs = append(errorLogs, log)
				}
			}
			if len(errorLogs) == 0 {
				continue
			}

			fmt.Fprintf(prompt, "### %s\n```\n", podKey)
			for i, log := range errorLogs {
				if logCount >= 20 || i >= 5 {
					break
				}
				msg := log.Message
				if len(msg) > 200 {
					msg = msg[:197] + "..."
				}
				fmt.Fprintf(prompt, "%s\n", msg)
				logCount++
			}
			prompt.WriteString("```\n\n")
		}
	}
}

// parseAnalysisResponse parses the AI response into structured format
func (a *AIIncidentAnalyzer) parseAnalysisResponse(response string, tokensUsed int) *AIAnalysisResult {
	result := &AIAnalysisResult{
		FullAnalysis:              response,
		TokensUsed:                tokensUsed,
		ImmediateActions:          make([]RecommendedAction, 0),
		LongTermActions:           make([]RecommendedAction, 0),
		MonitoringRecommendations: make([]string, 0),
		Confidence:                75, // Default confidence
	}

	// Extract sections from the response
	sections := parseSections(response)

	// Root Cause
	if rootCause, ok := sections["Root Cause"]; ok {
		result.RootCause = RootCauseAnalysis{
			Summary:     extractFirstSentence(rootCause),
			Explanation: rootCause,
			Confidence:  80,
		}
	}

	// Impact Assessment
	if impact, ok := sections["Impact Assessment"]; ok {
		result.Impact = ImpactAssessment{
			Severity:        extractSeverity(impact),
			ServiceDegraded: strings.Contains(strings.ToLower(impact), "degraded"),
			ServiceDown:     strings.Contains(strings.ToLower(impact), "down") || strings.Contains(strings.ToLower(impact), "outage"),
			DataLoss:        strings.Contains(strings.ToLower(impact), "data loss"),
		}
	}

	// Immediate Actions
	if actions, ok := sections["Immediate Actions"]; ok {
		result.ImmediateActions = extractActions(actions)
	}

	// Long-term Prevention
	if prevention, ok := sections["Long-term Prevention"]; ok {
		result.LongTermActions = extractActions(prevention)
	}

	// Monitoring Recommendations
	if monitoring, ok := sections["Monitoring Recommendations"]; ok {
		result.MonitoringRecommendations = extractBulletPoints(monitoring)
	}

	return result
}

// parseSections extracts sections from markdown content
func parseSections(content string) map[string]string {
	sections := make(map[string]string)
	lines := strings.Split(content, "\n")

	currentSection := ""
	var currentContent strings.Builder

	for _, line := range lines {
		// Check for section headers (## or ### followed by number and title)
		if strings.HasPrefix(line, "##") || strings.HasPrefix(line, "###") {
			// Save previous section
			if currentSection != "" {
				sections[currentSection] = strings.TrimSpace(currentContent.String())
			}

			// Extract section title
			title := strings.TrimLeft(line, "#")
			title = strings.TrimSpace(title)
			// Remove leading numbers like "1. " or "1: "
			if len(title) > 2 && title[0] >= '1' && title[0] <= '9' && (title[1] == '.' || title[1] == ':') {
				title = strings.TrimSpace(title[2:])
			}
			currentSection = title
			currentContent.Reset()
		} else if currentSection != "" {
			currentContent.WriteString(line)
			currentContent.WriteString("\n")
		}
	}

	// Save last section
	if currentSection != "" {
		sections[currentSection] = strings.TrimSpace(currentContent.String())
	}

	return sections
}

// extractFirstSentence extracts the first sentence from text
func extractFirstSentence(text string) string {
	text = strings.TrimSpace(text)
	// Find first sentence ending
	for i, char := range text {
		if char == '.' || char == '!' || char == '?' {
			if i < len(text)-1 && text[i+1] == ' ' {
				return text[:i+1]
			}
		}
	}
	// If no sentence ending found, return first 200 chars
	if len(text) > 200 {
		return text[:200] + "..."
	}
	return text
}

// extractSeverity extracts severity level from impact text
func extractSeverity(text string) string {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "critical") || strings.Contains(lower, "complete outage") {
		return "critical"
	}
	if strings.Contains(lower, "high") || strings.Contains(lower, "significant") {
		return "high"
	}
	if strings.Contains(lower, "medium") || strings.Contains(lower, "moderate") {
		return "medium"
	}
	return "low"
}

// extractActions extracts recommended actions from text
func extractActions(text string) []RecommendedAction {
	actions := make([]RecommendedAction, 0)
	lines := strings.Split(text, "\n")

	var currentAction *RecommendedAction
	priority := 1

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check for action item (numbered or bulleted)
		if len(line) > 2 && (line[0] == '-' || line[0] == '*' || (line[0] >= '1' && line[0] <= '9')) {
			// Save previous action
			if currentAction != nil {
				actions = append(actions, *currentAction)
			}

			// Parse action line
			actionText := strings.TrimLeft(line, "-*0123456789.) ")
			currentAction = &RecommendedAction{
				Title:       extractFirstSentence(actionText),
				Description: actionText,
				Priority:    priority,
			}
			priority++
		} else if strings.HasPrefix(line, "```") {
			// Start/end of code block
			continue
		} else if currentAction != nil && !strings.HasPrefix(line, "```") {
			// Check if this is a command
			if strings.HasPrefix(line, "kubectl") || strings.HasPrefix(line, "helm") || strings.HasPrefix(line, "docker") {
				currentAction.Command = line
			} else {
				// Append to description
				currentAction.Description += " " + line
			}
		}
	}

	// Save last action
	if currentAction != nil {
		actions = append(actions, *currentAction)
	}

	return actions
}

// extractBulletPoints extracts bullet points from text
func extractBulletPoints(text string) []string {
	points := make([]string, 0)
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) > 2 && (line[0] == '-' || line[0] == '*') {
			point := strings.TrimLeft(line, "-* ")
			if point != "" {
				points = append(points, point)
			}
		}
	}

	return points
}

// formatBytes formats bytes into human-readable format
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
