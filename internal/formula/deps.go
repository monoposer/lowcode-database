package formula

import (
	"errors"
	"fmt"
)

// ErrCycle indicates formula columns reference each other in a cycle.
var ErrCycle = errors.New("formula dependency cycle")

// Def is a formula column name and its expression.
type Def struct {
	Name string
	Expr string
}

// Sort returns defs in dependency order (dependencies first).
// Only edges between defs in the same slice are considered; other {{refs}} are ignored for ordering.
func Sort(defs []Def) ([]Def, error) {
	if len(defs) == 0 {
		return nil, nil
	}
	byName := make(map[string]Def, len(defs))
	for _, d := range defs {
		byName[d.Name] = d
	}

	inDegree := make(map[string]int, len(defs))
	dependents := make(map[string][]string, len(defs))
	for name := range byName {
		inDegree[name] = 0
	}
	for _, d := range defs {
		for _, ref := range Refs(d.Expr) {
			if ref == d.Name {
				return nil, fmt.Errorf("formula %q cannot reference itself", d.Name)
			}
			if _, ok := byName[ref]; !ok {
				continue
			}
			inDegree[d.Name]++
			dependents[ref] = append(dependents[ref], d.Name)
		}
	}

	var queue []string
	for name := range byName {
		if inDegree[name] == 0 {
			queue = append(queue, name)
		}
	}

	ordered := make([]Def, 0, len(defs))
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		ordered = append(ordered, byName[name])
		for _, dep := range dependents[name] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}
	if len(ordered) != len(defs) {
		return nil, fmt.Errorf("%w", ErrCycle)
	}
	return ordered, nil
}

// DetectCycle returns an error if formulas[name]=expr introduces or completes a dependency cycle.
func DetectCycle(formulas map[string]string, name, expr string) error {
	merged := make(map[string]string, len(formulas)+1)
	for k, v := range formulas {
		merged[k] = v
	}
	merged[name] = expr

	defs := make([]Def, 0, len(merged))
	for n, e := range merged {
		defs = append(defs, Def{Name: n, Expr: e})
	}
	_, err := Sort(defs)
	return err
}
