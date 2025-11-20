package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/diagnostics"
	"github.com/ignacio/lumo/internal/diagnostics/formatters"
	"github.com/ignacio/lumo/internal/intelligence/vectorstore"
)

// PromptBuilder constructs prompts for AI analysis of diagnostic results.
type PromptBuilder struct {
	includeThinking bool
	focusAreas      []string
	useTOON         bool // Use TOON format for diagnostic data (30-60% token reduction)

	// RAG integration
	ragEnabled  bool
	vectorStore vectorstore.VectorStore
	similarityK int
	minScore    float32
	log         *logrus.Logger
}

// NewPromptBuilder creates a new prompt builder.
// By default, uses TOON format for 30-60% token reduction.
func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{
		includeThinking: true,
		useTOON:         true, // Default to TOON for token efficiency
	}
}

// WithThinking controls whether to include thinking/reasoning in responses.
func (pb *PromptBuilder) WithThinking(include bool) *PromptBuilder {
	pb.includeThinking = include
	return pb
}

// WithFocus sets specific areas to focus analysis on.
func (pb *PromptBuilder) WithFocus(areas ...string) *PromptBuilder {
	pb.focusAreas = areas
	return pb
}

// WithTOON controls whether to use TOON format for diagnostic data.
// TOON achieves 30-60% token reduction compared to JSON/Markdown.
func (pb *PromptBuilder) WithTOON(use bool) *PromptBuilder {
	pb.useTOON = use
	return pb
}

// WithRAG enables RAG (Retrieval Augmented Generation) with historical context.
// Provides similar incidents from vector store to enhance AI analysis.
func (pb *PromptBuilder) WithRAG(store vectorstore.VectorStore, k int, minScore float32, log *logrus.Logger) *PromptBuilder {
	pb.ragEnabled = true
	pb.vectorStore = store
	pb.similarityK = k
	pb.minScore = minScore
	pb.log = log
	return pb
}

// BuildSystemPrompt creates the system prompt that defines the AI's role.
func (pb *PromptBuilder) BuildSystemPrompt() string {
	basePrompt := `You are an expert SRE/DevOps engineer analyzing system diagnostics. Your role is to:

1. Analyze diagnostic data from remote systems
2. Identify performance issues, resource constraints, and potential failures
3. Provide actionable recommendations with specific commands
4. Assess risk levels for proposed changes
5. Prioritize findings based on severity and impact

Guidelines:
- Be specific and technical in your analysis
- Always provide evidence from the diagnostic data
- Recommend concrete commands when applicable
- Assess risk levels honestly (safe, low, moderate, high, critical)
- Prioritize recommendations (critical, high, medium, low)
- Consider system stability when recommending changes
- If data is missing or unclear, note limitations in your analysis

Output Format:
Provide your analysis in JSON format with this structure:
{
  "summary": "Brief overview of system health",
  "overall_health": "healthy|degraded|critical",
  "confidence": 0.0-1.0,
  "findings": [
    {
      "category": "CPU|Memory|Disk|Process|Service|Network",
      "severity": "info|warning|error|critical",
      "title": "Brief finding title",
      "description": "Detailed explanation",
      "evidence": {"key": "value"},
      "related_checks": ["check_name"]
    }
  ],
  "recommendations": [
    {
      "priority": "critical|high|medium|low",
      "title": "Brief recommendation",
      "description": "What to do and why",
      "commands": ["specific commands to run"],
      "risk": "safe|low|moderate|high|critical",
      "estimated_impact": "Expected improvement",
      "related_findings": [0, 1]
    }
  ]
}`

	// Add TOON format explanation if enabled
	if pb.useTOON {
		basePrompt += `

Data Format:
Diagnostic data is provided in TOON (Token-Oriented Object Notation) format for efficiency.
TOON is similar to YAML but optimized for LLMs:
- Uniform arrays use tabular notation: array_name[count]{field1,field2,...}:
- Each row is comma-separated values
- Non-uniform data uses YAML-like key: value format
- This format reduces tokens by 30-60% while maintaining clarity

Example TOON:
metrics[3]{name,value,unit}:
  cpu_usage,45.5,percent
  memory_usage,71.2,percent
  disk_usage,82.0,percent

You can parse and analyze TOON data naturally - treat arrays as tables and key-value pairs as structured data.`
	}

	return basePrompt
}

