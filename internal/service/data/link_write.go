package data

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/monoposer/lowcode-database/internal/service/calc"
	"github.com/monoposer/lowcode-database/internal/service/shared"
)

func (s *Data) persistLinks(ctx context.Context, pool *pgxpool.Pool, tenantID, tableName, recordID string, cols []shared.FullColumnMeta, links map[string][]string) error {
	if len(links) == 0 {
		return nil
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := persistLinksTx(ctx, tx, tenantID, tableName, recordID, cols, links); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func persistLinksTx(ctx context.Context, tx pgx.Tx, tenantID, tableName, recordID string, cols []shared.FullColumnMeta, links map[string][]string) error {
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
		if err := calc.ReplaceLinks(ctx, tx, tenantID, tableName, recordID, fieldKey, toTable, ids); err != nil {
			return err
		}
		if !shared.CfgBool(col.Config, "bidirectional") {
			continue
		}
		inv := shared.LinkInverseFieldKey(col.Config)
		if inv == "" {
			continue
		}
		if err := calc.DeleteInverseForField(ctx, tx, tenantID, inv, oldIDs, recordID); err != nil {
			return err
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
				// Teable ManyOne inverse: reassign child → this parent and detach from other parents' many field.
				oldParents, _ := calc.ListToIDs(ctx, tx, tenantID, toID, inv)
				if err := calc.ReplaceLinks(ctx, tx, tenantID, toTable, toID, inv, tableName, []string{recordID}); err != nil {
					return err
				}
				for _, oldParent := range oldParents {
					if oldParent == recordID {
						continue
					}
					if err := calc.DeleteEdge(ctx, tx, tenantID, oldParent, fieldKey, toID); err != nil {
						return err
					}
				}
				continue
			}
			if err := calc.InsertInverse(ctx, tx, tenantID, tableName, recordID, inv, toTable, toID); err != nil {
				return err
			}
		}
	}
	return nil
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
