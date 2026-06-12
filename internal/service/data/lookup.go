package data

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/monoposer/lowcode-database/internal/apiv1"
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
		return s.resolveFormulaColumnValue(ctx, tableID, col.Name, rowAlias, argAcc, visiting, allCols, aliases)
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

func (s *Data) resolveFormulaColumnValue(
	ctx context.Context,
	tableID, formulaName, rowAlias string,
	argAcc *argAccumulator,
	visiting map[string]bool,
	allCols []shared.FullColumnMeta,
	aliases *joinAliasRegistry,
) (resolvedLookupValue, error) {
	needed := collectFormulaNeededRefs(formulaName, allCols)
	baseRefs, extraFrom, err := s.tableExprRefs(ctx, tableID, rowAlias, argAcc, visiting, allCols, aliases, needed)
	if err != nil {
		return resolvedLookupValue{}, err
	}
	allDefs := shared.FormulaDefs(allCols)
	defs := filterFormulaDefs(allDefs, needed)
	steps, err := formulacompile.BuildSteps(rowAlias, baseRefs, defs)
	if err != nil {
		return resolvedLookupValue{}, err
	}
	for _, st := range steps {
		extraFrom += aliases.appendJoin(st.LateralJoinSQL())
	}
	var selectExpr string
	for _, st := range steps {
		if st.Name == formulaName {
			selectExpr = st.SelectRef()
			break
		}
	}
	if selectExpr == "" {
		return resolvedLookupValue{}, fmt.Errorf("formula %q not found on table %q", formulaName, tableID)
	}
	var pgType string
	for _, c := range allCols {
		if c.Name == formulaName {
			pgType = c.PgType
			break
		}
	}
	return resolvedLookupValue{
		SelectExpr: selectExpr,
		ExtraFrom:  extraFrom,
		PgType:     pgType,
	}, nil
}

// tableExprRefs builds {{name}} → SQL refs for formulas/rollups/lookups on one table row alias.
// When needed is non-nil, only columns in that set are resolved (virtual columns are skipped otherwise).
func (s *Data) tableExprRefs(
	ctx context.Context,
	tableID, rowAlias string,
	argAcc *argAccumulator,
	visiting map[string]bool,
	allCols []shared.FullColumnMeta,
	aliases *joinAliasRegistry,
	needed map[string]struct{},
) (map[string]string, string, error) {
	if aliases == nil {
		aliases = newJoinAliasRegistry()
	}
	refs := map[string]string{}
	var extraFrom strings.Builder
	for _, c := range allCols {
		if needed != nil {
			if _, ok := needed[c.Name]; !ok {
				continue
			}
		}
		if !c.IsVirtual && c.Kind != "relation_fk" {
			refs[c.Name] = c.Name
			continue
		}
		if c.Kind == "relation_fk" {
			refs[c.Name] = c.Name
		}
	}
	for _, c := range allCols {
		if needed != nil {
			if _, ok := needed[c.Name]; !ok {
				continue
			}
		}
		switch c.Kind {
		case "lookup":
			res, err := s.resolveLookupColumnValue(ctx, tableID, &c, rowAlias, argAcc, mapsCloneBool(visiting), aliases)
			if err != nil {
				return nil, "", err
			}
			refs[c.Name] = res.SelectExpr
			extraFrom.WriteString(res.ExtraFrom)
		case "rollup":
			res, err := s.resolveRollupColumnValue(ctx, tableID, &c, rowAlias, argAcc, allCols)
			if err != nil {
				return nil, "", err
			}
			refs[c.Name] = res.SelectExpr
		}
	}
	return refs, extraFrom.String(), nil
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

func filterFormulaDefs(allDefs []formulacompile.Def, needed map[string]struct{}) []formulacompile.Def {
	if len(needed) == 0 {
		return allDefs
	}
	out := make([]formulacompile.Def, 0, len(allDefs))
	for _, d := range allDefs {
		if _, ok := needed[d.Name]; ok {
			out = append(out, d)
		}
	}
	return out
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

// resolveLookupCells maps lookup column values to local FK columns (saveGraph and similar write paths).
// When both lookup and FK column are present, the explicit FK value wins.
func (s *Data) resolveLookupCells(ctx context.Context, tx pgx.Tx, tableID string, cells map[string]*apiv1.Value) (map[string]*apiv1.Value, error) {
	if len(cells) == 0 {
		return cells, nil
	}
	specs, err := s.meta().LoadLookupWriteSpecs(ctx, tableID)
	if err != nil {
		return nil, err
	}
	if len(specs) == 0 {
		return cells, nil
	}

	out := make(map[string]*apiv1.Value, len(cells))
	for k, v := range cells {
		out[k] = v
	}

	for lookupName, spec := range specs {
		lv, ok := out[lookupName]
		if !ok || lv == nil {
			continue
		}
		if _, hasFK := out[spec.LocalFKColumn]; hasFK {
			delete(out, lookupName)
			continue
		}
		fkVal, err := s.resolveOneLookupValueTx(ctx, tx, spec, lv)
		if err != nil {
			return nil, fmt.Errorf("lookup %q: %w", lookupName, err)
		}
		out[spec.LocalFKColumn] = fkVal
		delete(out, lookupName)
	}
	return out, nil
}

func (s *Data) resolveOneLookupValueTx(ctx context.Context, tx pgx.Tx, spec shared.LookupWriteSpec, lookupVal *apiv1.Value) (*apiv1.Value, error) {
	searchArg := shared.ValueToAnyForColumn(lookupVal, spec.SearchPgType)
	if searchArg == nil {
		return nil, fmt.Errorf("lookup value is required")
	}

	q := fmt.Sprintf(`SELECT t.%s FROM %s.%s AS t WHERE t.%s = $1`,
		pgx.Identifier{spec.RefColumn}.Sanitize(),
		pgx.Identifier{spec.TargetSchema}.Sanitize(),
		pgx.Identifier{spec.TargetTable}.Sanitize(),
		pgx.Identifier{spec.SearchColumn}.Sanitize(),
	)
	args := []any{searchArg}
	argIdx := 2
	if len(spec.Filter) > 0 && len(spec.TargetCols) > 0 {
		filterSQL, filterArgs, err := linkedTableFilterSQL(map[string]any{"filter": spec.Filter}, "t", spec.TargetCols, argIdx)
		if err != nil {
			return nil, err
		}
		if filterSQL != "" {
			q += " AND (" + filterSQL + ")"
			args = append(args, filterArgs...)
		}
	}
	q += " LIMIT 2"

	rows, err := tx.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []any
	for rows.Next() {
		var v any
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		matches = append(matches, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no %s row matches %s = %v", spec.TargetTable, spec.SearchColumn, searchArg)
	case 1:
		return shared.DBCellValue(matches[0], spec.LocalFKPgType), nil
	default:
		return nil, fmt.Errorf("ambiguous %s match for %s = %v", spec.TargetTable, spec.SearchColumn, searchArg)
	}
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
	w, err := dsl.Parse(filterMap)
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
