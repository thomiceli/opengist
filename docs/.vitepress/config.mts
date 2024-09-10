import {defineConfig} from 'vitepress'

// https://vitepress.dev/reference/site-config
export default defineConfig({
    title: "Opengist Docs",
    description: "Documention for Opengist",
    base: '/docs/master/',
    themeConfig: {
        // https://vitepress.dev/reference/default-theme-config
        logo: 'https://raw.githubusercontent.com/thomiceli/opengist/master/public/opengist.svg',
        nav: [
            { text: 'Demo', link: 'https://demo.opengist.io' },
            { text: 'Translate', link: 'https://tr.opengist.io' }
        ],

        sidebar: [
            {
                text: '', items: [
                    {text: 'Introduction', link: '/'},
                    {text: 'Installation', link: '/installation', items: [
                        {text: 'Docker', link: '/installation/docker'},
                        {text: 'Binary', link: '/installation/binary'},
                        {text: 'Source', link: '/installation/source'},
                        ],
                        collapsed: true
                    },
                    {text: 'Update', link: '/update'},
                ], collapsed: false
            },
            {
                text: 'Configuration', base: '/configuration', items: [
                    {text: 'Configure Opengist', link: '/index'},
                    {text: 'OAuth Providers', link: '/oauth-providers'},
                    {text: 'Custom assets', link: '/custom-assets'},
                    {text: 'Custom links', link: '/custom-links'},
                    {text: 'Cheat Sheet', link: '/cheat-sheet'},
                ], collapsed: false
            },
            {
                text: 'Usage', base: '/usage', items: [
                    {text: 'Init via Git', link: '/init-via-git'},
                    {text: 'Embed Gist', link: '/embed'},
                    {text: 'Gist as JSON', link: '/gist-json'},
                    {text: 'Import Gists from Github', link: '/import-from-github-gist'},
                    {text: 'Git push options', link: '/git-push-options'},
                ], collapsed: false
            },
            {
                text: 'Administration', base: '/administration', items: [
                    {text: 'Run with systemd', link: '/run-with-systemd'},
                    {text: 'Reverse proxy', items: [
                        {text: 'Nginx', link: '/nginx-reverse-proxy'},
                    ], collapsed: true},
                    {text: 'Fail2ban', link: '/fail2ban-setup'},
                    {text: 'Healthcheck', link: '/healthcheck'},
                ], collapsed: false
            },
            {
                text: 'Contributing', base: '/contributing', items: [
                    {text: 'Community', link: '/community'},
                    {text: 'Development', link: '/development'},
                ], collapsed: false
            },

        ],

        socialLinks: [
            {icon: 'github', link: 'https://github.com/thomiceli/opengist'}
        ],
        editLink: {
            pattern: 'https://github.com/thomiceli/opengist/edit/master/docs/:path'
        },
        // @ts-ignore
        lastUpdated: true,
    },
    ignoreDeadLinks: true
})
