<script setup>
import { useData, withBase } from 'vitepress'

const { isDark } = useData()
</script>

<template>
  <section class="og-showcase">
    <div class="og-showcase__text">
      <h2 class="og-showcase__title">A clean, familiar interface</h2>
      <p class="og-showcase__desc">
        Create, browse and share snippets with syntax highlighting, revisions and more.
      </p>
    </div>

    <div class="og-showcase__frame">
      <img
        v-show="!isDark"
        class="og-showcase__img"
        :src="withBase('/opengist-demo.png')"
        alt="Opengist web interface"
      />
      <img
        v-show="isDark"
        class="og-showcase__img"
        :src="withBase('/opengist-demo-dark.png')"
        alt="Opengist web interface"
      />
    </div>
  </section>
</template>

<style scoped>
.og-showcase {
  padding: 3rem 1.5rem;
  text-align: center;
}
@media (max-width: 640px) {
  .og-showcase {
    padding: 2rem 1.25rem;
  }
}

.og-showcase__text {
  max-width: 34rem;
  margin: 0 auto 2rem;
}
.og-showcase__title {
  margin: 0;
  border: 0;
  font-size: 1.6rem;
  line-height: 1.2;
  font-weight: 700;
  letter-spacing: -0.01em;
}
.og-showcase__desc {
  margin: 0.5rem 0 0;
  font-size: 0.95rem;
  color: var(--vp-c-text-2);
}

/* Smaller, centered screenshot frame. */
.og-showcase__frame {
  max-width: 72rem;
  margin: 0 auto;
  border-radius: 0.75rem;
  overflow: hidden;
}
.og-showcase__img {
  display: block;
  width: 100%;
  height: auto;
  padding: 0;
}
</style>
