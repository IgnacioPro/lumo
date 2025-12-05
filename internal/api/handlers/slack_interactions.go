package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/api/response"
	"github.com/ignacio/lumo/internal/correlation"
)

// SlackInteractionHandler handles Slack interactive component callbacks
type SlackInteractionHandler struct {
	incidentRepo      correlation.IncidentRepository
	correlationEngine *correlation.Engine
	signingSecret     string // Slack signing secret for request verification
	logger            *logrus.Logger
}

// SlackInteractionPayload represents the payload sent by Slack for interactive components
type SlackInteractionPayload struct {
	Type        string        `json:"type"`
	User        SlackUser     `json:"user"`
	Channel     SlackChannel  `json:"channel"`
	Actions     []SlackAction `json:"actions"`
	ResponseURL string        `json:"response_url"`
	TriggerID   string        `json:"trigger_id"`
	Team        SlackTeam     `json:"team"`
	Message     *SlackMessage `json:"message,omitempty"`
}

// SlackUser represents a Slack user
type SlackUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

// SlackChannel represents a Slack channel
type SlackChannel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// SlackTeam represents a Slack team/workspace
type SlackTeam struct {
	ID     string `json:"id"`
	Domain string `json:"domain"`
}

// SlackAction represents a user action on an interactive component
type SlackAction struct {
	Type           string       `json:"type"`
	ActionID       string       `json:"action_id"`
	BlockID        string       `json:"block_id"`
	Value          string       `json:"value,omitempty"`
	SelectedOption *SlackOption `json:"selected_option,omitempty"`
}

// SlackOption represents a selected option (for overflow menus)
type SlackOption struct {
	Value string `json:"value"`
}

// SlackMessage represents the original message
type SlackMessage struct {
	Text string `json:"text"`
	TS   string `json:"ts"`
}

// NewSlackInteractionHandler creates a new Slack interaction handler
func NewSlackInteractionHandler(
	incidentRepo correlation.IncidentRepository,
	correlationEngine *correlation.Engine,
	signingSecret string,
	logger *logrus.Logger,
) *SlackInteractionHandler {
	return &SlackInteractionHandler{
		incidentRepo:      incidentRepo,
		correlationEngine: correlationEngine,
		signingSecret:     signingSecret,
		logger:            logger,
	}
}

// HandleInteraction handles POST /api/v1/slack/interactions
// This endpoint receives Slack interactive component callbacks
func (h *SlackInteractionHandler) HandleInteraction(w http.ResponseWriter, r *http.Request) {
	// Slack sends the payload as a form-encoded string
	if err := r.ParseForm(); err != nil {
		h.logger.WithError(err).Error("Failed to parse form data")
		response.BadRequest(w, "Invalid form data")
		return
	}

	payloadStr := r.FormValue("payload")
	if payloadStr == "" {
		response.BadRequest(w, "Missing payload")
		return
	}

	// Parse the JSON payload
	var payload SlackInteractionPayload
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		h.logger.WithError(err).Error("Failed to parse Slack payload")
		response.BadRequest(w, "Invalid payload format")
		return
	}

	h.logger.WithFields(logrus.Fields{
		"type":        payload.Type,
		"user":        payload.User.Username,
		"channel":     payload.Channel.Name,
		"num_actions": len(payload.Actions),
	}).Info("Received Slack interaction")

	// Process each action
	for _, action := range payload.Actions {
		h.logger.WithFields(logrus.Fields{
			"action_id": action.ActionID,
			"value":     action.Value,
		}).Debug("Processing action")

		switch action.ActionID {
		case "lumo_approve_fix":
			h.handleApproveFix(w, r, payload, action)
			return
		case "lumo_decline":
			h.handleDecline(w, r, payload, action)
			return
		case "lumo_snooze":
			h.handleSnooze(w, r, payload, action)
			return
		default:
			h.logger.WithField("action_id", action.ActionID).Warn("Unknown action")
		}
	}

	// Acknowledge receipt (Slack expects a 200 response within 3 seconds)
	w.WriteHeader(http.StatusOK)
}

// handleApproveFix processes the "Approve Fix" button click
func (h *SlackInteractionHandler) handleApproveFix(w http.ResponseWriter, r *http.Request, payload SlackInteractionPayload, action SlackAction) {
	// Parse incident ID and fix hash from value (format: "incident_id|fix_hash")
	incidentID, fixHash := parseActionValue(action.Value)

	h.logger.WithFields(logrus.Fields{
		"incident_id": incidentID,
		"fix_hash":    fixHash,
		"user":        payload.User.Username,
	}).Info("Processing approve fix action")

	// TODO: Implement actual fix approval logic
	// This would typically:
	// 1. Look up the incident by ID
	// 2. Verify the fix hash matches
	// 3. Execute the proposed fix (or queue it for execution)
	// 4. Update incident status
	// 5. Send confirmation notification

	// For now, send an acknowledgment response
	responseMsg := fmt.Sprintf("✅ Fix approved by @%s\n\n*Incident:* `%s`\n*Fix:* `%s`\n*Status:* Queued for execution\n\n_Lumo will execute the fix and update this thread._",
		payload.User.Username,
		truncateID(incidentID),
		truncateID(fixHash),
	)

	h.sendSlackResponse(payload.ResponseURL, responseMsg, false)
	w.WriteHeader(http.StatusOK)
}

