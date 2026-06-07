package migrations

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed meta/*.up.sql
var metaFS embed.FS

//go:embed data/*.up.sql
var dataFS embed.FS

// FS returns embedded *.up.sql files for target (meta or data).
func FS(target string) (fs.FS, error) {
	switch target {
	case "meta":
		return fs.Sub(metaFS, "meta")
	case "data":
		return fs.Sub(dataFS, "data")
	default:
		return nil, fmt.Errorf("unknown migration target %q (use meta or data)", target)
	}
}
