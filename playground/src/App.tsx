import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { AgGridReact } from 'ag-grid-react'
import {
  AllCommunityModule,
  ModuleRegistry,
  themeQuartz,
  type ColDef,
  type GridApi,
  type GridReadyEvent,
} from 'ag-grid-community'
import 'ag-grid-community/styles/ag-grid.css'
import 'ag-grid-community/styles/ag-theme-quartz.css'
import './App.css'
import {
  bulkDeleteRows,
  cellFromString,
  createColumn,
  createIndex,
  createRow,
  createTable,
  deleteColumn,
  deleteIndex,
  deleteTable,
  formatCell,
  getDatabaseConnection,
  getTableSchema,
  importRows,
  listRows,
  listTables,
  listTypes,
  renameTable,
  updateColumn,
  type Column,
  type ColType,
  type CellValue,
  type Index,
  type Row,
} from './api'
import { FormulaEditor } from './components/FormulaEditor'

ModuleRegistry.registerModules([AllCommunityModule])

const theme = themeQuartz

function isWritableColumn(c: Column) {
  return c.typeId !== 'formula' && c.typeId !== 'relationship' && c.typeId !== 'lookup'
}

function isGridColumn(c: Column) {
  return c.typeId !== 'relationship' && c.typeId !== 'lookup'
}

function isFormulaColumn(c: Column) {
  return c.typeId === 'formula'
}

function columnExpression(c: Column): string {
  const cfg = c.config
  if (!cfg) return ''
  const expr = cfg.expression ?? cfg.formula
  return typeof expr === 'string' ? expr : ''
}

function isPhysicalColumn(c: Column) {
  return !c.pgColumn?.startsWith('v_')
}