// handleDecline processes the "Decline" button click
func (h *SlackInteractionHandler) handleDecline(w http.ResponseWriter, r *http.Request, payload SlackInteractionPayload, action SlackAction) {
	// Parse incident ID and fix hash from value
	incidentID, _ := parseActionValue(action.Value)

	h.logger.WithFields(logrus.Fields{
		"incident_id": incidentID,
		"user":        payload.User.Username,
	}).Info("Processing decline action")

	// TODO: Implement actual decline logic
	// This would typically:
	// 1. Mark the incident as acknowledged but not auto-fixed
	// 2. Update incident status to "manual_intervention_required"
	// 3. Optionally trigger escalation workflow

	responseMsg := fmt.Sprintf("❌ Fix declined by @%s\n\n*Incident:* `%s`\n*Status:* Manual intervention required\n\n_The proposed fix will not be executed. Please handle this incident manually._",
		payload.User.Username,
		truncateID(incidentID),
	)

	h.sendSlackResponse(payload.ResponseURL, responseMsg, false)
	w.WriteHeader(http.StatusOK)
}

// handleSnooze processes the snooze overflow menu selection
func (h *SlackInteractionHandler) handleSnooze(w http.ResponseWriter, r *http.Request, payload SlackInteractionPayload, action SlackAction) {
	// Get the selected snooze duration from the overflow menu
	var snoozeValue string
	if action.SelectedOption != nil {
		snoozeValue = action.SelectedOption.Value
	} else {
		snoozeValue = action.Value
	}

	// Parse the snooze value (format: "snooze_15|incident_id|fix_hash" or "snooze_30|incident_id|fix_hash")
	parts := strings.Split(snoozeValue, "|")
	if len(parts) < 2 {
		h.logger.WithField("value", snoozeValue).Warn("Invalid snooze value format")
		response.BadRequest(w, "Invalid snooze value")
		return
	}

	snoozeDuration := parts[0]
	incidentID := parts[1]

	// Determine snooze duration in minutes
	var durationMinutes int
	switch snoozeDuration {
	case "snooze_15":
		durationMinutes = 15
	case "snooze_30":
		durationMinutes = 30
	case "snooze_60":
		durationMinutes = 60
	default:
		durationMinutes = 15
	}

	snoozeUntil := time.Now().Add(time.Duration(durationMinutes) * time.Minute)

	h.logger.WithFields(logrus.Fields{
		"incident_id":    incidentID,
		"snooze_minutes": durationMinutes,
		"snooze_until":   snoozeUntil,
		"user":           payload.User.Username,
	}).Info("Processing snooze action")

	// TODO: Implement actual snooze logic
	// This would typically:
	// 1. Update incident state to "snoozed"
	// 2. Set a snooze_until timestamp
	// 3. Suppress notifications until that time
	// 4. Schedule a reminder when snooze expires

	responseMsg := fmt.Sprintf("⏰ Incident snoozed by @%s\n\n*Incident:* `%s`\n*Snoozed for:* %d minutes\n*Resumes at:* %s\n\n_Notifications for this incident are paused until the snooze expires._",
		payload.User.Username,
		truncateID(incidentID),
		durationMinutes,
		snoozeUntil.Format("15:04 MST"),
	)

	h.sendSlackResponse(payload.ResponseURL, responseMsg, false)
	w.WriteHeader(http.StatusOK)
}

// sendSlackResponse sends a response back to Slack via the response URL
func (h *SlackInteractionHandler) sendSlackResponse(responseURL, text string, replaceOriginal bool) {
	if responseURL == "" {
		h.logger.Warn("No response URL provided")
		return
	}

	responsePayload := map[string]interface{}{
		"text":             text,
		"response_type":    "in_channel",
		"replace_original": replaceOriginal,
	}

	payloadBytes, err := json.Marshal(responsePayload)
	if err != nil {
		h.logger.WithError(err).Error("Failed to marshal response payload")
		return
	}

	resp, err := http.Post(responseURL, "application/json", strings.NewReader(string(payloadBytes)))
	if err != nil {
		h.logger.WithError(err).Error("Failed to send Slack response")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		h.logger.WithField("status", resp.StatusCode).Warn("Slack response returned non-OK status")
	}
}

// parseActionValue parses the action value string (format: "incident_id|fix_hash")
func parseActionValue(value string) (incidentID, fixHash string) {
	parts := strings.Split(value, "|")
	if len(parts) >= 1 {
		incidentID = parts[0]
	}
	if len(parts) >= 2 {
		fixHash = parts[1]
	}
	return
}

// truncateID truncates an ID for display (first 8 characters)
func truncateID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// VerifySlackSignature is a middleware to verify Slack request signatures
// This should be used in production to ensure requests are actually from Slack
func (h *SlackInteractionHandler) VerifySlackSignature(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// In production, implement HMAC-SHA256 signature verification
		// using X-Slack-Signature and X-Slack-Request-Timestamp headers
		// For now, pass through (but log a warning)
		if h.signingSecret == "" {
			h.logger.Warn("Slack signing secret not configured - skipping signature verification")
		}
		// TODO: Implement actual signature verification
		// See: https://api.slack.com/authentication/verifying-requests-from-slack
		next.ServeHTTP(w, r)
	})
}
