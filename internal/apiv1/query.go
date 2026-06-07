package apiv1

type SortOrder struct {
	Attribute string `json:"attribute,omitempty"`
	SortOrder string `json:"sortOrder,omitempty"`
}
