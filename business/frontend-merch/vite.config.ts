import { defineConfig } from 'vite'

// The merch Host embeds this contribution in an iframe, so the build is a
// normal application and `entry` in module.json points at where it is served.
export default defineConfig({
  base: './',
  server: { host: '127.0.0.1', port: 5202, strictPort: true },
  build: { outDir: 'dist', emptyOutDir: true, sourcemap: true },
})
