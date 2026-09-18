import { fileURLToPath, URL } from 'node:url';

import vue from '@vitejs/plugin-vue';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5173,
    strictPort: true,
    // 开发期把 /api 与探针路径代理到 FastAPI，让浏览器视角下前后端同源：
    // 会话 Cookie 因此天然可用，无需处理跨站 Cookie 的诸多限制。
    proxy: {
      '/api': { target: 'http://127.0.0.1:8000', changeOrigin: false },
      '/healthz': { target: 'http://127.0.0.1:8000', changeOrigin: false },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: false,
    chunkSizeWarningLimit: 1500,
    rollupOptions: {
      output: {
        // 把体积大且更新频率低的依赖拆出来，利用浏览器缓存
        manualChunks(id: string) {
          if (id.includes('node_modules/element-plus') || id.includes('@element-plus/icons-vue')) {
            return 'element-plus';
          }
          if (id.includes('node_modules/leaflet')) return 'leaflet';
          return undefined;
        },
      },
    },
  },
});
