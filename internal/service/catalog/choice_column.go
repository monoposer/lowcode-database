package catalog

import (
	"github.com/solat/lowcode-database/internal/service/shared"
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/solat/lowcode-database/internal/columntype"
)

func choiceRefFromConfig(cfg map[string]any) string {
	if ref := shared.CfgString(cfg, "choice_id"); ref != "" {
		return ref
	}
	return shared.CfgString(cfg, "choice_name")
}

// ResolveChoiceColumnRef decides if a column uses a PG ENUM (Choice).
// type_id should be the choice logical name; config.choice_name is optional legacy.
func (s *Catalog) ResolveChoiceColumnRef(ctx context.Context, tid, typeID string, cfg map[string]any) (logicalName string, ok bool, err error) {
	if typeID == "enum" {
		return "", false, fmt.Errorf("type_id %q is not valid; use the choice logical name as typeId (see POST /v1/choices)", typeID)
	}
	if ref := choiceRefFromConfig(cfg); ref != "" {
		if _, _, e := s.ResolveChoicePgType(ctx, tid, ref); e != nil {
			return "", false, e
		}
		return ref, true, nil
	}
	if _, _, e := s.ResolveChoicePgType(ctx, tid, typeID); e == nil {
		return typeID, true, nil
	}
	return "", false, nil
}

func (s *Catalog) ChoiceColumnDDLType(ctx context.Context, tid, choiceRef string) (string, error) {
	enumSchema, enumType, err := s.ResolveChoicePgType(ctx, tid, choiceRef)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s",
		pgx.Identifier{enumSchema}.Sanitize(),
		pgx.Identifier{enumType}.Sanitize(),
	), nil
}

// ColumnPgTypeSQL returns PostgreSQL type SQL for a column (built-in or choice).
func (s *Catalog) ColumnPgTypeSQL(ctx context.Context, tid, typeID string, cfg map[string]any) string {
	if columntype.Kind(typeID) == "relation_fk" {
		if pg, err := s.B.RelationFKColumnPgType(ctx, tid, cfg); err == nil && pg != "" {
			return pg
		}
	}
	if columntype.IsBuiltIn(typeID) {
		return shared.EffectivePgType(columntype.PgType(typeID), columntype.Config(typeID))
	}
	ref := typeID
	if typeID == "enum" {
		ref = choiceRefFromConfig(cfg)
	}
	if ref == "" {
		return ""
	}
	ddl, err := s.ChoiceColumnDDLType(ctx, tid, ref)
	if err != nil {
		return ""
	}
	return ddl
}
