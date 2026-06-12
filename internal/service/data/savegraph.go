package data

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/solat/lowcode-database/internal/apiv1"
	"github.com/solat/lowcode-database/internal/apiv1/graph"
	"github.com/solat/lowcode-database/internal/apiv1/row"
	"github.com/solat/lowcode-database/internal/event"
	"github.com/solat/lowcode-database/internal/service/schema"
	"github.com/solat/lowcode-database/internal/service/shared"
	"strings"
)

// SaveGraph upserts a root row and nested relationship rows in one transaction.
func (s *Data) SaveGraph(ctx context.Context, req *graph.SaveGraphRequest) (graph.SaveGraphResponse, error) {
	tableID := req.TableId
	if tableID == "" {
		return nil, fmt.Errorf("table_id is required")
	}

	manyRels, err := s.meta().LoadManyRelationshipColumns(ctx, tableID)
	if err != nil {
		return nil, err
	}
	oneRels, err := s.meta().LoadOneRelationshipColumns(ctx, tableID)
	if err != nil {
		return nil, err
	}
	if err := req.ClassifySaveGraphFields(
		schema.RelationshipColumnNamesSet(manyRels),
		schema.RelationshipColumnNamesSet(oneRels),
	); err != nil {
		return nil, err
	}

	rootCols, rootSchema, rootTable, err := s.meta().LoadColumns(ctx, tableID)
	if err != nil {
		return nil, err
	}

	data, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}

	tx, err := data.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	schemaSvc := schema.New(s.B)
	outcome := &graph.SaveGraphSaveOutcome{
		One:             map[string]*row.Row{},
		Many:            map[string][]*graph.SaveGraphChildSaveOutcome{},
		ManyLinkColumns: map[string]string{},
	}

	rootCells, rootID, isUpdate, err := s.saveGraphRootRow(ctx, tx, tid, tableID, req, schemaSvc, oneRels, rootCols, rootSchema, rootTable, outcome)
	if err != nil {
		return nil, err
	}
	outcome.RootID = rootID
	outcome.RootCells = rootCells

	if err := s.saveGraphManyRelationships(ctx, tx, tid, tableID, rootID, req, schemaSvc, manyRels, outcome); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	echo := graph.BuildSaveGraphEcho(req, outcome)
	s.emitSaveGraphHooks(ctx, isUpdate, tableID, manyRels, oneRels, outcome)
	return echo, nil
}

func (s *Data) saveGraphRootRow(
	ctx context.Context,
	tx pgx.Tx,
	tid string,
	tableID string,
	req *graph.SaveGraphRequest,
	schemaSvc *schema.Schema,
	oneRels map[string]shared.RelationshipColumn,
	rootCols []shared.ColumnMeta,
	rootSchema, rootTable string,
	outcome *graph.SaveGraphSaveOutcome,
) (rootCells map[string]*apiv1.Value, rootID string, isUpdate bool, err error) {
	rootCells = shared.NormalizeInputCells(req.RootCells, rootCols)
	for relName, relInput := range req.OneRelationships {
		if relInput == nil {
			continue
		}
		rel, ok := oneRels[relName]
		if !ok {
			return nil, "", false, fmt.Errorf("unknown one relationship column %q", relName)
		}
		relatedRow, fkVal, err := s.writeOneRelationshipRow(ctx, tx, rel, relInput)
		if err != nil {
			return nil, "", false, fmt.Errorf("relationship %q: %w", relName, err)
		}
		if fkVal != nil {
			fkColName, err := schemaSvc.MustFKColumnName(ctx, tid, tableID, rel.TargetColumnId)
			if err != nil {
				return nil, "", false, err
			}
			rootCells[fkColName] = fkVal
		}
		if relatedRow != nil {
			outcome.One[relName] = relatedRow
		}
	}

	rootCells, err = s.resolveLookupCells(ctx, tx, tableID, rootCells)
	if err != nil {
		return nil, "", false, err
	}

	isUpdate = req.Id != ""
	if !isUpdate {
		rootID, err = s.insertRowTx(ctx, tx, rootCols, rootSchema, rootTable, rootCells)
		if err != nil {
			return nil, "", false, err
		}
		if rootID == "" && len(req.ManyRelationships) == 0 && len(req.OneRelationships) == 0 {
			return nil, "", false, fmt.Errorf("no writable columns for root row")
		}
	} else {
		rootID = req.Id
		if len(rootCells) > 0 {
			if err := s.updateRowTx(ctx, tx, rootCols, rootSchema, rootTable, rootID, rootCells); err != nil {
				return nil, "", false, err
			}
		}
	}
	return rootCells, rootID, isUpdate, nil
}

