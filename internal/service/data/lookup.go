package data

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/monoposer/lowcode-database/internal/dsl"
	formulacompile "github.com/monoposer/lowcode-database/internal/formula"
	"github.com/monoposer/lowcode-database/internal/query"
	"github.com/monoposer/lowcode-database/internal/service/shared"
	"strings"
	"unicode"
)

type argAccumulator struct {
	args *[]any
}

func (a *argAccumulator) nextArgStart() int {
	if a == nil || a.args == nil {
		return 1
	}
	return len(*a.args) + 1
}

func (a *argAccumulator) append(vals ...any) {
	if a == nil || a.args == nil || len(vals) == 0 {
		return
	}
	*a.args = append(*a.args, vals...)
}

// resolvedLookupValue is the SQL value expression for a lookup target column on a joined row alias.
type resolvedLookupValue struct {
	SelectExpr string
	ExtraFrom  string
	PgType     string
}

func (s *Data) resolveLookupTargetValue(
	ctx context.Context,
	tableID, columnName, rowAlias string,
	argAcc *argAccumulator,
	visiting map[string]bool,
	aliases *joinAliasRegistry,
) (resolvedLookupValue, error) {
	if aliases == nil {
		aliases = newJoinAliasRegistry()
	}
	key := tableID + ":" + columnName
	if visiting[key] {
		return resolvedLookupValue{}, fmt.Errorf("lookup target cycle at %q on table %q", columnName, tableID)
	}
	visiting[key] = true
	defer delete(visiting, key)

	allCols, _, _, err := s.meta().LoadAllColumnMeta(ctx, tableID)
	if err != nil {
		return resolvedLookupValue{}, err
	}
	var col *shared.FullColumnMeta
	for i := range allCols {
		if allCols[i].Name == columnName {
			col = &allCols[i]
			break
		}
	}
	if col == nil {
		return resolvedLookupValue{}, fmt.Errorf("lookup target column %q not found on table %q", columnName, tableID)
	}

	switch col.Kind {
	case "lookup":
		return s.resolveLookupColumnValue(ctx, tableID, col, rowAlias, argAcc, visiting, aliases)
	case "rollup":
		return s.resolveRollupColumnValue(ctx, tableID, col, rowAlias, argAcc, allCols)
	case "formula":
		esc := strings.ReplaceAll(col.Name, "'", "''")
		a := quotedAlias(rowAlias)
		return resolvedLookupValue{
			SelectExpr: fmt.Sprintf(
				`(CASE WHEN jsonb_typeof(%s.data->'%s') = 'object' AND (%s.data->'%s') ? 'value' THEN %s.data#>>'{%s,value}' ELSE %s.data->>'%s' END)`,
				a, esc, a, esc, a, esc, a, esc,
			),
		}, nil
	default:
		if col.IsVirtual {
			return resolvedLookupValue{}, fmt.Errorf("lookup target %q (%s) is not supported", columnName, col.Kind)
		}
		return resolvedLookupValue{
			SelectExpr: quotedAlias(rowAlias) + "." + pgx.Identifier{col.Name}.Sanitize(),
			PgType:     col.PgType,
		}, nil
	}
}

