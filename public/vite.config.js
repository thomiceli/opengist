import { defineConfig } from 'vite'

export default defineConfig({
    root: './public',

    build: {
        // generate manifest.json in outDir
        outDir: '',
        assetsDir: 'assets',
        manifest: true,
        rollupOptions: {
            input: [
                './public/main.ts',
                './public/editor.ts',
                './public/admin.ts',
                './public/gist.ts',
                './public/embed.ts',
                './public/webauthn.ts'
            ]
        },
        assetsInlineLimit: 0,
    }
})