func (s *Data) saveGraphManyRelationships(
	ctx context.Context,
	tx pgx.Tx,
	tid string,
	tableID, rootID string,
	req *graph.SaveGraphRequest,
	schemaSvc *schema.Schema,
	manyRels map[string]shared.RelationshipColumn,
	outcome *graph.SaveGraphSaveOutcome,
) error {
	for relName, relInput := range req.ManyRelationships {
		if relInput == nil {
			continue
		}
		rel, ok := manyRels[relName]
		if !ok {
			return fmt.Errorf("unknown many relationship column %q", relName)
		}
		linkColName, err := schemaSvc.MustLinkColumnName(ctx, tid, rel.TargetTableId, rel.LinkColumnId)
		if err != nil {
			return err
		}
		outcome.ManyLinkColumns[relName] = linkColName
		children, err := s.saveGraphManyChildren(ctx, tx, tid, schemaSvc, rootID, rel, relInput)
		if err != nil {
			return fmt.Errorf("relationship %q: %w", relName, err)
		}
		outcome.Many[relName] = children
	}
	return nil
}

func (s *Data) saveGraphManyChildren(
	ctx context.Context,
	tx pgx.Tx,
	tid string,
	schemaSvc *schema.Schema,
	rootID string,
	rel shared.RelationshipColumn,
	relInput *graph.SaveGraphManyInput,
) ([]*graph.SaveGraphChildSaveOutcome, error) {
	childCols, childSchema, childTable, err := s.meta().LoadColumns(ctx, rel.TargetTableId)
	if err != nil {
		return nil, err
	}
	linkColName, err := schemaSvc.MustLinkColumnName(ctx, tid, rel.TargetTableId, rel.LinkColumnId)
	if err != nil {
		return nil, err
	}
	childOneRels, err := schemaSvc.LoadOneRelationshipColumns(ctx, rel.TargetTableId)
	if err != nil {
		return nil, err
	}
	childManyRels, err := schemaSvc.LoadManyRelationshipColumns(ctx, rel.TargetTableId)
	if err != nil {
		return nil, err
	}
	oneNames := schema.RelationshipColumnNamesSet(childOneRels)
	manyNames := schema.RelationshipColumnNamesSet(childManyRels)

	outcomes := make([]*graph.SaveGraphChildSaveOutcome, len(relInput.Data))
	var savedIDs []string

	for i, raw := range relInput.Data {
		payload, err := graph.ClassifySaveGraphRowPayload(raw, manyNames, oneNames)
		if err != nil {
			return nil, err
		}
		if payload.Delete {
			if payload.Id == "" {
				return nil, fmt.Errorf("delete requires id")
			}
			if err := s.deleteRowTx(ctx, tx, childSchema, childTable, payload.Id); err != nil {
				return nil, err
			}
			outcomes[i] = &graph.SaveGraphChildSaveOutcome{Deleted: true}
			continue
		}

		childOutcome := &graph.SaveGraphChildSaveOutcome{
			OneRelationships: map[string]*row.Row{},
		}

		cells := shared.NormalizeInputCells(payload.Cells, childCols)
		for oneName, oneInput := range payload.OneRelationships {
			childRel, ok := childOneRels[oneName]
			if !ok {
				return nil, fmt.Errorf("unknown one relationship column %q on child table", oneName)
			}
			relatedRow, fkVal, err := s.writeOneRelationshipRow(ctx, tx, childRel, oneInput)
			if err != nil {
				return nil, fmt.Errorf("relationship %q: %w", oneName, err)
			}
			if fkVal != nil {
				fkColName, err := schemaSvc.MustFKColumnName(ctx, tid, rel.TargetTableId, childRel.TargetColumnId)
				if err != nil {
					return nil, err
				}
				cells[fkColName] = fkVal
			}
			if relatedRow != nil {
				childOutcome.OneRelationships[oneName] = relatedRow
			}
		}

		cells, err = s.resolveLookupCells(ctx, tx, rel.TargetTableId, cells)
		if err != nil {
			return nil, err
		}
		cells[linkColName] = apiv1.StringValue(rootID)

		var childID string
		if payload.Id == "" {
			childID, err = s.insertRowTx(ctx, tx, childCols, childSchema, childTable, cells)
			if err != nil {
				return nil, err
			}
		} else {
			childID = payload.Id
			if err := s.updateRowTx(ctx, tx, childCols, childSchema, childTable, childID, cells); err != nil {
				return nil, err
			}
		}
		if childID != "" {
			savedIDs = append(savedIDs, childID)
			childOutcome.Row = &row.Row{Id: childID, Cells: cells}
		}
		outcomes[i] = childOutcome
	}

	if relInput.Sync.ReplaceMissing() {
		if err := s.deleteMissingChildrenTx(ctx, tx, childSchema, childTable, linkColName, rootID, savedIDs); err != nil {
			return nil, err
		}
	}
	return outcomes, nil
}

