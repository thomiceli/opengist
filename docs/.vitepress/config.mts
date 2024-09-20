import {defineConfig} from 'vitepress'

// https://vitepress.dev/reference/site-config
export default defineConfig({
    title: "Opengist",
    description: "Documention for Opengist",
    rewrites: {
        'index.md': 'index.md',
        'introduction.md': 'docs/index.md',
        ':path(.*)': 'docs/:path'
    },
    themeConfig: {
        // https://vitepress.dev/reference/default-theme-config
        logo: 'https://raw.githubusercontent.com/thomiceli/opengist/master/public/opengist.svg',
        logoLink: '/',
        nav: [
            { text: 'Demo', link: 'https://demo.opengist.io' },
            { text: 'Translate', link: 'https://tr.opengist.io' }
        ],

        sidebar: {
            '/docs/': [
            {
                text: '', items: [
                    {text: 'Introduction', link: '/docs'},
                    {text: 'Installation', link: '/docs/installation', items: [
                        {text: 'Docker', link: '/docs/installation/docker'},
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
                    {text: 'Admin panel', link: '/admin-panel'},
                ], collapsed: false
            },
            {
                text: 'Usage', base: '/docs/usage', items: [
                    {text: 'Init via Git', link: '/init-via-git'},
                    {text: 'Embed Gist', link: '/embed'},
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

        ]},

        socialLinks: [
            {icon: 'github', link: 'https://github.com/thomiceli/opengist'}
        ],
        editLink: {
            pattern: 'https://github.com/thomiceli/opengist/edit/stable/docs/:path'
        },
        // @ts-ignore
        lastUpdated: true,

    },
    head: [
        ['link', {rel: 'icon', href: '/favicon.svg'}],
    ],
    ignoreDeadLinks: true
})
