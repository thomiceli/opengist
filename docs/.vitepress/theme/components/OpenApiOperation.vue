<script setup lang="ts">
import { computed } from 'vue'
import { data } from '../../openapi.data'

const props = defineProps<{ id: string }>()

const op = computed(() =>
  data.groups.flatMap((g) => g.operations).find((o) => o.id === props.id)
)

// Split the path so `{param}` segments can be highlighted.
const pathParts = computed(() =>
  (op.value?.path ?? '')
    .split(/(\{[^}]+\})/g)
    .filter(Boolean)
    .map((text) => ({ text, param: text.startsWith('{') && text.endsWith('}') }))
)

// The Authorization header is implied by the spec's bearerAuth security scheme
// rather than declared as a parameter, so synthesize a row from `auth`.
const authHeader = computed(() => {
  const a = op.value?.auth
  if (!a || (!a.scopes.length && !a.anyToken && !a.anonymous)) return null
  const desc = a.scopes.map((s) => `<code>${s}</code>`).join(', ')
  return {
    name: 'Authorization',
    typeHtml: '<code>string</code>',
    required: !a.anonymous,
    descriptionHtml: desc
  }
})

// Request parameters grouped by location, in display order, empty groups dropped.
const paramGroups = computed(() => {
  const o = op.value
  if (!o) return []
  const headers = [
    ...(authHeader.value ? [authHeader.value] : []),
    ...o.parameters.filter((p) => p.in === 'header')
  ]
  const groups = [
    { label: 'Headers', descLabel: 'Scopes', optional: true, items: headers },
    { label: 'Path parameters', descLabel: 'Description', optional: false, items: o.parameters.filter((p) => p.in === 'path') },
    { label: 'Query parameters', descLabel: 'Description', optional: false, items: o.parameters.filter((p) => p.in === 'query') }
  ]
  return groups.filter((g) => g.items.length)
})

const hasRequest = computed(() => paramGroups.value.length > 0 || !!op.value?.requestBody)
</script>

<template>
  <div v-if="op" class="oas-op-grid">
    <!-- left / center: description, request, responses -->
    <div class="oas-main">
      <h1>{{ op.summary }}</h1>
      <div class="oas-endpoint">
        <span class="oas-method" :class="`m-${op.method.toLowerCase()}`">{{ op.method }}</span>
        <code class="oas-path"><span
          v-for="(part, i) in pathParts"
          :key="i"
          :class="{ 'oas-param': part.param }"
        >{{ part.text }}</span></code>
      </div>

      <div class="oas-desc" v-html="op.descriptionHtml" />

      <template v-if="hasRequest">
        <h2>Request</h2>

        <template v-for="g in paramGroups" :key="g.label">
          <h3>{{ g.label }}</h3>
          <table class="oas-table">
            <thead>
              <tr><th>Name</th><th>Type</th><th>{{ g.descLabel }}</th></tr>
            </thead>
            <tbody>
              <tr v-for="p in g.items" :key="p.name">
                <td>
                  <code>{{ p.name }}</code>
                  <span v-if="p.required" class="oas-req">required</span>
                  <span v-else-if="g.optional" class="oas-opt">optional</span>
                </td>
                <td v-html="p.typeHtml" />
                <td v-html="p.descriptionHtml" />
              </tr>
            </tbody>
          </table>
        </template>

        <template v-if="op.requestBody">
          <h3>Body parameters</h3>
          <p class="oas-bodyhead">
            <span v-html="op.requestBody.label" />
            <span v-if="op.requestBody.required" class="oas-req">required</span>
            <span class="oas-ct">application/json</span>
          </p>
          <table v-if="op.requestBody.rows.length" class="oas-table">
            <thead>
              <tr><th>Field</th><th>Type</th><th>Description</th></tr>
            </thead>
            <tbody>
              <tr v-for="(row, i) in op.requestBody.rows" :key="i" :class="{ 'oas-nested': row.depth }">
                <td>
                  <span class="oas-field" :style="{ paddingLeft: row.depth * 1.25 + 'rem' }">
                    <span v-if="row.depth" class="oas-nest">↳</span>
                    <code>{{ row.name }}</code>
                    <span v-if="row.required" class="oas-req">required</span>
                    <span v-else class="oas-opt">optional</span>
                  </span>
                </td>
                <td v-html="row.typeHtml" />
                <td v-html="row.descriptionHtml" />
              </tr>
            </tbody>
          </table>
          <p v-if="op.requestBody.note" class="oas-note" v-html="op.requestBody.note" />
        </template>
      </template>

      <h2>Responses</h2>
      <table class="oas-table">
        <thead>
          <tr><th>Status</th><th>Body</th><th>Description</th></tr>
        </thead>
        <tbody>
          <template v-for="r in op.responses" :key="r.status">
            <tr>
              <td><code class="oas-status" :class="`s-${r.status[0]}`">{{ r.status }}</code></td>
              <td><span v-if="r.typeLabel" v-html="r.typeLabel" /><span v-else>—</span></td>
              <td v-html="r.descriptionHtml" />
            </tr>
            <tr v-if="r.headers.length" class="oas-headers">
              <td></td>
              <td colspan="2">
                <span class="oas-hlabel">Headers:</span>
                <span v-for="h in r.headers" :key="h.name" class="oas-header"><code>{{ h.name }}</code></span>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>

    <!-- right: code samples -->
    <aside class="oas-code">
      <div class="oas-code-sticky">
        <div class="oas-code-label">Request</div>
        <div class="oas-code-block" v-html="op.sample.curlHtml" />
        <template v-if="op.sample.responseHtml">
          <div class="oas-code-label">
            Response
            <code class="oas-status" :class="`s-${op.sample.responseStatus[0]}`">{{ op.sample.responseStatus }}</code>
          </div>
          <div class="oas-code-block" v-html="op.sample.responseHtml" />
        </template>
      </div>
    </aside>
  </div>
  <div v-else><p>Unknown operation: <code>{{ id }}</code></p></div>
</template>
