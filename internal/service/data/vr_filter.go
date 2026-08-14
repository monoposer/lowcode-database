package data

import (
	"fmt"
	"strings"

	"github.com/monoposer/lowcode-database/internal/dsl"
	"github.com/monoposer/lowcode-database/internal/query"
	"github.com/monoposer/lowcode-database/internal/service/shared"
)

// vrFilterSQL turns a filter object into SQL predicates for record.data.
// Supports:
//   - DSL ({"type":"AND","val":[...]}, {"type":"ARRAY_HAS","attr":"tags","val":"x"}, …)
//   - legacy ({"op":"and","children":[...]}, {"field","op","value"}, in_record_ids / fts)
func vrFilterSQL(filter map[string]any, cols []shared.ColumnMeta, argN *int, args *[]any) ([]string, error) {
	if filter == nil || len(filter) == 0 {
		return nil, nil
	}
	if op, _ := filter["op"].(string); op == "in_record_ids" {
		return vrFilterInRecordIDs(filter, argN, args)
	}
	if isLegacyFTS(filter) {
		return vrFilterLegacyFTS(filter, argN, args)
	}

	normalized, err := normalizeVRFilter(filter)
	if err != nil {
		return nil, err
	}
	if normalized == nil {
		return nil, nil
	}

	w, err := dsl.ParseCached(normalized)
	if err != nil {
		return nil, err
	}
	if w.Type == "" {
		return nil, nil
	}

	attrMap, attrPgTypes := vrFilterAttrMaps(cols)
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

func vrFilterInRecordIDs(filter map[string]any, argN *int, args *[]any) ([]string, error) {
	ids, _ := filter["ids"].([]string)
	if ids == nil {
		if raw, ok := filter["ids"].([]any); ok {
			for _, x := range raw {
				ids = append(ids, fmt.Sprint(x))
			}
		}
	}
	pred := fmt.Sprintf(`record_id = ANY($%d::text[])`, *argN)
	*args = append(*args, ids)
	*argN++
	return []string{pred}, nil
}

func isLegacyFTS(filter map[string]any) bool {
	op, _ := filter["op"].(string)
	switch strings.ToLower(op) {
	case "fts", "fulltext", "search":
		return true
	default:
		return false
	}
}

func vrFilterLegacyFTS(filter map[string]any, argN *int, args *[]any) ([]string, error) {
	val := filter["value"]
	pred := fmt.Sprintf(
		`to_tsvector('simple', COALESCE(data->>'_fulltext_text','')) @@ to_tsquery('simple', $%d)`,
		*argN,
	)
	*args = append(*args, ftsQuery(fmt.Sprint(val)))
	*argN++
	return []string{pred}, nil
}

// normalizeVRFilter converts legacy shapes into DSL; leaves DSL unchanged.
func normalizeVRFilter(filter map[string]any) (map[string]any, error) {
	if filter == nil {
		return nil, nil
	}
	if _, ok := filter["type"]; ok {
		return filter, nil
	}
	op, _ := filter["op"].(string)
	opLower := strings.ToLower(strings.TrimSpace(op))

	if opLower == "and" || opLower == "or" {
		children, _ := filter["children"].([]any)
		val := make([]any, 0, len(children))
		for _, ch := range children {
			m, ok := ch.(map[string]any)
			if !ok {
				continue
			}
			n, err := normalizeVRFilter(m)
			if err != nil {
				return nil, err
			}
			if n != nil {
				val = append(val, n)
			}
		}
		if len(val) == 0 {
			return nil, nil
		}
		return map[string]any{
			"type": strings.ToUpper(opLower),
			"val":  val,
		}, nil
	}

	field, _ := filter["field"].(string)
	if field == "" {
		return nil, nil
	}
	dslType, err := legacyOpToDSL(opLower)
	if err != nil {
		return nil, err
	}
	out := map[string]any{"type": dslType, "attr": field}
	if dslType != "EMPTY" && dslType != "NOT_EMPTY" {
		out["val"] = filter["value"]
	}
	return out, nil
}

func legacyOpToDSL(op string) (string, error) {
	switch op {
	case "", "eq":
		return "EQ", nil
	case "neq", "ne", "!=":
		return "NEQ", nil
	case "like", "ilike":
		return "LIKE", nil
	case "gt":
		return "GT", nil
	case "gte":
		return "GTE", nil
	case "lt":
		return "LT", nil
	case "lte":
		return "LTE", nil
	case "in":
		return "IN", nil
	case "nin", "not_in":
		return "NIN", nil
	case "empty", "null":
		return "EMPTY", nil
	case "not_empty", "not_null":
		return "NOT_EMPTY", nil
	case "array_has", "has":
		return "ARRAY_HAS", nil
	case "array_not_has", "not_has":
		return "ARRAY_NOT_HAS", nil
	case "array_overlap", "overlap":
		return "ARRAY_OVERLAP", nil
	case "array_not_overlap":
		return "ARRAY_NOT_OVERLAP", nil
	case "array_contains", "contains":
		return "ARRAY_CONTAINS", nil
	case "array_not_contains":
		return "ARRAY_NOT_CONTAINS", nil
	default:
		return "", fmt.Errorf("virtual_records: filter op %q not supported", op)
	}
}

func vrFilterAttrMaps(cols []shared.ColumnMeta) (attrMap, attrPgTypes map[string]string) {
	attrMap = map[string]string{}
	attrPgTypes = map[string]string{}

	attrMap["id"] = "record_id"
	attrMap["record_id"] = "record_id"
	attrPgTypes["id"] = "text"
	attrPgTypes["record_id"] = "text"

	for _, c := range cols {
		if c.Name == "" {
			continue
		}
		pgType := strings.TrimSpace(c.PgType)
		var expr string
		if strings.HasSuffix(strings.ToLower(pgType), "[]") {
			expr = vrArrayExpr(c.Name, pgType)
		} else {
			expr = vrTypedScalarExpr(c.Name, pgType)
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

func jsonbKeyLit(name string) string {
	return "'" + strings.ReplaceAll(name, "'", "''") + "'"
}

// vrTypedScalarExpr reads a scalar (or calc-cache wrapper) from record.data.
func vrTypedScalarExpr(name, pgType string) string {
	k := jsonbKeyLit(name)
	base := fmt.Sprintf(
		`(CASE WHEN jsonb_typeof(data->%s)='object' AND (data->%s) ? 'value' THEN data->%s->>'value' ELSE data->>%s END)`,
		k, k, k, k,
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
			WHEN jsonb_typeof(data->%s)='array' THEN
				ARRAY(SELECT jsonb_array_elements_text(data->%s))
			WHEN jsonb_typeof(data->%s)='object' AND (data->%s) ? 'value' AND jsonb_typeof(data->%s->'value')='array' THEN
				ARRAY(SELECT jsonb_array_elements_text(data->%s->'value'))
			ELSE NULL
		END
	)::%s, '{}'::%s)`, k, k, k, k, k, k, cast, cast)
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
			PgType: c.PgType, IsNullable: c.IsNullable, Position: c.Position,
		})
	}
	return out
}
