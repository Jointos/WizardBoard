import { defineConfig } from 'vite';

export default defineConfig({
  base: './',
  build: {
    outDir: '../internal/webui/dist',
    emptyOutDir: true,
    // Phaser's SVG loader cannot decode Vite's inlined data URI; emit assets
    // as real files so the portal SVG loads through the normal loader.
    assetsInlineLimit: 0,
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
});
