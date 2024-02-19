import {defineConfig} from 'vitepress'

// https://vitepress.dev/reference/site-config
export default defineConfig({
    title: "Opengist Docs",
    description: "Documention for Opengist",
    themeConfig: {
        // https://vitepress.dev/reference/default-theme-config
        logo: 'https://raw.githubusercontent.com/thomiceli/opengist/master/public/opengist.svg',
        nav: [
            { text: 'Demo', link: 'https://demo.opengist.io' }
        ],

        sidebar: [
            {
                text: '', items: [
                    {text: 'Getting Started', link: '/'},
                    {text: 'Installation', link: '/installation'},
                    {text: 'Update', link: '/update'},
                ]
            },
            {
                text: 'Usage', base: '/usage', items: [
                    {text: 'Init via Git', link: '/init-via-git'},
                    {text: 'Embed', link: '/embed'},
                    {text: 'Gist as JSON', link: '/gist-json'},
                    {text: 'Import Gists from Github', link: '/import-from-github-gist'},
                ]
            },
            {
                text: 'Configuration', base: '/configuration', items: [
                    {text: 'Index', link: '/index'},
                    {text: 'Cheat Sheet', link: '/cheat-sheet'},
                ]
            },
            {
                text: 'Administration', base: '/administration', items: [
                    {text: 'Oauth Provides', link: '/oauth-providers'},
                    {text: 'Run with systemd', link: '/run-with-systemd'},
                    {text: 'Reverse proxy', items: [
                        {text: 'Nginx', link: '/nginx-reverse-proxy'},
                    ]},
                    {text: 'Fail2ban', link: '/fail2ban-setup'},
                    {text: 'Healthcheck', link: '/healthcheck'},
                ]
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
