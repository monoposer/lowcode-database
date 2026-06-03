package apiv1

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	SaveGraphSyncMerge   = "merge"
	SaveGraphSyncReplace = "replace"
)

// SaveGraphManyInput nested many-relationship payload ({ sync, data }).
type SaveGraphManyInput struct {
	Sync SaveGraphSyncMode   `json:"sync,omitempty"`
	Data []json.RawMessage   `json:"data,omitempty"`
}

// SaveGraphSyncMode controls child row synchronization for many relationships.
type SaveGraphSyncMode string

func (m SaveGraphSyncMode) ReplaceMissing() bool {
	return strings.EqualFold(string(m), SaveGraphSyncReplace)
}

// SaveGraphOneInput nested one-relationship payload (create/update related row).
type SaveGraphOneInput struct {
	Id     string            `json:"id,omitempty"`
	Delete bool              `json:"delete,omitempty"`
	Cells  map[string]*Value `json:"cells,omitempty"`
}

// SaveGraphRowPayload classified fields for one row (root, child, or nested).
type SaveGraphRowPayload struct {
	Id                string
	Delete            bool
	Cells             map[string]*Value
	OneRelationships  map[string]*SaveGraphOneInput
	ManyRelationships map[string]*SaveGraphManyInput
}

// SaveGraphRequest body for POST .../rows:saveGraph (flat root + nested relationships).
type SaveGraphRequest struct {
	TableId            string                         `json:"tableId,omitempty"`
	Id                 string                         `json:"id,omitempty"`
	Fields             map[string]json.RawMessage     `json:"-"`
	RootCells          map[string]*Value              `json:"-"`
	ManyRelationships  map[string]*SaveGraphManyInput `json:"-"`
	OneRelationships   map[string]*SaveGraphOneInput  `json:"-"`
	RelationshipSync   map[string]SaveGraphSyncMode   `json:"-"`
}

var saveGraphSkip = map[string]struct{}{
	"tableId": {}, "table_id": {}, "id": {}, "rowId": {}, "row_id": {}, "_sync": {},
}

// UnmarshalJSON accepts flat root fields plus relationship keys.
func (r *SaveGraphRequest) UnmarshalJSON(data []byte) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	r.Fields = make(map[string]json.RawMessage)
	r.RootCells = make(map[string]*Value)
	r.ManyRelationships = make(map[string]*SaveGraphManyInput)
	r.OneRelationships = make(map[string]*SaveGraphOneInput)
	r.RelationshipSync = make(map[string]SaveGraphSyncMode)

	if raw, ok := m["tableId"]; ok {
		_ = json.Unmarshal(raw, &r.TableId)
	}
	if raw, ok := m["table_id"]; ok && r.TableId == "" {
		_ = json.Unmarshal(raw, &r.TableId)
	}
	if raw, ok := m["id"]; ok {
		_ = json.Unmarshal(raw, &r.Id)
	}
	if raw, ok := m["rowId"]; ok && r.Id == "" {
		_ = json.Unmarshal(raw, &r.Id)
	}
	if raw, ok := m["row_id"]; ok && r.Id == "" {
		_ = json.Unmarshal(raw, &r.Id)
	}
	if raw, ok := m["_sync"]; ok {
		var syncMap map[string]string
		if err := json.Unmarshal(raw, &syncMap); err != nil {
			return err
		}
		for k, v := range syncMap {
			mode := SaveGraphSyncMode(strings.ToLower(strings.TrimSpace(v)))
			switch string(mode) {
			case SaveGraphSyncMerge, SaveGraphSyncReplace:
				r.RelationshipSync[k] = mode
			default:
				return fmt.Errorf("_sync.%s: must be merge or replace", k)
			}
		}
	}

	for k, raw := range m {
		if _, skip := saveGraphSkip[k]; skip {
			continue
		}
		r.Fields[k] = raw
	}
	return nil
}

// ClassifySaveGraphFields splits Fields into root cells vs relationship payloads.
func (r *SaveGraphRequest) ClassifySaveGraphFields(manyRelNames, oneRelNames map[string]struct{}) error {
	for name, raw := range r.Fields {
		if _, isMany := manyRelNames[name]; isMany {
			rel, err := ParseSaveGraphManyInput(raw)
			if err != nil {
				return err
			}
			if mode, ok := r.RelationshipSync[name]; ok {
				rel.Sync = mode
			}
			r.ManyRelationships[name] = rel
			continue
		}
		if _, isOne := oneRelNames[name]; isOne {
			rel, err := ParseSaveGraphOneInput(raw)
			if err != nil {
				return err
			}
			r.OneRelationships[name] = rel
			continue
		}
		v, err := unmarshalCellRaw(raw)
		if err != nil {
			return err
		}
		if v != nil {
			r.RootCells[name] = v
		}
	}
	return nil
}

