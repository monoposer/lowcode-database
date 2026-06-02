package apiv1

import (
	"encoding/json"
)

// SaveGraphResponse echoes the request JSON shape with generated ids filled in.
type SaveGraphResponse map[string]any

// SaveGraphChildSaveOutcome one saved row under a many relationship (index-aligned with input data).
type SaveGraphChildSaveOutcome struct {
	Deleted          bool
	Row              *Row
	OneRelationships map[string]*Row
}

// SaveGraphSaveOutcome internal save result used to build the echo response and webhooks.
type SaveGraphSaveOutcome struct {
	RootID          string
	RootCells       map[string]*Value
	One             map[string]*Row
	Many            map[string][]*SaveGraphChildSaveOutcome
	ManyLinkColumns map[string]string
}

// BuildSaveGraphEcho returns the request-shaped response with ids (and resolved FK columns) filled in.
func BuildSaveGraphEcho(req *SaveGraphRequest, out *SaveGraphSaveOutcome) SaveGraphResponse {
	if out == nil {
		return SaveGraphResponse{}
	}
	echo := SaveGraphResponse{}
	if out.RootID != "" {
		echo["id"] = out.RootID
	}
	for name, v := range out.RootCells {
		if name == "id" {
			continue
		}
		if native := ValueToNative(v); native != nil {
			echo[name] = native
		}
	}
	for relName, row := range out.One {
		if row == nil {
			continue
		}
		raw, ok := req.Fields[relName]
		if !ok {
			echo[relName] = rowToEchoMap(row, "")
			continue
		}
		echo[relName] = patchOneRelEcho(raw, row)
	}
	for relName, outcomes := range out.Many {
		raw, ok := req.Fields[relName]
		if !ok {
			continue
		}
		linkCol := out.ManyLinkColumns[relName]
		if patched, err := patchManyRelEcho(raw, outcomes, linkCol); err == nil && patched != nil {
			echo[relName] = patched
		}
	}
	return echo
}

func patchOneRelEcho(raw json.RawMessage, row *Row) map[string]any {
	m := rawJSONToMap(raw)
	if row != nil && row.Id != "" {
		m["id"] = row.Id
	}
	for k, v := range row.Cells {
		if _, exists := m[k]; !exists {
			if native := ValueToNative(v); native != nil {
				m[k] = native
			}
		}
	}
	return m
}

func patchManyRelEcho(raw json.RawMessage, outcomes []*SaveGraphChildSaveOutcome, linkCol string) (any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if raw[0] == '[' {
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, err
		}
		out := make([]map[string]any, 0, len(items))
		for i, itemRaw := range items {
			m := rawJSONToMap(itemRaw)
			if i < len(outcomes) && outcomes[i] != nil && !outcomes[i].Deleted {
				patchChildEchoMap(m, outcomes[i], linkCol)
			}
			out = append(out, m)
		}
		return out, nil
	}
	var shell map[string]json.RawMessage
	if err := json.Unmarshal(raw, &shell); err != nil {
		return nil, err
	}
	result := make(map[string]any, len(shell))
	var dataRaw []json.RawMessage
	if dr, ok := shell["data"]; ok {
		_ = json.Unmarshal(dr, &dataRaw)
	} else if dr, ok := shell["rows"]; ok {
		_ = json.Unmarshal(dr, &dataRaw)
	}
	if dataRaw != nil {
		patched, err := patchManyRelEcho(mustMarshalJSON(dataRaw), outcomes, linkCol)
		if err != nil {
			return nil, err
		}
		result["data"] = patched
	}
	for k, v := range shell {
		if k == "data" || k == "rows" {
			continue
		}
		var anyVal any
		_ = json.Unmarshal(v, &anyVal)
		result[k] = anyVal
	}
	return result, nil
}

func patchChildEchoMap(m map[string]any, outcome *SaveGraphChildSaveOutcome, linkCol string) {
	if outcome == nil || outcome.Row == nil {
		return
	}
	if outcome.Row.Id != "" {
		m["id"] = outcome.Row.Id
	}
	for k, v := range outcome.Row.Cells {
		if linkCol != "" && k == linkCol {
			continue
		}
		if _, exists := m[k]; !exists {
			if native := ValueToNative(v); native != nil {
				m[k] = native
			}
		}
	}
	for relName, oneRow := range outcome.OneRelationships {
		if oneRow == nil {
			continue
		}
		if nested, ok := m[relName].(map[string]any); ok {
			if oneRow.Id != "" {
				nested["id"] = oneRow.Id
			}
			continue
		}
		m[relName] = rowToEchoMap(oneRow, "")
	}
}

func rowToEchoMap(row *Row, skipCol string) map[string]any {
	m := map[string]any{}
	if row == nil {
		return m
	}
	if row.Id != "" {
		m["id"] = row.Id
	}
	for k, v := range row.Cells {
		if k == skipCol {
			continue
		}
		if native := ValueToNative(v); native != nil {
			m[k] = native
		}
	}
	return m
}

func rawJSONToMap(raw json.RawMessage) map[string]any {
	m := map[string]any{}
	if len(raw) == 0 {
		return m
	}
	_ = json.Unmarshal(raw, &m)
	return m
}

func mustMarshalJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// RootRow returns the saved root row for webhooks.
func (o *SaveGraphSaveOutcome) RootRow() *Row {
	if o == nil {
		return nil
	}
	return &Row{Id: o.RootID, Cells: o.RootCells}
}

// RelatedRowsForHooks returns related table rows for webhook emission.
func (o *SaveGraphSaveOutcome) RelatedRowsForHooks(manyRelNames map[string]struct{}) map[string][]*Row {
	out := map[string][]*Row{}
	if o == nil {
		return out
	}
	for name, row := range o.One {
		if row != nil {
			out[name] = []*Row{row}
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
