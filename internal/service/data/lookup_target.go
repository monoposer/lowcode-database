package data

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	formulacompile "github.com/solat/lowcode-database/internal/formula"
	"github.com/solat/lowcode-database/internal/service/catalog"
	"github.com/solat/lowcode-database/internal/service/schema"
	"github.com/solat/lowcode-database/internal/service/shared"
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

	allCols, _, _, err := catalog.New(s.B).LoadAllColumnMeta(ctx, tableID)
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
	rels, err := schema.New(s.B).LoadRelationshipColumns(ctx, hostTableID, []string{relName})
	if err != nil || len(rels) == 0 {
		return resolvedLookupValue{}, fmt.Errorf("lookup %q: relationship %q not found", col.Name, relName)
	}
	rel := rels[0]
	if rel.Cardinality != "one" || rel.TargetColumnId == "" {
		return resolvedLookupValue{}, fmt.Errorf("lookup %q: relationship must be cardinality one", col.Name)
	}
	baseFKPg, err := schema.New(s.B).ColumnPgColumnByRef(ctx, tid, hostTableID, rel.TargetColumnId)
	if err != nil {
		return resolvedLookupValue{}, err
	}
	tgtSchema, tgtTable, err := s.tableSchemaName(ctx, rel.TargetTableId)
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
	baseRefs, extraFrom, err := s.tableExprRefs(ctx, tableID, rowAlias, argAcc, visiting, allCols, aliases)
	if err != nil {
		return resolvedLookupValue{}, err
	}
	defs := shared.FormulaDefs(allCols)
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
func (s *Data) tableExprRefs(
	ctx context.Context,
	tableID, rowAlias string,
	argAcc *argAccumulator,
	visiting map[string]bool,
	allCols []shared.FullColumnMeta,
	aliases *joinAliasRegistry,
) (map[string]string, string, error) {
	if aliases == nil {
		aliases = newJoinAliasRegistry()
	}
	refs := map[string]string{}
	var extraFrom strings.Builder
	for _, c := range allCols {
		if !c.IsVirtual && c.Kind != "relation_fk" {
			refs[c.Name] = c.Name
			continue
		}
		if c.Kind == "relation_fk" {
			refs[c.Name] = c.Name
		}
	}
	for _, c := range allCols {
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

func (s *Data) tableSchemaName(ctx context.Context, tableID string) (schemaName, tableName string, err error) {
	_, schemaName, tableName, err = catalog.New(s.B).LoadAllColumnMeta(ctx, tableID)
	return schemaName, tableName, err
}

func mapsCloneBool(m map[string]bool) map[string]bool {
	out := make(map[string]bool, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
