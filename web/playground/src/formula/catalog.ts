/** Function names evaluated in-process from the efp AST. */
export type FormulaFn = { name: string; snippet: string; hint?: string }

export const FORMULA_FN_GROUPS: { label: string; fns: FormulaFn[] }[] = [
  {
    label: 'Logical',
    fns: [
      { name: 'IF', snippet: 'IF(condition, value_if_true, value_if_false)', hint: 'Conditional' },
      { name: 'AND', snippet: 'AND(a, b, ...)', hint: 'All true' },
      { name: 'OR', snippet: 'OR(a, b, ...)', hint: 'Any true' },
      { name: 'NOT', snippet: 'NOT(x)', hint: 'Negate' },
      { name: 'ISBLANK', snippet: 'ISBLANK(value)', hint: 'Empty or missing' },
    ],
  },
  {
    label: 'Math',
    fns: [
      { name: 'SUM', snippet: 'SUM(a, b, ...)', hint: 'Add values' },
      { name: 'MIN', snippet: 'MIN(a, b, ...)', hint: 'Smallest' },
      { name: 'MAX', snippet: 'MAX(a, b, ...)', hint: 'Largest' },
      { name: 'ABS', snippet: 'ABS(x)', hint: 'Absolute value' },
      { name: 'INT', snippet: 'INT(x)', hint: 'Round toward -∞' },
      { name: 'ROUND', snippet: 'ROUND(x, digits)', hint: 'Round' },
    ],
  },
  {
    label: 'Text',
    fns: [
      { name: 'CONCAT', snippet: 'CONCAT(a, b, ...)', hint: 'Join text' },
      { name: 'TEXT', snippet: 'TEXT(value)', hint: 'To text' },
      { name: 'TRIM', snippet: 'TRIM(text)', hint: 'Strip extra spaces' },
      { name: 'UPPER', snippet: 'UPPER(text)', hint: 'Uppercase' },
      { name: 'LOWER', snippet: 'LOWER(text)', hint: 'Lowercase' },
      { name: 'MID', snippet: 'MID(text, start, num_chars)', hint: '1-based substring' },
      { name: 'LEN', snippet: 'LEN(text)', hint: 'Character count' },
    ],
  },
  {
    label: 'Date',
    fns: [
      { name: 'DATE', snippet: 'DATE(year, month, day)', hint: 'Calendar date' },
      { name: 'DATEVALUE', snippet: 'DATEVALUE(text)', hint: 'Parse date text' },
      { name: 'TODAY', snippet: 'TODAY()', hint: 'Current date' },
      { name: 'NOW', snippet: 'NOW()', hint: 'Current date and time' },
      { name: 'YEAR', snippet: 'YEAR(date)', hint: 'Year' },
      { name: 'MONTH', snippet: 'MONTH(date)', hint: 'Month 1-12' },
      { name: 'DAY', snippet: 'DAY(date)', hint: 'Day of month' },
      { name: 'WEEKDAY', snippet: 'WEEKDAY(date, 1)', hint: 'Day of week' },
      { name: 'DATEDIF', snippet: 'DATEDIF(start, end, "Y")', hint: 'Age / date difference' },
    ],
  },
]

export const ALL_FORMULA_FN_NAMES = FORMULA_FN_GROUPS.flatMap((g) => g.fns.map((f) => f.name))

/** Ensure Excel-style leading `=` for display; stored expression may omit it. */
export function displayFormula(expr: string): string {
  const t = expr.trim()
  if (!t) return '='
  return t.startsWith('=') ? t : `=${t}`
}

/** Normalize for API config.expression (optional leading `=`). */
export function normalizeFormulaForSave(display: string): string {
  let t = display.trim()
  if (t.startsWith('=')) t = t.slice(1).trim()
  return t
}

export function insertAtCursor(
  text: string,
  selectionStart: number,
  selectionEnd: number,
  insert: string,
): { value: string; cursor: number } {
  const before = text.slice(0, selectionStart)
  const after = text.slice(selectionEnd)
  const value = before + insert + after
  const cursor = before.length + insert.length
  return { value, cursor }
}
