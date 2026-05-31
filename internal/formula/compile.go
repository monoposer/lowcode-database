// Package formula compiles Excel-style expressions to PostgreSQL via pg-formula.
package formula

import (
	"fmt"
	"regexp"
	"strings"

	pgformula "github.com/SolaTyolo/pg-formula/pkg/formula"
)

var columnRefRe = regexp.MustCompile(`\{\{([a-zA-Z_][a-zA-Z0-9_]*)\}\}`)

// Compile parses an Excel / formulajs-style expression and returns PostgreSQL SQL.
// Logical column names in {{name}} are mapped to alias.pg_column before calling pg-formula.
func Compile(expr, alias string, nameToPg map[string]string) (string, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return "NULL", nil
	}
	if alias == "" {
		alias = "_b"
	}

	rewritten, err := rewriteColumnRefs(expr, alias, nameToPg)
	if err != nil {
		return "", err
	}
	rewritten = normalizeSingleQuotedStrings(rewritten)

	sql, err := pgformula.ToPostgres(rewritten)
	if err != nil {
		return "", fmt.Errorf("formula: %w", err)
	}
	return sql, nil
}

func rewriteColumnRefs(expr, alias string, nameToPg map[string]string) (string, error) {
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
		// pg-formula: {{table.col}} → table.col in SQL output
		return "{{" + alias + "." + pg + "}}"
	})
	if err != nil {
		return "", err
	}
	return out, nil
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
