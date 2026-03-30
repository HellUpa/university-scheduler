import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [
    vue(),
    tailwindcss(),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  server: {
    host: true, // Нужно для Docker
    port: 5173,
    // proxy: {
    //   // HTTP запросы
    //   '/api': {
    //     target: 'http://backend:8080', // Имя сервиса в docker-compose
    //     changeOrigin: true,
    //     ws: true // Важно для вебсокетов!
    //   }
    // }
  }
})