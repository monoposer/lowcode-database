import { useEffect, useId, useRef, useState } from 'react'
import { IconChevron } from './icons'

export type StudioSelectOption = {
  value: string
  label: string
  sublabel?: string
  badge?: string
}

type Props = {
  value: string
  onChange: (value: string) => void
  options: StudioSelectOption[]
  placeholder?: string
  ariaLabel?: string
  className?: string
  disabled?: boolean
}

export function StudioSelect({
  value,
  onChange,
  options,
  placeholder = '— select —',
  ariaLabel,
  className,
  disabled,
}: Props) {
  const [open, setOpen] = useState(false)
  const rootRef = useRef<HTMLDivElement>(null)
  const listId = useId()
  const selected = options.find((o) => o.value === value)

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

  return (
    <div
      ref={rootRef}
      className={`studio-select${open ? ' studio-select--open' : ''}${className ? ` ${className}` : ''}`}
    >
      <button
        type="button"
        className="studio-select-trigger"
        aria-label={ariaLabel}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={listId}
        disabled={disabled}
        onClick={() => setOpen((v) => !v)}
      >
        {selected ? (
          <span className="studio-select-value">
            <span className="studio-select-value-main">{selected.label}</span>
            {selected.sublabel && selected.sublabel !== selected.label && (
              <span className="studio-select-value-sub">{selected.sublabel}</span>
            )}
          </span>
        ) : (
          <span className="studio-select-placeholder">{placeholder}</span>
        )}
        <IconChevron size={14} className="studio-select-chevron" />
      </button>

      {open && (
        <ul id={listId} className="studio-select-menu" role="listbox">
          {options.map((opt, i) => {
            const active = opt.value === value
            return (
              <li key={opt.value || `opt-${i}`} role="option" aria-selected={active}>
                <button
                  type="button"
                  className={`studio-select-option${active ? ' studio-select-option--active' : ''}`}
                  onClick={() => {
                    onChange(opt.value)
                    setOpen(false)
                  }}
                >
                  <span className="studio-select-option-check">{active ? '✓' : ''}</span>
                  <span className="studio-select-option-body">
                    <span className="studio-select-option-row">
                      <span className="studio-select-option-label">{opt.label}</span>
                      {opt.badge && (
                        <span className="studio-select-option-badge">{opt.badge}</span>
                      )}
                    </span>
                    {opt.sublabel && opt.sublabel !== opt.label && (
                      <span className="studio-select-option-sublabel">{opt.sublabel}</span>
                    )}
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

export function columnSelectOptions(
  columns: { name: string; label?: string; typeId?: string }[],
): StudioSelectOption[] {
  return columns.map((c) => {
    const sub = c.label?.trim()
    return {
      value: c.name,
      label: c.name,
      sublabel: sub && sub !== c.name ? sub : undefined,
      badge: c.typeId,
    }
  })
}

export function simpleSelectOptions(
  items: { value: string; label: string }[],
): StudioSelectOption[] {
  return items.map((i) => ({ value: i.value, label: i.label }))
}
