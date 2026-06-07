package platform

import (
	"github.com/monoposer/lowcode-database/internal/service/meta"
	"github.com/monoposer/lowcode-database/internal/service/shared"
)

type Platform struct {
	B *shared.Base
}

func New(b *shared.Base) *Platform {
	return &Platform{B: b}
}

func (s *Platform) meta() *meta.Read { return meta.New(s.B) }
