<script setup>
import Hero from '../components/home/Hero.vue'
import Showcase from '../components/home/Showcase.vue'
import Features from '../components/home/Features.vue'
// Add further sections here as the landing page grows (e.g. Features, FAQ).
</script>

<template>
  <main class="og-home">
    <div class="og-wrapper">
      <Hero />
      <Features />
      <Showcase />
      <!-- Empty growing section so the side borders run to the bottom. -->
      <div class="og-fill" />
    </div>
  </main>
</template>

<style scoped>
.og-home {
  width: 100%;
}

/* Centered content column framed with vertical borders (vite-plus style).
   Sections inside add their own border-bottom to act as horizontal rules.
   Width matches the navbar container so the home content aligns with the header. */
.og-wrapper {
  display: flex;
  flex-direction: column;
  max-width: var(--vp-layout-max-width);
  margin: 0 auto;
  /* Fill the viewport below the navbar so the side borders run to the bottom
     even when the content is shorter than the screen. */
  min-height: calc(100vh - var(--vp-nav-height));
  /* Gap inside the frame, so the left/right borders stay visible through it. */
  padding-bottom: 2rem;
  border-inline: var(--og-frame-border) solid var(--vp-c-divider);
}

/* Empty section that absorbs the remaining height, extending the side borders. */
.og-fill {
  flex: 1;
}
</style>
