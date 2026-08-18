package data

import (
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/monoposer/lowcode-database/internal/columntype"
	"github.com/monoposer/lowcode-database/internal/dsl"
	"github.com/monoposer/lowcode-database/internal/query"
	"github.com/monoposer/lowcode-database/internal/service/shared"
)

const vrRecordAlias = "_r"

// vrFilterSQL turns a DSL filter ({"type","attr","val"}) into SQL predicates for record.data.
func vrFilterSQL(filter map[string]any, cols []shared.ColumnMeta, argN *int, args *[]any) ([]string, error) {
	return vrFilterSQLOn(filter, cols, pgx.Identifier{"link_ref"}.Sanitize(), argN, args)
}

func vrFilterSQLOn(filter map[string]any, cols []shared.ColumnMeta, linkRef string, argN *int, args *[]any) ([]string, error) {
	if filter == nil || len(filter) == 0 {
		return nil, nil
	}

	w, err := dsl.ParseCached(filter)
	if err != nil {
		return nil, err
	}
	if w.Type == "" {
		return nil, nil
	}

	attrMap, attrPgTypes := vrFilterAttrMaps(cols, linkRef)
	sql, wArgs, err := query.BuildWhereWithTypes(w, attrMap, attrPgTypes, *argN)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(sql) == "" {
		return nil, nil
	}
	*args = append(*args, wArgs...)
	*argN += len(wArgs)
	return []string{sql}, nil
}

func vrFilterAttrMaps(cols []shared.ColumnMeta, linkRef string) (attrMap, attrPgTypes map[string]string) {
	attrMap = map[string]string{}
	attrPgTypes = map[string]string{}
	if strings.TrimSpace(linkRef) == "" {
		linkRef = pgx.Identifier{"link_ref"}.Sanitize()
	}

	attrMap["id"] = vrRecordAlias + ".record_id"
	attrMap["record_id"] = vrRecordAlias + ".record_id"
	attrPgTypes["id"] = "text"
	attrPgTypes["record_id"] = "text"
	attrMap["created_at"] = vrRecordAlias + ".created_at"
	attrMap["updated_at"] = vrRecordAlias + ".updated_at"
	attrPgTypes["created_at"] = "timestamptz"
	attrPgTypes["updated_at"] = "timestamptz"
	attrMap["_fulltext_text"] = `COALESCE(` + vrRecordAlias + `.data->>'_fulltext_text','')`
	attrPgTypes["_fulltext_text"] = "text"

	for _, c := range cols {
		if c.Name == "" || c.Name == "id" || c.Name == "record_id" || c.Name == "created_at" || c.Name == "updated_at" {
			if (c.Name == "created_at" || c.Name == "updated_at") && c.Id != "" && c.Id != c.Name {
				attrMap[c.Id] = vrRecordAlias + "." + c.Name
				attrPgTypes[c.Id] = "timestamptz"
			}
			continue
		}
		var expr, pgType string
		if columntype.IsLinkType(c.TypeId) {
			expr = vrLinkIDsExpr(linkRef, c.Name)
			pgType = "text[]"
		} else {
			pgType = strings.TrimSpace(c.PgType)
			if pgType == "" {
				pgType = shared.VirtualColumnPgType(c.TypeId, nil)
			}
			if strings.HasSuffix(strings.ToLower(pgType), "[]") {
				expr = vrArrayExpr(c.Name, pgType)
			} else {
				expr = vrTypedScalarExpr(c.Name, pgType)
			}
		}
		attrMap[c.Name] = expr
		if c.Id != "" {
			attrMap[c.Id] = expr
		}
		if pgType != "" {
			attrPgTypes[c.Name] = pgType
			if c.Id != "" {
				attrPgTypes[c.Id] = pgType
			}
		}
	}
	return attrMap, attrPgTypes
}

func vrLinkIDsExpr(linkRef, fieldKey string) string {
	return fmt.Sprintf(`COALESCE((
		SELECT array_agg(lr.to_record_id)
		FROM %s lr
		WHERE lr.tenant_id = %s.tenant_id
		  AND lr.from_record_id = %s.record_id
		  AND lr.from_field_id = %s
	), '{}'::text[])`, linkRef, vrRecordAlias, vrRecordAlias, jsonbKeyLit(fieldKey))
}

func jsonbKeyLit(name string) string {
	return "'" + strings.ReplaceAll(name, "'", "''") + "'"
}

// vrTypedScalarExpr reads a scalar (or calc-cache wrapper) from record.data.
func vrTypedScalarExpr(name, pgType string) string {
	k := jsonbKeyLit(name)
	base := fmt.Sprintf(
		`(CASE WHEN jsonb_typeof(%s.data->%s)='object' AND (%s.data->%s) ? 'value' THEN %s.data->%s->>'value' ELSE %s.data->>%s END)`,
		vrRecordAlias, k, vrRecordAlias, k, vrRecordAlias, k, vrRecordAlias, k,
	)
	switch strings.ToLower(strings.TrimSpace(pgType)) {
	case "numeric", "bigint", "integer", "int4", "int8", "double precision", "float8":
		return "(" + base + ")::numeric"
	case "boolean", "bool":
		return "(" + base + ")::boolean"
	case "timestamptz", "timestamp with time zone", "timestamp", "date":
		return "(" + base + ")::timestamptz"
	default:
		return base
	}
}

// vrArrayExpr converts a jsonb array (or calc-cache {value:[...]}) into a PG array.
func vrArrayExpr(name, arrayPgType string) string {
	k := jsonbKeyLit(name)
	cast := strings.TrimSpace(arrayPgType)
	if cast == "" || !strings.HasSuffix(strings.ToLower(cast), "[]") {
		cast = "text[]"
	}
	return fmt.Sprintf(`COALESCE((
		CASE
			WHEN jsonb_typeof(%[1]s.data->%[2]s)='array' THEN
				ARRAY(SELECT jsonb_array_elements_text(%[1]s.data->%[2]s))
			WHEN jsonb_typeof(%[1]s.data->%[2]s)='object' AND (%[1]s.data->%[2]s) ? 'value' AND jsonb_typeof(%[1]s.data->%[2]s->'value')='array' THEN
				ARRAY(SELECT jsonb_array_elements_text(%[1]s.data->%[2]s->'value'))
			ELSE NULL
		END
	)::%[3]s, '{}'::%[3]s)`, vrRecordAlias, k, cast)
}

// vrFilterColumns prefers full column meta (incl. formula/lookup cache + type-level array pg types).
func vrFilterColumns(phys []shared.ColumnMeta, all []shared.FullColumnMeta) []shared.ColumnMeta {
	if len(all) == 0 {
		return phys
	}
	out := make([]shared.ColumnMeta, 0, len(all))
	for _, c := range all {
		if c.Name == "" {
			continue
		}
		out = append(out, shared.ColumnMeta{
			Id: c.Id, TableName: c.TableName, Name: c.Name, TypeId: c.TypeId,
			PgType: filterColumnPgType(c), IsNullable: c.IsNullable, Position: c.Position,
		})
	}
	return out
}

func filterColumnPgType(c shared.FullColumnMeta) string {
	if pg := shared.VirtualColumnPgType(c.TypeId, c.Config); pg != "" {
		return pg
	}
	return c.PgType
}
