package platform

import (
	"context"
	"fmt"
	"github.com/solat/lowcode-database/internal/apiv1/platform"

	"github.com/solat/lowcode-database/internal/platform/authn"
	"time"
)

func (s *Platform) CreateAPIKey(ctx context.Context, req *platform.CreateAPIKeyRequest) (*platform.CreateAPIKeyResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	plain, hash, prefix, err := authn.GenerateKey()
	if err != nil {
		return nil, err
	}
	meta := s.B.Tenants.MetaPool()
	var ak platform.APIKey
	var createdAt, updatedAt time.Time
	err = meta.QueryRow(ctx, `
		INSERT INTO lc_api_keys (tenant_id, name, key_hash, key_prefix, rate_limit_rps)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, key_prefix, enabled, rate_limit_rps, created_at, updated_at
	`, tid, req.Name, hash, prefix, req.RateLimitRps).Scan(
		&ak.Id, &ak.Name, &ak.KeyPrefix, &ak.Enabled, &ak.RateLimitRps, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	ak.CreatedAt = createdAt
	ak.UpdatedAt = updatedAt
	return &platform.CreateAPIKeyResponse{ApiKey: &ak, Key: plain}, nil
}

func (s *Platform) ListAPIKeys(ctx context.Context, _ *platform.ListAPIKeysRequest) (*platform.ListAPIKeysResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.B.Tenants.MetaPool().Query(ctx, `
		SELECT id, name, key_prefix, enabled, rate_limit_rps, created_at, updated_at
		FROM lc_api_keys WHERE tenant_id = $1 ORDER BY created_at
	`, tid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out platform.ListAPIKeysResponse
	for rows.Next() {
		var ak platform.APIKey
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&ak.Id, &ak.Name, &ak.KeyPrefix, &ak.Enabled, &ak.RateLimitRps, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		ak.CreatedAt = createdAt
		ak.UpdatedAt = updatedAt
		out.ApiKeys = append(out.ApiKeys, &ak)
	}
	return &out, rows.Err()
}

func (s *Platform) DeleteAPIKey(ctx context.Context, req *platform.DeleteAPIKeyRequest) (*platform.DeleteAPIKeyResponse, error) {
	tid, err := s.B.TenantID(ctx)
	if err != nil {
		return nil, err
	}
	if req.Id == "" {
		return nil, fmt.Errorf("id is required")
	}
	tag, err := s.B.Tenants.MetaPool().Exec(ctx, `DELETE FROM lc_api_keys WHERE id = $1 AND tenant_id = $2`, req.Id, tid)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("api key not found")
	}
	return &platform.DeleteAPIKeyResponse{}, nil
}
