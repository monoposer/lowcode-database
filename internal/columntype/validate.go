package columntype

import (
	"fmt"
	"regexp"
	"strings"
)

var columnTypeNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)
var checkNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

// Allowed ColumnTypeSpec.PgType ids (no aliases).
var columnTypeBasePgTypes = map[string]string{
	"text":     "text",
	"number":   "number",
	"datetime": "datetime",
	"boolean":  "boolean",
	"jsonb":    "jsonb",
}

// NormalizeColumnTypePgType maps an allowed base id to itself.
// Returns canonical id and false when not in the allowlist.
func NormalizeColumnTypePgType(pgType string) (string, bool) {
	canon, ok := columnTypeBasePgTypes[strings.TrimSpace(pgType)]
	return canon, ok
}

// ValidateColumnType checks a ColumnType document.
func ValidateColumnType(t *ColumnType) error {
	if t == nil {
		return fmt.Errorf("columnType is nil")
	}
	if t.APIVersion != "" && t.APIVersion != APIVersion {
		return fmt.Errorf("unsupported apiVersion %q", t.APIVersion)
	}
	if t.Kind != "" && t.Kind != KindColumnType {
		return fmt.Errorf("unsupported kind %q", t.Kind)
	}
	name := strings.TrimSpace(t.Metadata.Name)
	if name == "" {
		return fmt.Errorf("metadata.name is required")
	}
	if !columnTypeNameRe.MatchString(name) {
		return fmt.Errorf("metadata.name %q must match %s", name, columnTypeNameRe.String())
	}
	if _, ok := GetPgType(name); ok {
		return fmt.Errorf("metadata.name %q conflicts with built-in pgType", name)
	}
	return ValidateColumnTypeSpec(&t.Spec)
}

// ValidateColumnTypeSpec checks column type spec fields.
func ValidateColumnTypeSpec(spec *ColumnTypeSpec) error {
	if spec == nil {
		return fmt.Errorf("spec is nil")
	}
	pg := strings.TrimSpace(spec.PgType)
	if pg == "" {
		return fmt.Errorf("spec.pgType is required")
	}
	canon, ok := NormalizeColumnTypePgType(pg)
	if !ok {
		return fmt.Errorf("spec.pgType must be one of text, number, datetime, boolean, jsonb")
	}
	underlying, err := ResolveUnderlyingPgType(canon, spec.Precision, spec.Scale)
	if err != nil {
		return err
	}
	if spec.Array {
		if strings.HasSuffix(strings.ToLower(strings.TrimSpace(underlying)), "[]") {
			return fmt.Errorf("spec.pgType %q is already an array; omit spec.array", pg)
		}
	}
	if spec.Default != nil && strings.TrimSpace(*spec.Default) == "" {
		return fmt.Errorf("spec.default must not be empty when set")
	}
	if spec.RoundingMode != "" {
		if _, err := validateRoundingMode(spec.RoundingMode); err != nil {
			return err
		}
	}
	for i, c := range spec.Checks {
		if strings.TrimSpace(c.Expr) == "" {
			return fmt.Errorf("spec.checks[%d].expr is required", i)
		}
		if c.Name != "" && !checkNameRe.MatchString(c.Name) {
			return fmt.Errorf("spec.checks[%d].name %q invalid", i, c.Name)
		}
	}
	return nil
}

// ValidateTypeCatalog checks an import bundle.
func ValidateTypeCatalog(c *TypeCatalog) error {
	if c == nil {
		return fmt.Errorf("catalog is nil")
	}
	if c.APIVersion != "" && c.APIVersion != APIVersion {
		return fmt.Errorf("unsupported apiVersion %q", c.APIVersion)
	}
	seen := map[string]struct{}{}
	for i := range c.ColumnTypes {
		if err := ValidateColumnType(&c.ColumnTypes[i]); err != nil {
			return fmt.Errorf("columnTypes[%d]: %w", i, err)
		}
		n := c.ColumnTypes[i].Metadata.Name
		if _, dup := seen[n]; dup {
			return fmt.Errorf("duplicate columnType name %q", n)
		}
		seen[n] = struct{}{}
	}
	return nil
}

// NormalizeColumnType fills defaults on a ColumnType copy.
func NormalizeColumnType(t ColumnType) ColumnType {
	if t.APIVersion == "" {
		t.APIVersion = APIVersion
	}
	if t.Kind == "" {
		t.Kind = KindColumnType
	}
	t.Metadata.Name = strings.TrimSpace(t.Metadata.Name)
	if t.Metadata.Label == "" {
		t.Metadata.Label = t.Metadata.Name
	}
	t.Spec.PgType = strings.TrimSpace(t.Spec.PgType)
	if canon, ok := NormalizeColumnTypePgType(t.Spec.PgType); ok {
		t.Spec.PgType = canon
	}
	return t
}

// FormatNumericType returns numeric(p,s) when precision is set.
func FormatNumericType(precision, scale *int) string {
	if precision == nil {
		return "numeric"
	}
	p := *precision
	s := 0
	if scale != nil {
		s = *scale
	}
	return fmt.Sprintf("numeric(%d,%d)", p, s)
}

// ResolveUnderlyingPgType maps pgType id or raw PG type to SQL for DOMAIN / column DDL.
func ResolveUnderlyingPgType(pgType string, precision, scale *int) (string, error) {
	pgType = strings.TrimSpace(pgType)
	if pgType == "" {
		return "", fmt.Errorf("pgType is empty")
	}
	if strings.Contains(pgType, " ") || strings.HasSuffix(pgType, "[]") ||
		strings.HasPrefix(pgType, "numeric(") {
		return pgType, nil
	}
	if bt, ok := GetPgType(pgType); ok {
		if bt.ID == "number" {
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

func validateRoundingMode(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "half_up", "half_even", "ceil", "floor", "truncate":
		return s, nil
	default:
		return "", fmt.Errorf("spec.roundingMode must be half_up, half_even, ceil, floor, or truncate")
	}
}
