import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { parse } from 'yaml'

// The authoritative spec lives in the Go source tree.
export const SPEC_FILE = '../../internal/web/handlers/api/openapi.yaml'
const SPEC_URL = new URL(SPEC_FILE, import.meta.url)
const METHOD_ORDER = ['get', 'post', 'put', 'patch', 'delete']

// ---- shapes consumed by the renderer components (all JSON-serializable) ----
export interface ApiData {
  info: { title: string; version: string; descriptionHtml: string }
  authHtml: string
  servers: { url: string; description: string }[]
  groups: Group[]
  schemas: SchemaDef[]
}
export interface Group {
  name: string
  descriptionHtml: string
  operations: Operation[]
}
export interface Operation {
  id: string
  method: string
  path: string
  summary: string
  descriptionHtml: string
  auth: { anonymous: boolean; anyToken: boolean; scopes: string[] }
  parameters: Param[]
  requestBody: (BodyTable & { required: boolean }) | null
  responses: ResponseDef[]
  // Synthesized samples for the right-hand code panel (pre-highlighted HTML).
  sample: { curlHtml: string; responseStatus: string; responseHtml: string | null }
}
interface Param {
  name: string
  in: string
  required: boolean
  typeHtml: string
  descriptionHtml: string
}
interface Field {
  name: string
  typeHtml: string
  required: boolean
  descriptionHtml: string
  depth: number
}
// A body schema flattened into a field table (the inlined replacement for a
// schema link): its type label, whether it's an array, and its fields.
interface BodyTable {
  label: string
  arrayOf: boolean
  note: string
  rows: Field[]
}
interface ResponseDef {
  status: string
  descriptionHtml: string
  headers: { name: string; typeHtml: string; descriptionHtml: string }[]
  typeLabel: string | null
}
// A named component schema, rendered one-per-page.
export interface SchemaDef {
  name: string
  descriptionHtml: string
  note: string
  rows: Field[]
}

// Lightweight index used by config.mts (sidebar) and the dynamic-route
// generator. Synchronous and markdown-free so it's cheap to call.
export interface OpIndex {
  id: string
  method: string
  path: string
  summary: string
  tag: string
}

function readSpec(): any {
  return parse(readSpecRaw())
}

// Raw spec contents, for serving the file verbatim at /docs/api/openapi.yaml.
export function readSpecRaw(): string {
  return readFileSync(fileURLToPath(SPEC_URL), 'utf-8')
}

// Walk paths × methods in a stable order, yielding each operation.
function eachOperation(spec: any, cb: (op: any, method: string, path: string, item: any) => void) {
  for (const [path, item] of Object.entries<any>(spec.paths)) {
    for (const method of METHOD_ORDER) {
      const op = item[method]
      if (op) cb(op, method, path, item)
    }
  }
}

export function listOperations(): OpIndex[] {
  const spec = readSpec()
  const ops: OpIndex[] = []
  eachOperation(spec, (op, method, path) => {
    ops.push({
      id: op.operationId,
      method: method.toUpperCase(),
      path,
      summary: op.summary || op.operationId,
      tag: op.tags?.[0] || (spec.tags?.[0]?.name ?? '')
    })
  })
  return ops
}

// Names of the component schemas, for the sidebar and per-schema routes.
export function listSchemas(): string[] {
  return Object.keys(readSpec().components?.schemas || {})
}