func (s *Data) resolveLookupColumnValue(
	ctx context.Context,
	hostTableID string,
	col *shared.FullColumnMeta,
	rowAlias string,
	argAcc *argAccumulator,
	visiting map[string]bool,
	aliases *joinAliasRegistry,
) (resolvedLookupValue, error) {
	relName := shared.CfgString(col.Config, "relation_column_id")
	fieldName := shared.CfgString(col.Config, "target_column_id")
	if relName == "" || fieldName == "" {
		return resolvedLookupValue{}, fmt.Errorf("lookup %q: missing relation or target column", col.Name)
	}
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return resolvedLookupValue{}, err
	}
	rels, err := s.meta().LoadRelationshipColumns(ctx, hostTableID, []string{relName})
	if err != nil || len(rels) == 0 {
		return resolvedLookupValue{}, fmt.Errorf("lookup %q: relationship %q not found", col.Name, relName)
	}
	rel := rels[0]
	tgtSchema, tgtTable, err := s.tableSchemaName(ctx, rel.TargetTableId)
	if err != nil {
		return resolvedLookupValue{}, err
	}
	if rel.Cardinality == "many" && rel.LinkColumnId != "" {
		linkPg, err := s.meta().ColumnPgColumnByRef(ctx, tid, rel.TargetTableId, rel.LinkColumnId)
		if err != nil {
			return resolvedLookupValue{}, err
		}
		inner, err := s.resolveLookupTargetValue(ctx, rel.TargetTableId, fieldName, "_r", argAcc, visiting, aliases)
		if err != nil {
			return resolvedLookupValue{}, err
		}
		arrayPgType := shared.ScalarPgTypeToArray(inner.PgType)
		selectExpr := shared.LookupManyAggregateSQL(
			inner.SelectExpr, linkPg, tgtSchema, tgtTable, rowAlias, "", inner.ExtraFrom, arrayPgType,
		)
		return resolvedLookupValue{
			SelectExpr: selectExpr,
			PgType:     arrayPgType,
		}, nil
	}
	if rel.Cardinality != "one" || rel.TargetColumnId == "" {
		return resolvedLookupValue{}, fmt.Errorf("lookup %q: relationship must be cardinality one", col.Name)
	}
	baseFKPg, err := s.meta().ColumnPgColumnByRef(ctx, tid, hostTableID, rel.TargetColumnId)
	if err != nil {
		return resolvedLookupValue{}, err
	}
	joinAlias, hopSQL := aliases.ensureHopJoin(rowAlias, tgtSchema, tgtTable, col.Name, func(a string) string {
		return fmt.Sprintf(
			`LEFT JOIN %s.%s AS %s ON %s.%s = %s.id`,
			pgx.Identifier{tgtSchema}.Sanitize(),
			pgx.Identifier{tgtTable}.Sanitize(),
			quotedAlias(a),
			quotedAlias(rowAlias),
			pgx.Identifier{baseFKPg}.Sanitize(),
			quotedAlias(a),
		)
	})
	inner, err := s.resolveLookupTargetValue(ctx, rel.TargetTableId, fieldName, joinAlias, argAcc, visiting, aliases)
	if err != nil {
		return resolvedLookupValue{}, err
	}
	extra := hopSQL + inner.ExtraFrom
	return resolvedLookupValue{
		SelectExpr: inner.SelectExpr,
		ExtraFrom:  extra,
		PgType:     inner.PgType,
	}, nil
}

func (s *Data) resolveRollupColumnValue(
	ctx context.Context,
	tableID string,
	col *shared.FullColumnMeta,
	rowAlias string,
	argAcc *argAccumulator,
	allCols []shared.FullColumnMeta,
) (resolvedLookupValue, error) {
	plans, err := s.buildRollupPlans(ctx, tableID, allCols)
	if err != nil {
		return resolvedLookupValue{}, err
	}
	var plan *rollupPlan
	for i := range plans {
		if plans[i].ColumnName == col.Name {
			plan = &plans[i]
			break
		}
	}
	if plan == nil {
		return resolvedLookupValue{}, fmt.Errorf("rollup %q: could not build plan", col.Name)
	}
	rSQL, rArgs, err := s.buildRollupSQL(*plan, rowAlias, argAcc.nextArgStart())
	if err != nil {
		return resolvedLookupValue{}, err
	}
	argAcc.append(rArgs...)
	return resolvedLookupValue{
		SelectExpr: "(" + rSQL + ")",
		PgType:     col.PgType,
	}, nil
}

func collectFormulaNeededRefs(formulaName string, allCols []shared.FullColumnMeta) map[string]struct{} {
	exprByName := map[string]string{}
	for _, c := range allCols {
		if c.Kind != "formula" {
			continue
		}
		if e := shared.FormulaExpression(c.Config); e != "" {
			exprByName[c.Name] = e
		}
	}
	needed := map[string]struct{}{}
	var walk func(name string)
	walk = func(name string) {
		if _, ok := needed[name]; ok {
			return
		}
		needed[name] = struct{}{}
		expr, isFormula := exprByName[name]
		if !isFormula {
			return
		}
		for _, ref := range formulacompile.Refs(expr) {
			walk(ref)
		}
	}
	walk(formulaName)
	return needed
}

func (s *Data) tableSchemaName(ctx context.Context, tableID string) (schemaName, tableName string, err error) {
	_, schemaName, tableName, err = s.meta().LoadAllColumnMeta(ctx, tableID)
	return schemaName, tableName, err
}

