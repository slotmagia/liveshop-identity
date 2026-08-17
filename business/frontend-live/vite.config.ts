import { defineConfig } from 'vite'

export default defineConfig({
  server: { host: '127.0.0.1', port: 15193, strictPort: true },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    sourcemap: true,
    lib: { entry: 'src/main.ts', formats: ['es'], fileName: () => 'identity-live.js' },
  },
})
