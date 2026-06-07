package formula

import "fmt"

// BuildSteps validates formula defs in dependency order.
// Formulas are evaluated in the application (see Eval); they are not compiled to SQL.
func BuildSteps(_ string, _ map[string]string, defs []Def) error {
	sorted, err := Sort(defs)
	if err != nil {
		return err
	}
	for _, d := range sorted {
		if _, err := Parse(d.Expr); err != nil {
			return fmt.Errorf("formula %q: %w", d.Name, err)
		}
	}
	return nil
}