func mapsCloneBool(m map[string]bool) map[string]bool {
	out := make(map[string]bool, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// joinAliasRegistry assigns unique SQL table aliases and deduplicates JOIN clauses.
type joinAliasRegistry struct {
	used        map[string]struct{}
	emittedJoin map[string]struct{}
	relRowAlias map[string]string // relationship column name → shared related-table row alias
	hopAlias    map[string]string // fromRowAlias>schema.table → nested join alias
}

func newJoinAliasRegistry() *joinAliasRegistry {
	return &joinAliasRegistry{
		used:        make(map[string]struct{}),
		emittedJoin: make(map[string]struct{}),
		relRowAlias: make(map[string]string),
		hopAlias:    make(map[string]string),
	}
}

func (r *joinAliasRegistry) reserve(alias string) string {
	base := sanitizeSQLAlias(alias)
	if base == "" {
		base = "lk"
	}
	name := base
	for i := 0; ; i++ {
		if _, taken := r.used[name]; !taken {
			r.used[name] = struct{}{}
			return name
		}
		name = fmt.Sprintf("%s_%d", base, i+1)
	}
}

// sharedRelRowAlias returns one JOIN alias per relationship column (all lookups via same rel share it).
func (r *joinAliasRegistry) sharedRelRowAlias(relationshipColumnName string) string {
	if a, ok := r.relRowAlias[relationshipColumnName]; ok {
		return a
	}
	a := r.reserve("lk_rel_" + relationshipColumnName)
	r.relRowAlias[relationshipColumnName] = a
	return a
}

// ensureHopJoin returns the alias for a nested hop (from row → related table).
// joinSQL is returned only the first time this hop is needed; later callers reuse the alias.
func (r *joinAliasRegistry) ensureHopJoin(rowAlias, schema, table, lookupColName string, buildSQL func(joinAlias string) string) (joinAlias, joinSQL string) {
	key := rowAlias + ">" + schema + "." + table
	if a, ok := r.hopAlias[key]; ok {
		return a, ""
	}
	a := r.reserve(rowAlias + "_n_" + lookupColName)
	r.hopAlias[key] = a
	return a, r.appendJoin(buildSQL(a))
}

// appendJoin returns joinSQL the first time it is seen, or "" if an identical JOIN was already emitted.
func (r *joinAliasRegistry) appendJoin(joinSQL string) string {
	joinSQL = strings.TrimSpace(joinSQL)
	if joinSQL == "" {
		return ""
	}
	key := strings.Join(strings.Fields(joinSQL), " ")
	if _, ok := r.emittedJoin[key]; ok {
		return ""
	}
	r.emittedJoin[key] = struct{}{}
	return " " + joinSQL
}

func sanitizeSQLAlias(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '-' || r == '.':
			b.WriteByte('_')
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if len(out) > 55 {
		out = out[:55]
	}
	return out
}

func quotedAlias(alias string) string {
	return pgx.Identifier{alias}.Sanitize()
}

// hostRelJoinKey identifies the base LEFT JOIN to a related table from the query base row.
func hostRelJoinKey(schema, table, baseFKPgCol string) string {
	return schema + "." + table + "|" + baseFKPgCol
}

func linkedTableFilterSQL(cfg map[string]any, alias string, cols []shared.ColumnMeta, argStart int) (string, []any, error) {
	raw, ok := cfg["filter"]
	if !ok || raw == nil {
		return "", nil, nil
	}
	filterMap, ok := raw.(map[string]any)
	if !ok {
		return "", nil, fmt.Errorf("filter must be a JSON object")
	}
	w, err := dsl.ParseCached(filterMap)
	if err != nil {
		return "", nil, fmt.Errorf("filter: %w", err)
	}
	if w.Type == "" {
		return "", nil, nil
	}
	attrMap := map[string]string{}
	aliasQ := pgx.Identifier{alias}.Sanitize()
	attrPgTypes := map[string]string{}
	for _, c := range cols {
		colQ := aliasQ + "." + pgx.Identifier{c.Name}.Sanitize()
		attrMap[c.Id] = colQ
		attrMap[c.Name] = colQ
		if c.PgType != "" {
			attrPgTypes[c.Id] = c.PgType
			attrPgTypes[c.Name] = c.PgType
		}
	}
	return query.BuildWhereWithTypes(w, attrMap, attrPgTypes, argStart)
}
