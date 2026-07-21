<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'

const version = 'v1.14'
const dockerCommand = 'docker run -p 6157:6157 -v "$HOME/.opengist:/opengist" ghcr.io/thomiceli/opengist:1.14'

const stars = ref(null)
const formattedStars = computed(() => {
  const n = stars.value
  if (n == null) return ''
  // if (n >= 1000) return (n / 1000).toFixed(1).replace(/\.0$/, '') + 'k'
  return String(n)
})

// Cache the star count in localStorage so we don't hit the GitHub API on every
// page load / navigation (unauthenticated API is rate-limited to 60/hour/IP).
const STARS_KEY = 'og-gh-stars'
const STARS_TTL = 6 * 60 * 60 * 1000 // 6 hours

onMounted(async () => {
  try {
    const cached = JSON.parse(localStorage.getItem(STARS_KEY) || 'null')
    if (cached && Date.now() - cached.ts < STARS_TTL) {
      stars.value = cached.value
      return
    }
  } catch {}

  try {
    const res = await fetch('https://api.github.com/repos/thomiceli/opengist')
    if (res.ok) {
      stars.value = (await res.json()).stargazers_count
      try {
        localStorage.setItem(STARS_KEY, JSON.stringify({ value: stars.value, ts: Date.now() }))
      } catch {}
    }
  } catch {}
})

const copied = ref(false)
let timer

const copy = async () => {
  try {
    await navigator.clipboard.writeText(dockerCommand)
    copied.value = true
    clearTimeout(timer)
    timer = setTimeout(() => (copied.value = false), 1600)
  } catch {}
}

onBeforeUnmount(() => clearTimeout(timer))
</script>

<template>
  <section class="og-hero">
    <a
      class="og-badge"
      href="https://github.com/thomiceli/opengist/releases"
      target="_blank"
      rel="noopener noreferrer"
    >
      <span class="og-badge__dot" />
      {{ version }} released: Restful API + brand new docs page
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="og-badge__arrow">
        <path stroke-linecap="round" stroke-linejoin="round" d="m9 6 6 6-6 6" />
      </svg>
    </a>

    <img
      class="og-logo"
      src="https://raw.githubusercontent.com/thomiceli/opengist/master/public/img/opengist.svg"
      alt="Opengist"
    />

    <h1 class="og-title">
      <span class="og-title__shine">Opengist</span>
    </h1>

    <p class="og-tagline">
      Self-hosted and open-source pastebin powered by Git
    </p>

    <div class="og-actions">
      <a class="og-btn og-btn--primary" href="/docs">Get started</a>
      <a class="og-btn" href="https://demo.opengist.io" target="_blank" rel="noopener noreferrer">
        Live demo
      </a>
      <a
        class="og-btn og-btn--github"
        href="https://github.com/thomiceli/opengist"
        target="_blank"
        rel="noopener noreferrer"
        aria-label="Opengist on GitHub"
      >
        <svg class="og-btn__gh" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
          <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27s1.36.09 2 .27c1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8" />
        </svg>
        <span v-if="formattedStars" class="og-btn__stars">
          <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
            <path d="m12 17.27 6.18 3.73-1.64-7.03L22 9.24l-7.19-.61L12 2 9.19 8.63 2 9.24l5.46 4.73L5.82 21z" />
          </svg>
          {{ formattedStars }}
        </span>
      </a>
    </div>

    <div class="og-install">
      <span class="og-install__prompt">$</span>
      <code class="og-install__cmd">{{ dockerCommand }}</code>
      <button class="og-install__copy" type="button" aria-label="Copy command" @click="copy">
        <svg v-if="copied" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="m5 13 4 4L19 7" />
        </svg>
        <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <rect x="9" y="9" width="11" height="11" rx="2" />
          <path d="M5 15V5a2 2 0 0 1 2-2h10" />
        </svg>
      </button>
    </div>
  </section>
</template>

<style scoped>
.og-hero {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  padding: 2.5rem 1.5rem 2rem;
  border-bottom: 1px solid var(--vp-c-divider);
}

.og-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.35rem 0.9rem;
  border-radius: 999px;
  font-size: 0.825rem;
  font-weight: 500;
  color: var(--vp-c-brand-1);
  background: var(--vp-c-brand-soft);
  transition: filter 0.2s ease;
}
.og-badge:hover {
  filter: brightness(1.05);
}
.og-badge__dot {
  width: 0.45rem;
  height: 0.45rem;
  border-radius: 999px;
  background: var(--vp-c-brand-1);
}
.og-badge__arrow {
  width: 0.85rem;
  height: 0.85rem;
}

