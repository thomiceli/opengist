<script setup>
import { computed } from 'vue'
import { useData } from 'vitepress'
import Home from './layouts/Home.vue'
import SectionNav from './components/SectionNav.vue'
import DefaultTheme from 'vitepress/theme'

const { Layout } = DefaultTheme
const { frontmatter } = useData()

const isHome = computed(() => frontmatter.value.layout === 'home')
</script>

<template>
  <SectionNav v-if="!isHome" />
  <Layout>
    <template v-if="isHome" #home-hero-after>
      <Home />
    </template>
  </Layout>
</template>
