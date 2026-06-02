package shared

// LookupWriteSpec describes how to resolve a lookup column value into a local FK column on write.
type LookupWriteSpec struct {
	LookupName    string
	LocalFKColumn string
	LocalFKPgType string
	TargetTableID string
	TargetSchema  string
	TargetTable   string
	SearchColumn  string
	SearchPgType  string
	RefColumn     string
	RefPgType     string
	Filter        map[string]any
	TargetCols    []ColumnMeta
}
