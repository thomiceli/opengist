<script setup lang="ts">
import { computed } from 'vue'
import { data } from '../../openapi.data'

const props = defineProps<{ name: string }>()
const schema = computed(() => data.schemas.find((s) => s.name === props.name))
</script>

<template>
  <div v-if="schema" class="oas-schema">
    <h1>{{ schema.name }}</h1>
    <div v-html="schema.descriptionHtml" />
    <table v-if="schema.rows.length" class="oas-table">
      <thead>
        <tr><th>Field</th><th>Type</th><th>Description</th></tr>
      </thead>
      <tbody>
        <tr v-for="row in schema.rows" :key="row.name">
          <td>
            <code>{{ row.name }}</code>
            <span v-if="row.required" class="oas-req">required</span>
          </td>
          <td v-html="row.typeHtml" />
          <td v-html="row.descriptionHtml" />
        </tr>
      </tbody>
    </table>
    <p v-if="schema.note" class="oas-note" v-html="schema.note" />
  </div>
  <div v-else><p>Unknown schema: <code>{{ name }}</code></p></div>
</template>
