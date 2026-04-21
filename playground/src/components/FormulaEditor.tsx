import { useCallback, useMemo, useRef, useState } from 'react'
import {
  ALL_FORMULA_FN_NAMES,
  FORMULA_FN_GROUPS,
  displayFormula,
  insertAtCursor,
  normalizeFormulaForSave,
} from '../formula/catalog'

export type FormulaColumnRef = { name: string; typeId?: string }

type Props = {
  value: string
  onChange: (normalizedExpression: string) => void
  columns: FormulaColumnRef[]
  disabled?: boolean
}

/** Excel / formulajs-style editor: leading `=`, fn picker, `{{column}}` refs (pg-formula). */
export function FormulaEditor({ value, onChange, columns, disabled }: Props) {
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const [fnFilter, setFnFilter] = useState('')
  const display = useMemo(() => displayFormula(value), [value])

  const applyDisplay = useCallback(
    (nextDisplay: string, cursor?: number) => {
      onChange(normalizeFormulaForSave(nextDisplay))
      requestAnimationFrame(() => {
        const el = textareaRef.current
        if (el && cursor !== undefined) {
          el.focus()
          el.setSelectionRange(cursor, cursor)
        }
      })
    },
    [onChange],
  )

  const insertSnippet = useCallback(
    (snippet: string) => {
      const el = textareaRef.current
      const start = el?.selectionStart ?? display.length
      const end = el?.selectionEnd ?? start
      const { value: next, cursor } = insertAtCursor(display, start, end, snippet)
      applyDisplay(next, cursor)
    },
    [applyDisplay, display],
  )

  const insertColumnRef = useCallback(
    (name: string) => {
      insertSnippet(`{{${name}}}`)
    },
    [insertSnippet],
  )

  const filteredGroups = useMemo(() => {
    const q = fnFilter.trim().toUpperCase()
    if (!q) return FORMULA_FN_GROUPS
    return FORMULA_FN_GROUPS.map((g) => ({
      ...g,
      fns: g.fns.filter((f) => f.name.includes(q) || f.snippet.toUpperCase().includes(q)),
    })).filter((g) => g.fns.length > 0)
  }, [fnFilter])

  return (
    <div className="formula-editor">
      <div className="formula-bar">
        <span className="formula-bar-label" title="Excel formula bar">
          fx
        </span>
        <textarea
          ref={textareaRef}
          className="formula-input"
          value={display}
          disabled={disabled}
          spellCheck={false}
          rows={2}
          placeholder="=IF({{qty}}>0, SUM({{amount}}, {{tax}}), 0)"
          onChange={(e) => applyDisplay(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
              e.preventDefault()
            }
          }}
        />
      </div>

      <p className="formula-hint muted">
        Excel /{' '}
        <a href="https://github.com/SolaTyolo/pg-formula" target="_blank" rel="noreferrer">
          pg-formula
        </a>
        : use <code>{'{{column_name}}'}</code> for table columns; optional leading{' '}
        <code>=</code>. Functions from formulajs subset (SUM, IF, CONCAT, …).
      </p>

      <div className="formula-toolbar">
        <label className="formula-fn-search">
          Insert function
          <input
            type="search"
            value={fnFilter}
            onChange={(e) => setFnFilter(e.target.value)}
            placeholder="Filter…"
            disabled={disabled}
          />
        </label>
        <div className="formula-fn-list">
          {filteredGroups.map((g) => (
            <div key={g.label} className="formula-fn-group">
              <span className="formula-fn-group-label">{g.label}</span>
              {g.fns.map((f) => (
                <button
                  key={f.name}
                  type="button"
                  className="formula-fn-btn"
                  title={f.hint ? `${f.snippet} — ${f.hint}` : f.snippet}
                  disabled={disabled}
                  onClick={() => insertSnippet(f.snippet)}
                >
                  {f.name}
                </button>
              ))}
            </div>
          ))}
        </div>
      </div>

      {columns.length > 0 && (
        <div className="formula-cols">
          <span className="formula-cols-label">Insert column</span>
          {columns.map((c) => (
            <button
              key={c.name}
              type="button"
              className="formula-col-chip"
              disabled={disabled}
              title={`Insert {{${c.name}}}`}
              onClick={() => insertColumnRef(c.name)}
            >
              {c.name}
              {c.typeId ? <span className="formula-col-type">{c.typeId}</span> : null}
            </button>
          ))}
        </div>
      )}

      {value.trim() && (
        <details className="formula-preview">
          <summary>Stored expression (API config.expression)</summary>
          <code>{normalizeFormulaForSave(display)}</code>
        </details>
      )}

      <datalist id="formula-fn-datalist">
        {ALL_FORMULA_FN_NAMES.map((n) => (
          <option key={n} value={n} />
        ))}
      </datalist>
    </div>
  )
}
