package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"github.com/jackc/pgx/v5"
	"github.com/monoposer/lowcode-database/internal/columntype"
)

const lcColumnTypesTable = "lc_column_types"

func (s *Catalog) ListColumnTypes(ctx context.Context) ([]*ColumnTypeDef, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	rows, err := meta.Query(ctx, `
		SELECT name, label, spec, created_at, updated_at
		FROM lc_column_types WHERE tenant_id = $1 AND base_id = $2 ORDER BY name`, tid, baseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*ColumnTypeDef
	for rows.Next() {
		ct, err := scanColumnTypeRow(rows, baseID)
		if err != nil {
			return nil, err
		}
		out = append(out, ct)
	}
	return out, rows.Err()
}

func (s *Catalog) GetColumnType(ctx context.Context, id string) (*ColumnTypeDef, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	ct, err := s.loadColumnType(ctx, tid, id)
	if err != nil {
		return nil, err
	}
	return ct, nil
}

func (s *Catalog) CreateColumnType(ctx context.Context, req *ColumnTypeDef) (*ColumnTypeDef, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if _, ok := columntype.Get(name); ok && columntype.IsBuiltIn(name) {
		return nil, fmt.Errorf("name %q conflicts with built-in pgType", name)
	}
	if req.Spec == nil {
		return nil, fmt.Errorf("spec is required")
	}
	ct := columntype.NormalizeColumnType(columntype.ColumnType{
		Metadata: columntype.ColumnTypeMetadata{Name: name, Label: req.Label},
		Spec:     *req.Spec,
	})
	if err := columntype.ValidateColumnType(&ct); err != nil {
		return nil, err
	}
	specJSON, err := json.Marshal(ct.Spec)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	var createdAt, updatedAt time.Time
	err = meta.QueryRow(ctx, `
		INSERT INTO lc_column_types (tenant_id, base_id, name, label, spec)
		VALUES ($1, $2, $3, $4, $5::jsonb)
		ON CONFLICT (tenant_id, base_id, name) DO NOTHING
		RETURNING created_at, updated_at`,
		tid, baseID, name, ct.Metadata.Label, specJSON,
	).Scan(&createdAt, &updatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("columnType %q already exists", name)
		}
		return nil, fmt.Errorf("insert lc_column_types: %w", err)
	}
	out := &ColumnTypeDef{
		Id: name, Name: name, Label: ct.Metadata.Label, BaseId: baseID,
		Spec: &ct.Spec, RefKind: columntype.RefKindColumnType, CreatedAt: createdAt, UpdatedAt: updatedAt,
	}
	return out, nil
}

func (s *Catalog) UpdateColumnType(ctx context.Context, req *ColumnTypeDef) (*ColumnTypeDef, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return nil, err
	}
	cur, err := s.loadColumnType(ctx, tid, req.Id)
	if err != nil {
		return nil, err
	}
	if req.Label != "" {
		cur.Label = req.Label
	}
	if req.Spec != nil {
		ct := columntype.NormalizeColumnType(columntype.ColumnType{
			Metadata: columntype.ColumnTypeMetadata{Name: cur.Name, Label: cur.Label},
			Spec:     *req.Spec,
		})
		if err := columntype.ValidateColumnType(&ct); err != nil {
			return nil, err
		}
		cur.Spec = &ct.Spec
	}
	specJSON, err := json.Marshal(cur.Spec)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	_, err = meta.Exec(ctx, `
		UPDATE lc_column_types SET label = $4, spec = $5::jsonb, updated_at = now()
		WHERE tenant_id = $1 AND base_id = $2 AND name = $3`,
		tid, baseID, cur.Name, cur.Label, specJSON)
	if err != nil {
		return nil, err
	}
	ct, err := s.loadColumnType(ctx, tid, cur.Name)
	if err != nil {
		return nil, err
	}
	return ct, nil
}

func (s *Catalog) DeleteColumnType(ctx context.Context, id string) error {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return err
	}
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return err
	}
	cur, err := s.loadColumnType(ctx, tid, id)
	if err != nil {
		return err
	}
	meta := s.B.Tenants.MetaPool()
	var colCount int
	if err := meta.QueryRow(ctx, `
		SELECT COUNT(*) FROM lc_columns WHERE tenant_id = $1 AND base_id = $2 AND type_id = $3`,
		tid, baseID, cur.Name).Scan(&colCount); err != nil {
		return err
	}
	if colCount > 0 {
		return fmt.Errorf("columnType %q is used by %d column(s)", cur.Name, colCount)
	}
	if _, err := meta.Exec(ctx, `
		DELETE FROM lc_column_types WHERE tenant_id = $1 AND base_id = $2 AND name = $3`,
		tid, baseID, cur.Name); err != nil {
		return err
	}
	return nil
}

func (s *Catalog) ImportTypeCatalog(ctx context.Context, cat *columntype.TypeCatalog) (int, error) {
	if cat == nil {
		return 0, fmt.Errorf("catalog is required")
	}
	if err := columntype.ValidateTypeCatalog(cat); err != nil {
		return 0, err
	}
	n := 0
	for _, ct := range cat.ColumnTypes {
		ct = columntype.NormalizeColumnType(ct)
		_, err := s.CreateColumnType(ctx, &ColumnTypeDef{
			Name: ct.Metadata.Name, Label: ct.Metadata.Label,
			SchemaName: ct.Metadata.SchemaName, Spec: &ct.Spec,
		})
		if err != nil {
			return 0, fmt.Errorf("columnType %q: %w", ct.Metadata.Name, err)
		}
		n++
	}
	return n, nil
}

// ResolveColumnTypeRef reports whether typeID is a tenant columnType name.
func (s *Catalog) ResolveColumnTypeRef(ctx context.Context, tid, typeID string) (name string, ok bool, err error) {
	if typeID == "" {
		return "", false, nil
	}
	if _, err := columntype.Resolve(typeID); err == nil {
		return "", false, nil
	}
	ct, err := s.loadColumnType(ctx, tid, typeID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return "", false, nil
		}
		return "", false, err
	}
	return ct.Name, true, nil
}

func (s *Catalog) ColumnTypeDDL(ctx context.Context, tid, typeName string) (string, error) {
	ct, err := s.loadColumnType(ctx, tid, typeName)
	if err != nil {
		return "", err
	}
	_ = ct
	return typeName, nil
}

func (s *Catalog) loadColumnType(ctx context.Context, tid, name string) (*ColumnTypeDef, error) {
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	row := meta.QueryRow(ctx, `
		SELECT name, label, spec, created_at, updated_at
		FROM lc_column_types WHERE tenant_id = $1 AND base_id = $2 AND name = $3`, tid, baseID, name)
	ct, err := scanColumnTypeRow(row, baseID)
	if err != nil {
		return nil, fmt.Errorf("columnType %q not found: %w", name, err)
	}
	return ct, nil
}

type columnTypeScanner interface {
	Scan(dest ...any) error
}

func scanColumnTypeRow(row columnTypeScanner, baseID string) (*ColumnTypeDef, error) {
	var name, label string
	var specRaw []byte
	var createdAt, updatedAt time.Time
	if err := row.Scan(&name, &label, &specRaw, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	var spec columntype.ColumnTypeSpec
	if len(specRaw) > 0 {
		if err := json.Unmarshal(specRaw, &spec); err != nil {
			return nil, err
		}
	}
	return &ColumnTypeDef{
		Id: name, Name: name, Label: label, BaseId: baseID,
		Spec: &spec, RefKind: columntype.RefKindColumnType,
		CreatedAt: createdAt, UpdatedAt: updatedAt,
	}, nil
}
