package schema

import (
	"github.com/solat/lowcode-database/internal/apiv1"
	"github.com/solat/lowcode-database/internal/service/shared"
)

// PublicColumn sets Column.Id to the logical name and exposes resultTypeId from config.
func PublicColumn(c *apiv1.Column) {
	if c == nil {
		return
	}
	if c.Name != "" {
		c.Id = c.Name
	}
	if c.ResultTypeId == "" && c.Config != nil {
		c.ResultTypeId = shared.ConfigResultTypeID(c.Config)
	}
}
