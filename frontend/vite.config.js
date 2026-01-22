import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import ComponentsPlugin from 'unplugin-vue-components/vite'
import { AntDesignVueResolver } from 'unplugin-vue-components/resolvers'
import AutoImport from 'unplugin-auto-import/vite'

export default defineConfig({
  plugins: [
    vue(),
    AutoImport({
      imports: ['vue'],
      dts: false,
    }),
    ComponentsPlugin({
      resolvers: [
        AntDesignVueResolver({
          importStyle: 'css-in-js', // Better for Vite
        }),
      ],
      dts: false,
    }),
  ],
  server: {
    host: '0.0.0.0',
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
  },
  build: {
    chunkSizeWarningLimit: 1000,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('node_modules')) {
            if (id.includes('ant-design-vue') || id.includes('@ant-design')) {
              return 'vendor-antd';
            }
            return 'vendor';
          }
        }
      }
    }
  }
})
