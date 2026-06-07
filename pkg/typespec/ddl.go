package typespec

import (
	"fmt"
	"strings"
)

// ResolveUnderlyingPgType maps pgType id or raw PG type to SQL for CREATE DOMAIN … AS …
func ResolveUnderlyingPgType(pgType string, precision, scale *int) (string, error) {
	pgType = strings.TrimSpace(pgType)
	if pgType == "" {
		return "", fmt.Errorf("pgType is empty")
	}
	if strings.Contains(pgType, " ") || strings.HasSuffix(pgType, "[]") ||
		strings.HasPrefix(pgType, "numeric(") || strings.HasPrefix(pgType, "geometry") {
		return pgType, nil
	}
	if bt, ok := GetPgType(pgType); ok {
		if bt.Category == CategoryArray && bt.PgType != "" {
			return bt.PgType, nil
		}
		if bt.ID == "number" || bt.AliasOf == "number" || pgType == "numeric" || pgType == "precision" || pgType == "number" {
			if precision != nil || scale != nil {
				return FormatNumericType(precision, scale), nil
			}
		}
		if bt.PgType == "" {
			return "", fmt.Errorf("pgType %q is virtual and cannot back a columnType", pgType)
		}
		return bt.PgType, nil
	}
	return "", fmt.Errorf("unknown pgType %q", pgType)
}

// CreateColumnTypeSQL builds CREATE DOMAIN (PG implementation) for a tenant column type.
func CreateColumnTypeSQL(schemaName, typeName string, spec ColumnTypeSpec) (string, error) {
	if err := ValidateColumnTypeSpec(&spec); err != nil {
		return "", err
	}
	baseSQL, err := ResolveUnderlyingPgType(spec.PgType, spec.Precision, spec.Scale)
	if err != nil {
		return "", err
	}
	qSchema := quoteIdent(schemaName)
	qType := quoteIdent(typeName)
	var b strings.Builder
	fmt.Fprintf(&b, "CREATE DOMAIN %s.%s AS %s", qSchema, qType, baseSQL)
	if spec.NotNull {
		b.WriteString(" NOT NULL")
	}
	if spec.Default != nil {
		fmt.Fprintf(&b, " DEFAULT %s", strings.TrimSpace(*spec.Default))
	}
	for _, c := range spec.Checks {
		expr := strings.TrimSpace(c.Expr)
		if c.Name != "" {
			fmt.Fprintf(&b, " CONSTRAINT %s CHECK (%s)", quoteIdent(c.Name), expr)
		} else {
			fmt.Fprintf(&b, " CHECK (%s)", expr)
		}
	}
	return b.String(), nil
}

// DropColumnTypeSQL builds DROP DOMAIN IF EXISTS.
func DropColumnTypeSQL(schemaName, typeName string, cascade bool) string {
	q := fmt.Sprintf("DROP DOMAIN IF EXISTS %s.%s", quoteIdent(schemaName), quoteIdent(typeName))
	if cascade {
		q += " CASCADE"
	}
	return q
}

// QualifiedColumnTypePgType returns "schema"."typeName" for column DDL.
func QualifiedColumnTypePgType(schemaName, typeName string) string {
	return fmt.Sprintf("%s.%s", quoteIdent(schemaName), quoteIdent(typeName))
}

func quoteIdent(name string) string {
	name = strings.ReplaceAll(name, `"`, `""`)
	return `"` + name + `"`
}

// AlterColumnTypeAddCheckSQL appends CHECK to an existing PG DOMAIN type.
func AlterColumnTypeAddCheckSQL(schemaName, typeName string, check CheckSpec) string {
	expr := strings.TrimSpace(check.Expr)
	if check.Name != "" {
		return fmt.Sprintf("ALTER DOMAIN %s.%s ADD CONSTRAINT %s CHECK (%s)",
			quoteIdent(schemaName), quoteIdent(typeName), quoteIdent(check.Name), expr)
	}
	return fmt.Sprintf("ALTER DOMAIN %s.%s ADD CHECK (%s)",
		quoteIdent(schemaName), quoteIdent(typeName), expr)
}
