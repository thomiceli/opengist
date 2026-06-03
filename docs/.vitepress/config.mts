import {defineConfig} from 'vitepress'
import tailwindcss from '@tailwindcss/vite'
import {listOperations, listSchemas, readSpecRaw} from './openapi'

// Serve the raw OpenAPI spec verbatim at /docs/api/openapi.yaml — read from the
// Go source tree so it never drifts. Dev via middleware, build via asset emit.
let isSsrBuild = false
const openapiRawPlugin = {
    name: 'opengist:openapi-raw',
    configResolved(c: any) {
        isSsrBuild = !!c.build?.ssr
    },
    configureServer(server: any) {
        server.middlewares.use((req: any, res: any, next: any) => {
            if (req.url === '/docs/api/openapi.yaml') {
                res.setHeader('Content-Type', 'text/yaml; charset=utf-8')
                res.end(readSpecRaw())
            } else next()
        })
    },
    generateBundle(this: any) {
        if (isSsrBuild) return
        this.emitFile({type: 'asset', fileName: 'docs/api/openapi.yaml', source: readSpecRaw()})
    }
}

// Build the API Reference sidebar from the OpenAPI spec: one entry per
// operation (grouped by tag), then one entry per schema, plus the Overview.
const apiOps = listOperations()
const apiTags = [...new Set(apiOps.map(op => op.tag))]
const apiSidebar = [
    {text: 'Overview', link: '/docs/api'},
    ...apiTags.map(tag => ({
        text: tag,
        collapsed: false,
        items: apiOps
            .filter(op => op.tag === tag)
            .map(op => ({text: op.summary, link: `/docs/api/${op.id}`})),
    })),
    {
        text: 'Schemas',
        collapsed: true,
        items: listSchemas().map(name => ({text: name, link: `/docs/api/schemas/${name}`})),
    },
]

// Main docs sidebar, shared by the /docs/ pages and the standalone /changelog page.
const docsSidebar = [
    {
        text: '', items: [
            {text: 'Introduction', link: '/docs'},
            {text: 'Installation', link: '/docs/installation', items: [
                {text: 'Docker', link: '/docs/installation/docker'},
                {text: 'Kubernetes', link: '/docs/installation/kubernetes'},
                {text: 'Binary', link: '/docs/installation/binary'},
                {text: 'Source', link: '/docs/installation/source'},
                ],
                collapsed: true
            },
            {text: 'Update', link: '/docs/update'},
        ], collapsed: false
    },
    {
        text: 'Configuration', base: '/docs/configuration', items: [
            {text: 'Configure Opengist', link: '/configure'},
            {text: 'Databases', items: [
                {text: 'SQLite', link: '/databases/sqlite'},
                {text: 'PostgreSQL', link: '/databases/postgresql'},
                {text: 'MySQL', link: '/databases/mysql'},
                ], collapsed: true
            },
            {text: 'OAuth Providers', link: '/oauth-providers'},
            {text: 'Custom assets', link: '/custom-assets'},
            {text: 'Custom links', link: '/custom-links'},
            {text: 'Cheat Sheet', link: '/cheat-sheet'},
            {text: 'Metrics', link: '/metrics'},
            {text: 'Admin panel', link: '/admin-panel'},
        ], collapsed: false
    },
    {
        text: 'Usage', base: '/docs/usage', items: [
            {text: 'Init via Git', link: '/init-via-git'},
            {text: 'Embed Gist', link: '/embed'},
            {text: 'Access Tokens', link: '/access-tokens'},
            {text: 'Gist as JSON', link: '/gist-json'},
            {text: 'Import Gists from Github', link: '/import-from-github-gist'},
            {text: 'Git push options', link: '/git-push-options'},
        ], collapsed: false
    },
    {
        text: 'Administration', base: '/docs/administration', items: [
            {text: 'Run with systemd', link: '/run-with-systemd'},
            {text: 'Reverse proxy', items: [
                {text: 'Nginx', link: '/nginx-reverse-proxy'},
                {text: 'Traefik', link: '/traefik-reverse-proxy'},
            ], collapsed: true},
            {text: 'Fail2ban', link: '/fail2ban-setup'},
            {text: 'Healthcheck', link: '/healthcheck'},
        ], collapsed: false
    },
    {
        text: 'Contributing', base: '/docs/contributing', items: [
            {text: 'Community', link: '/community'},
            {text: 'Development', link: '/development'},
        ], collapsed: false
    },
]

// https://vitepress.dev/reference/site-config
const hostname = 'https://opengist.io'
const ogImage = `${hostname}/opengist-demo.png`

