package schema

import (
	"context"
	"github.com/monoposer/lowcode-database/internal/service/catalog"
	"github.com/monoposer/lowcode-database/internal/service/shared"
)

// PublicColumn sets Column.Id to the logical name and exposes resultTypeId from config.
func PublicColumn(c *Column) {
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

func listTableIndexesViaCatalog(s *Schema, ctx context.Context, tableName, schemaName, physicalName string) ([]*catalog.Index, error) {
	rows, err := catalog.New(s.B).ListPGIndexes(ctx, schemaName, physicalName)
	if err != nil {
		return nil, err
	}
	return catalog.New(s.B).PGIndexesToAPI(ctx, tableName, schemaName, physicalName, rows)
}
