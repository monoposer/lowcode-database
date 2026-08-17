package data

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/monoposer/lowcode-database/internal/event"
	"github.com/monoposer/lowcode-database/internal/service/calc"
	"github.com/monoposer/lowcode-database/internal/service/shared"
	"github.com/monoposer/lowcode-database/pkg/infra/postgres"
)

func (s *Data) resolveVRContext(ctx context.Context, tableName string) (tid, tenantID, vtID string, err error) {
	tid, err = s.B.TenantID(ctx)
	if err != nil {
		return "", "", "", err
	}
	tenantID, err = s.B.TenantID(ctx)
	if err != nil {
		return "", "", "", err
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return "", "", "", err
	}
	vtID, err = s.B.Tenants.TableVTID(ctx, tenantID, baseID, tableName)
	if err != nil {
		return "", "", "", err
	}
	return tid, tenantID, vtID, nil
}

func (s *Data) CreateRow(ctx context.Context, req *CreateRowRequest) (*CreateRowResponse, error) {
	ctx, tables, err := s.B.Tenants.AttachDataTables(ctx)
	if err != nil {
		return nil, err
	}
	_, tenantID, vtID, err := s.resolveVRContext(ctx, req.TableName)
	if err != nil {
		return nil, err
	}
	pool, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	tableName := req.TableName
	cols, _, _, err := s.meta().LoadColumns(ctx, tableName)
	if err != nil {
		return nil, err
	}
	if len(req.Cells) == 0 {
		return nil, fmt.Errorf("cells is empty")
	}

	native := cellsToNativeMap(req.Cells, cols)
	allCols, _, _, _ := s.meta().LoadAllColumnMeta(ctx, tableName)
	dataMap, linkMap := splitLinkCells(req.Cells, allCols)
	if len(dataMap) == 0 && len(native) > 0 {
		// keep scalars from native minus links
		dataMap = native
		for k := range linkMap {
			delete(dataMap, k)
		}
	}
	dataMap = s.applyFulltextOnWrite(ctx, tableName, cols, dataMap)

	recordID := uuid.NewString()
	tbl := tables.QRecord()
	payload, err := json.Marshal(dataMap)
	if err != nil {
		return nil, err
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s (record_id, tenant_id, vt_id, data, version)
		VALUES ($1, $2, $3, $4::jsonb, 1)`, tbl),
		recordID, tenantID, vtID, payload); err != nil {
		return nil, err
	}
	if err := s.persistLinks(ctx, pool, tenantID, tableName, recordID, allCols, linkMap); err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	_ = calc.EnqueueAfterUserEdit(ctx, meta, pool, tenantID, tableName, recordID, changedKeys(dataMap), len(linkMap) > 0)

	pending, _ := calc.HasPending(ctx, pool, recordID)
	resp := &CreateRowResponse{
		Row: &Row{
			Id:      recordID,
			Version: 1,
			Pending: pending,
			Cells:   hydrateCells(dataMap, allCols, linkMap, pending),
		},
	}
	s.B.EmitEvent(ctx, event.RecordsAfterInsert, tableName, map[string]any{"row": RowToMap(resp.Row)})
	return resp, nil
}

func (s *Data) UpdateRow(ctx context.Context, req *UpdateRowRequest) (*UpdateRowResponse, error) {
	ctx, tables, err := s.B.Tenants.AttachDataTables(ctx)
	if err != nil {
		return nil, err
	}
	_, tenantID, vtID, err := s.resolveVRContext(ctx, req.TableName)
	if err != nil {
		return nil, err
	}
	pool, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	tableName := req.TableName
	cols, _, _, err := s.meta().LoadColumns(ctx, tableName)
	if err != nil {
		return nil, err
	}
	if len(req.Cells) == 0 {
		return nil, fmt.Errorf("cells is empty")
	}

	native := cellsToNativeMap(req.Cells, cols)
	allCols, _, _, _ := s.meta().LoadAllColumnMeta(ctx, tableName)
	patch, linkMap := splitLinkCells(req.Cells, allCols)
	if len(patch) == 0 {
		for k, v := range native {
			patch[k] = v
		}
		for k := range linkMap {
			delete(patch, k)
		}
	}
	tbl := tables.QRecord()
	var dataJSON []byte
	var version int64
	sel := fmt.Sprintf(`SELECT data, version FROM %s WHERE record_id = $1`, tbl)
	sel, selArgs, _ := postgres.AndWhere(sel, []any{req.RowId}, 2,
		postgres.Eq{Col: "vt_id", Val: vtID}, postgres.Eq{Col: "tenant_id", Val: tenantID})
	if err := pool.QueryRow(ctx, sel, selArgs...).Scan(&dataJSON, &version); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("row not found")
		}
		return nil, err
	}
	merged := map[string]any{}
	if len(dataJSON) > 0 {
		_ = json.Unmarshal(dataJSON, &merged)
	}
	for k, v := range patch {
		merged[k] = v
	}
	merged = s.applyFulltextOnWrite(ctx, tableName, cols, merged)

	payload, err := json.Marshal(merged)
	if err != nil {
		return nil, err
	}
	tag, err := pool.Exec(ctx, fmt.Sprintf(`
		UPDATE %s SET data = $1::jsonb, version = version + 1, updated_at = now()
		WHERE record_id = $2 AND vt_id = $3 AND tenant_id = $4 AND version = $5`, tbl),
		payload, req.RowId, vtID, tenantID, version)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("row not found or version conflict")
	}
	if err := s.persistLinks(ctx, pool, tenantID, tableName, req.RowId, allCols, linkMap); err != nil {
		return nil, err
	}
	_ = calc.EnqueueAfterUserEdit(ctx, s.B.Tenants.MetaPool(), pool, tenantID, tableName, req.RowId, changedKeys(patch), len(linkMap) > 0)

	pending, _ := calc.HasPending(ctx, pool, req.RowId)
	links, _ := calc.LinksByRecords(ctx, pool, tenantID, []string{req.RowId})
	resp := &UpdateRowResponse{
		Row: &Row{
			Id:      req.RowId,
			Version: version + 1,
			Pending: pending,
			Cells:   hydrateCells(merged, allCols, links[req.RowId], pending),
		},
	}
	s.B.EmitEvent(ctx, event.RecordsAfterUpdate, tableName, map[string]any{"row": RowToMap(resp.Row)})
	return resp, nil
}

func (s *Data) DeleteRow(ctx context.Context, req *DeleteRowRequest) (*DeleteRowResponse, error) {
	ctx, tables, err := s.B.Tenants.AttachDataTables(ctx)
	if err != nil {
		return nil, err
	}
	_, tenantID, vtID, err := s.resolveVRContext(ctx, req.TableName)
	if err != nil {
		return nil, err
	}
	pool, err := s.B.Tenants.DataPool(ctx)
	if err != nil {
		return nil, err
	}
	tableName := req.TableName
	incoming, _ := calc.ListIncoming(ctx, pool, tenantID, req.RowId)
	tbl := tables.QRecord()
	del := fmt.Sprintf(`DELETE FROM %s WHERE record_id = $1`, tbl)
	del, delArgs, _ := postgres.AndWhere(del, []any{req.RowId}, 2,
		postgres.Eq{Col: "vt_id", Val: vtID}, postgres.Eq{Col: "tenant_id", Val: tenantID})
	tag, err := pool.Exec(ctx, del, delArgs...)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("row not found")
	}
	seen := map[string]bool{}
	for _, e := range incoming {
		if seen[e.FromRecordID] {
			continue
		}
		seen[e.FromRecordID] = true
		_ = calc.Enqueue(ctx, pool, tenantID, e.FromTableName, e.FromRecordID, nil)
	}
	s.B.EmitEvent(ctx, event.RecordsAfterDelete, tableName, map[string]any{"rowId": req.RowId})
	return &DeleteRowResponse{}, nil
}

func (s *Data) executeVRQuery(ctx context.Context, spec querySpec) (*QueryRowsResponse, error) {
	ctx, tables, err := s.B.Tenants.AttachDataTables(ctx)
	if err != nil {
		return nil, err
	}
	_, tenantID, vtID, err := s.resolveVRContext(ctx, spec.TableName)
	if err != nil {
		return nil, err
	}
	pool, err := s.B.Tenants.DataReadPool(ctx)
	if err != nil {
		return nil, err
	}

	physCols, _, _, err := s.meta().LoadColumns(ctx, spec.TableName)
	if err != nil {
		return nil, err
	}
	allCols, _, _, _ := s.meta().LoadAllColumnMeta(ctx, spec.TableName)

	pageSize := spec.PageSize
	if pageSize <= 0 {
		pageSize = s.B.MaxRow
	}
	if pageSize <= 0 {
		pageSize = 100
	}
	maxScan := s.maxScanRows()
	if spec.PageSize > maxScan {
		return nil, fmt.Errorf("pageSize %d exceeds max scan rows %d", spec.PageSize, maxScan)
	}
	if pageSize > maxScan {
		pageSize = maxScan
	}
	if len(spec.Filter) == 0 && spec.PageToken == "" && spec.PageSize <= 0 {
		// Unbounded list is paginated; never scan more than maxScan rows per request.
		pageSize = min32(pageSize, maxScan)
	}

	where := `vt_id = $1 AND tenant_id = $2`
	args := []any{vtID, tenantID}
	argN := 3

	if spec.PageToken != "" {
		where += fmt.Sprintf(` AND record_id > $%d`, argN)
		args = append(args, spec.PageToken)
		argN++
	}

	if spec.Filter != nil {
		filterCols := vrFilterColumns(physCols, allCols)
		preds, err := vrFilterSQL(spec.Filter, filterCols, &argN, &args)
		if err != nil {
			return nil, err
		}
		if len(preds) > 0 {
			where += " AND " + strings.Join(preds, " AND ")
		}
	}

	tbl := tables.QRecord()
	order := ` ORDER BY record_id`
	countLimitArg := argN
	countArgs := append(append([]any{}, args...), maxScan)
	countSQL := fmt.Sprintf(`SELECT COUNT(*) FROM (SELECT 1 FROM %s WHERE %s LIMIT $%d) _scan_cap`, tbl, where, countLimitArg)

	var total int32
	if err := pool.QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, err
	}

	limitArg := argN
	queryArgs := append(append([]any{}, args...), pageSize+1)
	selectSQL := fmt.Sprintf(`
		SELECT record_id, data, version, created_at, updated_at FROM %s WHERE %s%s LIMIT $%d`,
		tbl, where, order, limitArg)

	rows, err := pool.Query(ctx, selectSQL, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out QueryRowsResponse
	var lastID string
	var ids []string
	type scanned struct {
		id      string
		data    map[string]any
		version int64
	}
	var scannedRows []scanned
	for rows.Next() {
		var id string
		var dataJSON []byte
		var ver int64
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &dataJSON, &ver, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		lastID = id
		native := map[string]any{}
		if len(dataJSON) > 0 {
			_ = json.Unmarshal(dataJSON, &native)
		}
		if _, ok := native["created_at"]; !ok && !createdAt.IsZero() {
			native["created_at"] = createdAt.UTC().Format(time.RFC3339Nano)
		}
		if _, ok := native["updated_at"]; !ok && !updatedAt.IsZero() {
			native["updated_at"] = updatedAt.UTC().Format(time.RFC3339Nano)
		}
		ids = append(ids, id)
		scannedRows = append(scannedRows, scanned{id: id, data: native, version: ver})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	pending, _ := calc.PendingRecordIDs(ctx, pool, ids)
	linkMap, _ := calc.LinksByRecords(ctx, pool, tenantID, ids)
	for _, sc := range scannedRows {
		isPend := pending[sc.id]
		r := &Row{
			Id:      sc.id,
			Version: sc.version,
			Pending: isPend,
			Cells:   hydrateCells(sc.data, allCols, linkMap[sc.id], isPend),
		}
		out.Rows = append(out.Rows, r)
	}

	if int32(len(out.Rows)) > pageSize {
		out.Rows = out.Rows[:pageSize]
		out.NextPageToken = lastID
	}
	out.Count = total
	return &out, nil
}

func cellsToNativeMap(cells map[string]*shared.Value, cols []shared.ColumnMeta) map[string]any {
	normalized := shared.NormalizeInputCells(cells, cols)
	out := make(map[string]any, len(normalized))
	for k, v := range normalized {
		out[k] = shared.ValueToAnyForColumn(v, "")
	}
	return out
}

func ftsQuery(q string) string {
	parts := strings.Fields(q)
	if len(parts) == 0 {
		return ""
	}
	for i, p := range parts {
		p = strings.ReplaceAll(p, "'", "")
		p = strings.ReplaceAll(p, ":", "")
		parts[i] = p
	}
	return strings.Join(parts, " & ")
}

func (s *Data) GetRow(ctx context.Context, req *GetRowRequest) (*GetRowResponse, error) {
	ctx = withReadConsistency(ctx, req.Consistency)
	ctx, _, err := s.B.Tenants.AttachDataTables(ctx)
	if err != nil {
		return nil, err
	}
	_, tenantID, vtID, err := s.resolveVRContext(ctx, req.TableName)
	if err != nil {
		return nil, err
	}
	pool, err := s.B.Tenants.DataReadPool(ctx)
	if err != nil {
		return nil, err
	}
	allCols, _, _, err := s.meta().LoadAllColumnMeta(ctx, req.TableName)
	if err != nil {
		return nil, err
	}
	eng := &calc.Engine{Meta: s.B.Tenants.MetaPool(), Data: pool}
	live, ver, err := eng.LiveCompute(ctx, tenantID, req.TableName, req.RowId)
	if err != nil {
		return nil, err
	}
	rec, err := calc.ReadRecord(ctx, pool, tenantID, vtID, req.RowId)
	if err != nil {
		return nil, err
	}
	pending, _ := calc.HasPending(ctx, pool, req.RowId)
	if !pending && calc.CacheMismatch(rec.Data, live, fieldsFromMeta(allCols)) {
		writePool, werr := s.B.Tenants.DataPool(ctx)
		if werr == nil {
			_ = calc.Enqueue(ctx, writePool, tenantID, req.TableName, req.RowId, nil)
			pending = true
		}
	}
	links, _ := calc.LinksByRecords(ctx, pool, tenantID, []string{req.RowId})
	return &GetRowResponse{Row: &Row{
		Id:      req.RowId,
		Version: ver,
		Pending: pending,
		Cells:   hydrateCells(live, allCols, links[req.RowId], pending),
	}}, nil
}

func fieldsFromMeta(cols []shared.FullColumnMeta) []calc.Field {
	out := make([]calc.Field, 0, len(cols))
	for _, c := range cols {
		out = append(out, calc.Field{ID: c.Id, Name: c.Name, TypeID: c.TypeId, Config: c.Config})
	}
	return out
}
