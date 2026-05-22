import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

const serviceTarget = 'http://127.0.0.1:18080';

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: '../.build/embedweb/app',
    emptyOutDir: true,
  },
  server: {
    host: '127.0.0.1',
    port: 5173,
    strictPort: true,
    proxy: {
      '/healthz': {
        target: serviceTarget,
        changeOrigin: true,
      },
      '/boot': {
        target: serviceTarget,
        changeOrigin: true,
      },
      '/ws': {
        target: serviceTarget,
        ws: true,
        changeOrigin: true,
      },
    },
  },
  test: {
    environment: 'node',
  },
});
