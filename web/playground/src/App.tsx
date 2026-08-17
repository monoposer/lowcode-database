import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { AgGridReact } from 'ag-grid-react'
import {
  AllCommunityModule,
  ModuleRegistry,
  type ColDef,
  type GridApi,
  type GridReadyEvent,
  type CellValueChangedEvent,
} from 'ag-grid-community'
import './App.css'
import {
  bulkDeleteRows,
  cellToNative,
  createQuery,
  createColumn,
  createColumnType,
  deleteColumnType,
  createIndex,
  createRow,
  createTable,
  createTenant,
  listTenants,
  deleteQuery,
  deleteColumn,
  deleteIndex,
  deleteTable,
  formatCell,
  getDatabaseConnection,
  getTableSchema,
  listQueries,
  listRows,
  listTables,
  listTypes,
  queryRows,
  executeQuery,
  renameTable,
  updateRow,
  updateColumn,
  type Column,
  type ColType,
  type Query,
  type Index,
  type Row,
  type Tenant,
  type ApiOpts,
} from './api'
import { FilterBuilder, emptyFilterGroup } from './components/FilterBuilder'
import { ColumnHeader } from './components/ColumnHeader'
import { ColumnPicker } from './components/ColumnPicker'
import { RowFilterBar } from './components/RowFilterBar'
import { ArrayInput } from './components/ArrayInput'
import { FormulaEditor } from './components/FormulaEditor'
import { RelationPicker, loadRelationChoices, type RelationChoice } from './components/RelationPicker'
import {
  IconDatabase,
  IconPlus,
  IconRefresh,
  IconSearch,
  IconSettings,
  IconTable,
  IconTrash,
  IconTypes,
} from './components/icons'
import { buildFilterDSL, newFilterCondition, type FilterGroup } from './filter/dsl'
import { extractFilterParams } from './filter/params'
import {
  columnDisplay,
  columnExpression,
  columnHeaderPrefix,
  isComputedColumn,
  isFormulaColumn,
  isGridColumn,
  isLookupTargetColumn,
  isPhysicalColumn,
  isRelationshipColumn,
  isVirtualKind,
  isArrayColumn,
  isLinkManyColumn,
  isWritableColumn,
  relationshipCardinality,
  relationshipTargetTable,
  resolveColumnRef,
  typeBadgeColor,
} from './lib/column-utils'
import { gridTheme } from './lib/grid-theme'

ModuleRegistry.registerModules([AllCommunityModule])

function isScalarType(t: ColType) {
  const virtual = ['formula', 'link', 'lookup', 'rollup']
  return !virtual.includes(t.id)
}

function isTableIdType(t: ColType) {
  return t.id === 'text' || t.id === 'number'
}

async function loadPhysicalColumns(tableName: string, opts: ApiOpts): Promise<Column[]> {
  const schema = await getTableSchema(tableName, opts)
  return (schema.columns || []).filter((c) => !isVirtualKind(c))
}

async function loadLookupTargetColumns(tableName: string, opts: ApiOpts): Promise<Column[]> {
  const schema = await getTableSchema(tableName, opts)
  return (schema.columns || []).filter(isLookupTargetColumn)
}

type Page = 'editor' | 'queries' | 'types' | 'settings'

const PAGES: Page[] = ['editor', 'queries', 'types', 'settings']

