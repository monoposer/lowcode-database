// Package eventschema defines JSON Schemas and constants for lowcode-database
// event delivery. Import from other Go projects or copy schema/**/*.json for
// non-Go consumers.
//
// See README.md (English) and README.zh.md (中文) for envelope layout, event categories, and examples.
package eventschema

// Category groups event types by domain.
type Category string

const (
	CategoryRecords  Category = "records"
	CategoryMetadata Category = "metadata"
)

// Event type identifiers (envelope.type).
const (
	RecordsAfterInsert     = "records.after.insert"
	RecordsAfterUpdate     = "records.after.update"
	RecordsAfterDelete     = "records.after.delete"
	RecordsAfterBulkUpsert = "records.after.bulkUpsert"
	RecordsAfterBulkDelete = "records.after.bulkDelete"
	RecordsAfterBulkImport = "records.after.bulkImport"

	MetadataTableCreated        = "metadata.table.created"
	MetadataTableDeleted        = "metadata.table.deleted"
	MetadataTableRenamed        = "metadata.table.renamed"
	MetadataColumnCreated       = "metadata.column.created"
	MetadataColumnUpdated       = "metadata.column.updated"
	MetadataColumnDeleted       = "metadata.column.deleted"
	MetadataChoiceCreated       = "metadata.choice.created"
	MetadataChoiceUpdated       = "metadata.choice.updated"
	MetadataChoiceDeleted       = "metadata.choice.deleted"
	MetadataRelationCreated     = "metadata.relation.created"
	MetadataRelationDeleted     = "metadata.relation.deleted"
	MetadataIndexCreated          = "metadata.index.created"
	MetadataIndexDeleted          = "metadata.index.deleted"
	MetadataDataSourceCreated     = "metadata.datasource.created"
	MetadataDataSourceUpdated     = "metadata.datasource.updated"
	MetadataDataSourceDeleted     = "metadata.datasource.deleted"
)

// TypeInfo describes one deliverable event type for documentation and tooling.
type TypeInfo struct {
	Type        string   `json:"type"`
	Category    Category `json:"category"`
	Description string   `json:"description"`
}

// AllTypes documents every supported event type.
var AllTypes = []TypeInfo{
	// --- data (tenant row changes) ---
	{RecordsAfterInsert, CategoryRecords, "Fired after a single row is created; data.row is the row snapshot."},
	{RecordsAfterUpdate, CategoryRecords, "Fired after a single row is updated; data.row is the post-update snapshot."},
	{RecordsAfterDelete, CategoryRecords, "Fired after a single row is deleted; data.rowId is the primary key."},
	{RecordsAfterBulkUpsert, CategoryRecords, "Fired after bulk upsert; data.rows is an array of row snapshots."},
	{RecordsAfterBulkDelete, CategoryRecords, "Fired after bulk delete by ids; data.rowIds lists primary keys."},
	{RecordsAfterBulkImport, CategoryRecords, "Fired after import; includes data.rows and data.insertedCount."},

	// --- metadata (schema / platform meta changes) ---
	{MetadataTableCreated, CategoryMetadata, "Logical table created (meta row + data physical table)."},
	{MetadataTableDeleted, CategoryMetadata, "Logical table deleted."},
	{MetadataTableRenamed, CategoryMetadata, "Table renamed; includes oldName, newName, and table resource."},
	{MetadataColumnCreated, CategoryMetadata, "Column created (physical or virtual)."},
	{MetadataColumnUpdated, CategoryMetadata, "Column metadata or physical type changed."},
	{MetadataColumnDeleted, CategoryMetadata, "Column deleted."},
	{MetadataChoiceCreated, CategoryMetadata, "PG ENUM (choice) registered."},
	{MetadataChoiceUpdated, CategoryMetadata, "Choice enum values or metadata changed."},
	{MetadataChoiceDeleted, CategoryMetadata, "Choice deleted."},
	{MetadataRelationCreated, CategoryMetadata, "Table relation definition created."},
	{MetadataRelationDeleted, CategoryMetadata, "Table relation deleted."},
	{MetadataIndexCreated, CategoryMetadata, "PostgreSQL index created."},
	{MetadataIndexDeleted, CategoryMetadata, "Index deleted."},
	{MetadataDataSourceCreated, CategoryMetadata, "DataSource (view/query definition) created."},
	{MetadataDataSourceUpdated, CategoryMetadata, "DataSource updated."},
	{MetadataDataSourceDeleted, CategoryMetadata, "DataSource deleted."},
}

var knownTypes map[string]struct{}

func init() {
	knownTypes = make(map[string]struct{}, len(AllTypes))
	for _, t := range AllTypes {
		knownTypes[t.Type] = struct{}{}
	}
}

// ValidType reports whether typ is a known envelope.type value.
func ValidType(typ string) bool {
	_, ok := knownTypes[typ]
	return ok
}

// CategoryOf returns the category for typ, or empty if unknown.
func CategoryOf(typ string) Category {
	for _, t := range AllTypes {
		if t.Type == typ {
			return t.Category
		}
	}
	return ""
}