export async function buildApiData(): Promise<ApiData> {
  // Lazily pulled in so this module can be imported from config.mts without
  // dragging the markdown renderer into the config load path.
  const { createMarkdownRenderer } = await import('vitepress')
  const spec = readSpec()
  const md = await createMarkdownRenderer(fileURLToPath(new URL('..', import.meta.url)))
  // Block rendering is async because Shiki highlighting in VitePress 2 is
  // resolved via markdown-it-async; renderInline carries no code fences.
  const html = (s?: string) => (s ? md.renderAsync(s) : Promise.resolve(''))
  const inline = (s?: string) => (s ? md.renderInline(s) : '')
  // Drop the "**Required ...**" scope line from operation descriptions — the
  // Authorization header row already documents the required scopes.
  const stripRequired = (s?: string) =>
    s
      ? s
          .split('\n')
          .filter((l) => !l.trimStart().startsWith('**Required'))
          .join('\n')
      : s

  const refName = (ref: string) => ref.split('/').pop() as string
  const resolve = (ref: string): any =>
    ref
      .replace(/^#\//, '')
      .split('/')
      .reduce((acc, key) => acc?.[key], spec)
  const deref = (obj: any): any => (obj?.$ref ? resolve(obj.$ref) : obj)

  // Named schema references link to their per-schema page.
  const link = (name: string) => `<a href="/docs/api/schemas/${name}"><code>${name}</code></a>`

  const typeHtml = (s: any): string => {
    if (!s) return '<code>any</code>'
    if (s.$ref) return link(refName(s.$ref))
    if (s.oneOf) return s.oneOf.map(typeHtml).join(' | ')
    if (s.allOf) return s.allOf.map(typeHtml).join(' &amp; ')
    if (s.type === 'array') return typeHtml(s.items) + '[]'
    if (s.type === 'null') return '<code>null</code>'
    if (s.type === 'object' && s.additionalProperties)
      return `map&lt;string, ${typeHtml(s.additionalProperties)}&gt;`
    let t = s.type || 'object'
    if (s.format) t += ` &lt;${s.format}&gt;`
    let out = `<code>${t}</code>`
    if (Array.isArray(s.enum))
      out += `: ${s.enum.map((e: any) => `<code>${e}</code>`).join(' | ')}`
    return out
  }

  const constraints = (s: any): string => {
    if (!s) return ''
    const bits: string[] = []
    if (s.default !== undefined) bits.push(`default: ${s.default}`)
    if (s.minimum !== undefined) bits.push(`min: ${s.minimum}`)
    if (s.maximum !== undefined) bits.push(`max: ${s.maximum}`)
    if (s.pattern) bits.push(`pattern: <code>${s.pattern}</code>`)
    return bits.length ? ` <em>(${bits.join(', ')})</em>` : ''
  }

  // Flatten a schema to its full property set, resolving $ref and allOf
  // (including inherited fields) so each body can render a complete table.
  // `seen` guards against recursive schemas.
  const flatten = (
    s: any,
    seen: string[] = []
  ): { props: Record<string, any>; required: string[]; addl: any } => {
    const acc = { props: {} as Record<string, any>, required: [] as string[], addl: null as any }
    if (!s) return acc
    if (s.$ref) {
      const name = refName(s.$ref)
      return seen.includes(name) ? acc : flatten(resolve(s.$ref), [...seen, name])
    }
    for (const part of s.allOf || []) {
      const f = flatten(part, seen)
      Object.assign(acc.props, f.props)
      acc.required.push(...f.required)
      if (f.addl) acc.addl = f.addl
    }
    if (s.properties) Object.assign(acc.props, s.properties)
    if (s.required) acc.required.push(...s.required)
    if (s.additionalProperties) acc.addl = s.additionalProperties
    return acc
  }

  // Turn a request/response body schema into a renderable field table.
  const bodyTable = (schema: any): BodyTable => {
    let s = schema
    let arrayOf = false
    while (s?.$ref) s = resolve(s.$ref)
    if (s?.type === 'array') {
      arrayOf = true
      s = s.items
      while (s?.$ref) s = resolve(s.$ref)
    }
    const label = schema ? typeHtml(schema) : '<code>any</code>'
    const f = flatten(s)
    const rows: Field[] = Object.entries<any>(f.props).map(([pname, ps]) => ({
      name: pname,
      typeHtml: typeHtml(ps),
      required: f.required.includes(pname),
      descriptionHtml: inline(ps.description),
      depth: 0
    }))
    const note = f.addl ? `Additional properties: map&lt;string, ${typeHtml(f.addl)}&gt;` : ''
    return { label, arrayOf, note, rows }
  }

  // Recursively expand an object/array/map schema into indented field rows,
  // descending into nested objects (used for the request body table). `seen`
  // tracks $ref names on the current branch to break recursive schemas.
  const childrenOf = (schema: any, depth: number, seen: string[]): Field[] => {
    let s = schema
    const branch = [...seen]
    while (s) {
      if (s.$ref) {
        const n = refName(s.$ref)
        if (branch.includes(n)) return []
        branch.push(n)
        s = resolve(s.$ref)
      } else if (s.type === 'array') s = s.items
      else if (s.oneOf) s = s.oneOf.find((o: any) => o.type !== 'null') ?? s.oneOf[0]
      else break
    }
    if (!s) return []
    const f = flatten(s, branch)
    const entries = Object.entries<any>(f.props)
    // A pure map (additionalProperties only): descend into the value's fields.
    if (entries.length === 0 && f.addl) return childrenOf(f.addl, depth, branch)
    const out: Field[] = []
    for (const [name, ps] of entries) {
      out.push({
        name,
        typeHtml: typeHtml(ps),
        required: f.required.includes(name),
        descriptionHtml: inline(ps.description),
        depth
      })
      out.push(...childrenOf(ps, depth + 1, branch))
    }
    return out
  }

  // Synthesize a representative value for a schema (no examples in the spec).
  // `seen` tracks $ref names on the current branch to break recursive schemas.
  const sample = (schema: any, seen: string[] = []): any => {
    if (!schema) return null
    if (schema.$ref) {
      const name = refName(schema.$ref)
      if (seen.includes(name)) return null
      return sample(resolve(schema.$ref), [...seen, name])
    }
    if (schema.example !== undefined) return schema.example
    if (schema.default !== undefined) return schema.default
    if (Array.isArray(schema.enum)) return schema.enum[0]
    if (schema.allOf)
      return schema.allOf.reduce((acc: any, s: any) => Object.assign(acc, sample(s, seen)), {})
    if (schema.oneOf) {
      const pick = schema.oneOf.find((s: any) => s.type !== 'null') ?? schema.oneOf[0]
      return sample(pick, seen)
    }
    switch (schema.type) {
      case 'array':
        return [sample(schema.items, seen)]
      case 'object': {
        const obj: Record<string, any> = {}
        for (const [k, v] of Object.entries<any>(schema.properties || {})) obj[k] = sample(v, seen)
        if (schema.additionalProperties && typeof schema.additionalProperties === 'object')
          obj['<key>'] = sample(schema.additionalProperties, seen)
        return obj
      }
      case 'integer':
      case 'number':
        return 1
      case 'boolean':
        return true
      case 'null':
        return null
      default:
        if (schema.format === 'date-time') return '2024-01-01T00:00:00Z'
        if (schema.format === 'uri') return 'https://opengist.example/...'
        if (schema.format === 'binary') return '<binary data>'
        return 'string'
    }
  }

  const HOST = 'https://opengist.example.com'
  const base = spec.servers?.[0]?.url ?? ''
  const fence = (lang: string, code: string) => md.renderAsync(`\`\`\`${lang}\n${code}\n\`\`\``)

  const tagDefs: any[] = spec.tags || []
  const groups: Group[] = []
  for (const t of tagDefs)
    groups.push({ name: t.name, descriptionHtml: await html(t.description), operations: [] })
  const groupByName = new Map(groups.map((g) => [g.name, g]))

  for (const [path, item] of Object.entries<any>(spec.paths)) {
    const pathParams: any[] = (item.parameters || []).map(deref)
    for (const method of METHOD_ORDER) {
      const op = item[method]
      if (!op) continue
      const allParams: any[] = [...pathParams, ...(op.parameters || []).map(deref)]

      const sec: any[] = op.security ?? spec.security ?? []
      const auth = { anonymous: false, anyToken: false, scopes: [] as string[] }
      const scopeSet = new Set<string>()
      for (const req of sec) {
        if (Object.keys(req).length === 0) auth.anonymous = true
        else {
          const arr: string[] = req.bearerAuth || []
          if (arr.length === 0) auth.anyToken = true
          else arr.forEach((s) => scopeSet.add(s))
        }
      }
      auth.scopes = [...scopeSet]

      const parameters: Param[] = allParams.map((p) => ({
        name: p.name,
        in: p.in,
        required: !!p.required,
        typeHtml: typeHtml(p.schema),
        descriptionHtml: inline(p.description) + constraints(p.schema)
      }))

      let requestBody: Operation['requestBody'] = null
      let requestSchema: any = null
      if (op.requestBody) {
        const rb = deref(op.requestBody)
        requestSchema = rb.content?.['application/json']?.schema
        // Expand nested object fields inline so the whole body shape is visible.
        requestBody = {
          required: !!rb.required,
          ...bodyTable(requestSchema),
          rows: childrenOf(requestSchema, 0, [])
        }
      }

      // ---- curl request sample: -X, then --url, then headers / body ----
      const query = allParams
        .filter((p) => p.in === 'query' && (p.required || p.schema?.default !== undefined))
        .map((p) => `${p.name}=${p.schema?.default ?? sample(p.schema)}`)
        .join('&')
      const url = `${HOST}${base}${path}${query ? '?' + query : ''}`
      const curlLines = [
        `curl -X ${method.toUpperCase()}`,
        `  --url "${url}"`,
        `  -H "Authorization: Bearer og_xxxxxxxx"`
      ]
      if (requestSchema) {
        curlLines.push(`  -H "Content-Type: application/json"`)
        curlLines.push(`  -d '${JSON.stringify(sample(requestSchema), null, 2)}'`)
      }
      const curlHtml = await fence('bash', curlLines.join(' \\\n'))

      // ---- example response (first 2xx with a JSON body) for the code panel ----
      let responseStatus = ''
      let responseHtml: string | null = null
      for (const [status, resRaw] of Object.entries<any>(op.responses)) {
        if (!status.startsWith('2')) continue
        responseStatus = status
        const schema = deref(resRaw).content?.['application/json']?.schema
        if (schema) responseHtml = await fence('json', JSON.stringify(sample(schema), null, 2))
        break
      }

      const responses: ResponseDef[] = Object.entries<any>(op.responses).map(([status, resRaw]) => {
        const res = deref(resRaw)
        const content = res.content || {}
        const ct = content['application/json'] || Object.values<any>(content)[0]
        return {
          status,
          descriptionHtml: inline(res.description),
          headers: Object.entries<any>(res.headers || {}).map(([name, hRaw]) => {
            const h = deref(hRaw)
            return { name, typeHtml: typeHtml(h.schema), descriptionHtml: inline(h.description) }
          }),
          typeLabel: ct?.schema ? typeHtml(ct.schema) : null
        }
      })

      const built: Operation = {
        id: op.operationId,
        method: method.toUpperCase(),
        path,
        summary: op.summary || '',
        descriptionHtml: await html(stripRequired(op.description)),
        auth,
        parameters,
        requestBody,
        responses,
        sample: { curlHtml, responseStatus, responseHtml }
      }
      ;(groupByName.get(op.tags?.[0]) ?? groups[0])?.operations.push(built)
    }
  }

  // ---- component schemas, each rendered on its own page ----
  const schemas: SchemaDef[] = []
  for (const [name, s] of Object.entries<any>(spec.components?.schemas || {})) {
    const t = bodyTable(s)
    schemas.push({
      name,
      descriptionHtml: await html(s.description),
      note: t.note,
      rows: t.rows
    })
  }

  return {
    info: {
      title: spec.info.title,
      version: spec.info.version,
      descriptionHtml: await html(spec.info.description)
    },
    authHtml: await html(spec.components?.securitySchemes?.bearerAuth?.description),
    servers: (spec.servers || []).map((s: any) => ({
      url: s.url,
      description: s.description || ''
    })),
    groups,
    schemas
  }
}
