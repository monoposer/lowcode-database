package graph

import (
	"encoding/json"

	"github.com/solat/lowcode-database/internal/apiv1"
	"github.com/solat/lowcode-database/internal/apiv1/row"
)

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
		if native := apiv1.ValueToNative(v); native != nil {
			echo[name] = native
		}
	}
	for relName, r := range out.One {
		if r == nil {
			continue
		}
		raw, ok := req.Fields[relName]
		if !ok {
			echo[relName] = rowToEchoMap(r, "")
			continue
		}
		echo[relName] = patchOneRelEcho(raw, r)
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

func patchOneRelEcho(raw json.RawMessage, r *row.Row) map[string]any {
	m := rawJSONToMap(raw)
	if r != nil && r.Id != "" {
		m["id"] = r.Id
	}
	for k, v := range r.Cells {
		if _, exists := m[k]; !exists {
			if native := apiv1.ValueToNative(v); native != nil {
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
			if native := apiv1.ValueToNative(v); native != nil {
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

func rowToEchoMap(r *row.Row, skipCol string) map[string]any {
	m := map[string]any{}
	if r == nil {
		return m
	}
	if r.Id != "" {
		m["id"] = r.Id
	}
	for k, v := range r.Cells {
		if k == skipCol {
			continue
		}
		if native := apiv1.ValueToNative(v); native != nil {
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
