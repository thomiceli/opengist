import { h } from 'vue'
import type { Theme } from 'vitepress'
import DefaultTheme from 'vitepress/theme'
import Layout from "./Layout.vue";

export default {
  ...DefaultTheme,
  Layout,
  enhanceApp({ app, router, siteData }) {
    // ...
  }
} satisfies Theme
