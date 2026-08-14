package platform

import (
	"time"

	"github.com/monoposer/lowcode-database/internal/service/shared"
)

type CreateTenantRequest struct {
	Id             string   `json:"id,omitempty"`
	DisplayName    string   `json:"displayName,omitempty"`
	DataDsn        string   `json:"dataDsn,omitempty"`
	DataDsnWrite   string   `json:"dataDsnWrite,omitempty"`
	DataDsnReads   []string `json:"dataDsnReads,omitempty"`
	PoolMaxConns   int      `json:"poolMaxConns,omitempty"`
	CreateDatabase bool     `json:"createDatabase,omitempty"`
	// RecordStore is "shared" (public.record) or "dedicated" ({tenant_id}_record).
	RecordStore string `json:"recordStore,omitempty"`
}

type UpdateTenantRequest struct {
	DataDsn      string   `json:"dataDsn,omitempty"`
	DataDsnWrite string   `json:"dataDsnWrite,omitempty"`
	DataDsnReads []string `json:"dataDsnReads,omitempty"`
}

type UpdateTenantResponse struct {
	Id string `json:"id,omitempty"`
}

type CreateTenantResponse struct {
	Id          string `json:"id,omitempty"`
	RecordStore string `json:"recordStore,omitempty"`
}

type GetDatabaseConnectionRequest struct{}

type GetDatabaseConnectionResponse struct {
	Host               string `json:"host,omitempty"`
	Port               int32  `json:"port,omitempty"`
	Database           string `json:"database,omitempty"`
	User               string `json:"user,omitempty"`
	UrlWithoutPassword string `json:"urlWithoutPassword,omitempty"`
	PsqlCommand        string `json:"psqlCommand,omitempty"`
	PasswordSourceHint string `json:"passwordSourceHint,omitempty"`
}

type APIKey struct {
	Id           string    `json:"id,omitempty"`
	Name         string    `json:"name,omitempty"`
	KeyPrefix    string    `json:"keyPrefix,omitempty"`
	Enabled      bool      `json:"enabled,omitempty"`
	RateLimitRps int32     `json:"rateLimitRps,omitempty"`
	CreatedAt    time.Time `json:"createdAt,omitempty"`
	UpdatedAt    time.Time `json:"updatedAt,omitempty"`
}

type CreateAPIKeyRequest struct {
	Name         string `json:"name,omitempty"`
	RateLimitRps int32  `json:"rateLimitRps,omitempty"`
}

type CreateAPIKeyResponse struct {
	ApiKey *APIKey `json:"apiKey,omitempty"`
	Key    string  `json:"key,omitempty"`
}

type ListAPIKeysRequest struct{}

type ListAPIKeysResponse struct {
	ApiKeys []*APIKey `json:"apiKeys,omitempty"`
}

type DeleteAPIKeyRequest struct {
	Id string `json:"id,omitempty"`
}

type DeleteAPIKeyResponse struct{}

type PGStatStatement struct {
	QueryID        string  `json:"queryId,omitempty"`
	Query          string  `json:"query,omitempty"`
	Calls          int64   `json:"calls"`
	TotalExecTime  float64 `json:"totalExecTimeMs"`
	MeanExecTime   float64 `json:"meanExecTimeMs"`
	MinExecTime    float64 `json:"minExecTimeMs"`
	MaxExecTime    float64 `json:"maxExecTimeMs"`
	Rows           int64   `json:"rows"`
	SharedBlksHit  int64   `json:"sharedBlksHit"`
	SharedBlksRead int64   `json:"sharedBlksRead"`
}

type ListPGStatStatementsRequest struct {
	Limit int `json:"limit,omitempty"`
}

type ListPGStatStatementsResponse struct {
	Enabled    bool              `json:"enabled"`
	Statements []PGStatStatement `json:"statements"`
}

// Query is a saved projection + filter + sort (meta lc_queries).
type Query struct {
	Id        string              `json:"id,omitempty"`
	Name      string              `json:"name,omitempty"`
	Label     string              `json:"label,omitempty"`
	TableName   string              `json:"tableName,omitempty"`
	Filter    map[string]any      `json:"filter,omitempty"`
	Sort      []*shared.SortOrder `json:"sort,omitempty"`
	ColumnIds []string            `json:"columnIds,omitempty"`
	Config    map[string]any      `json:"config,omitempty"`
	CreatedAt time.Time           `json:"createdAt,omitempty"`
	UpdatedAt time.Time           `json:"updatedAt,omitempty"`
}

// ExecuteQueryRequest is the saved-query run input (used by the data service).
type ExecuteQueryRequest struct {
	TableName     string         `json:"tableName,omitempty"`
	QueryId     string         `json:"queryId,omitempty"`
	PageSize    int32          `json:"pageSize,omitempty"`
	PageToken   string         `json:"pageToken,omitempty"`
	Filter      map[string]any `json:"filter,omitempty"`
	Params      map[string]any `json:"params,omitempty"`
	ColumnIds   []string       `json:"columnIds,omitempty"`
	Consistency string         `json:"consistency,omitempty"`
}
