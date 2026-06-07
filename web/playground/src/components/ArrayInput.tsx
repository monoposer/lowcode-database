import { useMemo, useState } from 'react'

type Props = {
  value: string
  onChange: (value: string) => void
  placeholder?: string
  'aria-label'?: string
}

function parseTags(raw: string): string[] {
  const s = raw.trim()
  if (!s) return []
  if (s.startsWith('[')) {
    try {
      const parsed = JSON.parse(s) as unknown
      if (Array.isArray(parsed)) {
        return parsed.map((x) => String(x).trim()).filter(Boolean)
      }
    } catch {
      /* fall through */
    }
  }
  return s
    .split(',')
    .map((x) => x.trim())
    .filter(Boolean)
}

function joinTags(tags: string[]): string {
  return tags.join(', ')
}

export function ArrayInput({ value, onChange, placeholder, 'aria-label': ariaLabel }: Props) {
  const tags = useMemo(() => parseTags(value), [value])
  const [draft, setDraft] = useState('')

  const commitDraft = (raw: string) => {
    const next = raw.trim()
    if (!next) return
    const merged = [...tags]
    for (const part of next.split(',').map((x) => x.trim()).filter(Boolean)) {
      if (!merged.includes(part)) merged.push(part)
    }
    onChange(joinTags(merged))
    setDraft('')
  }

  const removeTag = (idx: number) => {
    onChange(joinTags(tags.filter((_, i) => i !== idx)))
  }

  return (
    <div className="array-input" aria-label={ariaLabel}>
      <div className="array-input-tags">
        {tags.map((tag, idx) => (
          <span key={`${tag}-${idx}`} className="array-input-tag">
            {tag}
            <button
              type="button"
              className="array-input-tag-remove"
              aria-label={`Remove ${tag}`}
              onClick={() => removeTag(idx)}
            >
              ×
            </button>
          </span>
        ))}
        <input
          className="array-input-field"
          value={draft}
          placeholder={tags.length ? 'Add…' : placeholder ?? 'a, b, c'}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ',') {
              e.preventDefault()
              commitDraft(draft)
            } else if (e.key === 'Backspace' && !draft && tags.length) {
              removeTag(tags.length - 1)
            }
          }}
          onBlur={() => commitDraft(draft)}
        />
      </div>
    </div>
  )
}
