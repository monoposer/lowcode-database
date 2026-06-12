package platform

import "time"

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
