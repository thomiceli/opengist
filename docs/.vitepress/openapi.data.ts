import { defineLoader } from 'vitepress'
import { buildApiData, SPEC_FILE, type ApiData } from './openapi'

// `data` is populated at build time and importable from any component.
declare const data: ApiData
export { data }
export type { ApiData }

export default defineLoader({
  // Rebuild whenever the spec changes (path is relative to this file).
  watch: [SPEC_FILE],
  load: (): Promise<ApiData> => buildApiData()
})
