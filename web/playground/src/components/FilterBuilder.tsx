import { useMemo } from 'react'
import {
  buildFilterDSL,
  dateTimeInputType,
  emptyFilterGroup,
  effectiveFilterTypeId,
  filterOpsForColumn,
  filterValueInputKind,
  newFilterCondition,
  type FilterGroup,
} from '../filter/dsl'
import { columnSelectOptions, simpleSelectOptions, StudioSelect } from './StudioSelect'

export type FilterColumn = {
  name: string
  label?: string
  typeId: string
  /** Effective value type from API (config.result_type_id). */
  resultTypeId?: string
  /** formula expression fallback when resultTypeId missing */
  expression?: string
}

type Props = {
  columns: FilterColumn[]
  value: FilterGroup
  onChange: (next: FilterGroup) => void
  /** First condition value input test id (e2e) */
  valueTestId?: string
}

export function FilterBuilder({ columns, value, onChange, valueTestId }: Props) {
  const columnTypes = useMemo(() => {
    const m: Record<string, string> = {}
    for (const c of columns) m[c.name] = c.typeId
    return m
  }, [columns])

  const columnExpressions = useMemo(() => {
    const m: Record<string, string> = {}
    for (const c of columns) {
      if (c.expression) m[c.name] = c.expression
    }
    return m
  }, [columns])

  const columnResultTypes = useMemo(() => {
    const m: Record<string, string> = {}
    for (const c of columns) {
      if (c.resultTypeId) m[c.name] = c.resultTypeId
    }
    return m
  }, [columns])

  const preview = useMemo(
    () => buildFilterDSL(value, columnTypes, columnExpressions, columnResultTypes),
    [value, columnTypes, columnExpressions, columnResultTypes],
  )

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

  const columnOptions = useMemo(
    () => [{ value: '', label: '— column —' }, ...columnSelectOptions(columns)],
    [columns],
  )

  return (
    <div className="filter-builder">
      <div className="filter-builder-header">
        <span className="filter-builder-title">Filter</span>
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
              { value: 'AND', label: 'all conditions (AND)' },
              { value: 'OR', label: 'any condition (OR)' },
            ]}
          />
        )}
      </div>

      {value.conditions.length === 0 && (
        <p className="muted filter-builder-empty">No filter — all rows match.</p>
      )}

      <ul className="filter-condition-list">
        {value.conditions.map((cond, idx) => {
          const col = columns.find((c) => c.name === cond.attr)
          const rawTypeId = col?.typeId ?? 'text'
          const filterType = effectiveFilterTypeId(rawTypeId, col?.expression, col?.resultTypeId)
          const ops = filterOpsForColumn(col ?? { name: cond.attr, typeId: 'text' })
          const opDef = ops.find((o) => o.op === cond.op) ?? ops[0]
          const label = col?.label || cond.attr || 'column'

          return (
            <li key={cond.id} className="filter-condition-row">
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
                        data-testid={idx === 0 ? valueTestId : undefined}
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
                        data-testid={idx === 0 ? valueTestId : undefined}
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
                      data-testid={idx === 0 ? valueTestId : undefined}
                      value={cond.val}
                      onChange={(e) => updateCondition(cond.id, { val: e.target.value })}
                      placeholder={
                        opDef.op === 'LIKE'
                          ? `{param} or keyword`
                          : opDef.listValue
                            ? 'a, b, c'
                            : `{param} or ${label}`
                      }
                    />
                  )
                })()
              ) : (
                <span className="filter-no-value muted">—</span>
              )}
              <button
                type="button"
                className="filter-remove-btn"
                aria-label="Remove condition"
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

      <button
        type="button"
        className="filter-add-btn"
        onClick={() => {
          const first = columns[0]?.name ?? ''
          const firstType = columns[0]?.typeId ?? 'text'
          onChange({
            ...value,
            conditions: [...value.conditions, newFilterCondition(first, firstType)],
          })
        }}
        disabled={!columns.length}
      >
        + Add condition
      </button>

      {preview ? (
        <details className="filter-preview">
          <summary className="muted">DSL preview</summary>
          <pre>{JSON.stringify(preview, null, 2)}</pre>
        </details>
      ) : null}
    </div>
  )
}

export { emptyFilterGroup }
