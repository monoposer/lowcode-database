package event

import (
	"fmt"
	"net/url"
	"strings"
)

// ResolveDeliveryURL returns an http(s) webhook URL from target_url or legacy sink_config["url"].
func ResolveDeliveryURL(_ string, targetURL string, sinkConfig map[string]any) (string, error) {
	targetURL = strings.TrimSpace(targetURL)
	if targetURL != "" {
		if !isHTTPScheme(targetURL) {
			return "", fmt.Errorf("delivery url must use http or https scheme")
		}
		return targetURL, nil
	}
	u := configString(sinkConfig, "url")
	if u == "" {
		return "", fmt.Errorf("target_url is required (https://…)")
	}
	if !isHTTPScheme(u) {
		return "", fmt.Errorf("delivery url must use http or https scheme")
	}
	return u, nil
}

// ValidateDeliveryURL checks that a delivery URL is a valid http(s) endpoint with host.
func ValidateDeliveryURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("delivery url is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse delivery url: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("unsupported delivery scheme %q (only http and https)", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("https delivery url requires host")
	}
	return nil
}

// SinkFromURL derives the sink label stored in lc_event_sinks.sink (always webhook).
func SinkFromURL(_ string) string {
	return SinkWebhook
}

func isHTTPScheme(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return true
	default:
		return false
	}
}

// ValidateSinkConfig resolves and validates a delivery endpoint (legacy API wrapper).
func ValidateSinkConfig(sink, targetURL string, sinkConfig map[string]any) error {
	u, err := ResolveDeliveryURL(sink, targetURL, sinkConfig)
	if err != nil {
		return err
	}
	return ValidateDeliveryURL(u)
}