func (s *Data) emitSaveGraphHooks(ctx context.Context, isUpdate bool, tableID string, manyRels, oneRels map[string]shared.RelationshipColumn, outcome *graph.SaveGraphSaveOutcome) {
	if s.B.Events == nil || outcome == nil {
		return
	}
	evType := event.RecordsAfterInsert
	if isUpdate {
		evType = event.RecordsAfterUpdate
	}
	s.B.EmitEvent(ctx, evType, tableID, map[string]any{
		"row": shared.RowToMap(outcome.RootRow()),
	})
	for relName, row := range outcome.One {
		if row == nil {
			continue
		}
		targetTable := tableID
		if rel, ok := oneRels[relName]; ok {
			targetTable = rel.TargetTableId
		}
		s.B.EmitEvent(ctx, event.RecordsAfterBulkUpsert, targetTable, map[string]any{
			"rows": []any{shared.RowToMap(row)},
		})
	}
	for relName, children := range outcome.Many {
		rel, ok := manyRels[relName]
		if !ok {
			continue
		}
		var rows []any
		for _, child := range children {
			if child == nil || child.Deleted || child.Row == nil {
				continue
			}
			rows = append(rows, shared.RowToMap(child.Row))
		}
		if len(rows) == 0 {
			continue
		}
		s.B.EmitEvent(ctx, event.RecordsAfterBulkUpsert, rel.TargetTableId, map[string]any{
			"rows": rows,
		})
	}
}

func (s *Data) updateRowTx(ctx context.Context, tx pgx.Tx, cols []shared.ColumnMeta, schemaName, tableName, rowID string, cells map[string]*apiv1.Value) error {
	cells = shared.NormalizeInputCells(cells, cols)
	var setParts []string
	var args []any
	argIdx := 1
	for _, c := range cols {
		val, ok := cells[c.Name]
		if !ok {
			continue
		}
		setParts = append(setParts, fmt.Sprintf("%s = $%d", pgx.Identifier{c.Name}.Sanitize(), argIdx))
		args = append(args, shared.ValueToAnyForColumn(val, c.PgType))
		argIdx++
	}
	if len(setParts) == 0 {
		return nil
	}
	setParts = shared.TouchUpdatedAtSQL(setParts)
	args = append(args, rowID)
	update := fmt.Sprintf(`UPDATE %s.%s SET %s WHERE id = $%d`,
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{tableName}.Sanitize(),
		strings.Join(setParts, ", "),
		argIdx,
	)
	_, err := tx.Exec(ctx, update, args...)
	return err
}

func (s *Data) deleteRowTx(ctx context.Context, tx pgx.Tx, schemaName, tableName, rowID string) error {
	del := fmt.Sprintf(`DELETE FROM %s.%s WHERE id = $1`,
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{tableName}.Sanitize(),
	)
	_, err := tx.Exec(ctx, del, rowID)
	return err
}

func (s *Data) deleteMissingChildrenTx(ctx context.Context, tx pgx.Tx, schemaName, tableName, linkColName, parentID string, keepIDs []string) error {
	if len(keepIDs) == 0 {
		del := fmt.Sprintf(`DELETE FROM %s.%s WHERE %s = $1`,
			pgx.Identifier{schemaName}.Sanitize(),
			pgx.Identifier{tableName}.Sanitize(),
			pgx.Identifier{linkColName}.Sanitize(),
		)
		_, err := tx.Exec(ctx, del, parentID)
		return err
	}
	del := fmt.Sprintf(`DELETE FROM %s.%s WHERE %s = $1 AND NOT (id::text = ANY($2::text[]))`,
		pgx.Identifier{schemaName}.Sanitize(),
		pgx.Identifier{tableName}.Sanitize(),
		pgx.Identifier{linkColName}.Sanitize(),
	)
	_, err := tx.Exec(ctx, del, parentID, keepIDs)
	return err
}

// writeOneRelationshipRow creates or updates the related row for a cardinality-one relationship.
func (s *Data) writeOneRelationshipRow(
	ctx context.Context,
	tx pgx.Tx,
	rel shared.RelationshipColumn,
	input *graph.SaveGraphOneInput,
) (*row.Row, *apiv1.Value, error) {
	if input == nil {
		return nil, nil, nil
	}
	if input.Delete {
		if input.Id == "" {
			return nil, nil, fmt.Errorf("delete requires id")
		}
		cols, schemaName, tableName, err := s.meta().LoadColumns(ctx, rel.TargetTableId)
		if err != nil {
			return nil, nil, err
		}
		_ = cols
		if err := s.deleteRowTx(ctx, tx, schemaName, tableName, input.Id); err != nil {
			return nil, nil, err
		}
		return nil, nil, nil
	}

	cols, schemaName, tableName, err := s.meta().LoadColumns(ctx, rel.TargetTableId)
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
	return &row.Row{Id: relatedID, Cells: cells}, apiv1.StringValue(relatedID), nil
}
