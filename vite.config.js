import { defineConfig } from 'vite'

export default defineConfig({
    root: './public',

    build: {
        // generate manifest.json in outDir
        outDir: '',
        assetsDir: 'assets',
        manifest: true,
        rollupOptions: {
            // overwrite default .html entry
            input: ['./public/main.js', './public/editor.js']
        }
    }
})