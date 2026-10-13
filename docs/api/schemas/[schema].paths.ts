import { listSchemas } from '../../.vitepress/openapi'

// One generated page per component schema, keyed by its name.
export default {
  paths() {
    return listSchemas().map((name) => ({ params: { schema: name, name } }))
  }
}
