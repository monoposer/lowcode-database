package formula

import (
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Step is one formula column in the final query (inline SELECT or LATERAL join).
type Step struct {
	Name      string
	SQL       string // compiled expression
	Inline    bool   // false → use LateralJoinSQL and SelectRef
	JoinAlias string
	ColAlias  string
}

// SelectRef is the SQL fragment for the outer SELECT list (before AS column alias).
func (s Step) SelectRef() string {
	if s.Inline {
		return "(" + s.SQL + ")"
	}
	return pgx.Identifier{s.JoinAlias}.Sanitize() + "." + pgx.Identifier{s.ColAlias}.Sanitize()
}

// LateralJoinSQL returns LEFT JOIN LATERAL ... or empty when Inline.
func (s Step) LateralJoinSQL() string {
	if s.Inline {
		return ""
	}
	return fmt.Sprintf(
		` LEFT JOIN LATERAL (SELECT (%s) AS %s) %s ON true`,
		s.SQL,
		pgx.Identifier{s.ColAlias}.Sanitize(),
		pgx.Identifier{s.JoinAlias}.Sanitize(),
	)
}

// BuildSteps compiles formula columns in dependency order.
// baseRefs maps {{name}} → physical column, lookup qualifier, or rollup subquery SQL.
// Referenced formulas use LATERAL so each is computed once; dependents reuse SelectRef.
func BuildSteps(alias string, baseRefs map[string]string, defs []Def) ([]Step, error) {
	sorted, err := Sort(defs)
	if err != nil {
		return nil, err
	}

	formulaNames := make(map[string]struct{}, len(sorted))
	for _, d := range sorted {
		formulaNames[d.Name] = struct{}{}
	}
	referenced := formulaNamesReferencedByFormulas(sorted)

	refs := mapsClone(baseRefs)
	var steps []Step

	for _, d := range sorted {
		sql, err := Compile(d.Expr, alias, refs)
		if err != nil {
			return nil, fmt.Errorf("formula %q: %w", d.Name, err)
		}

		useLateral := referenced[d.Name] || referencesFormula(d.Expr, formulaNames)
		step := Step{
			Name:      d.Name,
			SQL:       sql,
			Inline:    !useLateral,
			JoinAlias: "_f_" + d.Name,
			ColAlias:  "v_" + d.Name,
		}
		steps = append(steps, step)

		if useLateral {
			refs[d.Name] = step.SelectRef()
		} else {
			refs[d.Name] = "(" + sql + ")"
		}
	}
	return steps, nil
}

func formulaNamesReferencedByFormulas(defs []Def) map[string]bool {
	out := make(map[string]bool)
	names := make(map[string]struct{}, len(defs))
	for _, d := range defs {
		names[d.Name] = struct{}{}
	}
	for _, d := range defs {
		for _, ref := range Refs(d.Expr) {
			if _, ok := names[ref]; ok {
				out[ref] = true
			}
		}
	}
	return out
}

func referencesFormula(expr string, formulaNames map[string]struct{}) bool {
	for _, ref := range Refs(expr) {
		if _, ok := formulaNames[ref]; ok {
			return true
		}
	}
	return false
}

func mapsClone(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// formulaStubSQL is a minimal SQL fragment used when validating an expression
// against other formula columns without compiling the full table query.
const formulaStubSQL = "(0)"

// StubRef returns a validation-only ref value for another formula column.
func StubRef(name string) string {
	_ = name
	return formulaStubSQL
}

// IsFormulaStub reports whether ref is the validation stub.
func IsFormulaStub(ref string) bool {
	return strings.TrimSpace(ref) == formulaStubSQL
}
