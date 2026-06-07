package meta

import "github.com/monoposer/lowcode-database/internal/service/shared"

// Read is the cross-domain metadata read facade. Domain services use it instead of
// constructing catalog.Schema/platform siblings directly (avoids scattered coupling).
type Read struct {
	B *shared.Base
}

func New(b *shared.Base) *Read {
	if b == nil {
		return nil
	}
	return &Read{B: b}
}
