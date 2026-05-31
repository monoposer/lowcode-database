package platform

import "github.com/solat/lowcode-database/internal/service/shared"

type Platform struct {
	B *shared.Base
}

func New(b *shared.Base) *Platform {
	return &Platform{B: b}
}
