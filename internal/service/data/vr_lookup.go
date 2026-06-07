package data

import "context"

// rewriteVRLookupFilters is a no-op: list/query filters use record.data cache
// (unwrapped in vrFilterSQL). Lookup JOIN rewrite is replaced by calc_queue cache.
func (s *Data) rewriteVRLookupFilters(ctx context.Context, spec *querySpec) error {
	_ = ctx
	_ = spec
	return nil
}