function pageFromHash(): Page {
  const raw = (window.location.hash || '#/editor').replace(/^#\/?/, '')
  const first = raw.split('/')[0] as Page
  return PAGES.includes(first) ? first : 'editor'
}

export default function App() {
  const [apiBase, setApiBase] = useState(
    () => (import.meta.env.VITE_API_BASE as string) || 'http://localhost:8080',
  )
  const [tenantId, setTenantId] = useState('default')
  const [tenants, setTenants] = useState<Tenant[]>([])
  const [newTenantId, setNewTenantId] = useState('')
  const [newTenantDisplayName, setNewTenantDisplayName] = useState('')
  const [newTenantDataDsn, setNewTenantDataDsn] = useState('')
  const [newTenantCreateDb, setNewTenantCreateDb] = useState(false)
  const [newTenantRecordStore, setNewTenantRecordStore] = useState<'shared' | 'dedicated'>('shared')
  const [tables, setTables] = useState<{ id: string; name: string; label?: string; idType?: string }[]>([])
  const [selectedTable, setSelectedTable] = useState<string>('')
  const [columns, setColumns] = useState<Column[]>([])
  const [relationChoices, setRelationChoices] = useState<Record<string, RelationChoice[]>>({})
  const [indexes, setIndexes] = useState<Index[]>([])
  const [types, setTypes] = useState<ColType[]>([])
  const [rowData, setRowData] = useState<GridRow[]>([])
  const [err, setErr] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [conn, setConn] = useState<string | null>(null)
  const [renameTo, setRenameTo] = useState('')
  const [newTableName, setNewTableName] = useState('')
  const [newTableLabel, setNewTableLabel] = useState('')
  const [newTableIdType, setNewTableIdType] = useState('uuid')
  const [newColName, setNewColName] = useState('')
  const [newColLabel, setNewColLabel] = useState('')
  const [newColType, setNewColType] = useState('text')
  const [newColNullable, setNewColNullable] = useState(true)
  const [newCtName, setNewCtName] = useState('')
  const [newCtLabel, setNewCtLabel] = useState('')
  const [newCtPgType, setNewCtPgType] = useState('text')
  const [newCtArray, setNewCtArray] = useState(false)
  const [newIdxName, setNewIdxName] = useState('')
  const [newIdxCols, setNewIdxCols] = useState<string[]>([])
  const [newIdxUnique, setNewIdxUnique] = useState(false)
  const [newRowCells, setNewRowCells] = useState<Record<string, string>>({})
  const [newFormulaExpr, setNewFormulaExpr] = useState('')
  const [newLookupRelColumn, setNewLookupRelColumn] = useState('')
  const [newLookupTargetColumn, setNewLookupTargetColumn] = useState('')
  const [lookupTargetColumns, setLookupTargetColumns] = useState<Column[]>([])
  const [newRollupRelColumn, setNewRollupRelColumn] = useState('')
  const [newRollupAggregate, setNewRollupAggregate] = useState('count')
  const [newRollupTargetColumn, setNewRollupTargetColumn] = useState('')
  const [rollupTargetColumns, setRollupTargetColumns] = useState<Column[]>([])
  const [newLookupFilter, setNewLookupFilter] = useState('')
  const [newRollupFilter, setNewRollupFilter] = useState('')
  const [newRelCardinality, setNewRelCardinality] = useState<'many' | 'one'>('one')
  const [newRelTargetTable, setNewRelTargetTable] = useState('')
  const [newRelLinkColumn, setNewRelLinkColumn] = useState('')
  const [newRelFKColumn, setNewRelFKColumn] = useState('')
  const [relLinkColumns, setRelLinkColumns] = useState<Column[]>([])
  const [editingColumnId, setEditingColumnId] = useState<string | null>(null)
  const [editColName, setEditColName] = useState('')
  const [editColLabel, setEditColLabel] = useState('')
  const [editColNullable, setEditColNullable] = useState(true)
  const [editColExpr, setEditColExpr] = useState('')
  const [editColType, setEditColType] = useState('')
  const [queries, setQueries] = useState<Query[]>([])
  const [newDsName, setNewDsName] = useState('')
  const [newDsLabel, setNewDsLabel] = useState('')
  const [newDsFilter, setNewDsFilter] = useState(emptyFilterGroup)
  const [newDsSortCol, setNewDsSortCol] = useState('')
  const [newDsSortOrder, setNewDsSortOrder] = useState('DESC')
  const [newDsColumnIds, setNewDsColumnIds] = useState<string[]>([])
  const [dsQueryRows, setDsQueryRows] = useState<Row[]>([])
  const [dsQueryCount, setDsQueryCount] = useState<number | null>(null)
  const [dsQueryColumnIds, setDsQueryColumnIds] = useState<string[]>([])
  const [dsQueryTarget, setDsQueryTarget] = useState<Query | null>(null)
  const [dsQueryParamValues, setDsQueryParamValues] = useState<Record<string, string>>({})
  const [page, setPage] = useState<Page>(pageFromHash)
  const [tab, setTab] = useState<'rows' | 'schema'>('rows')
  const [tableSearch, setTableSearch] = useState('')
  const [rowFilter, setRowFilter] = useState(emptyFilterGroup)
  const [rowFilterActive, setRowFilterActive] = useState(false)
  const [rowCount, setRowCount] = useState<number | null>(null)
  const gridApi = useRef<GridApi | null>(null)

  const filteredTables = useMemo(() => {
    const q = tableSearch.trim().toLowerCase()
    if (!q) return tables
    return tables.filter(
      (t) =>
        t.name.toLowerCase().includes(q) ||
        (t.label?.toLowerCase().includes(q) ?? false),
    )
  }, [tables, tableSearch])

  const goPage = (p: Page) => {
    setPage(p)
    window.location.hash = `/${p}`
    if (p === 'queries') setTab('rows')
  }

  const opts = useMemo(
    () => ({
      baseUrl: apiBase,
      tenantId: tenantId || undefined,
    }),
    [apiBase, tenantId],
  )

  const refreshTenants = useCallback(async () => {
    try {
      const res = await listTenants({ baseUrl: apiBase, tenantId: 'default' })
      setTenants(res.tenants || [])
    } catch {
      /* keep current selection when list fails */
    }
  }, [apiBase])

  useEffect(() => {
    void refreshTenants()
  }, [refreshTenants])

  const tenantOptions = useMemo(() => {
    const ids = new Set(tenants.map((t) => t.tenantId))
    if (tenantId && !ids.has(tenantId)) {
      return [{ tenantId }, ...tenants]
    }
    return tenants
  }, [tenants, tenantId])

  const onTenantChange = (id: string) => {
    setTenantId(id)
    setErr(null)
  }

  useEffect(() => {
    const onHash = () => setPage(pageFromHash())
    window.addEventListener('hashchange', onHash)
    if (!window.location.hash) window.location.hash = '/editor'
    return () => window.removeEventListener('hashchange', onHash)
  }, [])

  const tableIDTypes = useMemo(() => types.filter(isTableIdType), [types])

  const lookupRelColumns = useMemo(
    () => columns.filter((c) => isRelationshipColumn(c)),
    [columns],
  )

  const rollupRelColumns = useMemo(
    () =>
      columns.filter(
        (c) => isRelationshipColumn(c) && relationshipCardinality(c) === 'many',
      ),
    [columns],
  )

  useEffect(() => {
    if (newColType !== 'lookup' || !newLookupRelColumn) {
      setLookupTargetColumns([])
      return
    }
    const rel = columns.find(
      (c) => c.name === newLookupRelColumn || c.id === newLookupRelColumn,
    )
    const targetTable = rel ? relationshipTargetTable(rel) : ''
    if (!targetTable) {
      setLookupTargetColumns([])
      return
    }
    let cancelled = false
    void loadLookupTargetColumns(targetTable, opts)
      .then((cols) => {
        if (cancelled) return
        setLookupTargetColumns(cols)
        setNewLookupTargetColumn((cur) =>
          cur && cols.some((c) => c.name === cur || c.id === cur) ? cur : '',
        )
      })
      .catch(() => {
        if (!cancelled) setLookupTargetColumns([])
      })
    return () => {
      cancelled = true
    }
  }, [newColType, newLookupRelColumn, columns, opts])

  useEffect(() => {
    if (newColType !== 'rollup' || !newRollupRelColumn) {
      setRollupTargetColumns([])
      return
    }
    const rel = columns.find(
      (c) => c.name === newRollupRelColumn || c.id === newRollupRelColumn,
    )
    const targetTable = rel ? relationshipTargetTable(rel) : ''
    if (!targetTable) {
      setRollupTargetColumns([])
      return
    }
    let cancelled = false
    void loadPhysicalColumns(targetTable, opts)
      .then((physical) => {
        if (cancelled) return
        setRollupTargetColumns(physical)
        setNewRollupTargetColumn((cur) =>
          cur && physical.some((c) => c.name === cur || c.id === cur) ? cur : '',
        )
      })
      .catch(() => {
        if (!cancelled) setRollupTargetColumns([])
      })
    return () => {
      cancelled = true
    }
  }, [newColType, newRollupRelColumn, columns, opts])

  useEffect(() => {
    if (
      newColType !== 'link' ||
      newRelCardinality !== 'many' ||
      !newRelTargetTable
    ) {
      setRelLinkColumns([])
      return
    }
    let cancelled = false
    void loadPhysicalColumns(newRelTargetTable, opts)
      .then((physical) => {
        if (cancelled) return
        setRelLinkColumns(physical)
        setNewRelLinkColumn((cur) =>
          cur && physical.some((c) => c.name === cur || c.id === cur) ? cur : '',
        )
      })
      .catch(() => {
        if (!cancelled) setRelLinkColumns([])
      })
    return () => {
      cancelled = true
    }
  }, [newColType, newRelCardinality, newRelTargetTable, opts])

  const refreshTypes = useCallback(() => {
    return listTypes(opts)
      .then((r) => {
        const list = r.types || []
        setTypes(list)
        if (list.length) {
          setNewColType((cur) => (list.some((t) => t.id === cur) ? cur : list[0].id))
          setNewTableIdType((cur) =>
            list.some((t) => t.id === cur && isTableIdType(t)) ? cur : 'uuid',
          )
        }
        return list
      })
      .catch(() => [] as ColType[])
  }, [opts])

  useEffect(() => {
    void refreshTypes()
  }, [refreshTypes])

  const builtinTypes = useMemo(
    () => types.filter((t) => t.refKind !== 'columnType' && !t.config?.columnType),
    [types],
  )
  const customTypes = useMemo(
    () => types.filter((t) => t.refKind === 'columnType' || t.config?.columnType === true),
    [types],
  )

  const refreshQueries = useCallback(async () => {
    if (!selectedTable) {
      setQueries([])
      return
    }
    try {
      const r = await listQueries(selectedTable, opts)
      setQueries(r.queries || [])
    } catch {
      setQueries([])
    }
  }, [selectedTable, opts])

  useEffect(() => {
    if (page === 'types') void refreshTypes()
  }, [page, refreshTypes])

  useEffect(() => {
    if (page === 'queries') void refreshQueries()
  }, [page, refreshQueries])

  useEffect(() => {
    if (page !== 'queries') return
    const grid = columns.filter(isGridColumn)
    if (!grid.length) return
    setNewDsFilter((prev) => {
      if (prev.conditions.length > 0) return prev
      return { conjunction: 'AND', conditions: [newFilterCondition(grid[0].name)] }
    })
  }, [page, columns])

  useEffect(() => {
    if (page !== 'queries') return
    const gridCols = columns.filter(isGridColumn)
    const ids = gridCols.map((c) => c.name)
    setNewDsColumnIds((prev) => {
      const kept = prev.filter((id) => ids.includes(id))
      return kept.length ? kept : ids.slice(0, 3)
    })
    if (!newDsSortCol && ids.length) setNewDsSortCol(ids[0])
  }, [columns, page, newDsSortCol])

  const refreshTables = useCallback(
    async (preferId?: string) => {
      setErr(null)
      setLoading(true)
      try {
        const res = await listTables(opts)
        const list = (res.tables || []).map((t) => ({
          id: t.id || t.name || '',
          name: t.name || t.id || '',
          label: t.label,
          idType: t.idType || 'uuid',
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

  useEffect(() => {
    const links = columns.filter(isRelationshipColumn)
    if (!links.length) {
      setRelationChoices({})
      return
    }
    let cancelled = false
    void (async () => {
      const next: Record<string, RelationChoice[]> = {}
      await Promise.all(
        links.map(async (c) => {
          const table = relationshipTargetTable(c)
          if (!table) return
          try {
            next[c.name] = await loadRelationChoices(table, opts)
          } catch {
            next[c.name] = []
          }
        }),
      )
      if (!cancelled) setRelationChoices(next)
    })()
    return () => {
      cancelled = true
    }
  }, [columns, opts])

  const filterColumns = useMemo(
    () =>
      columns.filter(isGridColumn).map((c) => ({
        name: c.name,
        label: c.label,
        typeId: c.typeId,
        resultTypeId:
          c.resultTypeId ||
          (typeof c.config?.result_type_id === 'string' ? c.config.result_type_id : undefined),
        expression: c.typeId === 'formula' ? columnExpression(c) : undefined,
        isArray: isArrayColumn(c, types),
      })),
    [columns, types],
  )

  const buildColumnFilterMeta = useCallback((cols: Column[]) => {
    const colTypes: Record<string, string> = {}
    const colExprs: Record<string, string> = {}
    const colResultTypes: Record<string, string> = {}
    const colIsArray: Record<string, boolean> = {}
    for (const c of cols) {
      colTypes[c.name] = c.typeId
      colIsArray[c.name] = isArrayColumn(c, types)
      const rt = c.resultTypeId || (c.config?.result_type_id as string | undefined)
      if (rt) colResultTypes[c.name] = rt
      if (c.typeId === 'formula') {
        const ex = columnExpression(c)
        if (ex) colExprs[c.name] = ex
      }
    }
    return { colTypes, colExprs, colResultTypes, colIsArray }
  }, [types])

  const fetchGridRows = useCallback(
    async (filterGroup: FilterGroup | null, silent = false) => {
      if (!selectedTable) return
      setErr(null)
      if (!silent) setLoading(true)
      try {
        const schema = await loadSchema()
        const cols = (schema?.columns || []).filter(isGridColumn)
        const { colTypes, colExprs, colResultTypes, colIsArray } = buildColumnFilterMeta(cols)
        const filter =
          filterGroup && filterGroup.conditions.length
            ? buildFilterDSL(filterGroup, colTypes, colExprs, colResultTypes, colIsArray)
            : undefined

        const lr = filter
          ? await queryRows(selectedTable, { pageSize: 100, filter }, opts)
          : await listRows(selectedTable, 100, opts)

        setRowData(
          (lr.rows || []).map((r) => {
            const cells = flattenCells(r, cols)
            // Preserve API record id; schema column "id" must not overwrite getRowId.
            return { ...cells, id: String(r.id ?? '') }
          }),
        )
        setRowCount(lr.count ?? lr.rows?.length ?? 0)
      } catch (e) {
        setErr(e instanceof Error ? e.message : String(e))
      } finally {
        if (!silent) setLoading(false)
      }
    },
    [selectedTable, opts, loadSchema, buildColumnFilterMeta],
  )

  const rowFilterRef = useRef(rowFilter)
  rowFilterRef.current = rowFilter

  const loadGrid = useCallback(
    async (silent = false) => {
      const fg = rowFilterRef.current
      await fetchGridRows(fg.conditions.length > 0 ? fg : null, silent)
    },
    [fetchGridRows],
  )

  useEffect(() => {
    setRowFilter(emptyFilterGroup)
    setRowFilterActive(false)
    setRowCount(null)
    if (page === 'editor' && tab === 'rows' && selectedTable) {
      void fetchGridRows(null)
    }
  }, [selectedTable])

  useEffect(() => {
    if (page !== 'editor' || tab !== 'rows' || !selectedTable) return
    void loadGrid()
  }, [page, tab, loadGrid])

  useEffect(() => {
    if (page !== 'editor' || tab !== 'rows' || !selectedTable) return

    const hasFilter = rowFilter.conditions.length > 0
    const delay = hasFilter ? 400 : 0

    const timer = setTimeout(() => {
      setRowFilterActive(hasFilter)
      void fetchGridRows(hasFilter ? rowFilter : null, hasFilter)
    }, delay)

    return () => clearTimeout(timer)
  }, [rowFilter, page, tab, selectedTable, fetchGridRows])

  const clearRowFilter = () => {
    setRowFilter(emptyFilterGroup)
    setRowFilterActive(false)
  }

  const physicalCols = useMemo(
    () => columns.filter(isPhysicalColumn),
    [columns],
  )

  const formulaRefColumns = useMemo(
    () =>
      columns
        .filter(
          (c) =>
            c.typeId !== 'link' &&
            (isPhysicalColumn(c) ||
              c.typeId === 'lookup' ||
              c.typeId === 'rollup' ||
              c.typeId === 'formula'),
        )
        .map((c) => ({ name: c.name, typeId: c.typeId })),
    [columns],
  )

  const colDefs: ColDef<GridRow>[] = useMemo(() => {
    // Skip schema "id" — pinned column already shows record id (unique colId required by AG Grid).
    const gridCols = columns.filter(isGridColumn).filter((c) => c.name !== 'id')
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
        colId: '__rowId',
        headerName: 'id',
        field: 'id',
        width: 280,
        pinned: 'left',
        editable: false,
      },
    ]
    for (const c of gridCols) {
      const label = c.label?.trim()
      const isLink = isRelationshipColumn(c)
      const many = isLinkManyColumn(c)
      const choices = relationChoices[c.name] || []
      const formatLink = (raw: unknown) => {
        const ids = formatCell(raw)
          .split(',')
          .map((s) => s.trim())
          .filter(Boolean)
        if (!ids.length) return ''
        return ids.map((id) => choices.find((x) => x.id === id)?.label || id).join(', ')
      }
      defs.push({
        colId: c.name,
        headerComponent: ColumnHeader,
        headerComponentParams: {
          name: c.name,
          label,
          typeId: c.typeId,
          prefix: columnHeaderPrefix(c),
        },
        field: c.name,
        flex: 1,
        minWidth: Math.max(140, c.name.length * 8 + (label ? label.length * 10 : 0)),
        wrapHeaderText: true,
        autoHeaderHeight: true,
        editable: isWritableColumn(c),
        valueFormatter: (p) => (isLink ? formatLink(p.value) : formatCell(p.value as string | undefined)),
        cellEditor: isLink && !many ? 'agSelectCellEditor' : undefined,
        cellEditorParams:
          isLink && !many ? { values: ['', ...choices.map((x) => x.id)] } : undefined,
        cellClass: isComputedColumn(c)
          ? 'computed-cell'
          : isWritableColumn(c)
            ? 'editable-cell'
            : undefined,
      })
    }
    return defs
  }, [columns, relationChoices])

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

  const onCreateQuery = () => {
    if (!selectedTable) {
      setErr('Select a table first.')
      return
    }
    const name = newDsName.trim()
    if (!name) {
      setErr('Query name is required.')
      return
    }
    void run(async () => {
      const body: Parameters<typeof createQuery>[0] = {
        name,
        tableName: selectedTable,
      }
      if (newDsColumnIds.length) body.columnIds = newDsColumnIds
      const label = newDsLabel.trim()
      if (label) body.label = label
      const colTypes: Record<string, string> = {}
      const colExprs: Record<string, string> = {}
      const colResultTypes: Record<string, string> = {}
      const colIsArray: Record<string, boolean> = {}
      for (const c of columns) {
        colTypes[c.name] = c.typeId
        colIsArray[c.name] = isArrayColumn(c, types)
        const rt = c.resultTypeId || (c.config?.result_type_id as string | undefined)
        if (rt) colResultTypes[c.name] = rt
        if (c.typeId === 'formula') {
          const ex = columnExpression(c)
          if (ex) colExprs[c.name] = ex
        }
      }
      const filter = buildFilterDSL(newDsFilter, colTypes, colExprs, colResultTypes, colIsArray)
      if (filter) body.filter = filter
      if (newDsSortCol) {
        body.sort = [{ attribute: newDsSortCol, sortOrder: newDsSortOrder }]
      }
      await createQuery(body, opts)
      setNewDsName('')
      setNewDsLabel('')
      setDsQueryRows([])
      setDsQueryCount(null)
      await refreshQueries()
    })
  }

  const onDeleteQuery = (ds: Query) => {
    if (!window.confirm(`Delete query "${ds.name || ds.id}"?`)) return
    void run(async () => {
      await deleteQuery(selectedTable, ds.name || ds.id, opts)
      await refreshQueries()
    })
  }

  const runSavedQuery = (ds: Query, paramValues: Record<string, string>) => {
    void run(async () => {
      const names = extractFilterParams(ds.filter)
      const params: Record<string, unknown> = {}
      for (const n of names) {
        params[n] = (paramValues[n] ?? '').trim()
      }
      const r = await executeQuery(selectedTable, ds.name || ds.id, { pageSize: 50, params }, opts)
      setDsQueryColumnIds(ds.columnIds || [])
      setDsQueryRows(r.rows || [])
      setDsQueryCount(r.count ?? r.rows?.length ?? 0)
      setDsQueryTarget(null)
    })
  }

  const onExecuteQuery = (ds: Query) => {
    const names = extractFilterParams(ds.filter)
    if (names.length === 0) {
      runSavedQuery(ds, {})
      return
    }
    setDsQueryTarget(ds)
    setDsQueryParamValues((prev) => {
      const next: Record<string, string> = {}
      for (const n of names) next[n] = prev[n] ?? ''
      return next
    })
  }

  const onCreateTable = () => {
    const name = newTableName.trim()
    if (!name) return
    void run(async () => {
      const body: { idType: string; label?: string } = { idType: newTableIdType }
      const label = newTableLabel.trim()
      if (label) body.label = label
      await createTable(name, opts, body)
      setNewTableName('')
      setNewTableLabel('')
      await refreshTables(name)
    })
  }

  const onCreateTenant = () => {
    const id = newTenantId.trim()
    if (!id) {
      setErr('Tenant id is required.')
      return
    }
    void run(async () => {
      const body: {
        id: string
        displayName?: string
        dataDsn?: string
        createDatabase?: boolean
        recordStore?: 'shared' | 'dedicated'
      } = { id, recordStore: newTenantRecordStore }
      const displayName = newTenantDisplayName.trim()
      if (displayName) body.displayName = displayName
      const dataDsn = newTenantDataDsn.trim()
      if (dataDsn) body.dataDsn = dataDsn
      if (newTenantCreateDb) body.createDatabase = true
      const created = await createTenant(body, { baseUrl: apiBase })
      setTenantId(id)
      setNewTenantId('')
      setNewTenantDisplayName('')
      setNewTenantDataDsn('')
      setNewTenantCreateDb(false)
      setNewTenantRecordStore('shared')
      await refreshTenants()
      if (created.key) {
        window.alert(
          `Tenant "${id}" created.\n` +
            `Public base: ${created.base?.baseId ?? `base_${id}`}\n` +
            `API key (copy now, shown once):\n${created.key}`,
        )
      }
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
      setErr('Formula column requires an Excel expression.')
      return
    }
    if (newColType === 'link') {
      if (!newRelTargetTable.trim()) {
        setErr('link requires a target table.')
        return
      }
      if (newRelCardinality === 'many' && !newRelLinkColumn) {
        setErr('link (many) requires link_column_id on the child table.')
        return
      }
      if (newRelCardinality === 'one' && !newRelFKColumn) {
        setErr('link (one) requires target_column_id on this table.')
        return
      }
    }
    void run(async () => {
      const body: Parameters<typeof createColumn>[0] = {
        tableName: selectedTable,
        name: newColName.trim(),
        typeId: newColType,
        isNullable: newColNullable,
        position: columns.length + 1,
      }
      const colLabel = newColLabel.trim()
      if (colLabel) body.label = colLabel
      if (newColType === 'formula') {
        body.config = { expression: newFormulaExpr.trim() }
      }
      if (newColType === 'link') {
        body.config = { target_table_name: newRelTargetTable.trim() }
        if (newRelCardinality === 'many') {
          body.config.link_column_id = newRelLinkColumn
        } else {
          body.config.target_column_id = newRelFKColumn
        }
      }
      if (newColType === 'lookup') {
        body.config = {
          relation_column_id: newLookupRelColumn,
          target_column_id: newLookupTargetColumn,
        }
        const lf = newLookupFilter.trim()
        if (lf) {
          try {
            body.config.filter = JSON.parse(lf) as Record<string, unknown>
          } catch {
            setErr('Lookup filter must be valid JSON.')
            return
          }
        }
      }
      if (newColType === 'rollup') {
        body.config = {
          relation_column_id: newRollupRelColumn,
          aggregate: newRollupAggregate,
        }
        if (newRollupTargetColumn) body.config.target_column_id = newRollupTargetColumn
        const rf = newRollupFilter.trim()
        if (rf) {
          try {
            body.config.filter = JSON.parse(rf) as Record<string, unknown>
          } catch {
            setErr('Rollup filter must be valid JSON.')
            return
          }
        }
      }
      await createColumn(body, opts)
      setNewColName('')
      setNewColLabel('')
      setNewFormulaExpr('')
      setNewLookupRelColumn('')
      setNewLookupTargetColumn('')
      setNewRollupRelColumn('')
      setNewRollupAggregate('count')
      setNewRollupTargetColumn('')
      setNewLookupFilter('')
      setNewRollupFilter('')
      setNewRelTargetTable('')
      setNewRelLinkColumn('')
      setNewRelFKColumn('')
      await loadGrid()
    })
  }

  const cancelColumnEdit = () => {
    setEditingColumnId(null)
    setEditColName('')
    setEditColLabel('')
    setEditColNullable(true)
    setEditColExpr('')
    setEditColType('')
  }

  const startColumnEdit = (col: Column) => {
    setEditingColumnId(col.name)
    setEditColName(col.name)
    setEditColLabel(col.label || '')
    setEditColNullable(col.isNullable !== false)
    setEditColExpr(columnExpression(col))
    setEditColType(col.typeId)
  }

  const onSaveColumn = (col: Column) => {
    const name = editColName.trim()
    if (!name) {
      setErr('Column name is required.')
      return
    }
    if (isFormulaColumn(col) && !editColExpr.trim()) {
      setErr('Expression cannot be empty.')
      return
    }
    void run(async () => {
      const body: Parameters<typeof updateColumn>[2] = { name }
      const label = editColLabel.trim()
      if (label !== (col.label || '')) body.label = label
      if (!isVirtualKind(col)) {
        body.isNullable = editColNullable
        if (editColType && editColType !== col.typeId) {
          body.typeId = editColType
        }
      }
      if (isFormulaColumn(col)) {
        body.config = { expression: editColExpr.trim() }
      }
      await updateColumn(selectedTable, col.name, body, opts)
      cancelColumnEdit()
      await loadGrid()
    })
  }

  const onDeleteColumn = (col: Column) => {
    if (!window.confirm(`Delete column "${col.name}"?`)) return
    void run(async () => {
      await deleteColumn(selectedTable, col.name, opts)
      await loadGrid()
    })
  }

  const onAddIndex = () => {
    if (!selectedTable || !newIdxName.trim() || !newIdxCols.length) return
    void run(async () => {
      await createIndex(
        {
          tableName: selectedTable,
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
    const fields: Record<string, unknown> = {}
    for (const c of writable) {
      const v = cellToNative(
        c.typeId,
        newRowCells[c.name] ?? '',
        false,
        isArrayColumn(c, types) || isLinkManyColumn(c),
      )
      if (v !== undefined) fields[c.name] = v
    }
    void run(async () => {
      await createRow(selectedTable, fields, opts)
      setNewRowCells({})
      await loadGrid()
    })
  }

  const onGridReady = (e: GridReadyEvent) => {
    gridApi.current = e.api
  }

  const onCellValueChanged = (event: CellValueChangedEvent<GridRow>) => {
    const field = event.colDef.field
    if (!selectedTable || !field || field === 'id') return
    const col = columns.find((c) => c.name === field)
    if (!col || !isWritableColumn(col)) return
    const rowId = event.data?.id
    if (!rowId) return
    const newVal = event.newValue == null ? '' : String(event.newValue)
    const oldVal = event.oldValue == null ? '' : String(event.oldValue)
    if (newVal === oldVal) return

    const cell = cellToNative(
      col.typeId,
      newVal,
      true,
      isArrayColumn(col, types) || isLinkManyColumn(col),
    )
    if (cell === undefined) return

    void (async () => {
      setErr(null)
      try {
        await updateRow(selectedTable, rowId, { [col.name]: cell }, opts)
        if (columns.some(isFormulaColumn)) {
          await loadGrid(true)
        }
      } catch (e) {
        setErr(e instanceof Error ? e.message : String(e))
        event.node.setDataValue(field, event.oldValue)
      }
    })()
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
    <div className="studio">
      {/* Icon nav rail */}
      <nav className="nav-rail">
        <div className="nav-rail-logo" title="lowcode-database">LC</div>
        <div className="nav-rail-items">
          <button
            type="button"
            className={`nav-rail-btn${page === 'editor' ? ' active' : ''}`}
            title="Table Editor"
            onClick={() => goPage('editor')}
          >
            <IconTable />
          </button>
          <button
            type="button"
            data-testid="tab-queries"
            className={`nav-rail-btn${page === 'queries' ? ' active' : ''}`}
            title="Queries"
            onClick={() => goPage('queries')}
          >
            <IconDatabase />
          </button>
          <button
            type="button"
            data-testid="tab-types"
            className={`nav-rail-btn${page === 'types' ? ' active' : ''}`}
            title="Column types"
            onClick={() => goPage('types')}
          >
            <IconTypes />
          </button>
        </div>
        <button
          type="button"
          className={`nav-rail-btn${page === 'settings' ? ' active' : ''}`}
          title="Settings"
          onClick={() => goPage('settings')}
        >
          <IconSettings />
        </button>
      </nav>

      {/* Table sidebar (editor / queries) */}
      {(page === 'editor' || page === 'queries') && (
        <aside className="table-sidebar">
          <div className="table-sidebar-header">
            <p className="table-sidebar-title">Tables</p>
            <div className="table-search">
              <span className="table-search-icon"><IconSearch size={14} /></span>
              <input
                value={tableSearch}
                onChange={(e) => setTableSearch(e.target.value)}
                placeholder="Search tables…"
              />
            </div>
          </div>
          <div className="table-list">
            {filteredTables.map((t) => (
              <button
                key={t.id}
                type="button"
                className={`table-list-item${selectedTable === t.id ? ' active' : ''}`}
                onClick={() => setSelectedTable(t.id)}
              >
                <span className="table-list-item-icon"><IconTable size={14} /></span>
                <span className="table-list-item-name">
                  {t.label?.trim() || t.name}
                </span>
              </button>
            ))}
            {!filteredTables.length && (
              <p className="muted" style={{ padding: '8px 12px' }}>No tables</p>
            )}
          </div>
          <div className="table-sidebar-footer create-table-form">
            <input
              data-testid="create-table-name"
              value={newTableName}
              onChange={(e) => setNewTableName(e.target.value)}
              placeholder="table_name"
            />
            <input
              data-testid="create-table-label"
              value={newTableLabel}
              onChange={(e) => setNewTableLabel(e.target.value)}
              placeholder="label (optional)"
            />
            <select
              data-testid="create-table-id-type"
              value={newTableIdType}
              onChange={(e) => setNewTableIdType(e.target.value)}
            >
              {(tableIDTypes.length ? tableIDTypes : [{ id: 'uuid', name: 'uuid' }]).map((t) => (
                <option key={t.id} value={t.id}>{t.name || t.id}</option>
              ))}
            </select>
            <button type="button" className="btn btn-primary btn-sm" data-testid="create-table-btn" onClick={() => void onCreateTable()}>
              <IconPlus size={14} /> New table
            </button>
          </div>
          {/* Hidden select for e2e */}
          <select
            data-testid="table-select"
            className="sr-only-select"
            value={selectedTable}
            onChange={(e) => setSelectedTable(e.target.value)}
            tabIndex={-1}
            aria-hidden
          >
            {tables.map((t) => (
              <option key={t.id} value={t.id}>
                {t.label?.trim() ? `${t.label} (${t.name})` : t.name}
              </option>
            ))}
          </select>
        </aside>
      )}

      <div className="main-panel">
        {/* Header */}
        {page === 'editor' && selectedTable && (
          <header className="studio-header">
            <div className="studio-header-breadcrumb">
              <span>Table Editor</span>
              <span className="sep">/</span>
              <span className="current">{selectedTable}</span>
            </div>
            <span className="studio-header-meta">
              id: {tables.find((t) => t.id === selectedTable)?.idType || 'uuid'}
            </span>
            <div className="studio-header-actions">
              <button type="button" className="btn btn-ghost btn-sm" data-testid="reload-btn" onClick={() => void loadGrid()}>
                <IconRefresh size={14} /> Refresh
              </button>
              <button type="button" className="btn btn-danger btn-sm" onClick={() => void onDeleteTable()}>
                <IconTrash size={14} /> Delete
              </button>
            </div>
          </header>
        )}

        {page === 'queries' && (
          <header className="studio-header">
            <div className="studio-header-breadcrumb">
              <span>Database</span>
              <span className="sep">/</span>
              <span className="current">Data Sources</span>
              {selectedTable && (
                <>
                  <span className="sep">/</span>
                  <span>{selectedTable}</span>
                </>
              )}
            </div>
          </header>
        )}

        {page === 'types' && (
          <header className="studio-header">
            <div className="studio-header-breadcrumb">
              <span className="current">Column types</span>
            </div>
            <div className="studio-header-actions">
              <button type="button" className="btn btn-ghost btn-sm" onClick={() => void refreshTypes()}>
                <IconRefresh size={14} /> Refresh
              </button>
            </div>
          </header>
        )}

        {page === 'settings' && (
          <header className="studio-header">
            <div className="studio-header-breadcrumb">
              <span className="current">Settings</span>
            </div>
          </header>
        )}

        {/* Editor tabs */}
        {page === 'editor' && (
          <div className="studio-tabs">
            <button
              type="button"
              data-testid="tab-rows"
              className={`studio-tab${tab === 'rows' ? ' active' : ''}`}
              onClick={() => setTab('rows')}
            >
              Data
            </button>
            <button
              type="button"
              data-testid="tab-schema"
              className={`studio-tab${tab === 'schema' ? ' active' : ''}`}
              onClick={() => setTab('schema')}
            >
              Structure
            </button>
          </div>
        )}

        <div className="studio-content">
        {page === 'queries' && (
          <div className="studio-content-scroll">
            <section>
              <h2>Queries</h2>
              <p className="muted">
                GET/POST /v1/admin/queries · POST /v1/data/queries/&#123;name&#125; · DELETE
                /v1/admin/queries/&#123;name&#125;. Filter uses column id; table = sidebar selection (
                {selectedTable || 'none'}).
              </p>
              <table className="meta-table">
                <thead>
                  <tr>
                    <th>name</th>
                    <th>label</th>
                    <th>columns</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  {queries.map((ds) => (
                    <tr key={ds.id}>
                      <td>{ds.name || ds.id}</td>
                      <td>{ds.label || '—'}</td>
                      <td className="mono">
                        {(ds.columnIds || []).length
                          ? (ds.columnIds || []).length
                          : 'all'}
                      </td>
                      <td className="meta-actions">
                        <button
                          type="button"
                          data-testid={`query-datasource-${ds.name || ds.id}`}
                          onClick={() => void onExecuteQuery(ds)}
                        >
                          Query
                        </button>
                        <button
                          type="button"
                          data-testid={`delete-datasource-${ds.name || ds.id}`}
                          onClick={() => void onDeleteQuery(ds)}
                        >
                          Delete
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
              {!queries.length && (
                <p className="muted">No queries for this table yet.</p>
              )}
              {dsQueryTarget && extractFilterParams(dsQueryTarget.filter).length > 0 && (
                <div className="ds-query-params" data-testid="datasource-query-params">
                  <h3>
                    Query parameters — {dsQueryTarget.name || dsQueryTarget.id}
                  </h3>
                  <p className="muted">
                    Filter uses {'{param}'} placeholders; enter values before running the query.
                  </p>
                  <div className="form-row">
                    {extractFilterParams(dsQueryTarget.filter).map((name) => (
                      <label key={name}>
                        {name}
                        <input
                          data-testid={`datasource-query-param-${name}`}
                          value={dsQueryParamValues[name] ?? ''}
                          onChange={(e) =>
                            setDsQueryParamValues((prev) => ({
                              ...prev,
                              [name]: e.target.value,
                            }))
                          }
                          placeholder={`{${name}}`}
                        />
                      </label>
                    ))}
                  </div>
                  <button
                    type="button"
                    data-testid="datasource-query-run"
                    onClick={() => runSavedQuery(dsQueryTarget, dsQueryParamValues)}
                  >
                    Run query
                  </button>
                  <button
                    type="button"
                    className="filter-remove-btn"
                    onClick={() => setDsQueryTarget(null)}
                  >
                    Cancel
                  </button>
                </div>
              )}
              {dsQueryRows.length > 0 && (
                <div className="ds-query-result" data-testid="datasource-query-result">
                  <h3>
                    Query result ({dsQueryCount ?? dsQueryRows.length} row
                    {(dsQueryCount ?? dsQueryRows.length) === 1 ? '' : 's'})
                  </h3>
                  <table className="meta-table">
                    <thead>
                      <tr>
                        <th>id</th>
                        {dsQueryColumnIds.map((colRef) => {
                          const col = resolveColumnRef(columns, colRef)
                          return <th key={colRef}>{columnDisplay(col || { id: colRef, name: colRef, typeId: '' })}</th>
                        })}
                      </tr>
                    </thead>
                    <tbody>
                      {dsQueryRows.map((row) => (
                        <tr key={row.id}>
                          <td className="mono">{row.id.slice(0, 8)}…</td>
                          {dsQueryColumnIds.map((colRef) => {
                            const col = resolveColumnRef(columns, colRef)
                            const cellKey = col?.name || colRef
                            return (
                              <td key={colRef}>{formatCell(row[cellKey]) || '—'}</td>
                            )
                          })}
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
              <div className="choice-create-panel ds-create-panel">
                <h3>Create query</h3>
                <div className="form-grid ds-name-row">
                  <div className="form-field">
                    <label htmlFor="create-datasource-name">Name</label>
                    <input
                      id="create-datasource-name"
                      data-testid="create-datasource-name"
                      value={newDsName}
                      onChange={(e) => setNewDsName(e.target.value)}
                      placeholder="active_items"
                    />
                  </div>
                  <div className="form-field">
                    <label htmlFor="create-datasource-label">Label</label>
                    <input
                      id="create-datasource-label"
                      value={newDsLabel}
                      onChange={(e) => setNewDsLabel(e.target.value)}
                      placeholder="optional"
                    />
                  </div>
                </div>
                <div className="ds-form-section">
                  <h4 className="ds-form-section-title">Filter</h4>
                <FilterBuilder
                  columns={columns.filter(isGridColumn).map((c) => ({
                    name: c.name,
                    label: c.label,
                    typeId: c.typeId,
                    resultTypeId:
                      c.resultTypeId ||
                      (typeof c.config?.result_type_id === 'string'
                        ? c.config.result_type_id
                        : undefined),
                    expression: c.typeId === 'formula' ? columnExpression(c) : undefined,
                  }))}
                  value={newDsFilter}
                  onChange={setNewDsFilter}
                  valueTestId="create-datasource-filter-val"
                />
                </div>
                <div className="ds-form-section">
                  <h4 className="ds-form-section-title">Sort</h4>
                  <div className="form-row ds-sort-row">
                    <label>
                      Column
                      <select
                        value={newDsSortCol}
                        onChange={(e) => setNewDsSortCol(e.target.value)}
                      >
                        <option value="">— none —</option>
                        {columns.filter(isGridColumn).map((c) => (
                          <option key={c.id} value={c.name}>
                            {c.name}{c.label?.trim() && c.label !== c.name ? ` · ${c.label.trim()}` : ''}
                          </option>
                        ))}
                      </select>
                    </label>
                    <label>
                      Order
                      <select
                        value={newDsSortOrder}
                        onChange={(e) => setNewDsSortOrder(e.target.value)}
                      >
                        <option value="ASC">ASC</option>
                        <option value="DESC">DESC</option>
                      </select>
                    </label>
                  </div>
                </div>

                <ColumnPicker
                  columns={columns.filter(isGridColumn).map((c) => ({
                    name: c.name,
                    label: c.label,
                    typeId: c.typeId,
                  }))}
                  selected={newDsColumnIds}
                  onChange={setNewDsColumnIds}
                />

                {err && page === 'queries' && (
                  <p className="error" data-testid="create-datasource-error">
                    {err}
                  </p>
                )}
                <button
                  type="button"
                  className="btn btn-primary"
                  data-testid="create-datasource-btn"
                  disabled={loading}
                  onClick={() => void onCreateQuery()}
                >
                  Create query
                </button>
              </div>
            </section>
          </div>
        )}

        {page === 'editor' && tab === 'schema' && (
          <div className="studio-content-scroll schema-panel">
            <div className="panel">
              <div className="panel-header"><h2>Columns</h2></div>
              <div className="panel-body">
              <p className="panel-desc">Manage table columns, types, and virtual column configs.</p>
              <table className="meta-table">
                <thead>
                  <tr>
                    <th>name</th>
                    <th>label</th>
                    <th>type</th>
                    <th>expression</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  {columns.map((c) => (
                    <tr key={c.id}>
                      <td>{c.name}</td>
                      <td>{c.label?.trim() || '—'}</td>
                      <td>
                        <span className={`type-badge ${typeBadgeColor(c.typeId)}`}>{c.typeId}</span>
                      </td>
                      <td className="mono">
                        {isFormulaColumn(c) ? (
                          <span className="formula-expr-preview" title={columnExpression(c)}>
                            {columnExpression(c) || '—'}
                          </span>
                        ) : (
                          '—'
                        )}
                      </td>
                      <td className="meta-actions">
                        <button
                          type="button"
                          data-testid={`edit-column-${c.name}`}
                          onClick={() => startColumnEdit(c)}
                        >
                          Edit
                        </button>
                        <button type="button" onClick={() => void onDeleteColumn(c)}>
                          Delete
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
              {editingColumnId && (() => {
                const col = columns.find((x) => x.name === editingColumnId)
                if (!col) return null
                return (
                  <div className="column-edit-panel" data-testid="column-edit-panel">
                    <h3>Edit column — {col.name}</h3>
                    <div className="form-row">
                      <label>
                        Name
                        <input
                          data-testid="edit-column-name"
                          value={editColName}
                          onChange={(e) => setEditColName(e.target.value)}
                        />
                      </label>
                      <label>
                        Label
                        <input
                          data-testid="edit-column-label"
                          value={editColLabel}
                          onChange={(e) => setEditColLabel(e.target.value)}
                          placeholder="optional"
                        />
                      </label>
                      {!isVirtualKind(col) ? (
                          <label>
                            Type
                            <select
                              data-testid="edit-column-type"
                              value={editColType}
                              onChange={(e) => setEditColType(e.target.value)}
                            >
                              {types.filter(isScalarType).map((t) => (
                                <option key={t.id} value={t.id}>
                                  {t.name || t.id}
                                </option>
                              ))}
                            </select>
                          </label>
                      ) : (
                        <label className="inline">
                          Type
                          <span className="type-badge">{col.typeId}</span>
                        </label>
                      )}
                      {!isVirtualKind(col) && (
                        <label className="inline">
                          <input
                            type="checkbox"
                            checked={editColNullable}
                            onChange={(e) => setEditColNullable(e.target.checked)}
                          />
                          nullable
                        </label>
                      )}
                    </div>
                    {isFormulaColumn(col) && (
                      <>
                        <p className="muted">Excel formula (evaluated in the app)</p>
                        <FormulaEditor
                          value={editColExpr}
                          onChange={setEditColExpr}
                          columns={formulaRefColumns}
                        />
                      </>
                    )}
                    {!isFormulaColumn(col) && !isVirtualKind(col) && (
                      <p className="muted">physical column name: {col.name}</p>
                    )}
                    <div className="form-row">
                      <button
                        type="button"
                        data-testid="save-column-btn"
                        onClick={() => void onSaveColumn(col)}
                      >
                        Save
                      </button>
                      <button type="button" onClick={cancelColumnEdit}>
                        Cancel
                      </button>
                    </div>
                  </div>
                )
              })()}
              <div className="form-row">
                <input
                  data-testid="add-column-name"
                  value={newColName}
                  onChange={(e) => setNewColName(e.target.value)}
                  placeholder="column name"
                />
                <input
                  data-testid="add-column-label"
                  value={newColLabel}
                  onChange={(e) => setNewColLabel(e.target.value)}
                  placeholder="label (optional)"
                />
                <select
                  data-testid="add-column-type"
                  value={newColType}
                  onChange={(e) => {
                    setNewColType(e.target.value)
                    if (e.target.value !== 'formula') setNewFormulaExpr('')
                    if (e.target.value !== 'lookup') {
                      setNewLookupRelColumn('')
                      setNewLookupTargetColumn('')
                      setNewLookupFilter('')
                    }
                    if (e.target.value !== 'rollup') {
                      setNewRollupRelColumn('')
                      setNewRollupAggregate('count')
                      setNewRollupTargetColumn('')
                      setNewRollupFilter('')
                    }
                    if (e.target.value !== 'link') {
                      setNewRelTargetTable('')
                      setNewRelLinkColumn('')
                      setNewRelFKColumn('')
                      setNewRelCardinality('one')
                    }
                  }}
                >
                  <optgroup label="Built-in">
                    {builtinTypes.map((t) => (
                      <option key={t.id} value={t.id}>
                        {t.name || t.id}
                      </option>
                    ))}
                  </optgroup>
                  {customTypes.length > 0 && (
                    <optgroup label="Column types">
                      {customTypes.map((t) => (
                        <option key={t.id} value={t.id}>
                          {t.label || t.name || t.id}
                          {t.config?.array === true || (typeof t.pgType === 'string' && t.pgType.endsWith('[]'))
                            ? ' []'
                            : ''}
                        </option>
                      ))}
                    </optgroup>
                  )}
                </select>
                <label className="inline">
                  <input
                    type="checkbox"
                    checked={newColNullable}
                    onChange={(e) => setNewColNullable(e.target.checked)}
                    disabled={newColType === 'formula' || newColType === 'link'}
                  />
                  nullable
                </label>
                <button type="button" data-testid="add-column-btn" onClick={() => void onAddColumn()}>
                  Add column
                </button>
              </div>
              {newColType === 'formula' && (
                <div className="formula-add-panel">
                  <h3>New formula (Excel)</h3>
                  <FormulaEditor
                    value={newFormulaExpr}
                    onChange={setNewFormulaExpr}
                    columns={formulaRefColumns}
                  />
                </div>
              )}
              {newColType === 'link' && (
                <div className="relationship-panel column-edit-panel">
                  <h3>link config</h3>
                  <p className="muted">
                    Virtual column linking to another table. Requires <code>target_table_name</code>{' '}
                    and either <code>link_column_id</code> (one-to-many) or{' '}
                    <code>target_column_id</code> (many-to-one on this table). Related IDs live in{' '}
                    <code>link_ref</code>.
                  </p>
                  <div className="form-row">
                    <label>
                      Cardinality
                      <select
                        data-testid="add-rel-cardinality"
                        value={newRelCardinality}
                        onChange={(e) => {
                          setNewRelCardinality(e.target.value as 'many' | 'one')
                          setNewRelLinkColumn('')
                          setNewRelFKColumn('')
                        }}
                      >
                        <option value="one">many-to-one (FK on this table)</option>
                        <option value="many">one-to-many (FK on child table)</option>
                      </select>
                    </label>
                    <label>
                      Target table
                      <select
                        data-testid="add-rel-target-table"
                        value={newRelTargetTable}
                        onChange={(e) => {
                          setNewRelTargetTable(e.target.value)
                          setNewRelLinkColumn('')
                        }}
                      >
                        <option value="">— select —</option>
                        {tables
                          .filter((t) => t.id !== selectedTable)
                          .map((t) => (
                            <option key={t.id} value={t.id}>
                              {t.name}
                            </option>
                          ))}
                      </select>
                    </label>
                    {newRelCardinality === 'one' ? (
                      <label>
                        FK column (this table)
                        <select
                          data-testid="add-rel-fk-column"
                          value={newRelFKColumn}
                          onChange={(e) => setNewRelFKColumn(e.target.value)}
                        >
                          <option value="">— select —</option>
                          {physicalCols.map((c) => (
                            <option key={c.id} value={c.name}>
                              {c.name} ({c.typeId})
                            </option>
                          ))}
                        </select>
                      </label>
                    ) : (
                      <label>
                        Link column (child table)
                        <select
                          data-testid="add-rel-link-column"
                          value={newRelLinkColumn}
                          onChange={(e) => setNewRelLinkColumn(e.target.value)}
                          disabled={!newRelTargetTable}
                        >
                          <option value="">— select —</option>
                          {relLinkColumns.map((c) => (
                            <option key={c.id} value={c.name}>
                              {c.name} ({c.typeId})
                            </option>
                          ))}
                        </select>
                      </label>
                    )}
                  </div>
                  {newRelCardinality === 'one' && (
                    <p className="muted">
                      Example: column <code>order_id</code> on this table → order table row.
                      table row.
                    </p>
                  )}
                  {newRelCardinality === 'many' && (
                    <p className="muted">
                      Example: child table has a column storing this table&apos;s row{' '}
                      <code>id</code>.
                    </p>
                  )}
                </div>
              )}
              {newColType === 'lookup' && (
                <div className="lookup-panel column-edit-panel">
                  <h3>lookup config</h3>
                  <p className="muted">
                    Projects values from a related table via a <strong>link</strong>.
                    <strong> One</strong>: single related row (e.g. customer name).
                    <strong> Many</strong>: aggregates child rows into an array (e.g. product names on an order) — filter with{' '}
                    <code>has</code> / <code>overlaps</code>.
                  </p>
                  {!lookupRelColumns.length && (
                    <p className="error">
                      No link columns on this table. Create a link column first.
                    </p>
                  )}
                  <div className="form-row">
                    <label>
                      Relationship column
                      <select
                        data-testid="add-lookup-rel-column"
                        value={newLookupRelColumn}
                        onChange={(e) => setNewLookupRelColumn(e.target.value)}
                      >
                        <option value="">— select —</option>
                        {lookupRelColumns.map((c) => {
                          const card = relationshipCardinality(c) ?? '?'
                          return (
                          <option key={c.id} value={c.name}>
                            {c.name} ({card}) → {relationshipTargetTable(c) || '?'}
                          </option>
                        )})}
                      </select>
                    </label>
                    <label>
                      Target column (related table)
                      <select
                        data-testid="add-lookup-target-column"
                        value={newLookupTargetColumn}
                        onChange={(e) => setNewLookupTargetColumn(e.target.value)}
                        disabled={!newLookupRelColumn}
                      >
                        <option value="">— select —</option>
                        {lookupTargetColumns.map((c) => (
                          <option key={c.id} value={c.name}>
                            {c.name} ({c.typeId})
                          </option>
                        ))}
                      </select>
                    </label>
                  </div>
                  <label className="block">
                    Filter on related rows (optional JSON, same DSL as saved queries)
                    <textarea
                      data-testid="add-lookup-filter"
                      value={newLookupFilter}
                      onChange={(e) => setNewLookupFilter(e.target.value)}
                      rows={3}
                      placeholder='{"type":"EQ","attr":"status","val":"active"}'
                      disabled={!newLookupRelColumn}
                    />
                  </label>
                  <p className="muted">
                    Only include related rows matching this filter (Teable/NocoDB-style linked record
                    filter).
                  </p>
                </div>
              )}
              {newColType === 'rollup' && (
                <div className="rollup-panel column-edit-panel">
                  <h3>rollup config</h3>
                  <p className="muted">
                    Aggregates rows from a related table via a <strong>link</strong>{' '}
                    column (cardinality many). Requires <code>relation_column_id</code> and{' '}
                    <code>aggregate</code>; <code>target_column_id</code> is optional for count.
                  </p>
                  {!rollupRelColumns.length && (
                    <p className="error">
                      No link (many) columns on this table. Create a link column
                      first (with <code>link_column_id</code> on the child table).
                    </p>
                  )}
                  <div className="form-row">
                    <label>
                      Relationship column (many)
                      <select
                        data-testid="add-rollup-rel-column"
                        value={newRollupRelColumn}
                        onChange={(e) => setNewRollupRelColumn(e.target.value)}
                      >
                        <option value="">— select —</option>
                        {rollupRelColumns.map((c) => (
                          <option key={c.id} value={c.name}>
                            {c.name} → {relationshipTargetTable(c) || '?'}
                          </option>
                        ))}
                      </select>
                    </label>
                    <label>
                      Aggregate
                      <select
                        data-testid="add-rollup-aggregate"
                        value={newRollupAggregate}
                        onChange={(e) => setNewRollupAggregate(e.target.value)}
                      >
                        <option value="count">count</option>
                        <option value="sum">sum</option>
                        <option value="min">min</option>
                        <option value="max">max</option>
                        <option value="avg">avg</option>
                      </select>
                    </label>
                    <label>
                      Target column (related table)
                      <select
                        data-testid="add-rollup-target-column"
                        value={newRollupTargetColumn}
                        onChange={(e) => setNewRollupTargetColumn(e.target.value)}
                        disabled={!newRollupRelColumn || newRollupAggregate === 'count'}
                      >
                        <option value="">
                          {newRollupAggregate === 'count' ? '— not used for count —' : '— select —'}
                        </option>
                        {rollupTargetColumns.map((c) => (
                          <option key={c.id} value={c.name}>
                            {c.name} ({c.typeId})
                          </option>
                        ))}
                      </select>
                    </label>
                  </div>
                  <label className="block">
                    Filter on related rows (optional JSON)
                    <textarea
                      data-testid="add-rollup-filter"
                      value={newRollupFilter}
                      onChange={(e) => setNewRollupFilter(e.target.value)}
                      rows={3}
                      placeholder='{"type":"AND","val":[{"type":"EQ","attr":"status","val":"paid"}]}'
                      disabled={!newRollupRelColumn}
                    />
                  </label>
                  <p className="muted">
                    Aggregate only linked rows that match this filter (e.g. sum paid order lines
                    only).
                  </p>
                </div>
              )}
              </div>
            </div>

            <div className="panel">
              <div className="panel-header"><h2>Indexes</h2></div>
              <div className="panel-body">
              <p className="panel-desc">
                Today: PostgreSQL indexes from pg_catalog on physical tables.
                Target (Virtual-Records RFC): per-vt partial JSONB / FTS indexes on{' '}
                <code>virtual_records</code> — not exposed here yet.
              </p>
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
                  data-testid="create-index-name"
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
                      checked={newIdxCols.includes(c.name)}
                      onChange={() => toggleIdxCol(c.name)}
                    />
                    {c.name}
                  </label>
                ))}
              </div>
              <button type="button" className="btn btn-primary" data-testid="create-index-btn" onClick={() => void onAddIndex()}>
                Create index
              </button>
              </div>
            </div>
          </div>
        )}

        {page === 'editor' && tab === 'rows' && (
          <div className="rows-view">
            <div className="toolbar">
              <button type="button" className="btn btn-primary btn-sm" data-testid="insert-row-btn" onClick={() => void onCreateRow()}>
                <IconPlus size={14} /> Insert row
              </button>
              <button type="button" className="btn btn-sm" onClick={() => void onDeleteSelected()}>
                <IconTrash size={14} /> Delete selected
              </button>
              <span className="toolbar-hint">Click cell to edit · computed columns are read-only</span>
            </div>
            <RowFilterBar
              columns={filterColumns}
              value={rowFilter}
              onChange={setRowFilter}
              onClear={clearRowFilter}
              loading={loading}
              rowCount={rowCount}
              active={rowFilterActive}
            />
            <div className="new-row-panel">
              <span className="toolbar-hint" style={{ margin: 0 }}>New row</span>
              <div className="new-row-fields">
              {columns.filter(isWritableColumn).map((c) => {
                const label = c.label?.trim()
                const showLabel = label && label !== c.name
                const aria = showLabel ? `${c.name} ${label}` : c.name
                return (
                <label key={c.id} className="cell-input">
                  <span className="cell-input-title">
                    <span className="cell-input-id">{c.name}</span>
                    {showLabel && <span className="cell-input-label">{label}</span>}
                  </span>
                  {isRelationshipColumn(c) && relationshipTargetTable(c) ? (
                    <RelationPicker
                      tableName={relationshipTargetTable(c)}
                      many={isLinkManyColumn(c)}
                      value={newRowCells[c.name] ?? ''}
                      onChange={(v) => setNewRowCells((prev) => ({ ...prev, [c.name]: v }))}
                      opts={opts}
                      aria-label={aria}
                      placeholder={`Select ${relationshipTargetTable(c)}`}
                    />
                  ) : isArrayColumn(c, types) ? (
                    <ArrayInput
                      aria-label={aria}
                      value={newRowCells[c.name] ?? ''}
                      onChange={(v) =>
                        setNewRowCells((prev) => ({ ...prev, [c.name]: v }))
                      }
                      placeholder="选项1, 选项2"
                    />
                  ) : (
                  <input
                    aria-label={aria}
                    value={newRowCells[c.name] ?? ''}
                    onChange={(e) =>
                      setNewRowCells((prev) => ({ ...prev, [c.name]: e.target.value }))
                    }
                    placeholder={c.typeId}
                  />
                  )}
                </label>
              )})}
              </div>
            </div>
            <div className="grid-host">
              <AgGridReact
                theme={gridTheme}
                rowData={rowData}
                columnDefs={colDefs}
                defaultColDef={{ sortable: true, filter: true, resizable: true, wrapHeaderText: true, autoHeaderHeight: true }}
                autoHeaderHeight
                rowSelection={{ mode: 'multiRow' }}
                singleClickEdit
                stopEditingWhenCellsLoseFocus
                onGridReady={onGridReady}
                onCellValueChanged={onCellValueChanged}
                getRowId={(p) => p.data.id}
              />
            </div>
          </div>
        )}

        {page === 'types' && (
          <div className="studio-content-scroll settings-page" data-testid="types-page">
            <h1>Column types</h1>
            <p className="muted" style={{ marginBottom: 24 }}>
              Tenant types (e.g. SELECT / MULTI_SELECT). Array-ness is <code>spec.array</code>, not column{' '}
              <code>config</code>. Create types here, then pick them when adding columns on a table.
            </p>

            <div className="settings-section panel">
              <div className="panel-header"><h2>Tenant column types</h2></div>
              <div className="panel-body">
                <table className="meta-table">
                  <thead>
                    <tr>
                      <th>name</th>
                      <th>label</th>
                      <th>pgType</th>
                      <th>array</th>
                      <th />
                    </tr>
                  </thead>
                  <tbody>
                    {customTypes.map((t) => (
                      <tr key={t.id}>
                        <td>
                          <code>{t.id}</code>
                        </td>
                        <td>{t.label || t.name || '—'}</td>
                        <td>
                          <code>{t.pgType || '?'}</code>
                        </td>
                        <td>{t.config?.array === true ? 'yes' : '—'}</td>
                        <td className="meta-actions">
                          <button
                            type="button"
                            data-testid={`delete-columntype-${t.id}`}
                            onClick={() => {
                              if (!window.confirm(`Delete column type "${t.id}"?`)) return
                              void run(async () => {
                                await deleteColumnType(t.id, opts)
                                await refreshTypes()
                              })
                            }}
                          >
                            Delete
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                {!customTypes.length && (
                  <p className="muted">No tenant column types yet. Create select / multi_select below.</p>
                )}

                <h3 style={{ marginTop: '1.25rem' }}>Create</h3>
                <div className="form-row">
                  <input
                    data-testid="add-columntype-name"
                    value={newCtName}
                    onChange={(e) => setNewCtName(e.target.value)}
                    placeholder="name (e.g. multi_select)"
                  />
                  <input
                    data-testid="add-columntype-label"
                    value={newCtLabel}
                    onChange={(e) => setNewCtLabel(e.target.value)}
                    placeholder="label (optional)"
                  />
                  <select
                    data-testid="add-columntype-pgtype"
                    value={newCtPgType}
                    onChange={(e) => setNewCtPgType(e.target.value)}
                  >
                    <option value="text">text</option>
                    <option value="number">number</option>
                    <option value="datetime">datetime</option>
                    <option value="boolean">boolean</option>
                    <option value="jsonb">jsonb</option>
                  </select>
                  <label className="inline">
                    <input
                      type="checkbox"
                      data-testid="add-columntype-array"
                      checked={newCtArray}
                      onChange={(e) => setNewCtArray(e.target.checked)}
                    />
                    array
                  </label>
                  <button
                    type="button"
                    className="btn btn-primary"
                    data-testid="add-columntype-btn"
                    onClick={() => {
                      const name = newCtName.trim()
                      if (!name) {
                        setErr('Column type name is required.')
                        return
                      }
                      void run(async () => {
                        await createColumnType(
                          {
                            name,
                            label: newCtLabel.trim() || undefined,
                            spec: { pgType: newCtPgType, array: newCtArray || undefined },
                          },
                          opts,
                        )
                        setNewCtName('')
                        setNewCtLabel('')
                        setNewCtArray(false)
                        setNewCtPgType('text')
                        await refreshTypes()
                        setNewColType(name)
                      })
                    }}
                  >
                    <IconPlus size={14} /> Add column type
                  </button>
                </div>
                {err && page === 'types' && <p className="error">{err}</p>}
              </div>
            </div>

            <div className="settings-section panel">
              <div className="panel-header"><h2>Built-in pgTypes</h2></div>
              <div className="panel-body">
                <p className="panel-desc">Platform scalars and virtual kinds (read-only).</p>
                <table className="meta-table">
                  <thead>
                    <tr>
                      <th>id</th>
                      <th>name</th>
                      <th>pgType</th>
                    </tr>
                  </thead>
                  <tbody>
                    {builtinTypes.map((t) => (
                      <tr key={t.id}>
                        <td>
                          <code>{t.id}</code>
                        </td>
                        <td>{t.name || t.id}</td>
                        <td>
                          <code>{t.pgType || '—'}</code>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        )}

        {page === 'settings' && (
          <div className="studio-content-scroll settings-page">
            <h1>Playground</h1>
            <p className="muted" style={{ marginBottom: 24 }}>lowcode-database debug console</p>

            <div className="settings-section panel">
              <div className="panel-header"><h2>Connection</h2></div>
              <div className="panel-body">
                <div className="form-grid">
                  <div className="form-field">
                    <label htmlFor="api-base">API base</label>
                    <input
                      id="api-base"
                      value={apiBase}
                      onChange={(e) => setApiBase(e.target.value)}
                      spellCheck={false}
                    />
                  </div>
                  <div className="form-field">
                    <label htmlFor="tenant-id">X-Tenant-Id</label>
                    <select
                      id="tenant-id"
                      data-testid="tenant-select"
                      value={tenantId}
                      onChange={(e) => onTenantChange(e.target.value)}
                      onFocus={() => void refreshTenants()}
                    >
                      {!tenantOptions.length && <option value={tenantId || 'default'}>{tenantId || 'default'}</option>}
                      {tenantOptions.map((t) => (
                        <option key={t.tenantId} value={t.tenantId}>
                          {t.name ? `${t.tenantId} — ${t.name}` : t.tenantId}
                        </option>
                      ))}
                    </select>
                  </div>
                </div>
                <p className="panel-desc">
                  X-Tenant-Id is required on every request. Pick a tenant from the list (loaded via GET /v1/admin/tenants).
                </p>
                <div className="form-row">
                  <button type="button" className="btn" onClick={() => void refreshTables()}>Refresh tables</button>
                  <button type="button" className="btn" onClick={() => void onConnection()}>DB connection info</button>
                  <button
                    type="button"
                    className="btn"
                    onClick={() => {
                      void (async () => {
                        try {
                          const res = await listTenants({ baseUrl: apiBase, tenantId: 'default' })
                          const list = res.tenants || []
                          setTenants(list)
                          setConn(`tenants: ${list.map((t) => t.tenantId).join(', ') || '(none)'}`)
                        } catch (e) {
                          setErr(e instanceof Error ? e.message : String(e))
                        }
                      })()
                    }}
                  >
                    List tenants
                  </button>
                </div>
                {conn && <pre className="conn">{conn}</pre>}
              </div>
            </div>

            <div className="settings-section panel">
              <div className="panel-header"><h2>Create tenant</h2></div>
              <div className="panel-body">
                <p className="panel-desc">POST /v1/admin/tenants — seeds a public base and default API key (plaintext returned once).</p>
                <div className="form-grid">
                  <div className="form-field">
                    <label htmlFor="create-tenant-id">Tenant id</label>
                    <input
                      id="create-tenant-id"
                      data-testid="create-tenant-id"
                      value={newTenantId}
                      onChange={(e) => setNewTenantId(e.target.value)}
                      placeholder="tenant_a"
                    />
                  </div>
                  <div className="form-field">
                    <label htmlFor="tenant-display">Display name</label>
                    <input
                      id="tenant-display"
                      value={newTenantDisplayName}
                      onChange={(e) => setNewTenantDisplayName(e.target.value)}
                      placeholder="optional"
                    />
                  </div>
                  <div className="form-field">
                    <label htmlFor="tenant-dsn">Data DSN</label>
                    <input
                      id="tenant-dsn"
                      value={newTenantDataDsn}
                      onChange={(e) => setNewTenantDataDsn(e.target.value)}
                      placeholder="optional"
                      spellCheck={false}
                    />
                  </div>
                  <div className="form-field">
                    <label htmlFor="tenant-record-store">Record store</label>
                    <select
                      id="tenant-record-store"
                      value={newTenantRecordStore}
                      onChange={(e) => setNewTenantRecordStore(e.target.value as 'shared' | 'dedicated')}
                    >
                      <option value="shared">shared (record)</option>
                      <option value="dedicated">dedicated ({'{tenant_id}'}_record)</option>
                    </select>
                  </div>
                </div>
                <label className="inline">
                  <input
                    type="checkbox"
                    checked={newTenantCreateDb}
                    onChange={(e) => setNewTenantCreateDb(e.target.checked)}
                  />
                  Create Postgres database
                </label>
                <div className="form-row">
                  <button type="button" className="btn btn-primary" data-testid="create-tenant-btn" onClick={() => void onCreateTenant()}>
                    Create tenant
                  </button>
                </div>
              </div>
            </div>

            {selectedTable && (
              <div className="settings-section panel">
                <div className="panel-header"><h2>Table: {selectedTable}</h2></div>
                <div className="panel-body">
                  <div className="form-field">
                    <label htmlFor="rename-table">Rename table</label>
                    <div className="rename-inline">
                      <input
                        id="rename-table"
                        value={renameTo}
                        onChange={(e) => setRenameTo(e.target.value)}
                        placeholder="new table name"
                      />
                      <button type="button" className="btn" onClick={() => void onRename()}>Rename</button>
                    </div>
                  </div>
                </div>
              </div>
            )}
          </div>
        )}
        </div>

        <footer className="status-bar">
          <h1 className="playground-brand">Playground</h1>
          <label className="tenant-inline" htmlFor="tenant-id-bar">
            <span className="tenant-label">X-Tenant-Id</span>
            <select
              id="tenant-id-bar"
              data-testid="tenant-select-bar"
              value={tenantId}
              onChange={(e) => onTenantChange(e.target.value)}
              onFocus={() => void refreshTenants()}
              title="Select tenant"
            >
              {!tenantOptions.length && <option value={tenantId || 'default'}>{tenantId || 'default'}</option>}
              {tenantOptions.map((t) => (
                <option key={t.tenantId} value={t.tenantId}>
                  {t.tenantId}
                </option>
              ))}
            </select>
          </label>
          <span>{apiBase.replace(/^https?:\/\//, '').split('/')[0]}</span>
          {loading && <span className="loading">Loading…</span>}
          {err && <span className="error">{err}</span>}
        </footer>
      </div>
    </div>
  )
}

type GridRow = { id: string } & Record<string, string | undefined>

function flattenCells(row: Row, cols: Column[]): Record<string, string | undefined> {
  const out: Record<string, string | undefined> = {}
  for (const c of cols) {
    if (c.name === 'id') continue
    const v = row[c.name]
    out[c.name] = v === undefined ? undefined : formatCell(v)
  }
  return out
}
