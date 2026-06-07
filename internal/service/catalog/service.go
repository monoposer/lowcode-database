package catalog

import "github.com/monoposer/lowcode-database/internal/service/shared"

type Catalog struct {
	B *shared.Base
}

func New(b *shared.Base) *Catalog {
	return &Catalog{B: b}
}
