package schema

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/monoposer/lowcode-database/internal/service/shared"
)

// ensureInverseLinkColumn creates the Teable-style symmetric link on the target table
// and cross-wires inverse_field_* on both sides. No-op when already linked or skipped.
// primaryDBID is the lc_columns UUID (PublicColumn replaces Column.Id with the logical name).
//
// Config inverse_field_name on create is the *desired name* for the new inverse field
// (Teable-style), not proof that a symmetric column already exists.
func (s *Schema) ensureInverseLinkColumn(ctx context.Context, tenantID, primaryDBID string, primary *Column, reqCfg map[string]any) (*Column, error) {
	if primary == nil || !shared.CfgBool(primary.Config, "bidirectional") {
		return primary, nil
	}
	if shared.CfgBool(reqCfg, "_skip_inverse") {
		return primary, nil
	}
	if shared.CfgString(primary.Config, "link_column_id") != "" || shared.CfgString(primary.Config, "target_column_id") != "" {
		// Physical-FK style links do not auto-create a symmetric virtual field.
		return primary, nil
	}

	targetTable := shared.CfgString(primary.Config, "target_table_name")
	if targetTable == "" {
		targetTable = shared.CfgString(primary.Config, "to_table_name")
	}
	if targetTable == "" {
		return primary, nil
	}

	invName := shared.CfgString(reqCfg, "inverse_field_name")
	if invName == "" {
		invName = shared.CfgString(primary.Config, "inverse_field_name")
	}
	if invName == "" {
		invName = primary.TableName
	}
	if err := shared.ValidateColumnName(invName); err != nil {
		return nil, fmt.Errorf("inverse link field name: %w", err)
	}

	invCard := strings.ToLower(shared.CfgString(primary.Config, "inverse_cardinality"))
	if invCard != "one" && invCard != "many" {
		invCard = shared.InverseLinkCardinality(shared.CfgString(primary.Config, "cardinality"))
	}

	// Already wired to an existing inverse column — only refresh refs.
	if invDBID, err := s.ResolveColumnDBID(ctx, tenantID, targetTable, invName); err == nil && invDBID != "" {
		if err := s.patchLinkInverseRefs(ctx, tenantID, primary.TableName, primaryDBID, invDBID, invName, invCard); err != nil {
			return nil, err
		}
		_ = s.patchLinkInverseRefs(ctx, tenantID, targetTable, invDBID, primaryDBID, primary.Name, shared.CfgString(primary.Config, "cardinality"))
		primary.Config["inverse_field_id"] = invDBID
		primary.Config["inverse_field_name"] = invName
		primary.Config["inverse_cardinality"] = invCard
		primary.Config["bidirectional"] = true
		return primary, nil
	}

	label := strings.TrimSpace(primary.TableName)
	if primary.Label != "" {
		label = primary.Label
	}

	inv, err := s.AddColumn(ctx, &Column{
		TableName:  targetTable,
		Name:       invName,
		Label:      label,
		TypeId:     "link",
		IsNullable: true,
		Position:   1000,
		Config: map[string]any{
			"target_table_name":   primary.TableName,
			"to_table_name":       primary.TableName,
			"cardinality":         invCard,
			"bidirectional":       true,
			"inverse_cardinality": shared.CfgString(primary.Config, "cardinality"),
			"inverse_field_id":    primaryDBID,
			"inverse_field_name":  primary.Name,
			"_skip_inverse":       true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create inverse link %s.%s: %w", targetTable, invName, err)
	}

	invDBID, err := s.ResolveColumnDBID(ctx, tenantID, targetTable, inv.Name)
	if err != nil {
		return nil, err
	}
	if err := s.patchLinkInverseRefs(ctx, tenantID, primary.TableName, primaryDBID, invDBID, inv.Name, invCard); err != nil {
		return nil, err
	}
	primary.Config["inverse_field_id"] = invDBID
	primary.Config["inverse_field_name"] = inv.Name
	primary.Config["inverse_cardinality"] = invCard
	primary.Config["bidirectional"] = true
	return primary, nil
}

func (s *Schema) patchLinkInverseRefs(ctx context.Context, tenantID, tableName, columnID, invID, invName, invCard string) error {
	baseID, err := s.B.BaseID(ctx)
	if err != nil {
		return err
	}
	patch, err := json.Marshal(map[string]any{
		"bidirectional":       true,
		"inverse_field_id":    invID,
		"inverse_field_name":  invName,
		"inverse_cardinality": invCard,
	})
	if err != nil {
		return err
	}
	_, err = s.B.Tenants.MetaPool().Exec(ctx, `
		UPDATE lc_columns
		SET config = COALESCE(config, '{}'::jsonb) || $1::jsonb,
		    updated_at = now()
		WHERE id = $2 AND tenant_id = $3 AND base_id = $4 AND table_name = $5`,
		patch, columnID, tenantID, baseID, tableName)
	return err
}

func (s *Schema) deleteInverseLinkColumn(ctx context.Context, cfg map[string]any) {
	if !shared.CfgBool(cfg, "bidirectional") {
		return
	}
	invTable := shared.CfgString(cfg, "target_table_name")
	if invTable == "" {
		invTable = shared.CfgString(cfg, "to_table_name")
	}
	invRef := shared.LinkInverseFieldKey(cfg)
	if invTable == "" || invRef == "" {
		return
	}
	_ = s.deleteColumn(ctx, invTable, invRef, true)
}