export default defineConfig({
    title: "Opengist",
    description: "Documentation for Opengist — a self-hosted pastebin powered by Git.",
    lang: 'en-US',
    sitemap: {
        hostname,
    },
    markdown: {
        config(md) {
            // Strip the "See here how to update Opengist." note that appears
            // under each version in the embedded CHANGELOG (source stays intact).
            md.core.ruler.push('strip_changelog_update_note', (state) => {
                const t = state.tokens
                for (let i = t.length - 1; i >= 0; i--) {
                    if (
                        t[i].type === 'inline' &&
                        /See here how to .*update.* Opengist\./i.test(t[i].content) &&
                        t[i - 1]?.type === 'paragraph_open'
                    ) {
                        t.splice(i - 1, 3)
                    }
                }
            })

            // Turn "#123" references into links to the matching GitHub PR/issue.
            md.inline.ruler.before('text', 'github_ref', (state, silent) => {
                const start = state.pos
                if (state.src.charCodeAt(start) !== 0x23 /* # */) return false
                // Require a boundary before '#' (avoid URL fragments like page#1).
                const prev = start > 0 ? state.src[start - 1] : ''
                if (prev && /[0-9A-Za-z]/.test(prev)) return false

                let pos = start + 1
                while (pos < state.posMax && /[0-9]/.test(state.src[pos])) pos++
                if (pos === start + 1) return false // no digits
                // Reject things like a hex color "#1a2" (digit run followed by a letter).
                if (pos < state.posMax && /[A-Za-z]/.test(state.src[pos])) return false

                const num = state.src.slice(start + 1, pos)
                if (!silent) {
                    const open = state.push('link_open', 'a', 1)
                    open.attrs = [
                        ['href', `https://github.com/thomiceli/opengist/pull/${num}`],
                        ['target', '_blank'],
                        ['rel', 'noreferrer'],
                    ]
                    state.push('text', '', 0).content = `#${num}`
                    state.push('link_close', 'a', -1)
                }
                state.pos = pos
                return true
            })
        },
    },
    vite: {
        plugins: [tailwindcss(), openapiRawPlugin]
    },
    rewrites: {
        'index.md': 'index.md',
        'introduction.md': 'docs/index.md',
        'changelog.md': 'changelog.md',
        ':path(.*)': 'docs/:path'
    },
    themeConfig: {
        // https://vitepress.dev/reference/default-theme-config
        logo: 'https://raw.githubusercontent.com/thomiceli/opengist/master/public/img/opengist.svg',
        logoLink: '/',
        nav: [
            { text: 'Docs', link: '/docs', activeMatch: '^/docs' },
            {
                text: 'Resources',
                items: [
                    { text: 'Demo', link: 'https://demo.opengist.io' },
                    { text: 'Translate', link: 'https://tr.opengist.io' },
                ]
            },
            {
                text: 'v1.13.0',
                items: [
                    { text: 'Changelog', link: '/changelog' },
                    { text: 'Releases', link: 'https://github.com/thomiceli/opengist/releases' },
                ]
            }
        ],

        sidebar: {
            // Standalone API Reference section: its own sidebar, separate from
            // the main docs navigation. Longest-prefix match means /docs/api
            // uses this instead of the '/docs/' sidebar below.
            '/docs/api': apiSidebar,
            '/docs/': docsSidebar,
            // Standalone /changelog page reuses the main docs sidebar.
            '/changelog': docsSidebar,
        },

        socialLinks: [
            {icon: 'github', link: 'https://github.com/thomiceli/opengist'},
            {icon: 'discord', link: 'https://discord.gg/9Pm3X5scZT'}
        ],
        editLink: {
            pattern: 'https://github.com/thomiceli/opengist/edit/v.1.12.2/docs/:path'
        },
        // @ts-ignore
        lastUpdated: true,

    },
    head: [
        ['link', {rel: 'icon', href: '/favicon.svg'}],
        ['meta', {name: 'theme-color', content: '#3c79e2'}],
        // Site-wide Open Graph / Twitter Card defaults (per-page values are
        // refined in transformPageData below).
        ['meta', {property: 'og:type', content: 'website'}],
        ['meta', {property: 'og:site_name', content: 'Opengist'}],
        ['meta', {property: 'og:image', content: ogImage}],
        ['meta', {name: 'twitter:card', content: 'summary_large_image'}],
        ['meta', {name: 'twitter:image', content: ogImage}],
    ],
    // Per-page meta: canonical URL, description, and Open Graph / Twitter tags
    // built from each page's title + description.
    transformPageData(pageData) {
        // Mirror the `rewrites` above to compute the deployed path.
        let out
        if (pageData.relativePath === 'index.md') out = 'index.md'
        else if (pageData.relativePath === 'introduction.md') out = 'docs/index.md'
        else if (pageData.relativePath === 'changelog.md') out = 'changelog.md'
        else out = `docs/${pageData.relativePath}`
        const path = out.replace(/(^|\/)index\.md$/, '$1').replace(/\.md$/, '.html')
        const url = `${hostname}/${path}`

        const base = pageData.title || 'Opengist'
        const title = base.includes('Opengist') ? base : `${base} | Opengist`
        const description =
            pageData.description ||
            pageData.frontmatter.description ||
            'Documentation for Opengist — a self-hosted pastebin powered by Git.'

        pageData.frontmatter.head ??= []
        pageData.frontmatter.head.push(
            ['link', {rel: 'canonical', href: url}],
            ['meta', {property: 'og:title', content: title}],
            ['meta', {property: 'og:description', content: description}],
            ['meta', {property: 'og:url', content: url}],
            ['meta', {name: 'twitter:title', content: title}],
            ['meta', {name: 'twitter:description', content: description}],
        )
    },
    ignoreDeadLinks: true
})
