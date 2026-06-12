package graph

import (
	"github.com/monoposer/lowcode-database/internal/service/meta"
	"github.com/monoposer/lowcode-database/internal/service/shared"
)

type Graph struct {
	B *shared.Base
}

func New(b *shared.Base) *Graph {
	return &Graph{B: b}
}

func (s *Graph) meta() *meta.Read { return meta.New(s.B) }
