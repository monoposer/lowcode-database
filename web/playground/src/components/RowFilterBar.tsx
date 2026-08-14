import { useMemo } from 'react'
import {
  dateTimeInputType,
  effectiveFilterTypeId,
  filterOpsForColumn,
  filterValueInputKind,
  newFilterCondition,
  type FilterGroup,
} from '../filter/dsl'
import type { FilterColumn } from './FilterBuilder'
import { IconPlus } from './icons'
import { columnSelectOptions, simpleSelectOptions, StudioSelect } from './StudioSelect'

type Props = {
  columns: FilterColumn[]
  value: FilterGroup
  onChange: (next: FilterGroup) => void
  onClear: () => void
  loading?: boolean
  rowCount?: number | null
  active?: boolean
}

export function RowFilterBar({
  columns,
  value,
  onChange,
  onClear,
  loading,
  rowCount,
  active,
}: Props) {
  const hasFilter = value.conditions.length > 0

  const updateCondition = (id: string, patch: Partial<FilterGroup['conditions'][0]>) => {
    onChange({
      ...value,
      conditions: value.conditions.map((c) => (c.id === id ? { ...c, ...patch } : c)),
    })
  }

  const onAttrChange = (id: string, attr: string) => {
    const col = columns.find((c) => c.name === attr)
    const ops = filterOpsForColumn(col ?? { name: attr, typeId: 'text' })
    const cur = value.conditions.find((c) => c.id === id)
    const op = ops.some((o) => o.op === cur?.op) ? cur!.op : ops[0].op
    updateCondition(id, { attr, op, val: '' })
  }

  const onOpChange = (id: string, attr: string, op: FilterGroup['conditions'][0]['op']) => {
    const col = columns.find((c) => c.name === attr)
    const filterType = effectiveFilterTypeId(
      col?.typeId ?? 'text',
      col?.expression,
      col?.resultTypeId,
    )
    const ops = filterOpsForColumn(col ?? { name: attr, typeId: 'text' })
    const opDef = ops.find((o) => o.op === op) ?? ops[0]
    const cur = value.conditions.find((c) => c.id === id)
    const prevKind = filterValueInputKind(
      effectiveFilterTypeId(col?.typeId ?? 'text', col?.expression, col?.resultTypeId),
      cur?.op ?? 'EQ',
      ops.find((o) => o.op === cur?.op)?.listValue,
      col?.typeId,
    )
    const nextKind = filterValueInputKind(filterType, op, opDef.listValue, col?.typeId)
    updateCondition(id, { op, val: prevKind === nextKind ? (cur?.val ?? '') : '' })
  }

  const columnOptions = useMemo(() => columnSelectOptions(columns), [columns])

  return (
    <div className={`row-filter-bar${active ? ' row-filter-bar--active' : ''}`}>
      <div className="row-filter-bar-head">
        <span className="row-filter-bar-title">Filter</span>
        {value.conditions.length > 1 && (
          <StudioSelect
            ariaLabel="Filter conjunction"
            value={value.conjunction}
            onChange={(v) =>
              onChange({
                ...value,
                conjunction: v === 'OR' ? 'OR' : 'AND',
              })
            }
            options={[
              { value: 'AND', label: 'All filters (AND)' },
              { value: 'OR', label: 'Any filter (OR)' },
            ]}
            className="row-filter-conjunction-select"
          />
        )}
        <div className="row-filter-bar-actions">
          {hasFilter && (
            <button type="button" className="btn btn-ghost btn-sm" disabled={loading} onClick={onClear}>
              Clear
            </button>
          )}
          <button
            type="button"
            className="btn btn-ghost btn-sm"
            disabled={!columns.length}
              onClick={() => {
              const first = columns[0]?.name ?? ''
              const isArray = columns[0]?.isArray === true
              onChange({
                ...value,
                conditions: [...value.conditions, newFilterCondition(first, isArray)],
              })
            }}
          >
            <IconPlus size={14} /> Add filter
          </button>
          {rowCount != null && (
            <span className="row-filter-count">
              {rowCount} row{rowCount === 1 ? '' : 's'}
              {active ? ' (filtered)' : ''}
            </span>
          )}
        </div>
      </div>

      {hasFilter && (
        <ul className="row-filter-list">
          {value.conditions.map((cond) => {
            const col = columns.find((c) => c.name === cond.attr)
            const rawTypeId = col?.typeId ?? 'text'
            const filterType = effectiveFilterTypeId(rawTypeId, col?.expression, col?.resultTypeId)
            const ops = filterOpsForColumn(col ?? { name: cond.attr, typeId: 'text' })
            const opDef = ops.find((o) => o.op === cond.op) ?? ops[0]

            return (
              <li key={cond.id} className="row-filter-item">
                <StudioSelect
                  ariaLabel="Filter column"
                  value={cond.attr}
                  onChange={(v) => onAttrChange(cond.id, v)}
                  options={columnOptions}
                />
                <StudioSelect
                  ariaLabel="Filter operator"
                  value={cond.op}
                  onChange={(v) => onOpChange(cond.id, cond.attr, v as typeof cond.op)}
                  options={simpleSelectOptions(ops.map((o) => ({ value: o.op, label: o.label })))}
                />
                {opDef.needsValue ? (
                  (() => {
                    const inputKind = filterValueInputKind(
                      filterType,
                      cond.op,
                      opDef.listValue,
                      rawTypeId,
                    )
                    if (inputKind === 'bool') {
                      return (
                        <StudioSelect
                          ariaLabel="Filter value"
                          value={cond.val}
                          onChange={(v) => updateCondition(cond.id, { val: v })}
                          placeholder="—"
                          options={[
                            { value: 'true', label: 'true' },
                            { value: 'false', label: 'false' },
                          ]}
                        />
                      )
                    }
                    if (inputKind === 'datetime') {
                      return (
                        <input
                          type={dateTimeInputType(filterType)}
                          aria-label="Filter value"
                          value={cond.val}
                          onChange={(e) => updateCondition(cond.id, { val: e.target.value })}
                        />
                      )
                    }
                    if (inputKind === 'number') {
                      return (
                        <input
                          type="number"
                          aria-label="Filter value"
                          value={cond.val}
                          onChange={(e) => updateCondition(cond.id, { val: e.target.value })}
                          placeholder={opDef.listValue ? '1, 2, 3' : '0'}
                        />
                      )
                    }
                    return (
                      <input
                        type="text"
                        aria-label="Filter value"
                        className="row-filter-value"
                        value={cond.val}
                        onChange={(e) => updateCondition(cond.id, { val: e.target.value })}
                        placeholder={
                          cond.op === 'ARRAY_OVERLAP' ||
                          cond.op === 'ARRAY_NOT_OVERLAP' ||
                          cond.op === 'ARRAY_CONTAINS' ||
                          cond.op === 'ARRAY_NOT_CONTAINS'
                            ? 'a, b, c'
                            : opDef.op === 'LIKE'
                              ? 'keyword…'
                              : 'value…'
                        }
                      />
                    )
                  })()
                ) : (
                  <span className="row-filter-no-value">—</span>
                )}
                <button
                  type="button"
                  className="row-filter-remove"
                  aria-label="Remove filter"
                  onClick={() =>
                    onChange({
                      ...value,
                      conditions: value.conditions.filter((c) => c.id !== cond.id),
                    })
                  }
                >
                  ×
                </button>
              </li>
            )
          })}
        </ul>
      )}

      {!hasFilter && (
        <p className="row-filter-hint muted">Filters apply automatically as you type.</p>
      )}
    </div>
  )
}
