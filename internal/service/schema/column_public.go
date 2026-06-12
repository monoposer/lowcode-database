package schema

import (
	"context"
	apiv1schema "github.com/solat/lowcode-database/internal/apiv1/schema"
	"github.com/solat/lowcode-database/internal/service/catalog"
	"github.com/solat/lowcode-database/internal/service/shared"
)

// PublicColumn sets Column.Id to the logical name and exposes resultTypeId from config.
func PublicColumn(c *apiv1schema.Column) {
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

func listTableIndexesViaCatalog(s *Schema, ctx context.Context, tableID, schemaName, tableName string) ([]*apiv1schema.Index, error) {
	rows, err := catalog.New(s.B).ListPGIndexes(ctx, schemaName, tableName)
	if err != nil {
		return nil, err
	}
	return catalog.New(s.B).PGIndexesToAPI(ctx, tableID, schemaName, tableName, rows)
}
