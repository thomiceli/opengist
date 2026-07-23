import { defineConfig } from 'vite'
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
    root: './public-old',
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
                './public-old/ts/admin.ts',
                './public-old/ts/auto.ts',
                './public-old/ts/dark.ts',
                './public-old/ts/editor.ts',
                './public-old/ts/embed.ts',
                './public-old/ts/gist.ts',
                './public-old/ts/light.ts',
                './public-old/ts/main.ts',
                './public-old/ts/style_preferences.ts',
                './public-old/ts/webauthn.ts',
            ]
        },
        assetsInlineLimit: 0,
    }
})