export default function App() {
  const [apiBase, setApiBase] = useState(
    () => (import.meta.env.VITE_API_BASE as string) || 'http://localhost:8080',
  )
  const [tenantId, setTenantId] = useState('default')
  const [tables, setTables] = useState<{ id: string; name: string }[]>([])
  const [selectedTable, setSelectedTable] = useState<string>('')
  const [columns, setColumns] = useState<Column[]>([])
  const [indexes, setIndexes] = useState<Index[]>([])
  const [types, setTypes] = useState<ColType[]>([])
  const [rowData, setRowData] = useState<GridRow[]>([])
  const [err, setErr] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [conn, setConn] = useState<string | null>(null)
  const [importText, setImportText] = useState('[\n  { "column_name": "value" }\n]')
  const [renameTo, setRenameTo] = useState('')
  const [newTableName, setNewTableName] = useState('')
  const [newColName, setNewColName] = useState('')
  const [newColType, setNewColType] = useState('text')
  const [newColNullable, setNewColNullable] = useState(true)
  const [newIdxName, setNewIdxName] = useState('')
  const [newIdxCols, setNewIdxCols] = useState<string[]>([])
  const [newIdxUnique, setNewIdxUnique] = useState(false)
  const [newRowCells, setNewRowCells] = useState<Record<string, string>>({})
  const [newFormulaExpr, setNewFormulaExpr] = useState('')
  const [editingFormulaId, setEditingFormulaId] = useState<string | null>(null)
  const [editingFormulaExpr, setEditingFormulaExpr] = useState('')
  const [tab, setTab] = useState<'rows' | 'schema'>('rows')
  const gridApi = useRef<GridApi | null>(null)

  const opts = useMemo(
    () => ({ baseUrl: apiBase, tenantId: tenantId || undefined }),
    [apiBase, tenantId],
  )

  useEffect(() => {
    void listTypes(opts)
      .then((r) => {
        const list = r.types || []
        setTypes(list)
        if (list.length) {
          setNewColType((cur) => (list.some((t) => t.id === cur) ? cur : list[0].id))
        }
      })
      .catch(() => {})
  }, [opts])

  const refreshTables = useCallback(
    async (preferId?: string) => {
      setErr(null)
      setLoading(true)
      try {
        const res = await listTables(opts)
        const list = (res.tables || []).map((t) => ({
          id: t.id || t.name || '',
          name: t.name || t.id || '',
        }))
        setTables(list)
        setSelectedTable((cur) => {
          if (!list.length) return ''
          if (preferId && list.some((t) => t.id === preferId)) return preferId
          if (cur && list.some((t) => t.id === cur)) return cur
          return list[0].id
        })
      } catch (e) {
        setErr(e instanceof Error ? e.message : String(e))
      } finally {
        setLoading(false)
      }
    },
    [opts],
  )

  useEffect(() => {
    void refreshTables()
  }, [refreshTables])

  const loadSchema = useCallback(async () => {
    if (!selectedTable) {
      setColumns([])
      setIndexes([])
      return
    }
    const schema = await getTableSchema(selectedTable, opts)
    setColumns(schema.columns || [])
    setIndexes(schema.indexes || [])
    return schema
  }, [selectedTable, opts])

  const loadGrid = useCallback(async () => {
    if (!selectedTable) return
    setErr(null)
    setLoading(true)
    try {
      const schema = await loadSchema()
      const cols = (schema?.columns || []).filter(isGridColumn)
      const lr = await listRows(selectedTable, 200, opts)
      setRowData(
        (lr.rows || []).map((r) => ({
          id: r.id,
          ...flattenCells(r, cols),
        })),
      )
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }, [selectedTable, opts, loadSchema])

  useEffect(() => {
    void loadGrid()
  }, [loadGrid])

  const physicalCols = useMemo(
    () => columns.filter(isPhysicalColumn),
    [columns],
  )

  const formulaRefColumns = useMemo(
    () =>
      columns
        .filter((c) => isPhysicalColumn(c) && !isFormulaColumn(c))
        .map((c) => ({ name: c.name, typeId: c.typeId })),
    [columns],
  )

  const colDefs: ColDef<GridRow>[] = useMemo(() => {
    const gridCols = columns.filter(isGridColumn)
    const defs: ColDef<GridRow>[] = [
      {
        colId: '__select',
        headerName: '',
        checkboxSelection: true,
        headerCheckboxSelection: true,
        width: 52,
        pinned: 'left',
        sortable: false,
        filter: false,
      },
      {
        headerName: 'id',
        field: 'id',
        width: 280,
        pinned: 'left',
        editable: false,
      },
    ]
    for (const c of gridCols) {
      const fx = isFormulaColumn(c) ? 'ƒ ' : ''
      defs.push({
        headerName: `${fx}${c.name} (${c.typeId})`,
        field: c.id,
        flex: 1,
        minWidth: 120,
        editable: false,
        valueFormatter: (p) => formatCell(p.value as string | undefined),
        cellClass: isFormulaColumn(c) ? 'formula-cell' : undefined,
      })
    }
    return defs
  }, [columns])

  const run = async (fn: () => Promise<void>) => {
    setErr(null)
    setLoading(true)
    try {
      await fn()
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }

  const onCreateTable = () => {
    const name = newTableName.trim()
    if (!name) return
    void run(async () => {
      await createTable(name, opts)
      setNewTableName('')
      await refreshTables(name)
    })
  }

  const onDeleteTable = () => {
    if (!selectedTable || !window.confirm(`Delete table "${selectedTable}"?`)) return
    void run(async () => {
      await deleteTable(selectedTable, opts)
      await refreshTables()
    })
  }

  const onAddColumn = () => {
    if (!selectedTable || !newColName.trim()) return
    if (newColType === 'formula' && !newFormulaExpr.trim()) {
      setErr('Formula column requires an expression (Excel / pg-formula).')
      return
    }
    void run(async () => {
      const body: Parameters<typeof createColumn>[0] = {
        tableId: selectedTable,
        name: newColName.trim(),
        typeId: newColType,
        isNullable: newColNullable,
        position: columns.length + 1,
      }
      if (newColType === 'formula') {
        body.config = { expression: newFormulaExpr.trim() }
      }
      await createColumn(body, opts)
      setNewColName('')
      setNewFormulaExpr('')
      await loadGrid()
    })
  }

  const onSaveFormulaColumn = (col: Column) => {
    if (!editingFormulaExpr.trim()) {
      setErr('Expression cannot be empty.')
      return
    }
    void run(async () => {
      await updateColumn(col.id, { config: { expression: editingFormulaExpr.trim() } }, opts)
      setEditingFormulaId(null)
      setEditingFormulaExpr('')
      await loadGrid()
    })
  }

  const onDeleteColumn = (col: Column) => {
    if (!window.confirm(`Delete column "${col.name}"?`)) return
    void run(async () => {
      await deleteColumn(col.id, opts)
      await loadGrid()
    })
  }

  const onAddIndex = () => {
    if (!selectedTable || !newIdxName.trim() || !newIdxCols.length) return
    void run(async () => {
      await createIndex(
        {
          tableId: selectedTable,
          name: newIdxName.trim(),
          columnIds: newIdxCols,
          isUnique: newIdxUnique,
        },
        opts,
      )
      setNewIdxName('')
      setNewIdxCols([])
      await loadGrid()
    })
  }

  const onDeleteIndex = (idx: Index) => {
    const id = idx.pgIndex || idx.id
    if (!window.confirm(`Drop index "${id}"?`)) return
    void run(async () => {
      await deleteIndex(id, opts)
      await loadGrid()
    })
  }

  const onCreateRow = () => {
    if (!selectedTable) return
    const writable = columns.filter(isWritableColumn)
    const cells: Record<string, CellValue> = {}
    for (const c of writable) {
      const v = cellFromString(c.typeId, newRowCells[c.id] ?? '')
      if (v) cells[c.id] = v
    }
    void run(async () => {
      await createRow(selectedTable, cells, opts)
      setNewRowCells({})
      await loadGrid()
    })
  }

  const onGridReady = (e: GridReadyEvent) => {
    gridApi.current = e.api
  }

  const onDeleteSelected = async () => {
    const api = gridApi.current
    if (!api || !selectedTable) return
    const selected = api.getSelectedRows() as GridRow[]
    const ids = selected.map((r) => r.id).filter(Boolean)
    if (!ids.length) {
      setErr('Select at least one row.')
      return
    }
    void run(async () => {
      await bulkDeleteRows(selectedTable, ids, opts)
      await loadGrid()
    })
  }

  const onImport = async () => {
    if (!selectedTable) return
    let rows: Record<string, unknown>[]
    try {
      rows = JSON.parse(importText) as Record<string, unknown>[]
      if (!Array.isArray(rows)) throw new Error('JSON must be an array of objects')
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e))
      return
    }
    void run(async () => {
      await importRows(selectedTable, rows, opts)
      await loadGrid()
    })
  }

  const onConnection = async () => {
    setErr(null)
    try {
      const c = await getDatabaseConnection(opts)
      setConn(
        [c.urlWithoutPassword, '', c.psqlCommand, '', c.passwordSourceHint].join('\n'),
      )
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e))
    }
  }

  const onRename = async () => {
    if (!selectedTable || !renameTo.trim()) return
    const newName = renameTo.trim()
    void run(async () => {
      await renameTable(selectedTable, newName, opts)
      setRenameTo('')
      await refreshTables(newName)
    })
  }

  const toggleIdxCol = (colId: string) => {
    setNewIdxCols((cur) =>
      cur.includes(colId) ? cur.filter((x) => x !== colId) : [...cur, colId],
    )
  }

  return (
    <div className="layout">
      <aside className="sidebar">
        <h1>Playground</h1>
        <label>
          API base
          <input
            value={apiBase}
            onChange={(e) => setApiBase(e.target.value)}
            spellCheck={false}
          />
        </label>
        <label>
          X-Tenant-Id
          <input value={tenantId} onChange={(e) => setTenantId(e.target.value)} />
        </label>
        <button type="button" onClick={() => void refreshTables()}>
          Refresh tables
        </button>
        <button type="button" onClick={() => void onConnection()}>
          DB connection info
        </button>

        <div className="section">
          <h2>Create table</h2>
          <div className="rename-row">
            <input
              data-testid="create-table-name"
              value={newTableName}
              onChange={(e) => setNewTableName(e.target.value)}
              placeholder="table_name"
            />
            <button type="button" data-testid="create-table-btn" onClick={() => void onCreateTable()}>
              Create
            </button>
          </div>
        </div>

        <label>
          Table
          <select
            data-testid="table-select"
            value={selectedTable}
            onChange={(e) => setSelectedTable(e.target.value)}
          >
            {tables.map((t) => (
              <option key={t.id} value={t.id}>
                {t.name}
              </option>
            ))}
          </select>
        </label>
        <div className="rename-row">
          <input
            value={renameTo}
            onChange={(e) => setRenameTo(e.target.value)}
            placeholder="new table name"
          />
          <button type="button" onClick={() => void onRename()}>
            Rename
          </button>
        </div>
        <button type="button" className="danger" onClick={() => void onDeleteTable()}>
          Delete table
        </button>
        <button type="button" data-testid="reload-btn" onClick={() => void loadGrid()}>
          Reload
        </button>

        {loading && <p className="muted">Loading…</p>}
        {err && <p className="error">{err}</p>}
        {conn && <pre className="conn">{conn}</pre>}
      </aside>

      <main className="main">
        <div className="tabs">
          <button
            type="button"
            data-testid="tab-rows"
            className={tab === 'rows' ? 'active' : ''}
            onClick={() => setTab('rows')}
          >
            Rows
          </button>
          <button
            type="button"
            data-testid="tab-schema"
            className={tab === 'schema' ? 'active' : ''}
            onClick={() => setTab('schema')}
          >
            Schema
          </button>
        </div>

        {tab === 'schema' && (
          <div className="schema-panel">
            <section>
              <h2>Columns</h2>
              <p className="muted">GET/POST /v1/columns · PATCH/DELETE /v1/columns/&#123;id&#125;</p>
              <table className="meta-table">
                <thead>
                  <tr>
                    <th>name</th>
                    <th>type</th>
                    <th>expression / pg</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  {columns.map((c) => (
                    <tr key={c.id}>
                      <td>{c.name}</td>
                      <td>{c.typeId}</td>
                      <td className="mono">
                        {isFormulaColumn(c) ? (
                          <span className="formula-expr-preview" title={columnExpression(c)}>
                            {columnExpression(c) || '—'}
                          </span>
                        ) : (
                          c.pgColumn
                        )}
                      </td>
                      <td className="meta-actions">
                        {isFormulaColumn(c) && (
                          <button
                            type="button"
                            onClick={() => {
                              setEditingFormulaId(c.id)
                              setEditingFormulaExpr(columnExpression(c))
                            }}
                          >
                            Edit ƒ
                          </button>
                        )}
                        <button type="button" onClick={() => void onDeleteColumn(c)}>
                          Delete
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
              {editingFormulaId && (
                <div className="formula-edit-panel">
                  <h3>Edit formula</h3>
                  <FormulaEditor
                    value={editingFormulaExpr}
                    onChange={setEditingFormulaExpr}
                    columns={formulaRefColumns}
                  />
                  <div className="form-row">
                    <button
                      type="button"
                      onClick={() => {
                        const col = columns.find((x) => x.id === editingFormulaId)
                        if (col) void onSaveFormulaColumn(col)
                      }}
                    >
                      Save expression
                    </button>
                    <button
                      type="button"
                      onClick={() => {
                        setEditingFormulaId(null)
                        setEditingFormulaExpr('')
                      }}
                    >
                      Cancel
                    </button>
                  </div>
                </div>
              )}
              <div className="form-row">
                <input
                  data-testid="add-column-name"
                  value={newColName}
                  onChange={(e) => setNewColName(e.target.value)}
                  placeholder="column name"
                />
                <select
                  data-testid="add-column-type"
                  value={newColType}
                  onChange={(e) => {
                    setNewColType(e.target.value)
                    if (e.target.value !== 'formula') setNewFormulaExpr('')
                  }}
                >
                  {types.map((t) => (
                    <option key={t.id} value={t.id}>
                      {t.name || t.id}
                    </option>
                  ))}
                </select>
                <label className="inline">
                  <input
                    type="checkbox"
                    checked={newColNullable}
                    onChange={(e) => setNewColNullable(e.target.checked)}
                    disabled={newColType === 'formula'}
                  />
                  nullable
                </label>
                <button type="button" data-testid="add-column-btn" onClick={() => void onAddColumn()}>
                  Add column
                </button>
              </div>
              {newColType === 'formula' && (
                <div className="formula-add-panel">
                  <h3>New formula (Excel / pg-formula)</h3>
                  <FormulaEditor
                    value={newFormulaExpr}
                    onChange={setNewFormulaExpr}
                    columns={formulaRefColumns}
                  />
                </div>
              )}
            </section>

            <section>
              <h2>Indexes (PG catalog)</h2>
              <p className="muted">GET/POST /v1/indexes · DELETE /v1/indexes/&#123;pgIndexName&#125;</p>
              <table className="meta-table">
                <thead>
                  <tr>
                    <th>name</th>
                    <th>pg index</th>
                    <th>columns</th>
                    <th>unique</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  {indexes.map((idx) => (
                    <tr key={idx.id}>
                      <td>{idx.name}</td>
                      <td className="mono">{idx.pgIndex || idx.id}</td>
                      <td>{(idx.columnIds || []).join(', ')}</td>
                      <td>{idx.isUnique ? 'yes' : ''}</td>
                      <td>
                        <button type="button" onClick={() => void onDeleteIndex(idx)}>
                          Drop
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
              <div className="form-row">
                <input
                  value={newIdxName}
                  onChange={(e) => setNewIdxName(e.target.value)}
                  placeholder="index name"
                />
                <label className="inline">
                  <input
                    type="checkbox"
                    checked={newIdxUnique}
                    onChange={(e) => setNewIdxUnique(e.target.checked)}
                  />
                  unique
                </label>
              </div>
              <div className="idx-cols">
                {physicalCols.map((c) => (
                  <label key={c.id} className="inline">
                    <input
                      type="checkbox"
                      checked={newIdxCols.includes(c.id)}
                      onChange={() => toggleIdxCol(c.id)}
                    />
                    {c.name}
                  </label>
                ))}
              </div>
              <button type="button" onClick={() => void onAddIndex()}>
                Create index
              </button>
            </section>
          </div>
        )}

        {tab === 'rows' && (
          <>
            <div className="import-panel">
              <span>Import JSON rows</span>
              <textarea
                value={importText}
                onChange={(e) => setImportText(e.target.value)}
                rows={3}
              />
              <button type="button" onClick={() => void onImport()}>
                Import
              </button>
              <button type="button" onClick={() => void onDeleteSelected()}>
                Delete selected
              </button>
            </div>
            <div className="new-row-panel">
              <span>New row</span>
              {columns.filter(isWritableColumn).map((c) => (
                <label key={c.id} className="cell-input">
                  {c.name}
                  <input
                    value={newRowCells[c.id] ?? ''}
                    onChange={(e) =>
                      setNewRowCells((prev) => ({ ...prev, [c.id]: e.target.value }))
                    }
                    placeholder={c.typeId}
                  />
                </label>
              ))}
              <button type="button" data-testid="insert-row-btn" onClick={() => void onCreateRow()}>
                Insert row
              </button>
            </div>
            <div className="ag-theme-quartz grid-host">
              <AgGridReact
                theme={theme}
                rowData={rowData}
                columnDefs={colDefs}
                defaultColDef={{ sortable: true, filter: true, resizable: true }}
                rowSelection={{ mode: 'multiRow' }}
                onGridReady={onGridReady}
                getRowId={(p) => p.data.id}
              />
            </div>
          </>
        )}
      </main>
    </div>
  )
}

type GridRow = { id: string } & Record<string, string | undefined>

function flattenCells(row: Row, cols: Column[]): Record<string, string | undefined> {
  const out: Record<string, string | undefined> = {}
  for (const c of cols) {
    const v = row.cells?.[c.id]
    out[c.id] = v === undefined ? undefined : formatCell(v)
  }
  return out
}
