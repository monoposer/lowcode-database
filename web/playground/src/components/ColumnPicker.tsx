import { typeBadgeColor } from '../lib/column-utils'

export type ColumnPickerItem = {
  name: string
  label?: string
  typeId: string
}

type Props = {
  columns: ColumnPickerItem[]
  selected: string[]
  onChange: (next: string[]) => void
}

export function ColumnPicker({ columns, selected, onChange }: Props) {
  const allSelected = columns.length > 0 && columns.every((c) => selected.includes(c.name))
  const noneSelected = selected.length === 0

  const toggle = (name: string) => {
    onChange(
      selected.includes(name) ? selected.filter((id) => id !== name) : [...selected, name],
    )
  }

  const selectAll = () => onChange(columns.map((c) => c.name))
  const clearAll = () => onChange([])

  return (
    <div className="column-picker">
      <div className="column-picker-head">
        <div>
          <h4 className="column-picker-title">Column projection</h4>
          <p className="column-picker-desc">
            Pick columns for this view. Leave none selected for <code>SELECT *</code>.
          </p>
        </div>
        <span className="column-picker-count">
          {noneSelected ? 'All columns' : `${selected.length} / ${columns.length} selected`}
        </span>
      </div>

      <div className="column-picker-toolbar">
        <button type="button" className="btn btn-sm" onClick={selectAll} disabled={allSelected}>
          Select all
        </button>
        <button type="button" className="btn btn-sm btn-ghost" onClick={clearAll} disabled={noneSelected}>
          Clear selection
        </button>
      </div>

      <div className="column-picker-grid">
        {columns.map((c) => {
          const label = c.label?.trim()
          const showLabel = label && label !== c.name
          const checked = selected.includes(c.name)
          return (
            <button
              key={c.name}
              type="button"
              className={`column-picker-item${checked ? ' column-picker-item--selected' : ''}`}
              onClick={() => toggle(c.name)}
              aria-pressed={checked}
            >
              <span className={`column-picker-check${checked ? ' column-picker-check--on' : ''}`}>
                {checked ? '✓' : ''}
              </span>
              <span className="column-picker-body">
                <span className="column-picker-name">{c.name}</span>
                {showLabel && <span className="column-picker-label">{label}</span>}
                <span className={`column-picker-type type-badge ${typeBadgeColor(c.typeId)}`}>
                  {c.typeId}
                </span>
              </span>
            </button>
          )
        })}
      </div>

      {!columns.length && <p className="muted">No queryable columns on this table.</p>}
    </div>
  )
}
