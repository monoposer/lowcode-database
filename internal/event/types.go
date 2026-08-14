package event

// Event type identifiers used by shared.EmitEvent after row/schema writes.
// metadata.* is published on the bus as schema.*; webhooks consume the bus.
const (
	RecordsAfterInsert     = "records.after.insert"
	RecordsAfterUpdate     = "records.after.update"
	RecordsAfterDelete     = "records.after.delete"
	RecordsAfterBulkUpsert = "records.after.bulkUpsert"
	RecordsAfterBulkDelete = "records.after.bulkDelete"

	MetadataTableCreated    = "metadata.table.created"
	MetadataTableDeleted    = "metadata.table.deleted"
	MetadataTableRenamed    = "metadata.table.renamed"
	MetadataColumnCreated   = "metadata.column.created"
	MetadataColumnUpdated   = "metadata.column.updated"
	MetadataColumnDeleted   = "metadata.column.deleted"
	MetadataRelationCreated = "metadata.relation.created"
	MetadataRelationDeleted = "metadata.relation.deleted"
	MetadataIndexCreated    = "metadata.index.created"
	MetadataIndexDeleted    = "metadata.index.deleted"
	MetadataQueryCreated    = "metadata.query.created"
	MetadataQueryUpdated    = "metadata.query.updated"
	MetadataQueryDeleted    = "metadata.query.deleted"
)
