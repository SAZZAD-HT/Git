import { defineConfig } from 'vitest/config';
import type { Plugin } from 'vite';
import react from '@vitejs/plugin-react';
import { rmSync } from 'node:fs';

/**
 * Removes the previous build's files but keeps the committed dist/placeholder.html,
 * which the Go server shows when the app has not been built (see web/embed.go).
 */
function cleanDistKeepPlaceholder(): Plugin {
  return {
    name: 'anvil-clean-dist',
    apply: 'build',
    buildStart() {
      rmSync('dist/assets', { recursive: true, force: true });
      rmSync('dist/index.html', { force: true });
    },
  };
}

// In development, Vite serves the app and proxies API and Git routes to a local `anvil serve`.
export default defineConfig({
  plugins: [react(), cleanDistKeepPlaceholder()],
  build: { outDir: 'dist', emptyOutDir: false, sourcemap: false },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://127.0.0.1:3000',
      '/healthz': 'http://127.0.0.1:3000',
      '/readyz': 'http://127.0.0.1:3000',
    },
  },
  test: { environment: 'node' },
});