// BuildAnalysisPrompt creates the user prompt with diagnostic data.
func (pb *PromptBuilder) BuildAnalysisPrompt(req *AnalysisRequest) (string, error) {
	return pb.BuildAnalysisPromptWithContext(context.Background(), req)
}

// BuildAnalysisPromptWithContext creates the user prompt with diagnostic data and optional RAG context.
func (pb *PromptBuilder) BuildAnalysisPromptWithContext(ctx context.Context, req *AnalysisRequest) (string, error) {
	var sb strings.Builder

	// Add RAG historical context if enabled
	if pb.ragEnabled && pb.vectorStore != nil {
		similarIncidents, err := pb.retrieveSimilarIncidents(ctx, req.Report)
		if err != nil {
			if pb.log != nil {
				pb.log.WithError(err).Warn("Failed to retrieve similar incidents from RAG")
			}
		} else if len(similarIncidents) > 0 {
			sb.WriteString("# Historical Context (Similar Past Incidents)\n\n")
			sb.WriteString("The following similar incidents were found in the history:\n\n")

			for i, match := range similarIncidents {
				sb.WriteString(fmt.Sprintf("### Incident %d (similarity: %.0f%%)\n",
					i+1, match.Score*100))
				sb.WriteString("```\n")
				sb.WriteString(match.Document.Content)
				sb.WriteString("\n```\n\n")

				// Add resolution if available
				if resolution, ok := match.Document.Metadata[vectorstore.MetadataResolution].(string); ok {
					sb.WriteString(fmt.Sprintf("**Resolution:** %s\n\n", resolution))
				}
			}

			sb.WriteString("---\n\n")
		}
	}

	// Add system information
	sb.WriteString("# System Information\n\n")
	sb.WriteString(pb.formatSystemInfo(req.SystemInfo))
	sb.WriteString("\n\n")

	// Add selected checks information if specific checks were requested
	if len(req.SelectedChecks) > 0 {
		sb.WriteString("# Selected Checks\n\n")
		sb.WriteString("**IMPORTANT:** Only the following checks were requested:\n")
		for _, check := range req.SelectedChecks {
			sb.WriteString(fmt.Sprintf("- %s\n", check))
		}
		sb.WriteString("\nMissing data for other system areas (CPU, memory, disk, etc.) is EXPECTED and NOT a problem.\n")
		sb.WriteString("Focus your analysis ONLY on the data provided from these selected checks.\n\n")
	}

	// Add diagnostic report summary
	sb.WriteString("# Diagnostic Report\n\n")
	sb.WriteString(pb.formatReportSummary(req.Report))
	sb.WriteString("\n\n")

	// Add detailed results
	sb.WriteString("# Detailed Results\n\n")
	if pb.useTOON {
		// Use TOON format for token efficiency (30-60% reduction)
		sb.WriteString("```toon\n")
		toonFormatter := formatters.NewToonFormatter()
		toonOutput := toonFormatter.FormatReport(req.Report)
		sb.WriteString(toonOutput)
		sb.WriteString("\n```\n")
	} else {
		// Traditional markdown format
		for _, result := range req.Report.Results {
			sb.WriteString(pb.formatCheckResult(result))
			sb.WriteString("\n")
		}
	}

	// Add focus areas if specified
	if len(req.Focus) > 0 {
		sb.WriteString("\n# Focus Areas\n\n")
		sb.WriteString("Please pay special attention to these areas:\n")
		for _, area := range req.Focus {
			sb.WriteString(fmt.Sprintf("- %s\n", area))
		}
		sb.WriteString("\n")
	}

	// Add analysis request
	sb.WriteString("\n# Analysis Request\n\n")
	if pb.ragEnabled {
		sb.WriteString("Please analyze this diagnostic data and provide:\n")
		sb.WriteString("1. Compare with the similar historical incidents provided above (if any)\n")
		sb.WriteString("2. A summary of overall system health\n")
		sb.WriteString("3. Specific findings with evidence from the data\n")
		sb.WriteString("4. Identify if this matches any known patterns\n")
		sb.WriteString("5. Recommend actions based on what worked in the past\n")
		sb.WriteString("6. Highlight any differences that might require a new approach\n\n")
	} else {
		sb.WriteString("Please analyze this diagnostic data and provide:\n")
		sb.WriteString("1. A summary of overall system health\n")
		sb.WriteString("2. Specific findings with evidence from the data\n")
		sb.WriteString("3. Prioritized recommendations with commands\n")
		sb.WriteString("4. Risk assessment for each recommendation\n\n")
	}
	sb.WriteString("Respond in the JSON format specified in your system prompt.\n")

	return sb.String(), nil
}

