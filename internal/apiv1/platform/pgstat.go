package platform

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
