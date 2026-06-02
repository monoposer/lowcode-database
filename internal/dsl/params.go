package dsl

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var filterParamRe = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*)\}`)

// ParamNames returns sorted unique `{param}` placeholders in a filter tree.
func ParamNames(filter map[string]any) []string {
	if len(filter) == 0 {
		return nil
	}
	seen := make(map[string]struct{})
	collectParamNames(filter, seen)
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func collectParamNames(node map[string]any, seen map[string]struct{}) {
	if node == nil {
		return
	}
	t, _ := node["type"].(string)
	switch strings.ToUpper(strings.TrimSpace(t)) {
	case "AND", "OR":
		raw, _ := node["val"].([]any)
		for _, item := range raw {
			if sub, ok := item.(map[string]any); ok {
				collectParamNames(sub, seen)
			}
		}
	default:
		if v, ok := node["val"]; ok {
			recordParamNames(v, seen)
		}
	}
}

func recordParamNames(v any, seen map[string]struct{}) {
	switch x := v.(type) {
	case string:
		for _, m := range filterParamRe.FindAllStringSubmatch(x, -1) {
			seen[m[1]] = struct{}{}
		}
	case []any:
		for _, item := range x {
			recordParamNames(item, seen)
		}
	case []string:
		for _, s := range x {
			recordParamNames(s, seen)
		}
	}
}

// SubstituteParams replaces `{name}` placeholders in filter string values using params.
func SubstituteParams(filter map[string]any, params map[string]any) (map[string]any, error) {
	if len(filter) == 0 {
		return filter, nil
	}
	out, err := substituteNode(filter, params)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func substituteNode(node map[string]any, params map[string]any) (map[string]any, error) {
	if node == nil {
		return nil, nil
	}
	out := make(map[string]any, len(node))
	for k, v := range node {
		switch k {
		case "val":
			typ, _ := node["type"].(string)
			switch strings.ToUpper(strings.TrimSpace(typ)) {
			case "AND", "OR":
				raw, ok := v.([]any)
				if !ok {
					out[k] = v
					continue
				}
				children := make([]any, 0, len(raw))
				for _, item := range raw {
					sub, ok := item.(map[string]any)
					if !ok {
						children = append(children, item)
						continue
					}
					replaced, err := substituteNode(sub, params)
					if err != nil {
						return nil, err
					}
					children = append(children, replaced)
				}
				out[k] = children
			default:
				replaced, err := substituteValue(v, params)
				if err != nil {
					return nil, err
				}
				out[k] = replaced
			}
		default:
			out[k] = v
		}
	}
	return out, nil
}

func substituteValue(v any, params map[string]any) (any, error) {
	switch x := v.(type) {
	case string:
		return substituteString(x, params)
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			r, err := substituteValue(item, params)
			if err != nil {
				return nil, err
			}
			out[i] = r
		}
		return out, nil
	case []string:
		out := make([]string, len(x))
		for i, s := range x {
			r, err := substituteString(s, params)
			if err != nil {
				return nil, err
			}
			out[i] = r.(string)
		}
		return out, nil
	default:
		return v, nil
	}
}

func substituteString(s string, params map[string]any) (any, error) {
	if !filterParamRe.MatchString(s) {
		return s, nil
	}
	var missing []string
	out := filterParamRe.ReplaceAllStringFunc(s, func(tok string) string {
		name := tok[1 : len(tok)-1]
		if p, ok := params[name]; ok {
			return fmt.Sprint(p)
		}
		missing = append(missing, name)
		return tok
	})
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("filter param %q is required", missing[0])
	}
	return out, nil
}
