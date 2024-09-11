<script setup>
import { useData } from 'vitepress'
import Home from './Home.vue'
import DefaultTheme from 'vitepress/theme'

const { Layout } = DefaultTheme
const { frontmatter } = useData()
</script>

<template>
  <Layout>
    <template v-if="frontmatter.layout === 'home'" #home-hero-after>
      <Home />
    </template>
  </Layout>
</template>