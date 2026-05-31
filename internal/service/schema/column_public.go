package schema

import "github.com/solat/lowcode-database/internal/apiv1"

// PublicColumn sets Column.Id to the logical name (same convention as Table.Id = name).
func PublicColumn(c *apiv1.Column) {
	if c == nil {
		return
	}
	if c.Name != "" {
		c.Id = c.Name
	}
}
