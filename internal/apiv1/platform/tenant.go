package platform

import "github.com/monoposer/lowcode-database/internal/apiv1/schema"

type ListTypesRequest struct{}

type ListTypesResponse struct {
	Types []*schema.Type `json:"types,omitempty"`
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

type CreateTenantRequest struct {
	Id             string   `json:"id,omitempty"`
	DisplayName    string   `json:"displayName,omitempty"`
	DataDsn        string   `json:"dataDsn,omitempty"`
	DataDsnWrite   string   `json:"dataDsnWrite,omitempty"`
	DataDsnReads   []string `json:"dataDsnReads,omitempty"`
	PoolMaxConns   int      `json:"poolMaxConns,omitempty"`
	CreateDatabase bool     `json:"createDatabase,omitempty"`
	// RecordStore is "shared" (public.record) or "dedicated" ({tenant_id}_record).
	// Physical DB isolation is dataDsn, not this field.
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
