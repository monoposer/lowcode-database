package sink

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ValidateCreate checks sink-specific required fields for create/update.
func ValidateCreate(sinkType, targetURL string, sinkConfig map[string]any) error {
	switch NormalizeSinkType(sinkType) {
	case TypeWebhook:
		if strings.TrimSpace(targetURL) == "" {
			if url := configString(sinkConfig, "url"); url == "" {
				return fmt.Errorf("target_url is required for webhook sink")
			}
		}
	case TypeRedis:
		if configString(sinkConfig, "url") == "" {
			return fmt.Errorf("sink_config.url is required for redis sink")
		}
		mode := configString(sinkConfig, "mode")
		if mode == "" {
			mode = "stream"
		}
		switch mode {
		case "stream":
			if configString(sinkConfig, "stream") == "" {
				return fmt.Errorf("sink_config.stream is required for redis stream mode")
			}
		case "pubsub":
			if configString(sinkConfig, "channel") == "" {
				return fmt.Errorf("sink_config.channel is required for redis pubsub mode")
			}
		default:
			return fmt.Errorf("sink_config.mode must be stream or pubsub")
		}
	case TypeRabbitMQ:
		if configString(sinkConfig, "url") == "" {
			return fmt.Errorf("sink_config.url is required for rabbitmq sink")
		}
		if configString(sinkConfig, "exchange") == "" {
			return fmt.Errorf("sink_config.exchange is required for rabbitmq sink")
		}
	case TypeKafka:
		if len(configStringSlice(sinkConfig, "brokers")) == 0 {
			return fmt.Errorf("sink_config.brokers is required for kafka sink")
		}
		if configString(sinkConfig, "topic") == "" {
			return fmt.Errorf("sink_config.topic is required for kafka sink")
		}
	default:
		return fmt.Errorf("unknown sink_type %q", sinkType)
	}
	return nil
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

func configStringSlice(m map[string]any, key string) []string {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	switch t := v.(type) {
	case []string:
		var out []string
		for _, s := range t {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
		return out
	case []any:
		var out []string
		for _, item := range t {
			if s := strings.TrimSpace(fmt.Sprint(item)); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
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

func parseEvents(raw []byte) []string {
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
