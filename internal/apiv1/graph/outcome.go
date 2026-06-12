package graph

import (
	"github.com/monoposer/lowcode-database/internal/apiv1/row"
)

func (o *SaveGraphSaveOutcome) RootRow() *row.Row {
	if o == nil {
		return nil
	}
	return &row.Row{Id: o.RootID, Cells: o.RootCells}
}

func (o *SaveGraphSaveOutcome) RelatedRowsForHooks(manyRelNames map[string]struct{}) map[string][]*row.Row {
	out := map[string][]*row.Row{}
	if o == nil {
		return out
	}
	for name, r := range o.One {
		if r != nil {
			out[name] = []*row.Row{r}
		}
	}
	for name, children := range o.Many {
		if _, isMany := manyRelNames[name]; !isMany {
			continue
		}
		for _, child := range children {
			if child == nil || child.Deleted || child.Row == nil {
				continue
			}
			out[name] = append(out[name], child.Row)
		}
	}
	return out
}
