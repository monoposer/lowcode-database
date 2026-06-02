package formula

// Refs returns logical column names referenced as {{name}} in expr (first occurrence order, deduped).
func Refs(expr string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, m := range columnRefRe.FindAllStringSubmatch(expr, -1) {
		if len(m) < 2 {
			continue
		}
		name := m[1]
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}
