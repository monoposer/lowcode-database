package data

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/solat/lowcode-database/internal/apiv1"
	"github.com/solat/lowcode-database/internal/service/catalog"
	"github.com/solat/lowcode-database/internal/service/shared"
)

// writeOneRelationshipRow creates or updates the related row for a cardinality-one relationship.
func (s *Data) writeOneRelationshipRow(
	ctx context.Context,
	tx pgx.Tx,
	rel shared.RelationshipColumn,
	input *apiv1.SaveGraphOneInput,
) (*apiv1.Row, *apiv1.Value, error) {
	if input == nil {
		return nil, nil, nil
	}
	if input.Delete {
		if input.Id == "" {
			return nil, nil, fmt.Errorf("delete requires id")
		}
		cols, schemaName, tableName, err := catalog.New(s.B).LoadColumns(ctx, rel.TargetTableId)
		if err != nil {
			return nil, nil, err
		}
		_ = cols
		if err := s.deleteRowTx(ctx, tx, schemaName, tableName, input.Id); err != nil {
			return nil, nil, err
		}
		return nil, nil, nil
	}

	cols, schemaName, tableName, err := catalog.New(s.B).LoadColumns(ctx, rel.TargetTableId)
	if err != nil {
		return nil, nil, err
	}
	cells := shared.NormalizeInputCells(input.Cells, cols)
	cells, err = s.resolveLookupCells(ctx, tx, rel.TargetTableId, cells)
	if err != nil {
		return nil, nil, err
	}

	var relatedID string
	if input.Id != "" {
		relatedID = input.Id
		if err := s.updateRowTx(ctx, tx, cols, schemaName, tableName, relatedID, cells); err != nil {
			return nil, nil, err
		}
	} else {
		relatedID, err = s.insertRowTx(ctx, tx, cols, schemaName, tableName, cells)
		if err != nil {
			return nil, nil, err
		}
	}
	if relatedID == "" {
		return nil, nil, fmt.Errorf("relationship %q: no writable columns on related table", rel.Id)
	}
	return &apiv1.Row{Id: relatedID, Cells: cells}, apiv1.StringValue(relatedID), nil
}
