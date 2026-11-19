import { defineConfig } from 'vite'
import tailwindcss from "@tailwindcss/vite";
import { resolve } from 'node:path'

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
            // absolute paths, so the dev dependency scanner (which resolves
            // inputs from root) and the build (which resolves from the cwd)
            // both find them
            input: [
                'ts/main.ts',
                'ts/editor.ts',
                'ts/webauthn.ts',
                'ts/auto.ts',
                'ts/light.ts',
                'ts/dark.ts',
                'ts/embed.ts',
            ].map((file) => resolve(import.meta.dirname, file))
        },
        assetsInlineLimit: 0,
    }
})
