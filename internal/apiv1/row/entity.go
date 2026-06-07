package row

import "github.com/monoposer/lowcode-database/internal/apiv1"

type Row struct {
	Id      string                  `json:"id,omitempty"`
	Version int64                   `json:"version,omitempty"`
	Pending bool                    `json:"pending,omitempty"`
	Cells   map[string]*apiv1.Value `json:"cells,omitempty"`
}
