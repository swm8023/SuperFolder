import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: '../.build/embedweb/app',
    emptyOutDir: true,
  },
  test: {
    environment: 'node',
  },
});
