import { h } from 'vue'
import type { Theme } from 'vitepress'
import DefaultTheme from 'vitepress/theme'
import Layout from "./Layout.vue";
import OpenApiOperation from "./components/OpenApiOperation.vue";
import OpenApiSchema from "./components/OpenApiSchema.vue";
import './style.css'
import './openapi.css'

export default {
  ...DefaultTheme,
  Layout,
  enhanceApp({ app, router, siteData }) {
    app.component('OpenApiOperation', OpenApiOperation)
    app.component('OpenApiSchema', OpenApiSchema)
  }
} satisfies Theme
