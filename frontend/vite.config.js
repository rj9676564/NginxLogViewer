import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    host: '0.0.0.0', // Allow external access if needed
    proxy: {
      '/ws': {
        target: 'ws://localhost:58080',
        ws: true
      },
      '/api': {
        target: 'http://localhost:58080',
        changeOrigin: true
      }
    }
  }
})
