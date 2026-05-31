package shared

// ColumnMeta is a physical column for row read/write.
type ColumnMeta struct {
	Id         string
	TableId    string
	Name       string
	TypeId     string
	PgType     string
	IsNullable bool
	Position   int32
}

// RelationshipColumn describes a relationship column for expand queries.
type RelationshipColumn struct {
	Id             string
	TargetTableId  string
	LinkColumnId   string
	TargetColumnId string
	Cardinality    string
}

// FullColumnMeta includes virtual column metadata for query building.
type FullColumnMeta struct {
	Id         string
	TableId    string
	Name       string
	TypeId     string
	Kind       string
	PgType     string
	IsNullable bool
	Position   int32
	Config     map[string]any
	IsVirtual  bool
}

// CachedColumnMetaBundle is stored in meta cache for a table.
type CachedColumnMetaBundle struct {
	Cols       []FullColumnMeta
	SchemaName string
	TableName  string
}
