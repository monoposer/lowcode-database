import type { Column, ColType } from '../api'

function isLinkTypeId(typeId: string) {
  return typeId === 'link'
}

export function isWritableColumn(c: Column) {
  return (
    c.typeId !== 'formula' &&
    !isLinkTypeId(c.typeId) &&
    c.typeId !== 'lookup' &&
    c.typeId !== 'rollup'
  )
}

/** Array-ness lives on the type (columnType.spec.array), exposed as Type.config.array / pgType[]. */
export function isArrayType(t: ColType | undefined): boolean {
  if (!t) return false
  if (t.config?.array === true) return true
  return typeof t.pgType === 'string' && t.pgType.endsWith('[]')
}

export function isArrayColumn(c: Column, types: ColType[]): boolean {
  return isArrayType(types.find((t) => t.id === c.typeId))
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

/** Scalar/link fields stored in record.data (not formula/lookup/rollup). */
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
  return cfgString(c.config, 'target_table_name')
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
  if (typeId === 'number') return 'type-num'
  if (typeId === 'text') return 'type-text'
  if (typeId === 'boolean') return 'type-bool'
  if (typeId === 'formula') return 'type-formula'
  if (isLinkTypeId(typeId)) return 'type-fk'
  if (typeId === 'lookup' || typeId === 'rollup') return 'type-virtual'
  if (typeId === 'datetime') return 'type-date'
  if (typeId === 'jsonb') return 'type-json'
  return 'type-default'
}
