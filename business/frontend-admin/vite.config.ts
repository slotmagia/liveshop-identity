import { defineConfig } from 'vite'

// The admin Host embeds this contribution in an iframe, so the build is a
// normal application and `entry` in module.json points at where it is served.
export default defineConfig({
  base: './',
  server: { host: '127.0.0.1', port: 15201, strictPort: true },
  build: { outDir: 'dist', emptyOutDir: true, sourcemap: true },
})
