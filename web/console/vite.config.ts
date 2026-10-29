import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  server: {
    host: '127.0.0.1',
    port: 5173,
    strictPort: true,
    // 保留浏览器 Host，使 Admin API 能严格校验同源 Origin；不依赖转发身份头。
    proxy: { '/api/admin/v1': { target: 'http://127.0.0.1:8000', changeOrigin: false } },
  },
});
