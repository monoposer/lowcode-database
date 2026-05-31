package data

import "github.com/solat/lowcode-database/internal/service/shared"

type Data struct {
	B *shared.Base
}

func New(b *shared.Base) *Data {
	return &Data{B: b}
}
