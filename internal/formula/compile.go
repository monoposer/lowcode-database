package formula

import (
	"fmt"
	pgformula "github.com/SolaTyolo/pg-formula/pkg/formula"
	"github.com/jackc/pgx/v5"
	"regexp"
	"strings"
)

// Package formula compiles Excel-style expressions to PostgreSQL via pg-formula.

var columnRefRe = regexp.MustCompile(`\{\{([a-zA-Z_][a-zA-Z0-9_]*)\}\}`)

// Compile parses an Excel / formulajs-style expression and returns PostgreSQL SQL.
// Logical column names in {{name}} are mapped to physical SQL identifiers before calling pg-formula.
func Compile(expr, alias string, nameToPg map[string]string) (string, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return "NULL", nil
	}
	if alias == "" {
		alias = "_b"
	}

	rewritten, sqlExprs, err := rewriteColumnRefs(expr, alias, nameToPg)
	if err != nil {
		return "", err
	}
	rewritten = normalizeSingleQuotedStrings(rewritten)

	sql, err := pgformula.ToPostgres(rewritten)
	if err != nil {
		return "", fmt.Errorf("formula: %w", err)
	}
	for placeholder, subSQL := range sqlExprs {
		sql = strings.ReplaceAll(sql, placeholder, subSQL)
	}
	return sql, nil
}

func rewriteColumnRefs(expr, alias string, nameToPg map[string]string) (string, map[string]string, error) {
	sqlExprs := map[string]string{}
	var err error
	out := columnRefRe.ReplaceAllStringFunc(expr, func(m string) string {
		if err != nil {
			return m
		}
		sub := columnRefRe.FindStringSubmatch(m)
		if len(sub) < 2 {
			err = fmt.Errorf("invalid formula reference %q", m)
			return m
		}
		pg, ok := nameToPg[sub[1]]
		if !ok {
			err = fmt.Errorf("formula references unknown column %q", sub[1])
			return m
		}
		ref, placeholder, isSQL := formatColumnRef(alias, sub[1], pg)
		if isSQL {
			sqlExprs[placeholder] = strings.TrimSpace(pg)
		}
		return ref
	})
	if err != nil {
		return "", nil, err
	}
	return out, sqlExprs, nil
}

// formatColumnRef maps a logical column to pg-formula reference syntax.
// SQL subexpressions (rollup) use a placeholder token replaced after ToPostgres.
// Qualified names (lookup join) pass through as table.column.
func formatColumnRef(alias, logicalName, pg string) (ref string, sqlPlaceholder string, isSQL bool) {
	trimmed := strings.TrimSpace(pg)
	if strings.HasPrefix(trimmed, "(") {
		placeholder := alias + ".__lc_" + logicalName + "__"
		return "{{" + placeholder + "}}", placeholder, true
	}
	if strings.Contains(pg, ".") {
		return "{{" + pg + "}}", "", false
	}
	return "{{" + alias + "." + pg + "}}", "", false
}

// normalizeSingleQuotedStrings maps SQL-style 'text' literals to Excel "text" for pg-formula.
func normalizeSingleQuotedStrings(expr string) string {
	var b strings.Builder
	i := 0
	for i < len(expr) {
		if i+1 < len(expr) && expr[i:i+2] == "{{" {
			if close := strings.Index(expr[i+2:], "}}"); close >= 0 {
				end := i + 2 + close + 2
				b.WriteString(expr[i:end])
				i = end
				continue
			}
		}
		if expr[i] == '\'' {
			j := i + 1
			for j < len(expr) && expr[j] != '\'' {
				j++
			}
			if j < len(expr) {
				b.WriteByte('"')
				b.WriteString(expr[i+1 : j])
				b.WriteByte('"')
				i = j + 1
				continue
			}
		}
		b.WriteByte(expr[i])
		i++
	}
	return b.String()
}

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
