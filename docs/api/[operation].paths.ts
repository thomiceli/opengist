import { listOperations } from '../.vitepress/openapi'

// One generated page per operation, keyed by operationId.
export default {
  paths() {
    return listOperations().map((op) => ({
      params: { operation: op.id, id: op.id, title: `${op.method} ${op.path}` }
    }))
  }
}
