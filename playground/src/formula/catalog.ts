/** Function names aligned with pg-formula formulajs catalog (subset). */
export type FormulaFn = { name: string; snippet: string; hint?: string }

export const FORMULA_FN_GROUPS: { label: string; fns: FormulaFn[] }[] = [
  {
    label: 'Logical',
    fns: [
      { name: 'IF', snippet: 'IF(condition, value_if_true, value_if_false)', hint: 'Conditional' },
      { name: 'IFS', snippet: 'IFS(cond1, val1, cond2, val2, ...)', hint: 'Multi-condition IF' },
      { name: 'SWITCH', snippet: 'SWITCH(expr, val1, res1, ...)', hint: 'Match expression' },
      { name: 'AND', snippet: 'AND(a, b, ...)', hint: 'All true' },
      { name: 'OR', snippet: 'OR(a, b, ...)', hint: 'Any true' },
      { name: 'NOT', snippet: 'NOT(x)', hint: 'Negate' },
      { name: 'XOR', snippet: 'XOR(a, b, ...)', hint: 'Exclusive or' },
      { name: 'IFERROR', snippet: 'IFERROR(value, fallback)', hint: 'On error' },
      { name: 'IFNA', snippet: 'IFNA(value, fallback)', hint: 'On #N/A' },
    ],
  },
  {
    label: 'Math',
    fns: [
      { name: 'SUM', snippet: 'SUM(a, b, ...)', hint: 'Add values' },
      { name: 'AVERAGE', snippet: 'AVERAGE(a, b, ...)', hint: 'Mean' },
      { name: 'MIN', snippet: 'MIN(a, b, ...)', hint: 'Minimum' },
      { name: 'MAX', snippet: 'MAX(a, b, ...)', hint: 'Maximum' },
      { name: 'COUNT', snippet: 'COUNT(a, b, ...)', hint: 'Count non-null' },
      { name: 'PRODUCT', snippet: 'PRODUCT(a, b, ...)', hint: 'Multiply' },
      { name: 'ABS', snippet: 'ABS(x)', hint: 'Absolute value' },
      { name: 'ROUND', snippet: 'ROUND(x, digits)', hint: 'Round' },
      { name: 'MOD', snippet: 'MOD(n, d)', hint: 'Remainder' },
      { name: 'POWER', snippet: 'POWER(base, exp)', hint: 'Exponent' },
      { name: 'SQRT', snippet: 'SQRT(x)', hint: 'Square root' },
      { name: 'INT', snippet: 'INT(x)', hint: 'Integer part' },
    ],
  },
  {
    label: 'Text',
    fns: [
      { name: 'CONCAT', snippet: 'CONCAT(a, b, ...)', hint: 'Join text' },
      { name: 'TEXTJOIN', snippet: 'TEXTJOIN(delimiter, ignore_empty, ...)', hint: 'Join with delimiter' },
      { name: 'LEN', snippet: 'LEN(text)', hint: 'Length' },
      { name: 'LOWER', snippet: 'LOWER(text)', hint: 'Lowercase' },
      { name: 'UPPER', snippet: 'UPPER(text)', hint: 'Uppercase' },
      { name: 'TRIM', snippet: 'TRIM(text)', hint: 'Trim spaces' },
      { name: 'LEFT', snippet: 'LEFT(text, n)', hint: 'Left substring' },
      { name: 'RIGHT', snippet: 'RIGHT(text, n)', hint: 'Right substring' },
      { name: 'MID', snippet: 'MID(text, start, len)', hint: 'Mid substring' },
      { name: 'FIND', snippet: 'FIND(find, within, start?)', hint: 'Find position' },
      { name: 'SUBSTITUTE', snippet: 'SUBSTITUTE(text, old, new, nth?)', hint: 'Replace text' },
    ],
  },
  {
    label: 'Date',
    fns: [
      { name: 'DATE', snippet: 'DATE(year, month, day)', hint: 'Build date' },
      { name: 'YEAR', snippet: 'YEAR(date)', hint: 'Year' },
      { name: 'MONTH', snippet: 'MONTH(date)', hint: 'Month' },
      { name: 'DAY', snippet: 'DAY(date)', hint: 'Day' },
      { name: 'TODAY', snippet: 'TODAY()', hint: 'Current date' },
      { name: 'NOW', snippet: 'NOW()', hint: 'Current timestamp' },
      { name: 'EDATE', snippet: 'EDATE(start, months)', hint: 'Shift months' },
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

/** Normalize for API config.expression (pg-formula accepts optional `=`). */
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
