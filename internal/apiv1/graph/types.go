package graph

import (
	"encoding/json"
	"strings"

	"github.com/solat/lowcode-database/internal/apiv1"
	"github.com/solat/lowcode-database/internal/apiv1/row"
)

const (
	SaveGraphSyncMerge   = "merge"
	SaveGraphSyncReplace = "replace"
)

type SaveGraphManyInput struct {
	Sync SaveGraphSyncMode `json:"sync,omitempty"`
	Data []json.RawMessage `json:"data,omitempty"`
}

type SaveGraphSyncMode string

func (m SaveGraphSyncMode) ReplaceMissing() bool {
	return strings.EqualFold(string(m), SaveGraphSyncReplace)
}

type SaveGraphOneInput struct {
	Id     string                  `json:"id,omitempty"`
	Delete bool                    `json:"delete,omitempty"`
	Cells  map[string]*apiv1.Value `json:"cells,omitempty"`
}

type SaveGraphRowPayload struct {
	Id                string
	Delete            bool
	Cells             map[string]*apiv1.Value
	OneRelationships  map[string]*SaveGraphOneInput
	ManyRelationships map[string]*SaveGraphManyInput
}

type SaveGraphRequest struct {
	TableId           string                         `json:"tableId,omitempty"`
	Id                string                         `json:"id,omitempty"`
	Fields            map[string]json.RawMessage     `json:"-"`
	RootCells         map[string]*apiv1.Value        `json:"-"`
	ManyRelationships map[string]*SaveGraphManyInput `json:"-"`
	OneRelationships  map[string]*SaveGraphOneInput  `json:"-"`
	RelationshipSync  map[string]SaveGraphSyncMode   `json:"-"`
}

var saveGraphSkip = map[string]struct{}{
	"tableId": {}, "table_id": {}, "id": {}, "rowId": {}, "row_id": {}, "_sync": {},
}

type SaveGraphResponse map[string]any

type SaveGraphChildSaveOutcome struct {
	Deleted          bool
	Row              *row.Row
	OneRelationships map[string]*row.Row
}

type SaveGraphSaveOutcome struct {
	RootID          string
	RootCells       map[string]*apiv1.Value
	One             map[string]*row.Row
	Many            map[string][]*SaveGraphChildSaveOutcome
	ManyLinkColumns map[string]string
}
