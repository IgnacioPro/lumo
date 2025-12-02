package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/api/response"
	"github.com/ignacio/lumo/internal/correlation"
)

// IncidentsHandler handles incident-related requests
type IncidentsHandler struct {
	engine *correlation.Engine
	repo   *correlation.InMemoryIncidentRepository
	logger *logrus.Logger
}

// NewIncidentsHandler creates a new incidents handler
func NewIncidentsHandler(
	engine *correlation.Engine,
	repo *correlation.InMemoryIncidentRepository,
	logger *logrus.Logger,
) *IncidentsHandler {
	return &IncidentsHandler{
		engine: engine,
		repo:   repo,
		logger: logger,
	}
}

// ListIncidents handles GET /api/v1/incidents
func (h *IncidentsHandler) ListIncidents(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	filters := make(map[string]interface{})

	// State filter
	if state := r.URL.Query().Get("state"); state != "" {
		filters["state"] = correlation.IncidentState(state)
	}

	// Category filter
	if category := r.URL.Query().Get("category"); category != "" {
		filters["category"] = correlation.IncidentCategory(category)
	}

	// Severity filter
	if severity := r.URL.Query().Get("severity"); severity != "" {
		// Convert to models.EventSeverity if needed
		filters["severity"] = severity
	}

	// List incidents from repository
	incidents, err := h.repo.List(r.Context(), filters)
	if err != nil {
		h.logger.WithError(err).Error("Failed to list incidents")
		response.InternalServerError(w, "Failed to list incidents")
		return
	}

	// Also include open incidents from the engine
	openIncidents := h.engine.GetOpenIncidents()

	// Build response
	resp := map[string]interface{}{
		"incidents":      incidents,
		"open_incidents": openIncidents,
		"total_count":    len(incidents),
		"open_count":     len(openIncidents),
	}

	response.Success(w, resp)
}

// GetIncident handles GET /api/v1/incidents/{id}
func (h *IncidentsHandler) GetIncident(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		response.BadRequest(w, "Incident ID is required")
		return
	}

	incidentID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(w, "Invalid incident ID format")
		return
	}

	incident, err := h.repo.GetByID(r.Context(), incidentID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			response.NotFound(w, "Incident not found")
			return
		}
		h.logger.WithError(err).Error("Failed to get incident")
		response.InternalServerError(w, "Failed to retrieve incident")
		return
	}

	response.Success(w, incident)
}

// GetIncidentAnalysis handles GET /api/v1/incidents/{id}/analysis
// Returns a beautiful HTML page with the full AI analysis
func (h *IncidentsHandler) GetIncidentAnalysis(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		response.BadRequest(w, "Incident ID is required")
		return
	}

	incidentID, err := uuid.Parse(idStr)
	if err != nil {
		response.BadRequest(w, "Invalid incident ID format")
		return
	}

	incident, err := h.repo.GetByID(r.Context(), incidentID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			response.NotFound(w, "Incident not found")
			return
		}
		h.logger.WithError(err).Error("Failed to get incident")
		response.InternalServerError(w, "Failed to retrieve incident")
		return
	}

	// Generate HTML page
	html := h.generateAnalysisHTML(incident)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(html))
}

// GetOpenIncidents handles GET /api/v1/incidents/open
func (h *IncidentsHandler) GetOpenIncidents(w http.ResponseWriter, r *http.Request) {
	openIncidents := h.engine.GetOpenIncidents()

	resp := map[string]interface{}{
		"incidents": openIncidents,
		"count":     len(openIncidents),
	}

	response.Success(w, resp)
}

