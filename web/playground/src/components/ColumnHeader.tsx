import type { CustomHeaderProps } from 'ag-grid-react'
import { typeBadgeColor } from '../lib/column-utils'

export type ColumnHeaderParams = {
  name: string
  label?: string
  typeId: string
  prefix?: string
}

export function ColumnHeader(props: CustomHeaderProps & ColumnHeaderParams) {
  const { name, label, typeId, prefix = '' } = props
  const display = label?.trim()
  const showLabel = display && display !== name

  return (
    <div className="col-header">
      <div className="col-header-row">
        <span className="col-header-id" title={name}>
          {prefix}
          {name}
        </span>
        <span className={`col-header-type type-badge ${typeBadgeColor(typeId)}`}>{typeId}</span>
      </div>
      {showLabel && (
        <span className="col-header-display" title={display}>
          {display}
        </span>
      )}
    </div>
  )
}
