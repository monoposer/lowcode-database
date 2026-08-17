package data

import (
	"fmt"

	"github.com/monoposer/lowcode-database/internal/service/calc"
	"github.com/monoposer/lowcode-database/internal/service/shared"
)

func splitLinkCells(cells map[string]*shared.Value, cols []shared.FullColumnMeta) (data map[string]any, links map[string][]string) {
	linkCols := map[string]bool{}
	calcCols := map[string]bool{}
	for _, c := range cols {
		if calc.IsLinkType(c.TypeId) || calc.IsLinkType(c.Kind) {
			linkCols[c.Name] = true
			linkCols[c.Id] = true
		}
		if calc.IsCalcType(c.TypeId) || calc.IsCalcType(c.Kind) {
			calcCols[c.Name] = true
			calcCols[c.Id] = true
		}
	}
	data = map[string]any{}
	links = map[string][]string{}
	for k, v := range cells {
		if linkCols[k] {
			links[k] = cellToIDs(v)
			continue
		}
		if calcCols[k] {
			continue
		}
		if k == "_fulltext_text" {
			continue
		}
		data[k] = shared.ValueToAnyForColumn(v, "")
	}
	return data, links
}

func cellToIDs(v *shared.Value) []string {
	if v == nil {
		return nil
	}
	if v.StringValue != nil {
		if *v.StringValue == "" {
			return nil
		}
		return []string{*v.StringValue}
	}
	native := shared.ValueToNative(v)
	return asStringSlice(native)
}

func linkFieldMeta(cols []shared.FullColumnMeta, ref string) (shared.FullColumnMeta, bool) {
	for _, c := range cols {
		if !calc.IsLinkType(c.TypeId) && !calc.IsLinkType(c.Kind) {
			continue
		}
		if c.Name == ref || c.Id == ref {
			return c, true
		}
	}
	return shared.FullColumnMeta{}, false
}

func hydrateCells(native map[string]any, cols []shared.FullColumnMeta, links map[string][]string, pending bool) map[string]*shared.Value {
	out := map[string]*shared.Value{}
	for _, c := range cols {
		// Row.Id carries the primary key; skip duplicating it into cells.
		if c.Name == "id" {
			continue
		}
		if calc.IsLinkType(c.TypeId) || calc.IsLinkType(c.Kind) {
			ids := links[c.Name]
			if ids == nil {
				ids = links[c.Id]
			}
			if ids == nil {
				ids = []string{}
			}
			out[c.Name] = shared.JsonValue(ids)
			continue
		}
		if calc.IsCalcType(c.TypeId) || calc.IsCalcType(c.Kind) {
			raw := native[c.Name]
			if raw == nil {
				raw = native["_rollup_"+c.Name]
			}
			val := calc.UnwrapCache(raw)
			cell := shared.DBCellValue(val, c.PgType)
			if pending {
				// keep value; UI uses row.pending
			}
			if cell != nil {
				out[c.Name] = cell
			}
			continue
		}
		if v, ok := native[c.Name]; ok {
			out[c.Name] = shared.DBCellValue(calc.UnwrapCache(v), c.PgType)
		}
	}
	return out
}

func changedKeys(m map[string]any) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

func mustIDs(v any) []string {
	return asStringSlice(v)
}

func asStringSlice(v any) []string {
	switch t := v.(type) {
	case string:
		if t == "" {
			return nil
		}
		return []string{t}
	case []any:
		var out []string
		for _, x := range t {
			out = append(out, fmt.Sprint(x))
		}
		return out
	case []string:
		return t
	default:
		if v == nil {
			return nil
		}
		s := fmt.Sprint(v)
		if s == "" || s == "<nil>" {
			return nil
		}
		return []string{s}
	}
}
