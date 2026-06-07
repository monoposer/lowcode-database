import type { Column } from '../api'

function isLinkTypeId(typeId: string) {
  return typeId === 'link' || typeId === 'relationship' || typeId === 'relation_fk'
}

export function isWritableColumn(c: Column) {
  return (
    c.typeId !== 'formula' &&
    !isLinkTypeId(c.typeId) &&
    c.typeId !== 'lookup' &&
    c.typeId !== 'rollup'
  )
}

export function isArrayColumnType(typeId: string, config?: Record<string, unknown>) {
  if (typeId.endsWith('_array')) return true
  return config?.array === true
}

/** typeId used for cell parse/format (legacy *_array suffix when config.array is set). */
export function valueTypeId(typeId: string, config?: Record<string, unknown>) {
  if (isArrayColumnType(typeId, config) && !typeId.endsWith('_array')) {
    return `${typeId}_array`
  }
  return typeId
}

export function isGridColumn(c: Column) {
  return !isLinkTypeId(c.typeId)
}

export function isComputedColumn(c: Column) {
  return c.typeId === 'formula' || c.typeId === 'lookup' || c.typeId === 'rollup'
}

export function columnHeaderPrefix(c: Column): string {
  switch (c.typeId) {
    case 'formula':
      return 'ƒ '
    case 'lookup':
      return '↗ '
    case 'rollup':
      return 'Σ '
    default:
      return ''
  }
}

export function isFormulaColumn(c: Column) {
  return c.typeId === 'formula'
}

export function columnExpression(c: Column): string {
  const cfg = c.config
  if (!cfg) return ''
  const expr = cfg.expression ?? cfg.formula
  return typeof expr === 'string' ? expr : ''
}

/** Stored column under current API (typed PG column or rls_table cell). RFC: fields live in virtual_records.data JSONB. */
export function isPhysicalColumn(c: Column) {
  return !isVirtualKind(c)
}

/** Query-time virtual kinds (no dedicated storage column). Same under Virtual-Records RFC. */
export function isVirtualKind(c: Column) {
  return (
    c.typeId === 'formula' ||
    isLinkTypeId(c.typeId) ||
    c.typeId === 'lookup' ||
    c.typeId === 'rollup'
  )
}

export function cfgString(cfg: Record<string, unknown> | undefined, key: string): string {
  const v = cfg?.[key]
  return typeof v === 'string' ? v : ''
}

export function isRelationshipColumn(c: Column) {
  return isLinkTypeId(c.typeId)
}

export function relationshipCardinality(c: Column): 'one' | 'many' | undefined {
  const cfg = c.config
  if (!cfg) return undefined
  if (cfg.cardinality === 'one' || cfg.cardinality === 'many') return cfg.cardinality
  if (cfgString(cfg, 'link_column_id')) return 'many'
  if (cfgString(cfg, 'target_column_id')) return 'one'
  return undefined
}

export function relationshipTargetTable(c: Column): string {
  return cfgString(c.config, 'target_table_id')
}

export function isLookupTargetColumn(c: Column) {
  return !isLinkTypeId(c.typeId)
}

export function columnDisplay(c: Column): string {
  const label = c.label?.trim()
  return label || c.name
}

export function resolveColumnRef(columns: Column[], ref: string): Column | undefined {
  return columns.find((c) => c.name === ref || c.id === ref)
}

export function typeBadgeColor(typeId: string): string {
  if (typeId === 'uuid' || typeId === 'int8' || typeId === 'number') return 'type-id'
  if (typeId === 'text' || typeId === 'varchar') return 'type-text'
  if (typeId === 'bool' || typeId === 'boolean') return 'type-bool'
  if (typeId === 'number' || typeId === 'double' || typeId === 'integer') return 'type-num'
  if (typeId === 'formula') return 'type-formula'
  if (isLinkTypeId(typeId)) return 'type-fk'
  if (typeId === 'lookup' || typeId === 'rollup') return 'type-virtual'
  if (typeId.includes('timestamp') || typeId === 'date' || typeId === 'datetime') return 'type-date'
  if (typeId === 'json' || typeId === 'jsonb') return 'type-json'
  return 'type-default'
}
