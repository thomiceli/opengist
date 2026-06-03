<script setup>
const features = [
  {
    title: 'Public, unlisted & private',
    desc: 'Set the visibility of every snippet - public, unlisted, or fully private.',
  },
  {
    title: 'Powered by Git',
    desc: 'Each snippet is a Git repository. Clone and push over HTTP or SSH, or create one with a plain git push.',
  },
  {
    title: 'Git CLI & API',
    desc: 'Manage your snippets straight from the git command line, or via the API.',
  },
  {
    title: 'Revisions & diffs',
    desc: 'Browse the full history of a snippet with diffs between every revision.',
  },
  {
    title: 'Rich rendering',
    desc: 'Syntax highlighting for hundreds of languages, plus Markdown with Mermaid, LaTeX, CSV and media files.',
  },
  {
    title: 'Files & uploads',
    desc: 'Multiple files per snippet, including binary files and images, with a download as ZIP.',
  },
  {
    title: 'Likes, forks & topics',
    desc: 'Star and fork snippets, and organize them with topics for easy discovery.',
  },
  {
    title: 'Embed & JSON',
    desc: 'Drop a snippet into any webpage with the embed widget, or fetch it as JSON.',
  },
  {
    title: 'Full-text & code search',
    desc: 'Search across gists via code or metadata.',
  },
  {
    title: 'OAuth, LDAP & passkeys',
    desc: 'Log in with GitHub, GitLab, Gitea or OIDC, LDAP, and secure accounts with TOTP or WebAuthn.',
  },
  {
    title: 'Admin & instance controls',
    desc: 'Require login for a private instance, allow anonymous gists, manage users, and expose Prometheus metrics.',
  },
  {
    title: 'Self-hosted your way',
    desc: 'A single binary, Docker image or K8s deployment, backed by SQLite, PostgreSQL or MySQL.',
  },
]
</script>

<template>
  <section class="og-features">
    <div class="og-features__head">
      <h2 class="og-features__title">Features</h2>
      <p class="og-features__desc">
        All you need to self-host, manage and share your snippets.
      </p>
    </div>

    <ul class="og-features__grid">
      <li v-for="f in features" :key="f.title" class="og-feature">
        <h3 class="og-feature__title">{{ f.title }}</h3>
        <p class="og-feature__desc">{{ f.desc }}</p>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.og-features {
  border-bottom: var(--og-frame-border) solid var(--vp-c-divider);
}

.og-features__head {
  padding: 3rem 1.5rem 2.5rem;
  text-align: center;
}
.og-features__title {
  margin: 0;
  border: 0;
  font-size: 1.8rem;
  line-height: 1.2;
  font-weight: 700;
  letter-spacing: -0.01em;
}
.og-features__desc {
  margin: 0.6rem 0 0;
  font-size: 1rem;
  color: var(--vp-c-text-2);
}

/* Flush grid: cells separated by the same frame border (no radius, no margin). */
.og-features__grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: var(--og-frame-border);
  margin: 0;
  padding: 0;
  list-style: none;
  background: var(--vp-c-divider);
  border-top: var(--og-frame-border) solid var(--vp-c-divider);
}

.og-feature {
  padding: 1.75rem;
  background: var(--vp-c-bg);
}
.og-feature__title {
  margin: 0;
  border: 0;
  font-size: 1.05rem;
  font-weight: 600;
  letter-spacing: -0.01em;
}
.og-feature__desc {
  margin: 0.4rem 0 0;
  font-size: 0.9rem;
  line-height: 1.55;
  color: var(--vp-c-text-2);
}

@media (max-width: 900px) {
  .og-features__grid {
    grid-template-columns: repeat(2, 1fr);
  }
}
@media (max-width: 600px) {
  .og-features__grid {
    grid-template-columns: 1fr;
  }
}
</style>
