package shared

import "github.com/solat/lowcode-database/internal/apiv1"

// CellByRef reads a cell value keyed by logical column name (preferred) or legacy meta UUID.
func CellByRef(cells map[string]*apiv1.Value, c ColumnMeta) (*apiv1.Value, bool) {
	if cells == nil {
		return nil, false
	}
	if v, ok := cells[c.Name]; ok {
		return v, true
	}
	if c.Id != "" && c.Id != c.Name {
		if v, ok := cells[c.Id]; ok {
			return v, true
		}
	}
	return nil, false
}

// CellsToNames re-keys a cell map to logical column names for API responses.
func CellsToNames(cells map[string]*apiv1.Value, cols []ColumnMeta) map[string]*apiv1.Value {
	if len(cells) == 0 {
		return cells
	}
	out := make(map[string]*apiv1.Value, len(cells))
	for _, c := range cols {
		if v, ok := CellByRef(cells, c); ok {
			out[c.Name] = v
		}
	}
	for k, v := range cells {
		if _, ok := out[k]; ok {
			continue
		}
		out[k] = v
	}
	return out
}

// NormalizeInputCells accepts cells keyed by column name or legacy UUID and returns name-keyed cells.
func NormalizeInputCells(cells map[string]*apiv1.Value, cols []ColumnMeta) map[string]*apiv1.Value {
	if len(cells) == 0 {
		return cells
	}
	byID := make(map[string]ColumnMeta, len(cols))
	byName := make(map[string]ColumnMeta, len(cols))
	for _, c := range cols {
		byName[c.Name] = c
		if c.Id != "" {
			byID[c.Id] = c
		}
	}
	out := make(map[string]*apiv1.Value, len(cells))
	for key, v := range cells {
		if key == "" {
			continue
		}
		if c, ok := byName[key]; ok {
			out[c.Name] = v
			continue
		}
		if c, ok := byID[key]; ok {
			out[c.Name] = v
			continue
		}
		out[key] = v
	}
	return out
}
