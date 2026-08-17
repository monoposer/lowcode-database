package migrations

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed meta/*.up.sql
var metaFS embed.FS

// FS returns embedded meta *.up.sql files.
func FS() (fs.FS, error) {
	sub, err := fs.Sub(metaFS, "meta")
	if err != nil {
		return nil, fmt.Errorf("migrations/meta: %w", err)
	}
	return sub, nil
}
