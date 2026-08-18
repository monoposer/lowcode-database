package data

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/monoposer/lowcode-database/internal/service/calc"
	"github.com/monoposer/lowcode-database/internal/service/shared"
)

type linkPeer struct {
	TableName string
	RecordID  string
}

func (s *Data) persistLinks(ctx context.Context, pool *pgxpool.Pool, tenantID, tableName, recordID string, cols []shared.FullColumnMeta, links map[string][]string) ([]linkPeer, error) {
	if len(links) == 0 {
		return nil, nil
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	peers, err := persistLinksTx(ctx, tx, tenantID, tableName, recordID, cols, links)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return peers, nil
}

func persistLinksTx(ctx context.Context, tx pgx.Tx, tenantID, tableName, recordID string, cols []shared.FullColumnMeta, links map[string][]string) ([]linkPeer, error) {
	seen := map[string]struct{}{}
	var peers []linkPeer
	addPeer := func(table, id string) {
		id = trimLinkID(id)
		if table == "" || id == "" {
			return
		}
		if id == recordID && table == tableName {
			return
		}
		key := table + "\x00" + id
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		peers = append(peers, linkPeer{TableName: table, RecordID: id})
	}
	for fieldRef, ids := range links {
		col, ok := linkFieldMeta(cols, fieldRef)
		if !ok {
			continue
		}
		toTable := shared.CfgString(col.Config, "to_table_name")
		if toTable == "" {
			toTable = shared.CfgString(col.Config, "target_table_name")
		}
		fieldKey := col.Name
		oldIDs, _ := calc.ListToIDs(ctx, tx, tenantID, recordID, fieldKey)
		for _, id := range oldIDs {
			addPeer(toTable, id)
		}
		if linkCardinality(col) == "one" && len(ids) > 1 {
			ids = ids[:1]
		}
		for _, id := range ids {
			addPeer(toTable, id)
		}
		if err := calc.ReplaceLinks(ctx, tx, tenantID, tableName, recordID, fieldKey, toTable, ids); err != nil {
			return nil, err
		}
		if !shared.CfgBool(col.Config, "bidirectional") {
			continue
		}
		inv := shared.LinkInverseFieldKey(col.Config)
		if inv == "" {
			continue
		}
		if err := calc.DeleteInverseForField(ctx, tx, tenantID, inv, oldIDs, recordID); err != nil {
			return nil, err
		}
		invCard := stringsToCard(shared.CfgString(col.Config, "inverse_cardinality"))
		if invCard == "" {
			invCard = shared.InverseLinkCardinality(shared.CfgString(col.Config, "cardinality"))
		}
		for _, toID := range ids {
			toID = trimLinkID(toID)
			if toID == "" {
				continue
			}
			if invCard == "one" {
				oldParents, _ := calc.ListToIDs(ctx, tx, tenantID, toID, inv)
				if err := calc.ReplaceLinks(ctx, tx, tenantID, toTable, toID, inv, tableName, []string{recordID}); err != nil {
					return nil, err
				}
				for _, oldParent := range oldParents {
					addPeer(tableName, oldParent)
					if oldParent == recordID {
						continue
					}
					if err := calc.DeleteEdge(ctx, tx, tenantID, oldParent, fieldKey, toID); err != nil {
						return nil, err
					}
				}
				continue
			}
			if err := calc.InsertInverse(ctx, tx, tenantID, tableName, recordID, inv, toTable, toID); err != nil {
				return nil, err
			}
		}
	}
	return peers, nil
}

func enqueueLinkPeers(ctx context.Context, pool *pgxpool.Pool, tenantID, selfTable, selfID string, peers []linkPeer) {
	for _, p := range peers {
		if p.TableName == "" || p.RecordID == "" {
			continue
		}
		if p.TableName == selfTable && p.RecordID == selfID {
			continue
		}
		_ = calc.Enqueue(ctx, pool, tenantID, p.TableName, p.RecordID, nil)
	}
}

func stringsToCard(s string) string {
	switch s {
	case "one", "many":
		return s
	default:
		return ""
	}
}

func trimLinkID(s string) string {
	if s == "<nil>" {
		return ""
	}
	return s
}
