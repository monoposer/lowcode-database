package event

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const SinkWebhook = "webhook"

var knownSinks = map[string]struct{}{
	SinkWebhook: {},
}

func NormalizeSink(s string) string {
	if s == "" {
		return SinkWebhook
	}
	return s
}

func ValidSink(s string) bool {
	if s == "" {
		return true
	}
	_, ok := knownSinks[s]
	return ok
}

func configString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func parseHeaders(raw []byte) map[string]string {
	out := map[string]string{}
	if len(raw) == 0 || string(raw) == "null" {
		return out
	}
	var hm map[string]string
	if err := json.Unmarshal(raw, &hm); err != nil {
		var anyMap map[string]any
		if json.Unmarshal(raw, &anyMap) == nil {
			for k, v := range anyMap {
				if strings.TrimSpace(k) != "" {
					out[k] = fmt.Sprint(v)
				}
			}
		}
		return out
	}
	for k, v := range hm {
		if strings.TrimSpace(k) != "" {
			out[k] = v
		}
	}
	return out
}

func parseSinkConfig(raw []byte) map[string]any {
	out := map[string]any{}
	if len(raw) == 0 || string(raw) == "null" {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}

func parseEventTypes(raw []byte) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var evList []string
	_ = json.Unmarshal(raw, &evList)
	return evList
}

func eventMatches(evList []string, eventType string) bool {
	if len(evList) == 0 {
		return true
	}
	for _, e := range evList {
		if e == eventType {
			return true
		}
	}
	return false
}

// DeliveryConfig controls retry, dead-letter persistence, and metrics.
type DeliveryConfig struct {
	RetryMax       int
	RetryInitial   time.Duration
	DLQEnabled     bool
	MetricsEnabled bool
}

func DeliveryConfigFrom(retryMax, retryInitialMS int, dlqEnabled bool, metricsBackend string) DeliveryConfig {
	if retryMax <= 0 {
		retryMax = 3
	}
	if retryInitialMS <= 0 {
		retryInitialMS = 500
	}
	metrics := metricsBackend == "prometheus"
	return DeliveryConfig{
		RetryMax:       retryMax,
		RetryInitial:   time.Duration(retryInitialMS) * time.Millisecond,
		DLQEnabled:     dlqEnabled,
		MetricsEnabled: metrics,
	}
}
