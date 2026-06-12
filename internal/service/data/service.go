package data

import (
	"github.com/solat/lowcode-database/internal/service/meta"
	"github.com/solat/lowcode-database/internal/service/shared"
)

type Data struct {
	B *shared.Base
}

func New(b *shared.Base) *Data {
	return &Data{B: b}
}

func (s *Data) meta() *meta.Read { return meta.New(s.B) }
