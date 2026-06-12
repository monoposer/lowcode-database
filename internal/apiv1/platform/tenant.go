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
	Id             string `json:"id,omitempty"`
	DisplayName    string `json:"displayName,omitempty"`
	DataDsn        string `json:"dataDsn,omitempty"`
	ReadDsn        string `json:"readDsn,omitempty"`
	ReadOnly       bool   `json:"readOnly,omitempty"`
	PoolMaxConns   int    `json:"poolMaxConns,omitempty"`
	CreateDatabase bool   `json:"createDatabase,omitempty"`
}

type CreateTenantResponse struct {
	Id string `json:"id,omitempty"`
}
