package calc

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	formulacompile "github.com/monoposer/lowcode-database/internal/formula"
	"github.com/monoposer/lowcode-database/internal/service/shared"
)

type Field struct {
	ID     string
	Name   string
	TypeID string
	Config map[string]any
}

func (f Field) key() string {
	if f.Name != "" {
		return f.Name
	}
	return f.ID
}

func ComputeFields(ctx context.Context, pool *pgxpool.Pool, rec *Rec, fields []Field, _ map[string]string, vtByTable map[string]string, only []string) (map[string]any, error) {
	want := map[string]bool{}
	for _, id := range only {
		if id != "" {
			want[id] = true
		}
	}
	byName := map[string]Field{}
	for _, f := range fields {
		byName[f.Name] = f
		byName[f.ID] = f
	}

	var formulaDefs []formulacompile.Def
	var lookups, rollups []Field
	for _, f := range fields {
		if len(want) > 0 && !want[f.Name] && !want[f.ID] {
			continue
		}
		switch strings.ToLower(f.TypeID) {
		case "formula":
			expr := shared.FormulaExpression(f.Config)
			if expr != "" {
				formulaDefs = append(formulaDefs, formulacompile.Def{Name: f.Name, Expr: expr})
			}
		case "lookup":
			lookups = append(lookups, f)
		case "rollup":
			rollups = append(rollups, f)
		}
	}

	out := map[string]any{}
	ordered, err := formulacompile.Sort(formulaDefs)
	if err != nil {
		return nil, err
	}
	env := withUnwrapped(rec.Data)
	for _, d := range ordered {
		v, err := formulacompile.EvalExpr(d.Expr, env)
		if err != nil {
			out[d.Name] = WrapCache(nil, CacheError)
			continue
		}
		out[d.Name] = WrapCache(v, CacheValid)
		env[d.Name] = v
		rec.Data[d.Name] = out[d.Name]
	}

	for _, f := range lookups {
		v, err := computeLookup(ctx, pool, rec, f, fields, vtByTable)
		if err != nil {
			out[f.Name] = WrapCache(nil, CacheError)
			continue
		}
		out[f.Name] = WrapCache(v, CacheValid)
	}
	for _, f := range rollups {
		v, err := computeRollup(ctx, pool, rec, f, vtByTable)
		if err != nil {
			out[f.Name] = WrapCache(nil, CacheError)
			continue
		}
		out[f.Name] = WrapCache(v, CacheValid)
	}
	return out, nil
}

func withUnwrapped(data map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range data {
		u := UnwrapCache(v)
		out[k] = u
		// also keep wrapper so JSONBCellSQL still works
		if _, ok := asMap(v); ok {
			out[k] = v
		} else {
			out[k] = u
		}
	}
	return out
}

func computeLookup(ctx context.Context, pool *pgxpool.Pool, rec *Rec, f Field, _ []Field, vtByTable map[string]string) (any, error) {
	linkField := firstCfg(f.Config, "link_field_id", "relation_column_id")
	targetField := firstCfg(f.Config, "target_field_id", "target_column_id")
	if linkField == "" || targetField == "" {
		return nil, fmt.Errorf("lookup %s missing link/target", f.Name)
	}
	toIDs, err := ListToIDs(ctx, pool, rec.WSID, rec.ID, linkField)
	if err != nil {
		return nil, err
	}
	if len(toIDs) == 0 {
		return nil, nil
	}
	toTable := firstCfg(f.Config, "to_table_id", "target_table_id")
	vtID := vtByTable[toTable]
	if vtID == "" {
		// try using stored to_table_id as vt
		vtID = toTable
	}
	recs, err := ReadRecords(ctx, pool, rec.WSID, vtID, toIDs)
	if err != nil {
		return nil, err
	}
	var values []any
	for _, id := range toIDs {
		r := recs[id]
		if r == nil {
			continue
		}
		values = append(values, UnwrapCache(r.Data[targetField]))
	}
	if len(values) == 0 {
		return nil, nil
	}
	if len(values) == 1 {
		return values[0], nil
	}
	return values, nil
}

func computeRollup(ctx context.Context, pool *pgxpool.Pool, rec *Rec, f Field, vtByTable map[string]string) (any, error) {
	linkField := firstCfg(f.Config, "link_field_id", "relation_column_id")
	targetField := firstCfg(f.Config, "target_field_id", "target_column_id")
	agg := strings.ToLower(firstCfg(f.Config, "aggregation", "aggregate"))
	if agg == "" {
		agg = "count"
	}
	childIDs, err := ListToIDs(ctx, pool, rec.WSID, rec.ID, linkField)
	if err != nil {
		return nil, err
	}
	if agg == "count" {
		return float64(len(childIDs)), nil
	}
	toTable := firstCfg(f.Config, "to_table_id", "target_table_id")
	vtID := vtByTable[toTable]
	if vtID == "" {
		vtID = toTable
	}
	recs, err := ReadRecords(ctx, pool, rec.WSID, vtID, childIDs)
	if err != nil {
		return nil, err
	}
	var nums []float64
	for _, id := range childIDs {
		r := recs[id]
		if r == nil {
			continue
		}
		n, ok := toFloat(UnwrapCache(r.Data[targetField]))
		if ok {
			nums = append(nums, n)
		}
	}
	if len(nums) == 0 {
		return nil, nil
	}
	switch agg {
	case "sum":
		s := 0.0
		for _, n := range nums {
			s += n
		}
		return s, nil
	case "min":
		m := nums[0]
		for _, n := range nums[1:] {
			if n < m {
				m = n
			}
		}
		return m, nil
	case "max":
		m := nums[0]
		for _, n := range nums[1:] {
			if n > m {
				m = n
			}
		}
		return m, nil
	case "avg":
		s := 0.0
		for _, n := range nums {
			s += n
		}
		return s / float64(len(nums)), nil
	default:
		return nil, fmt.Errorf("unsupported aggregation %q", agg)
	}
}

func firstCfg(cfg map[string]any, keys ...string) string {
	for _, k := range keys {
		if s := shared.CfgString(cfg, k); s != "" {
			return s
		}
	}
	return ""
}

func toFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int32:
		return float64(t), true
	case int64:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(t, 64)
		return f, err == nil
	default:
		return 0, false
	}
}
