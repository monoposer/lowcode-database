import { colorSchemeDark, themeQuartz } from 'ag-grid-community'

/** Dark studio theme — use with AG Grid v33+ Theming API (no legacy ag-grid.css). */
export const gridTheme = themeQuartz.withPart(colorSchemeDark).withParams({
  accentColor: '#3ecf8e',
  backgroundColor: '#171717',
  foregroundColor: '#ededed',
  chromeBackgroundColor: '#1f1f1f',
  borderColor: '#2e2e2e',
  rowHoverColor: 'rgba(255, 255, 255, 0.04)',
  selectedRowBackgroundColor: 'rgba(62, 207, 142, 0.12)',
  headerTextColor: '#8b8b8b',
  headerFontWeight: 600,
  fontSize: 13,
  dataFontSize: 13,
  wrapperBorder: false,
  fontFamily: ['Inter', 'system-ui', 'sans-serif'],
})
