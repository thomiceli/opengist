import { defineConfig } from 'vite'
import tailwindcss from "@tailwindcss/vite";
import { cpSync, mkdirSync, rmSync } from 'node:fs';
import { dirname, resolve } from 'node:path';

const ROOT_DIR = resolve(process.cwd(), 'public');
const TEMP_OUT_DIR = '.vite-build';

const copyBuildOutputsPlugin = {
    name: 'opengist-copy-build-outputs',
    apply: 'build',
    closeBundle() {
        const tempOutRoot = resolve(ROOT_DIR, TEMP_OUT_DIR);
        const tempAssetsDir = resolve(tempOutRoot, 'assets');
        const tempManifest = resolve(tempOutRoot, '.vite', 'manifest.json');

        const finalAssetsDir = resolve(ROOT_DIR, 'assets');
        const finalManifest = resolve(ROOT_DIR, '.vite', 'manifest.json');

        rmSync(finalAssetsDir, { recursive: true, force: true });
        cpSync(tempAssetsDir, finalAssetsDir, { recursive: true });

        mkdirSync(dirname(finalManifest), { recursive: true });
        cpSync(tempManifest, finalManifest);

        rmSync(tempOutRoot, { recursive: true, force: true });
    },
};

export default defineConfig({
    root: './public',
    plugins: [
        tailwindcss(),
        copyBuildOutputsPlugin,
    ],
    server: {
        cors: {
            origin: 'http://localhost:6157',
        },
    },
    build: {
        // Vite 7 forbids writing directly to root, so build in a temp folder
        // and copy outputs back to the historical locations after build.
        outDir: TEMP_OUT_DIR,
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
