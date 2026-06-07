package data

import (
	"testing"

	"github.com/monoposer/lowcode-database/internal/service/shared"
)

func TestCheckBulkSize(t *testing.T) {
	s := &Data{B: shared.NewBase(nil, 100)}
	s.B.MaxBulkItems = 2
	if err := s.checkBulkSize(2, "bulk"); err != nil {
		t.Fatal(err)
	}
	if err := s.checkBulkSize(3, "bulk"); err == nil {
		t.Fatal("expected limit error")
	}
}

func TestMin32(t *testing.T) {
	if min32(3, 5) != 3 || min32(8, 2) != 2 {
		t.Fatal("min32")
	}
}
