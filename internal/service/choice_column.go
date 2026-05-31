package service

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/solat/lowcode-database/internal/columntype"
)

func choiceRefFromConfig(cfg map[string]any) string {
	if ref := cfgString(cfg, "choice_id"); ref != "" {
		return ref
	}
	return cfgString(cfg, "choice_name")
}

// resolveChoiceColumnRef decides if a column uses a PG ENUM (Choice).
// type_id should be the choice logical name; config.choice_name is optional legacy.
func (s *LowcodeService) resolveChoiceColumnRef(ctx context.Context, tid, typeID string, cfg map[string]any) (logicalName string, ok bool, err error) {
	if typeID == "enum" {
		return "", false, fmt.Errorf("type_id %q is not valid; use the choice logical name as typeId (see POST /v1/choices)", typeID)
	}
	if ref := choiceRefFromConfig(cfg); ref != "" {
		if _, _, e := s.resolveChoicePgType(ctx, tid, ref); e != nil {
			return "", false, e
		}
		return ref, true, nil
	}
	if _, _, e := s.resolveChoicePgType(ctx, tid, typeID); e == nil {
		return typeID, true, nil
	}
	return "", false, nil
}

func (s *LowcodeService) choiceColumnDDLType(ctx context.Context, tid, choiceRef string) (string, error) {
	enumSchema, enumType, err := s.resolveChoicePgType(ctx, tid, choiceRef)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s",
		pgx.Identifier{enumSchema}.Sanitize(),
		pgx.Identifier{enumType}.Sanitize(),
	), nil
}

// columnPgTypeSQL returns PostgreSQL type SQL for a column (built-in or choice).
func (s *LowcodeService) columnPgTypeSQL(ctx context.Context, tid, typeID string, cfg map[string]any) string {
	if columntype.IsBuiltIn(typeID) {
		return effectivePgType(columntype.PgType(typeID), columntype.Config(typeID))
	}
	ref := typeID
	if typeID == "enum" {
		ref = choiceRefFromConfig(cfg)
	}
	if ref == "" {
		return ""
	}
	ddl, err := s.choiceColumnDDLType(ctx, tid, ref)
	if err != nil {
		return ""
	}
	return ddl
}