// GetIncidentStats handles GET /api/v1/incidents/stats
func (h *IncidentsHandler) GetIncidentStats(w http.ResponseWriter, r *http.Request) {
	stats := map[string]interface{}{
		"open_incidents":     h.engine.GetOpenIncidentCount(),
		"total_in_memory":    h.repo.Count(r.Context()),
		"open_in_repo":       h.repo.CountByState(r.Context(), correlation.IncidentStateOpen),
		"resolved_in_repo":   h.repo.CountByState(r.Context(), correlation.IncidentStateResolved),
		"analyzing_in_repo":  h.repo.CountByState(r.Context(), correlation.IncidentStateAnalyzing),
		"suppressed_in_repo": h.repo.CountByState(r.Context(), correlation.IncidentStateSuppressed),
	}

	response.Success(w, stats)
}

// generateAnalysisHTML creates a beautiful HTML page for incident analysis
func (h *IncidentsHandler) generateAnalysisHTML(incident *correlation.Incident) string {
	// Severity color
	severityColor := map[string]string{
		"critical": "#dc2626",
		"high":     "#ea580c",
		"medium":   "#ca8a04",
		"low":      "#2563eb",
	}
	color := severityColor[string(incident.Severity)]
	if color == "" {
		color = "#6b7280"
	}

	// Category emoji
	categoryEmoji := map[correlation.IncidentCategory]string{
		correlation.CategoryMemory:     "💾",
		correlation.CategoryCrash:      "💥",
		correlation.CategoryImage:      "🖼",
		correlation.CategoryStorage:    "💿",
		correlation.CategoryNode:       "🖥",
		correlation.CategoryScheduling: "📋",
		correlation.CategoryDeployment: "🚀",
		correlation.CategoryNetwork:    "🌐",
		correlation.CategoryUnknown:    "❓",
	}
	emoji := categoryEmoji[incident.Category]
	if emoji == "" {
		emoji = "📋"
	}

	// Build timeline HTML
	var timelineHTML strings.Builder
	for i, entry := range incident.Timeline {
		if i >= 10 {
			timelineHTML.WriteString("<li style='color: #6b7280;'>... and " + strconv.Itoa(len(incident.Timeline)-10) + " more events</li>")
			break
		}
		timelineHTML.WriteString("<li><code>" + entry.Timestamp.Format("15:04:05") + "</code> ")
		timelineHTML.WriteString("<strong>" + entry.EventType + "</strong> on " + entry.Resource)
		timelineHTML.WriteString("<br><small style='color: #6b7280;'>" + entry.Message + "</small></li>")
	}

	// Build AI analysis HTML
	aiAnalysisHTML := "<p style='color: #6b7280;'>AI analysis not available</p>"
	if incident.AIAnalysis != nil {
		aiAnalysisHTML = "<div class='ai-analysis'>"
		aiAnalysisHTML += "<h3>Root Cause</h3>"
		aiAnalysisHTML += "<p>" + incident.AIAnalysis.RootCause.Explanation + "</p>"

		if len(incident.AIAnalysis.ImmediateActions) > 0 {
			aiAnalysisHTML += "<h3>Immediate Actions</h3><ol>"
			for _, action := range incident.AIAnalysis.ImmediateActions {
				aiAnalysisHTML += "<li><strong>" + action.Title + "</strong>"
				if action.Command != "" {
					aiAnalysisHTML += "<pre><code>" + action.Command + "</code></pre>"
				}
				aiAnalysisHTML += "</li>"
			}
			aiAnalysisHTML += "</ol>"
		}

		if len(incident.AIAnalysis.MonitoringRecommendations) > 0 {
			aiAnalysisHTML += "<h3>Monitoring Recommendations</h3><ul>"
			for _, rec := range incident.AIAnalysis.MonitoringRecommendations {
				aiAnalysisHTML += "<li>" + rec + "</li>"
			}
			aiAnalysisHTML += "</ul>"
		}

		aiAnalysisHTML += "</div>"
	}

	// Build affected resources HTML
	var resourcesHTML strings.Builder
	for _, resource := range incident.AffectedResources {
		resourcesHTML.WriteString("<span class='resource-badge'>" + resource.Kind + "/" + resource.Name + "</span> ")
	}

	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>` + incident.Title + ` - Lumo Incident Analysis</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            line-height: 1.6;
            color: #1f2937;
            background: #f3f4f6;
        }
        .header {
            background: linear-gradient(135deg, ` + color + ` 0%, ` + color + `dd 100%);
            color: white;
            padding: 2rem;
            text-align: center;
        }
        .header h1 { font-size: 1.5rem; margin-bottom: 0.5rem; }
        .header .meta { font-size: 0.9rem; opacity: 0.9; }
        .container { max-width: 900px; margin: 0 auto; padding: 2rem; }
        .card {
            background: white;
            border-radius: 12px;
            padding: 1.5rem;
            margin-bottom: 1.5rem;
            box-shadow: 0 1px 3px rgba(0,0,0,0.1);
        }
        .card h2 { color: #374151; margin-bottom: 1rem; font-size: 1.2rem; }
        .stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 1rem; }
        .stat { text-align: center; padding: 1rem; background: #f9fafb; border-radius: 8px; }
        .stat-value { font-size: 1.5rem; font-weight: bold; color: ` + color + `; }
        .stat-label { font-size: 0.85rem; color: #6b7280; }
        .timeline { list-style: none; }
        .timeline li { padding: 0.75rem 0; border-bottom: 1px solid #e5e7eb; }
        .timeline li:last-child { border-bottom: none; }
        .timeline code { background: #f3f4f6; padding: 0.2rem 0.4rem; border-radius: 4px; font-size: 0.85rem; }
        .resource-badge {
            display: inline-block;
            background: #e0e7ff;
            color: #3730a3;
            padding: 0.25rem 0.5rem;
            border-radius: 4px;
            font-size: 0.85rem;
            margin: 0.25rem;
        }
        .ai-analysis h3 { color: #374151; margin: 1rem 0 0.5rem; font-size: 1rem; }
        .ai-analysis pre {
            background: #1f2937;
            color: #10b981;
            padding: 1rem;
            border-radius: 8px;
            overflow-x: auto;
            font-size: 0.85rem;
        }
        .ai-analysis ul, .ai-analysis ol { padding-left: 1.5rem; }
        .ai-analysis li { margin: 0.5rem 0; }
        .footer { text-align: center; padding: 2rem; color: #6b7280; font-size: 0.85rem; }
    </style>
</head>
<body>
    <div class="header">
        <h1>` + emoji + ` ` + incident.Title + `</h1>
        <div class="meta">
            Incident ID: ` + incident.ID.String()[:8] + ` | 
            Duration: ` + incident.Duration().String() + ` |
            State: ` + string(incident.State) + `
        </div>
    </div>
    <div class="container">
        <div class="card">
            <h2>📊 Overview</h2>
            <div class="stats">
                <div class="stat">
                    <div class="stat-value">` + strconv.Itoa(len(incident.Events)) + `</div>
                    <div class="stat-label">Events</div>
                </div>
                <div class="stat">
                    <div class="stat-value">` + strconv.Itoa(len(incident.AffectedResources)) + `</div>
                    <div class="stat-label">Resources</div>
                </div>
                <div class="stat">
                    <div class="stat-value">` + string(incident.Severity) + `</div>
                    <div class="stat-label">Severity</div>
                </div>
                <div class="stat">
                    <div class="stat-value">` + string(incident.Category) + `</div>
                    <div class="stat-label">Category</div>
                </div>
            </div>
        </div>
        <div class="card">
            <h2>🎯 Affected Resources</h2>
            <div>` + resourcesHTML.String() + `</div>
        </div>
        <div class="card">
            <h2>📅 Timeline</h2>
            <ul class="timeline">` + timelineHTML.String() + `</ul>
        </div>
        <div class="card">
            <h2>🤖 AI Analysis</h2>
            ` + aiAnalysisHTML + `
        </div>
    </div>
    <div class="footer">
        Generated by Lumo Incident Intelligence Platform<br>
        ` + incident.OpenedAt.Format("2006-01-02 15:04:05 MST") + `
    </div>
</body>
</html>`

	return html
}
