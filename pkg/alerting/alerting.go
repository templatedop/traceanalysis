// Package alerting provides notification integrations for alerting
package alerting

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/traceanalysis/rag-temporal/internal/config"
	"github.com/traceanalysis/rag-temporal/internal/types"
)

// Notifier is the interface for sending notifications
type Notifier interface {
	// Notify sends a notification
	Notify(ctx context.Context, alert *Alert) error
	// Name returns the notifier name
	Name() string
	// Enabled returns whether the notifier is enabled
	Enabled() bool
}

// Alert represents an alert to be sent
type Alert struct {
	ID          string           `json:"id"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Severity    types.Severity   `json:"severity"`
	Findings    []types.Finding  `json:"findings,omitempty"`
	Services    []string         `json:"services,omitempty"`
	Timestamp   time.Time        `json:"timestamp"`
	Source      string           `json:"source"`
	Links       []string         `json:"links,omitempty"`
}

// AlertManager manages multiple notifiers
type AlertManager struct {
	notifiers []Notifier
}

// NewAlertManager creates a new alert manager
func NewAlertManager(cfg *config.AlertingConfig) *AlertManager {
	am := &AlertManager{
		notifiers: []Notifier{},
	}

	if cfg.Slack.Enabled {
		am.notifiers = append(am.notifiers, NewSlackNotifier(&cfg.Slack))
	}
	if cfg.PagerDuty.Enabled {
		am.notifiers = append(am.notifiers, NewPagerDutyNotifier(&cfg.PagerDuty))
	}
	if cfg.Webhook.Enabled {
		am.notifiers = append(am.notifiers, NewWebhookNotifier(&cfg.Webhook))
	}

	return am
}

// Notify sends an alert to all enabled notifiers
func (am *AlertManager) Notify(ctx context.Context, alert *Alert) error {
	var lastErr error
	for _, notifier := range am.notifiers {
		if notifier.Enabled() {
			if err := notifier.Notify(ctx, alert); err != nil {
				lastErr = err
				// Continue with other notifiers
			}
		}
	}
	return lastErr
}

// NotifyFindings sends alerts for critical findings
func (am *AlertManager) NotifyFindings(ctx context.Context, result *types.AnalysisResult) error {
	// Filter critical/high findings
	var criticalFindings []types.Finding
	for _, f := range result.Findings {
		if f.Severity == types.SeverityCritical || f.Severity == types.SeverityHigh {
			criticalFindings = append(criticalFindings, f)
		}
	}

	if len(criticalFindings) == 0 {
		return nil
	}

	// Determine highest severity
	severity := types.SeverityHigh
	for _, f := range criticalFindings {
		if f.Severity == types.SeverityCritical {
			severity = types.SeverityCritical
			break
		}
	}

	// Build services list
	serviceSet := make(map[string]bool)
	for _, f := range criticalFindings {
		for _, s := range f.AffectedServices {
			serviceSet[s] = true
		}
	}
	services := make([]string, 0, len(serviceSet))
	for s := range serviceSet {
		services = append(services, s)
	}

	alert := &Alert{
		ID:          result.RequestID,
		Title:       fmt.Sprintf("[%s] Observability Analysis Alert", severity),
		Description: result.Summary,
		Severity:    severity,
		Findings:    criticalFindings,
		Services:    services,
		Timestamp:   time.Now(),
		Source:      "observability-analysis",
	}

	return am.Notify(ctx, alert)
}

// SlackNotifier sends notifications to Slack
type SlackNotifier struct {
	webhookURL string
	channel    string
	client     *http.Client
	enabled    bool
}

// NewSlackNotifier creates a new Slack notifier
func NewSlackNotifier(cfg *config.SlackConfig) *SlackNotifier {
	return &SlackNotifier{
		webhookURL: cfg.WebhookURL,
		channel:    cfg.Channel,
		client:     &http.Client{Timeout: 10 * time.Second},
		enabled:    cfg.Enabled && cfg.WebhookURL != "",
	}
}

// SlackMessage represents a Slack message
type SlackMessage struct {
	Channel     string            `json:"channel,omitempty"`
	Username    string            `json:"username,omitempty"`
	IconEmoji   string            `json:"icon_emoji,omitempty"`
	Text        string            `json:"text,omitempty"`
	Attachments []SlackAttachment `json:"attachments,omitempty"`
}

// SlackAttachment represents a Slack attachment
type SlackAttachment struct {
	Color      string       `json:"color"`
	Title      string       `json:"title"`
	Text       string       `json:"text"`
	Fields     []SlackField `json:"fields,omitempty"`
	Footer     string       `json:"footer,omitempty"`
	Timestamp  int64        `json:"ts,omitempty"`
}

// SlackField represents a field in a Slack attachment
type SlackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// Notify sends a notification to Slack
func (s *SlackNotifier) Notify(ctx context.Context, alert *Alert) error {
	if !s.enabled {
		return nil
	}

	color := s.severityColor(alert.Severity)

	// Build fields
	fields := []SlackField{
		{Title: "Severity", Value: string(alert.Severity), Short: true},
		{Title: "Source", Value: alert.Source, Short: true},
	}

	if len(alert.Services) > 0 {
		fields = append(fields, SlackField{
			Title: "Services",
			Value: joinStrings(alert.Services, ", "),
			Short: false,
		})
	}

	// Add findings
	for i, f := range alert.Findings {
		if i >= 5 {
			fields = append(fields, SlackField{
				Title: "Additional Findings",
				Value: fmt.Sprintf("+%d more findings", len(alert.Findings)-5),
				Short: false,
			})
			break
		}
		fields = append(fields, SlackField{
			Title: fmt.Sprintf("[%s] %s", f.Severity, f.Title),
			Value: truncateString(f.Description, 200),
			Short: false,
		})
	}

	msg := SlackMessage{
		Channel:   s.channel,
		Username:  "Observability Analysis",
		IconEmoji: ":mag:",
		Attachments: []SlackAttachment{
			{
				Color:     color,
				Title:     alert.Title,
				Text:      alert.Description,
				Fields:    fields,
				Footer:    "Observability Analysis Framework",
				Timestamp: alert.Timestamp.Unix(),
			},
		},
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal slack message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send slack message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("slack returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Name returns the notifier name
func (s *SlackNotifier) Name() string {
	return "slack"
}

// Enabled returns whether the notifier is enabled
func (s *SlackNotifier) Enabled() bool {
	return s.enabled
}

func (s *SlackNotifier) severityColor(severity types.Severity) string {
	switch severity {
	case types.SeverityCritical:
		return "#dc3545" // Red
	case types.SeverityHigh:
		return "#fd7e14" // Orange
	case types.SeverityMedium:
		return "#ffc107" // Yellow
	case types.SeverityLow:
		return "#28a745" // Green
	default:
		return "#6c757d" // Gray
	}
}

// PagerDutyNotifier sends notifications to PagerDuty
type PagerDutyNotifier struct {
	routingKey string
	client     *http.Client
	enabled    bool
}

// NewPagerDutyNotifier creates a new PagerDuty notifier
func NewPagerDutyNotifier(cfg *config.PagerDutyConfig) *PagerDutyNotifier {
	return &PagerDutyNotifier{
		routingKey: cfg.RoutingKey,
		client:     &http.Client{Timeout: 10 * time.Second},
		enabled:    cfg.Enabled && cfg.RoutingKey != "",
	}
}

// PagerDutyEvent represents a PagerDuty event
type PagerDutyEvent struct {
	RoutingKey  string                 `json:"routing_key"`
	EventAction string                 `json:"event_action"` // trigger, acknowledge, resolve
	DedupKey    string                 `json:"dedup_key,omitempty"`
	Payload     PagerDutyPayload       `json:"payload"`
	Links       []PagerDutyLink        `json:"links,omitempty"`
	Images      []PagerDutyImage       `json:"images,omitempty"`
}

// PagerDutyPayload represents the event payload
type PagerDutyPayload struct {
	Summary       string                 `json:"summary"`
	Severity      string                 `json:"severity"` // critical, error, warning, info
	Source        string                 `json:"source"`
	Component     string                 `json:"component,omitempty"`
	Group         string                 `json:"group,omitempty"`
	Class         string                 `json:"class,omitempty"`
	CustomDetails map[string]interface{} `json:"custom_details,omitempty"`
	Timestamp     string                 `json:"timestamp,omitempty"`
}

// PagerDutyLink represents a link
type PagerDutyLink struct {
	Href string `json:"href"`
	Text string `json:"text"`
}

// PagerDutyImage represents an image
type PagerDutyImage struct {
	Src  string `json:"src"`
	Href string `json:"href,omitempty"`
	Alt  string `json:"alt,omitempty"`
}

// Notify sends a notification to PagerDuty
func (p *PagerDutyNotifier) Notify(ctx context.Context, alert *Alert) error {
	if !p.enabled {
		return nil
	}

	pdSeverity := p.convertSeverity(alert.Severity)

	// Build custom details
	customDetails := map[string]interface{}{
		"findings_count": len(alert.Findings),
		"services":       alert.Services,
	}

	for i, f := range alert.Findings {
		if i >= 5 {
			break
		}
		customDetails[fmt.Sprintf("finding_%d", i+1)] = fmt.Sprintf("[%s] %s: %s", f.Severity, f.Title, f.Description)
	}

	event := PagerDutyEvent{
		RoutingKey:  p.routingKey,
		EventAction: "trigger",
		DedupKey:    alert.ID,
		Payload: PagerDutyPayload{
			Summary:       fmt.Sprintf("%s: %s", alert.Title, truncateString(alert.Description, 200)),
			Severity:      pdSeverity,
			Source:        alert.Source,
			Component:     "observability-analysis",
			CustomDetails: customDetails,
			Timestamp:     alert.Timestamp.Format(time.RFC3339),
		},
	}

	// Add links
	for _, link := range alert.Links {
		event.Links = append(event.Links, PagerDutyLink{
			Href: link,
			Text: "View Details",
		})
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal pagerduty event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://events.pagerduty.com/v2/enqueue", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send pagerduty event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pagerduty returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Name returns the notifier name
func (p *PagerDutyNotifier) Name() string {
	return "pagerduty"
}

// Enabled returns whether the notifier is enabled
func (p *PagerDutyNotifier) Enabled() bool {
	return p.enabled
}

func (p *PagerDutyNotifier) convertSeverity(severity types.Severity) string {
	switch severity {
	case types.SeverityCritical:
		return "critical"
	case types.SeverityHigh:
		return "error"
	case types.SeverityMedium:
		return "warning"
	case types.SeverityLow:
		return "info"
	default:
		return "info"
	}
}

// WebhookNotifier sends notifications to a generic webhook
type WebhookNotifier struct {
	url     string
	headers map[string]string
	client  *http.Client
	enabled bool
}

// NewWebhookNotifier creates a new webhook notifier
func NewWebhookNotifier(cfg *config.WebhookConfig) *WebhookNotifier {
	return &WebhookNotifier{
		url:     cfg.URL,
		headers: cfg.Headers,
		client:  &http.Client{Timeout: 10 * time.Second},
		enabled: cfg.Enabled && cfg.URL != "",
	}
}

// Notify sends a notification to the webhook
func (w *WebhookNotifier) Notify(ctx context.Context, alert *Alert) error {
	if !w.enabled {
		return nil
	}

	body, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("failed to marshal alert: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", w.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	for k, v := range w.headers {
		req.Header.Set(k, v)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Name returns the notifier name
func (w *WebhookNotifier) Name() string {
	return "webhook"
}

// Enabled returns whether the notifier is enabled
func (w *WebhookNotifier) Enabled() bool {
	return w.enabled
}

// Helper functions

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