.og-logo {
  width: 4.5rem;
  height: 4.5rem;
  margin: 2rem 0 1.25rem;
}

.og-title {
  margin: 0;
  font-size: clamp(1.25rem, 6vw, 2.75rem);
  line-height: 1.18;
  font-weight: 800;
  letter-spacing: -0.02em;
}
.og-title__shine {
  display: block;
  background: linear-gradient(
    110deg,
    var(--vp-c-text-1) 0%,
    var(--vp-c-text-1) 40%,
    var(--vp-c-brand-1) 50%,
    var(--vp-c-text-1) 60%,
    var(--vp-c-text-1) 100%
  );
  background-size: 250% 100%;
  background-position: 100% 0;
  background-clip: text;
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  animation: og-shine 6s ease-in-out 0.3s 1 forwards;
}
@keyframes og-shine {
  to {
    background-position: 0 0;
  }
}

.og-tagline {
  max-width: 34rem;
  margin: 1.2rem 0 0;
  font-size: 1.35rem;
  line-height: 1;
  color: var(--vp-c-text-1);
}
.og-license {
  margin: 0.85rem 0 0;
  font-size: 0.85rem;
  color: var(--vp-c-text-3);
}

.og-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: center;
  gap: 0.75rem;
  margin-top: 2rem;
}
.og-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 2.875rem;
  padding: 0 1.5rem;
  border-radius: 0.625rem;
  border: 1px solid var(--vp-c-divider);
  background: var(--vp-c-bg-soft);
  color: var(--vp-c-text-1);
  font-size: 0.95rem;
  font-weight: 600;
  transition: border-color 0.2s ease, background-color 0.2s ease, color 0.2s ease;
}
.og-btn:hover {
  border-color: var(--vp-c-brand-1);
  color: var(--vp-c-brand-1);
}
.og-btn--primary {
  border-color: transparent;
  background: var(--vp-c-brand-3);
  color: #fff;
}
.og-btn--primary:hover {
  background: var(--vp-c-brand-2);
  color: #fff;
}
.og-btn--icon {
  width: 2.875rem;
  padding: 0;
}
.og-btn--icon svg {
  width: 1.35rem;
  height: 1.35rem;
}

/* GitHub button with star count. */
.og-btn--github {
  gap: 0.6rem;
  padding: 0 1.1rem;
}
.og-btn__gh {
  width: 1.35rem;
  height: 1.35rem;
}
.og-btn__stars {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  padding-left: 0.6rem;
  border-left: 1px solid var(--vp-c-divider);
  font-variant-numeric: tabular-nums;
}
.og-btn__stars svg {
  width: 0.95rem;
  height: 0.95rem;
  color: #e3b341;
}

.og-install {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  max-width: 100%;
  margin-top: 2rem;
  padding: 0.6rem 0.6rem 0.6rem 1rem;
  border-radius: 0.75rem;
  border: 1px solid var(--vp-c-divider);
  background: var(--vp-c-bg-alt);
  font-family: var(--vp-font-family-mono);
}
.og-install__prompt {
  color: var(--vp-c-brand-1);
  user-select: none;
}
.og-install__cmd {
  flex: 1;
  min-width: 0; /* allow the long command to shrink + scroll instead of overflowing */
  overflow-x: auto;
  white-space: nowrap;
  font-size: 0.85rem;
  color: var(--vp-c-text-1);
  background: transparent;
  padding: 0;
}
.og-install__copy {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2.1rem;
  height: 2.1rem;
  border-radius: 0.5rem;
  color: var(--vp-c-text-2);
  background: var(--vp-c-bg);
  border: 1px solid var(--vp-c-divider);
  transition: color 0.2s ease, border-color 0.2s ease;
}
.og-install__copy:hover {
  color: var(--vp-c-brand-1);
  border-color: var(--vp-c-brand-1);
}
.og-install__copy svg {
  width: 1.05rem;
  height: 1.05rem;
}

@media (max-width: 640px) {
  .og-hero {
    padding: 2.5rem 1.25rem 2.5rem;
  }
  .og-install {
    width: 100%;
  }
}

@media (min-width: 640px) {
  .og-hero {
    padding-top: 3.5rem;
  }
}
</style>