// ClassifySaveGraphRowPayload classifies one row object (child under many relationship).
func ClassifySaveGraphRowPayload(data []byte, manyRelNames, oneRelNames map[string]struct{}) (*SaveGraphRowPayload, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	out := &SaveGraphRowPayload{
		Cells:             make(map[string]*Value),
		OneRelationships:  make(map[string]*SaveGraphOneInput),
		ManyRelationships: make(map[string]*SaveGraphManyInput),
	}
	if raw, ok := m["id"]; ok {
		_ = json.Unmarshal(raw, &out.Id)
	}
	if raw, ok := m["delete"]; ok {
		_ = json.Unmarshal(raw, &out.Delete)
	}
	if raw, ok := m["cells"]; ok {
		cells, err := parseLegacyCells(raw)
		if err != nil {
			return nil, err
		}
		out.Cells = cells
		return out, nil
	}

	for name, raw := range m {
		switch name {
		case "id", "delete", "cells":
			continue
		}
		if _, isMany := manyRelNames[name]; isMany {
			rel, err := ParseSaveGraphManyInput(raw)
			if err != nil {
				return nil, err
			}
			out.ManyRelationships[name] = rel
			continue
		}
		if _, isOne := oneRelNames[name]; isOne {
			rel, err := ParseSaveGraphOneInput(raw)
			if err != nil {
				return nil, err
			}
			out.OneRelationships[name] = rel
			continue
		}
		v, err := unmarshalCellRaw(raw)
		if err != nil {
			return nil, err
		}
		if v != nil {
			out.Cells[name] = v
		}
	}
	return out, nil
}

// ParseSaveGraphOneInput parses a nested one-relationship object.
func ParseSaveGraphOneInput(data []byte) (*SaveGraphOneInput, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("one relationship payload is required")
	}
	switch data[0] {
	case '{':
		// ok
	case '[':
		return nil, fmt.Errorf("one relationship payload must be an object, not an array")
	default:
		return nil, fmt.Errorf("one relationship payload must be a JSON object")
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	out := &SaveGraphOneInput{Cells: make(map[string]*Value)}
	if raw, ok := m["id"]; ok {
		_ = json.Unmarshal(raw, &out.Id)
	}
	if raw, ok := m["delete"]; ok {
		_ = json.Unmarshal(raw, &out.Delete)
	}
	if raw, ok := m["cells"]; ok {
		cells, err := parseLegacyCells(raw)
		if err != nil {
			return nil, err
		}
		out.Cells = cells
		return out, nil
	}
	skip := map[string]struct{}{"id": {}, "delete": {}, "cells": {}}
	cells, err := parseFlatRowFields(m, skip)
	if err != nil {
		return nil, err
	}
	out.Cells = cells
	return out, nil
}

// ParseSaveGraphManyInput parses a many-relationship payload.
// Preferred shape is a JSON array [...]. Optional object { sync, data } or legacy { rows, deleteMissing }.
func ParseSaveGraphManyInput(data []byte) (*SaveGraphManyInput, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("many relationship payload is required")
	}
	if data[0] == '[' {
		var items []json.RawMessage
		if err := json.Unmarshal(data, &items); err != nil {
			return nil, err
		}
		return &SaveGraphManyInput{Sync: SaveGraphSyncMerge, Data: items}, nil
	}
	if data[0] != '{' {
		return nil, fmt.Errorf("many relationship payload must be a JSON array or object")
	}
	var shell struct {
		Sync          string            `json:"sync"`
		Data          []json.RawMessage `json:"data"`
		Rows          []json.RawMessage `json:"rows"`
		DeleteMissing bool              `json:"deleteMissing"`
	}
	if err := json.Unmarshal(data, &shell); err != nil {
		return nil, err
	}
	items := shell.Data
	if items == nil {
		items = shell.Rows
	}
	if items == nil {
		return nil, fmt.Errorf("many relationship payload requires data array")
	}
	sync := SaveGraphSyncMode(strings.ToLower(strings.TrimSpace(shell.Sync)))
	if sync == "" {
		if shell.DeleteMissing {
			sync = SaveGraphSyncReplace
		} else {
			sync = SaveGraphSyncMerge
		}
	}
	switch string(sync) {
	case SaveGraphSyncMerge, SaveGraphSyncReplace:
	default:
		return nil, fmt.Errorf("relationship sync %q must be merge or replace", shell.Sync)
	}
	return &SaveGraphManyInput{Sync: sync, Data: items}, nil
}
