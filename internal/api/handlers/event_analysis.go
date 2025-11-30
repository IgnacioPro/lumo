package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ignacio/lumo/internal/api/response"
)

// GetEventAnalysisHTML handles GET /api/v1/events/:id/analysis
// Returns a beautiful HTML view of the event with full AI analysis
func (h *EventsHandler) GetEventAnalysisHTML(w http.ResponseWriter, r *http.Request) {
	eventIDStr := chi.URLParam(r, "id")
	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		response.BadRequest(w, "Invalid event ID")
		return
	}

	event, err := h.eventRepo.GetByID(r.Context(), eventID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			response.NotFound(w, "Event not found")
			return
		}
		h.logger.WithError(err).Error("Failed to get event")
		response.InternalServerError(w, "Failed to retrieve event")
		return
	}

	// Render HTML template
	tmpl := template.Must(template.New("analysis").Funcs(template.FuncMap{
		"formatTime": func(t time.Time) string {
			return t.Format("Monday, January 2, 2006 at 3:04:05 PM MST")
		},
		"title": func(s string) string {
			// Convert kebab-case to Title Case
			words := strings.Split(s, "-")
			for i, word := range words {
				if len(word) > 0 {
					words[i] = strings.ToUpper(word[:1]) + word[1:]
				}
			}
			return strings.Join(words, " ")
		},
		"string": func(v interface{}) string {
			// Convert any type to string
			return fmt.Sprint(v)
		},
		"severityColor": func(severity interface{}) string {
			// Convert EventSeverity type to string
			sev := fmt.Sprint(severity)
			switch sev {
			case "critical":
				return "#dc2626" // red-600
			case "high":
				return "#ea580c" // orange-600
			case "medium":
				return "#ca8a04" // yellow-600
			case "low":
				return "#2563eb" // blue-600
			default:
				return "#6b7280" // gray-500
			}
		},
		"severityEmoji": func(severity interface{}) string {
			// Convert EventSeverity type to string
			sev := fmt.Sprint(severity)
			switch sev {
			case "critical":
				return "🔴"
			case "high":
				return "🟠"
			case "medium":
				return "🟡"
			case "low":
				return "🔵"
			default:
				return "⚪"
			}
		},
		"nl2br": func(text string) template.HTML {
			return template.HTML(strings.ReplaceAll(template.HTMLEscapeString(text), "\n", "<br>"))
		},
		"markdown": func(text string) template.HTML {
			// Simple markdown-to-HTML conversion
			// Replace code blocks
			html := text
			html = strings.ReplaceAll(html, "```bash\n", "<pre><code class=\"language-bash\">")
			html = strings.ReplaceAll(html, "```yaml\n", "<pre><code class=\"language-yaml\">")
			html = strings.ReplaceAll(html, "```\n", "</code></pre>")
			html = strings.ReplaceAll(html, "```", "</code></pre>")

			// Replace headers
			html = strings.ReplaceAll(html, "### ", "<h3>")
			html = strings.ReplaceAll(html, "\n\n", "</h3>\n<p>")

			// Replace inline code
			for strings.Contains(html, "`") {
				html = strings.Replace(html, "`", "<code>", 1)
				html = strings.Replace(html, "`", "</code>", 1)
			}

			// Replace bold
			for strings.Contains(html, "**") {
				html = strings.Replace(html, "**", "<strong>", 1)
				html = strings.Replace(html, "**", "</strong>", 1)
			}

			return template.HTML(html)
		},
	}).Parse(htmlTemplate))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, event); err != nil {
		h.logger.WithError(err).Error("Failed to render template")
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
	}
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Event Analysis - {{.EventType}} - Lumo</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            line-height: 1.6;
            color: #1f2937;
            background: linear-gradient(to bottom, #f9fafb, #ffffff);
            padding: 2rem 1rem;
        }
        .container {
            max-width: 900px;
            margin: 0 auto;
            background: white;
            border-radius: 12px;
            box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06);
            overflow: hidden;
        }
        .header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 2rem;
        }
        .header h1 {
            font-size: 1.875rem;
            font-weight: 700;
            margin-bottom: 0.5rem;
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }
        .header .subtitle {
            font-size: 1rem;
            opacity: 0.9;
        }
        .severity-badge {
            display: inline-block;
            padding: 0.25rem 0.75rem;
            border-radius: 9999px;
            font-size: 0.875rem;
            font-weight: 600;
            text-transform: uppercase;
            background: rgba(255, 255, 255, 0.2);
        }
        .content {
            padding: 2rem;
        }
        .section {
            margin-bottom: 2rem;
            padding-bottom: 2rem;
            border-bottom: 1px solid #e5e7eb;
        }
        .section:last-child {
            border-bottom: none;
        }
        .section h2 {
            font-size: 1.25rem;
            font-weight: 600;
            color: #374151;
            margin-bottom: 1rem;
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }
        .section h3 {
            font-size: 1.125rem;
            font-weight: 600;
            color: #4b5563;
            margin: 1.5rem 0 0.75rem 0;
        }
        .section p {
            margin-bottom: 1rem;
            color: #6b7280;
        }
        .metadata {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1rem;
            margin-top: 1rem;
        }
        .metadata-item {
            background: #f9fafb;
            padding: 1rem;
            border-radius: 8px;
            border-left: 4px solid {{severityColor .Severity}};
        }
        .metadata-label {
            font-size: 0.75rem;
            font-weight: 600;
            text-transform: uppercase;
            color: #9ca3af;
            margin-bottom: 0.25rem;
        }
        .metadata-value {
            font-size: 1rem;
            font-weight: 500;
            color: #1f2937;
        }
        .ai-analysis {
            background: #f0f9ff;
            border: 2px solid #0ea5e9;
            border-radius: 8px;
            padding: 1.5rem;
        }
        .ai-analysis h2 {
            color: #0369a1;
        }
        pre {
            background: #1f2937;
            color: #f9fafb;
            padding: 1rem;
            border-radius: 6px;
            overflow-x: auto;
            margin: 1rem 0;
            font-size: 0.875rem;
        }
        code {
            background: #f3f4f6;
            padding: 0.125rem 0.375rem;
            border-radius: 4px;
            font-family: "Monaco", "Courier New", monospace;
            font-size: 0.875rem;
            color: #dc2626;
        }
        pre code {
            background: transparent;
            padding: 0;
            color: #10b981;
        }
        .footer {
            background: #f9fafb;
            padding: 1.5rem 2rem;
            text-align: center;
            color: #6b7280;
            font-size: 0.875rem;
        }
        .footer a {
            color: #667eea;
            text-decoration: none;
            font-weight: 600;
        }
        .timestamp {
            color: #9ca3af;
            font-size: 0.875rem;
        }
        @media (max-width: 640px) {
            body { padding: 1rem 0.5rem; }
            .container { border-radius: 8px; }
            .header { padding: 1.5rem; }
            .header h1 { font-size: 1.5rem; }
            .content { padding: 1.5rem; }
            .metadata { grid-template-columns: 1fr; }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>
                {{severityEmoji .Severity}}
                {{.EventType | title}}
            </h1>
            <div class="subtitle">
                <span class="severity-badge">{{string .Severity}} Severity</span>
                <span class="timestamp">• {{formatTime .EventTimestamp}}</span>
            </div>
        </div>

        <div class="content">
            <!-- Event Details -->
            <div class="section">
                <h2>📋 Event Details</h2>
                <div class="metadata">
                    <div class="metadata-item">
                        <div class="metadata-label">Resource</div>
                        <div class="metadata-value">{{.ResourceKind}}/{{.ResourceName}}</div>
                    </div>
                    {{if .Namespace}}
                    <div class="metadata-item">
                        <div class="metadata-label">Namespace</div>
                        <div class="metadata-value">{{.Namespace}}</div>
                    </div>
                    {{end}}
                    <div class="metadata-item">
                        <div class="metadata-label">Event ID</div>
                        <div class="metadata-value" style="font-family: monospace; font-size: 0.75rem;">{{.ID}}</div>
                    </div>
                </div>
                <div style="margin-top: 1.5rem; padding: 1rem; background: #fef3c7; border-left: 4px solid #f59e0b; border-radius: 4px;">
                    <strong>Message:</strong><br>
                    {{nl2br .Message}}
                </div>
            </div>

            <!-- AI Analysis -->
            {{if .AIAnalysis}}
            <div class="section ai-analysis">
                <h2>🤖 AI-Powered Analysis</h2>
                <div style="margin-top: 1rem;">
                    {{markdown .AIAnalysis}}
                </div>
            </div>
            {{else}}
            <div class="section">
                <h2>🤖 AI Analysis</h2>
                <p style="color: #9ca3af; font-style: italic;">AI analysis is not available for this event.</p>
            </div>
            {{end}}
        </div>

        <div class="footer">
            Powered by <a href="https://github.com/ignacio/lumo" target="_blank">⚡ Lumo</a>
            — Intelligent SRE Automation
        </div>
    </div>
</body>
</html>`
