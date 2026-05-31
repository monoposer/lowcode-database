package data

import (
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/solat/lowcode-database/internal/dsl"
	"github.com/solat/lowcode-database/internal/query"
	"github.com/solat/lowcode-database/internal/service/shared"
)

func linkedTableFilterSQL(cfg map[string]any, alias string, cols []shared.ColumnMeta, argStart int) (string, []any, error) {
	raw, ok := cfg["filter"]
	if !ok || raw == nil {
		return "", nil, nil
	}
	filterMap, ok := raw.(map[string]any)
	if !ok {
		return "", nil, fmt.Errorf("filter must be a JSON object")
	}
	w, err := dsl.Parse(filterMap)
	if err != nil {
		return "", nil, fmt.Errorf("filter: %w", err)
	}
	if w.Type == "" {
		return "", nil, nil
	}
	attrMap := map[string]string{}
	aliasQ := pgx.Identifier{alias}.Sanitize()
	for _, c := range cols {
		colQ := aliasQ + "." + pgx.Identifier{c.Name}.Sanitize()
		attrMap[c.Id] = colQ
		attrMap[c.Name] = colQ
	}
	return query.BuildWhere(w, attrMap, argStart)
}