// retrieveSimilarIncidents queries the vector store for similar historical incidents
func (pb *PromptBuilder) retrieveSimilarIncidents(ctx context.Context, report *diagnostics.Report) ([]*vectorstore.Match, error) {
	if pb.vectorStore == nil {
		return nil, nil
	}

	// Build query from current report (focus on failed/warning checks)
	var queryBuilder strings.Builder

	for _, result := range report.Results {
		if result.Severity == diagnostics.SeverityCritical ||
			result.Severity == diagnostics.SeverityError ||
			result.Severity == diagnostics.SeverityWarning {
			queryBuilder.WriteString(fmt.Sprintf("%s: %s. ", result.Name, result.Message))
		}
	}

	query := queryBuilder.String()
	if query == "" {
		return nil, nil // No issues to query
	}

	// Query vector store
	matches, err := pb.vectorStore.Query(ctx, query, pb.similarityK)
	if err != nil {
		return nil, err
	}

	// Filter by minimum similarity score
	filtered := make([]*vectorstore.Match, 0)
	for _, match := range matches {
		if match.Score >= pb.minScore {
			filtered = append(filtered, match)
		}
	}

	if pb.log != nil && len(filtered) > 0 {
		pb.log.WithFields(logrus.Fields{
			"query":   query[:min(50, len(query))],
			"matches": len(filtered),
		}).Debug("Retrieved similar incidents from RAG")
	}

	return filtered, nil
}

// formatSystemInfo formats system information for the prompt.
func (pb *PromptBuilder) formatSystemInfo(info SystemInfo) string {
	var sb strings.Builder

	if info.Hostname != "" {
		sb.WriteString(fmt.Sprintf("- **Hostname:** %s\n", info.Hostname))
	}
	if info.Platform != "" {
		sb.WriteString(fmt.Sprintf("- **Platform:** %s\n", info.Platform))
	}
	if info.Architecture != "" {
		sb.WriteString(fmt.Sprintf("- **Architecture:** %s\n", info.Architecture))
	}
	if info.KernelVersion != "" {
		sb.WriteString(fmt.Sprintf("- **Kernel:** %s\n", info.KernelVersion))
	}
	if info.UptimeDays > 0 {
		sb.WriteString(fmt.Sprintf("- **Uptime:** %.1f days\n", info.UptimeDays))
	}
	if info.Environment != "" {
		sb.WriteString(fmt.Sprintf("- **Environment:** %s\n", info.Environment))
	}
	if len(info.Tags) > 0 {
		sb.WriteString("- **Tags:** ")
		tags := make([]string, 0, len(info.Tags))
		for k, v := range info.Tags {
			tags = append(tags, fmt.Sprintf("%s=%s", k, v))
		}
		sb.WriteString(strings.Join(tags, ", "))
		sb.WriteString("\n")
	}

	return sb.String()
}

// formatReportSummary formats the diagnostic report summary.
func (pb *PromptBuilder) formatReportSummary(report *diagnostics.Report) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("- **Total Checks:** %d\n", report.Summary.TotalChecks))
	sb.WriteString(fmt.Sprintf("- **OK:** %d\n", report.Summary.OKCount))
	sb.WriteString(fmt.Sprintf("- **Info:** %d\n", report.Summary.InfoCount))
	sb.WriteString(fmt.Sprintf("- **Warnings:** %d\n", report.Summary.WarningCount))
	sb.WriteString(fmt.Sprintf("- **Errors:** %d\n", report.Summary.ErrorCount))
	sb.WriteString(fmt.Sprintf("- **Critical:** %d\n", report.Summary.CriticalCount))
	sb.WriteString(fmt.Sprintf("- **Duration:** %v\n", report.Duration))

	return sb.String()
}

