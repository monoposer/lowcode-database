package data

import "fmt"

func (s *Data) maxScanRows() int32 {
	if s.B.MaxScanRows > 0 {
		return s.B.MaxScanRows
	}
	if s.B.MaxRow > 0 {
		return s.B.MaxRow
	}
	return 1000
}

func (s *Data) maxBulkItems() int {
	if s.B.MaxBulkItems > 0 {
		return s.B.MaxBulkItems
	}
	return 500
}

func min32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func (s *Data) checkBulkSize(n int, kind string) error {
	max := s.maxBulkItems()
	if n > max {
		return fmt.Errorf("%s size %d exceeds hard limit %d", kind, n, max)
	}
	return nil
}
