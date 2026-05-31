package schema

import "github.com/solat/lowcode-database/internal/service/shared"

type Schema struct {
	B *shared.Base
}

func New(b *shared.Base) *Schema {
	return &Schema{B: b}
}
