package shared

// LookupTargetAllowed reports whether a column on the related table may be used as lookup target_column_id.
func LookupTargetAllowed(kind string) bool {
	switch kind {
	case "formula", "lookup", "rollup":
		return true
	default:
		return !IsVirtualKind(kind)
	}
}
