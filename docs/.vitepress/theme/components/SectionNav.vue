<script setup>
import { computed } from 'vue'
import { useRoute, withBase } from 'vitepress'

const route = useRoute()
const isApi = computed(() => route.path.startsWith(withBase('/docs/api')))

const items = computed(() => [
  { text: 'Guide', link: '/docs', active: !isApi.value },
  { text: 'API', link: '/docs/api', active: isApi.value },
])
</script>

<template>
  <div class="og-sectionbar">
    <div class="og-sectionbar__inner">
      <a
        v-for="it in items"
        :key="it.link"
        :href="withBase(it.link)"
        class="og-sectionbar__link"
        :class="{ 'is-active': it.active }"
      >
        {{ it.text }}
      </a>
    </div>
  </div>
</template>

<style scoped>
.og-sectionbar {
  display: none;
}

@media (min-width: 960px) {
  .og-sectionbar {
    position: fixed;
    top: var(--vp-nav-height);
    left: 0;
    right: 0;
    z-index: 28;
    display: block;
    height: var(--og-bar-height);
    background-color: var(--vp-nav-bg-color);
  }
}

/* Same container + frame borders as the top navbar so the bar lines up: full
   --vp-layout-max-width centered, 32px inner padding, and 2px vertical + bottom
   borders matching the rest of the frame. */
.og-sectionbar__inner {
  display: flex;
  align-items: center;
  gap: 1.5rem;
  height: 100%;
  max-width: var(--vp-layout-max-width);
  margin: 0 auto;
  padding-inline: 32px;
  border-inline: var(--og-frame-border) solid var(--vp-c-divider);
  border-bottom: var(--og-frame-border) solid var(--vp-c-divider);
}

.og-sectionbar__link {
  position: relative;
  display: inline-flex;
  align-items: center;
  height: 100%;
  font-size: 14px;
  font-weight: 600;
  color: var(--vp-c-text-2);
  transition: color 0.2s ease;
}
.og-sectionbar__link:hover {
  color: var(--vp-c-text-1);
}
.og-sectionbar__link.is-active {
  color: var(--vp-c-brand-1);
}
.og-sectionbar__link.is-active::after {
  content: "";
  position: absolute;
  left: 0;
  right: 0;
  bottom: -1px;
  height: 2px;
  background: var(--vp-c-brand-1);
}
</style>