// formatCheckResult formats a single check result.
func (pb *PromptBuilder) formatCheckResult(result *diagnostics.CheckResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## %s\n\n", result.Name))
	sb.WriteString(fmt.Sprintf("- **Status:** %s\n", result.Status))
	sb.WriteString(fmt.Sprintf("- **Severity:** %s\n", result.Severity))

	if result.Message != "" {
		sb.WriteString(fmt.Sprintf("- **Message:** %s\n", result.Message))
	}

	if result.Error != "" {
		sb.WriteString(fmt.Sprintf("- **Error:** %s\n", result.Error))
	}

	// Format metrics
	if len(result.Metrics) > 0 {
		sb.WriteString("\n**Metrics:**\n\n")
		for _, metric := range result.Metrics {
			sb.WriteString(fmt.Sprintf("- `%s`: %v\n", metric.Name, formatMetricValue(metric.Value)))
		}
	}

	return sb.String()
}

// formatMetricValue formats a metric value for display.
func formatMetricValue(value interface{}) string {
	switch v := value.(type) {
	case float64:
		return fmt.Sprintf("%.2f", v)
	case []interface{}:
		// Format arrays (e.g., top processes)
		if len(v) > 0 {
			if bytes, err := json.Marshal(v); err == nil {
				return string(bytes)
			}
		}
		return fmt.Sprintf("%v", v)
	case map[string]interface{}:
		// Format objects
		if bytes, err := json.Marshal(v); err == nil {
			return string(bytes)
		}
		return fmt.Sprintf("%v", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ParseAnalysisResponse parses the AI provider's JSON response.
func ParseAnalysisResponse(content string, provider string, model string) (*AnalysisResponse, error) {
	var rawResponse struct {
		Summary       string  `json:"summary"`
		OverallHealth string  `json:"overall_health"`
		Confidence    float64 `json:"confidence"`
		Findings      []struct {
			Category      string                 `json:"category"`
			Severity      string                 `json:"severity"`
			Title         string                 `json:"title"`
			Description   string                 `json:"description"`
			Evidence      map[string]interface{} `json:"evidence"`
			RelatedChecks []string               `json:"related_checks"`
		} `json:"findings"`
		Recommendations []struct {
			Priority        string   `json:"priority"`
			Title           string   `json:"title"`
			Description     string   `json:"description"`
			Commands        []string `json:"commands"`
			Risk            string   `json:"risk"`
			EstimatedImpact string   `json:"estimated_impact"`
			RelatedFindings []int    `json:"related_findings"`
		} `json:"recommendations"`
	}

	contentLength := len(content)

	// Try to extract JSON from markdown code blocks if present
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	} else if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}

	if err := json.Unmarshal([]byte(content), &rawResponse); err != nil {
		// Create detailed error with content preview
		preview := content
		if len(preview) > 500 {
			preview = preview[:500] + "... [truncated]"
		}

		return nil, fmt.Errorf("failed to parse AI response: %w\nProvider: %s\nModel: %s\nContent length: %d\nContent preview: %s",
			err, provider, model, contentLength, preview)
	}

	// Convert to AnalysisResponse
	response := &AnalysisResponse{
		Summary:       rawResponse.Summary,
		OverallHealth: HealthStatus(rawResponse.OverallHealth),
		Confidence:    rawResponse.Confidence,
		Provider:      provider,
		Model:         model,
	}

	// Convert findings
	response.Findings = make([]Finding, len(rawResponse.Findings))
	for i, f := range rawResponse.Findings {
		response.Findings[i] = Finding{
			Category:      f.Category,
			Severity:      parseSeverity(f.Severity),
			Title:         f.Title,
			Description:   f.Description,
			Evidence:      f.Evidence,
			RelatedChecks: f.RelatedChecks,
		}
	}

	// Convert recommendations
	response.Recommendations = make([]Recommendation, len(rawResponse.Recommendations))
	for i, r := range rawResponse.Recommendations {
		response.Recommendations[i] = Recommendation{
			Priority:        Priority(r.Priority),
			Title:           r.Title,
			Description:     r.Description,
			Commands:        r.Commands,
			Risk:            RiskLevel(r.Risk),
			EstimatedImpact: r.EstimatedImpact,
			RelatedFindings: r.RelatedFindings,
		}
	}

	return response, nil
}

// parseSeverity converts string severity to diagnostics.Severity.
func parseSeverity(s string) diagnostics.Severity {
	switch strings.ToLower(s) {
	case "info":
		return diagnostics.SeverityInfo
	case "warning":
		return diagnostics.SeverityWarning
	case "error":
		return diagnostics.SeverityError
	case "critical":
		return diagnostics.SeverityCritical
	default:
		return diagnostics.SeverityInfo
	}
}
