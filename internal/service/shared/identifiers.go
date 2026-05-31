package shared

import (
	"fmt"
	"regexp"
	"strings"
)

var pgObjectNameRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,62}$`)

func ValidateTableName(name string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if !pgObjectNameRe.MatchString(name) {
		return fmt.Errorf("name must match %s (English identifier, PG-compatible)", pgObjectNameRe.String())
	}
	return nil
}

func ValidateColumnName(name string) error {
	if err := ValidateTableName(name); err != nil {
		return err
	}
	if strings.EqualFold(name, "id") {
		return fmt.Errorf("column name %q is reserved (row primary key)", name)
	}
	return nil
}
