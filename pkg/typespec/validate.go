package typespec

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var columnTypeNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)
var checkNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

// ValidateColumnType checks a portable ColumnType document.
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
	if bt, ok := GetPgType(name); ok && !bt.Deprecated {
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
	if _, err := ResolveUnderlyingPgType(pg, spec.Precision, spec.Scale); err != nil {
		return err
	}
	if spec.Default != nil && strings.TrimSpace(*spec.Default) == "" {
		return fmt.Errorf("spec.default must not be empty when set")
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

// ParseNumericModifiers extracts precision/scale from a pg type string like numeric(18,2).
func ParseNumericModifiers(pgType string) (precision, scale *int, ok bool) {
	pgType = strings.TrimSpace(pgType)
	if !strings.HasPrefix(pgType, "numeric(") || !strings.HasSuffix(pgType, ")") {
		return nil, nil, false
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(pgType, "numeric("), ")")
	parts := strings.Split(inner, ",")
	if len(parts) != 2 {
		return nil, nil, false
	}
	p, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	s, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil {
		return nil, nil, false
	}
	return &p, &s, true
}
