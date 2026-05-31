package graph

import "github.com/solat/lowcode-database/internal/service/shared"

type Graph struct {
	B *shared.Base
}

func New(b *shared.Base) *Graph {
	return &Graph{B: b}
}
