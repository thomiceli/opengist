import { defineConfig } from 'vite'
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
    root: './public',
    plugins: [
        tailwindcss(),
    ],
    server: {
        cors: {
            origin: 'http://localhost:6157',
        },
    },
    build: {
        // generate manifest.json in outDir
        outDir: '',
        assetsDir: 'assets',
        manifest: true,
        rollupOptions: {
            input: [
                './public/ts/admin.ts',
                './public/ts/auto.ts',
                './public/ts/dark.ts',
                './public/ts/editor.ts',
                './public/ts/embed.ts',
                './public/ts/gist.ts',
                './public/ts/light.ts',
                './public/ts/main.ts',
                './public/ts/style_preferences.ts',
                './public/ts/webauthn.ts',
            ]
        },
        assetsInlineLimit: 0,
    }
})
