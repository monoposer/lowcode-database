import { useEffect, useMemo, useRef, useState } from 'react'
import { getTableSchema, listRows, type ApiOpts, type Row } from '../api'
import { pickRelationDisplayColumn } from '../lib/column-utils'
import { IconChevron } from './icons'

export type RelationChoice = { id: string; label: string }

export async function loadRelationChoices(tableName: string, opts: ApiOpts): Promise<RelationChoice[]> {
  const schema = await getTableSchema(tableName, opts)
  const display = pickRelationDisplayColumn(schema.columns || [])
  const lr = await listRows(tableName, 100, opts)
  return (lr.rows || []).map((row) => ({
    id: String(row.id ?? ''),
    label: relationRowLabel(row, display?.name),
  }))
}

export function relationRowLabel(row: Row, displayName?: string): string {
  const id = String(row.id ?? '')
  if (!displayName) return id
  const raw = row[displayName]
  if (raw == null || raw === '') return id
  const text = typeof raw === 'object' ? JSON.stringify(raw) : String(raw).trim()
  if (!text || text === id) return id
  return `${text}`
}

type Props = {
  tableName: string
  many?: boolean
  value: string
  onChange: (value: string) => void
  opts: ApiOpts
  placeholder?: string
  'aria-label'?: string
  allowEmpty?: boolean
}

function parseIds(raw: string): string[] {
  return raw
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)
}

export function RelationPicker({
  tableName,
  many = false,
  value,
  onChange,
  opts,
  placeholder = 'Select related row',
  'aria-label': ariaLabel,
  allowEmpty = true,
}: Props) {
  const [choices, setChoices] = useState<RelationChoice[]>([])
  const [open, setOpen] = useState(false)
  const [loading, setLoading] = useState(false)
  const rootRef = useRef<HTMLDivElement>(null)
  const selectedIds = useMemo(() => parseIds(value), [value])

  useEffect(() => {
    if (!tableName) {
      setChoices([])
      return
    }
    let cancelled = false
    const load = () => {
      setLoading(true)
      void loadRelationChoices(tableName, opts)
        .then((rows) => {
          if (!cancelled) setChoices(rows)
        })
        .catch(() => {
          if (!cancelled) setChoices([])
        })
        .finally(() => {
          if (!cancelled) setLoading(false)
        })
    }
    load()
    return () => {
      cancelled = true
    }
  }, [tableName, opts, open])

  useEffect(() => {
    if (!open) return
    const onDoc = (e: MouseEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false)
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', onDoc)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDoc)
      document.removeEventListener('keydown', onKey)
    }
  }, [open])

  const selectedLabels = selectedIds.map((id) => choices.find((c) => c.id === id)?.label || id)

  const toggle = (id: string) => {
    if (many) {
      const next = selectedIds.includes(id)
        ? selectedIds.filter((x) => x !== id)
        : [...selectedIds, id]
      onChange(next.join(', '))
      return
    }
    onChange(id === selectedIds[0] && allowEmpty ? '' : id)
    setOpen(false)
  }

  const triggerText = loading
    ? 'Loading…'
    : selectedLabels.length
      ? selectedLabels.join(', ')
      : placeholder

  return (
    <div
      ref={rootRef}
      className={`relation-picker${open ? ' relation-picker--open' : ''}${many ? ' relation-picker--many' : ''}`}
    >
      <button
        type="button"
        className="relation-picker-trigger"
        aria-label={ariaLabel}
        aria-haspopup="listbox"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
      >
        <span className={selectedLabels.length ? 'relation-picker-value' : 'relation-picker-placeholder'}>
          {triggerText}
        </span>
        <IconChevron size={14} className="relation-picker-chevron" />
      </button>
      {open && (
        <ul className="relation-picker-menu" role="listbox">
          {allowEmpty && !many && (
            <li role="option" aria-selected={!selectedIds.length}>
              <button
                type="button"
                className={`relation-picker-option${!selectedIds.length ? ' relation-picker-option--active' : ''}`}
                onClick={() => {
                  onChange('')
                  setOpen(false)
                }}
              >
                — none —
              </button>
            </li>
          )}
          {!choices.length && !loading && (
            <li className="relation-picker-empty">No rows in {tableName}</li>
          )}
          {choices.map((c) => {
            const active = selectedIds.includes(c.id)
            return (
              <li key={c.id} role="option" aria-selected={active}>
                <button
                  type="button"
                  className={`relation-picker-option${active ? ' relation-picker-option--active' : ''}`}
                  onClick={() => toggle(c.id)}
                >
                  {many && <span className="relation-picker-check">{active ? '✓' : ''}</span>}
                  <span className="relation-picker-option-body">
                    <span className="relation-picker-option-label">{c.label}</span>
                    {c.label !== c.id && <span className="relation-picker-option-id">{c.id}</span>}
                  </span>
                </button>
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}
