import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api/v1/workflows': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/api/v1/executor': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/health': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/api/v1/agents': {
        target: 'http://localhost:8081',
        changeOrigin: true,
      },
      '/api/v1/tools': {
        target: 'http://localhost:8081',
        changeOrigin: true,
      },
      '/api/v1/sandbox': {
        target: 'http://localhost:8081',
        changeOrigin: true,
      },
      '/api/v1/notifications': {
        target: 'http://localhost:8081',
        changeOrigin: true,
      },
    },
  },
})